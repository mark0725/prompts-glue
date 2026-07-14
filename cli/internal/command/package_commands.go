package command

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// remoteFetchPackage 从 server 拉取指定版本规则的包内容并写入本地 repository，
// 递归拉取其依赖。visited 按 name@version 去重，depth 上限防止异常环路。
// 返回实际解析到的版本号。
func remoteFetchPackage(ctx context.Context, client *RemoteClient, repo Repository, name, versionRule string, visited map[string]bool, depth int) (string, error) {
	if depth > 32 {
		return "", fmt.Errorf("dependency depth exceeded for %s", name)
	}
	resp, err := client.content(ctx, name, versionRule)
	if err != nil {
		return "", err
	}
	resolved := resp.Package.Version
	key := name + "@" + resolved
	if !visited[key] {
		if _, err := repo.WritePackage(serverToCLIMeta(resp.Package), resp.Content); err != nil {
			return "", err
		}
		visited[key] = true
		for _, dep := range resp.Package.Dependencies {
			depVersion := dep.ResolvedVersion
			if depVersion == "" {
				depVersion = dep.VersionRule
			}
			if depVersion == "" || dep.Optional {
				continue
			}
			if _, err := remoteFetchPackage(ctx, client, repo, dep.Name, depVersion, visited, depth+1); err != nil {
				return "", fmt.Errorf("dependency %s: %w", dep.Name, err)
			}
		}
	}
	return resolved, nil
}

func newAddCommand(root *rootOptions) *cobra.Command {
	var fromFile string
	cmd := &cobra.Command{
		Use:   "add <package>",
		Short: "安装或添加提示词包到本地 repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			repo := NewRepository(cfg.Repository.Path)
			if fromFile != "" {
				pkg, err := repo.ImportFile(fromFile)
				if err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("added %s@%s", pkg.Meta.Name, pkg.Meta.Version), pkg)
			}
			if len(args) == 0 {
				return fmt.Errorf("package argument or --file is required")
			}
			if cfg.Local {
				pkg, err := repo.ReadPackage(args[0], "")
				if err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("already present %s@%s", pkg.Meta.Name, pkg.Meta.Version), pkg)
			}
			// 远程：从 server 拉取包与依赖并写入本地 repository。
			client, err := newRemoteClient(cfg)
			if err != nil {
				return err
			}
			if err := client.requireToken(); err != nil {
				return err
			}
			name, versionRule := splitNameVersion(args[0])
			if versionRule == "" {
				versionRule = "latest"
			}
			resolved, err := remoteFetchPackage(cmd.Context(), client, repo, name, versionRule, map[string]bool{}, 0)
			if err != nil {
				return err
			}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("added %s@%s", name, resolved), map[string]string{"name": name, "version": resolved})
		},
	}
	cmd.Flags().StringVar(&fromFile, "file", "", "从本地提示词包 Markdown 文件导入")
	return cmd
}

func newRemoveCommand(root *rootOptions) *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "remove <package>",
		Short: "从本地 repository 移除提示词包",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			repo := NewRepository(cfg.Repository.Path)
			if err := repo.RemovePackage(args[0], version); err != nil {
				return err
			}
			label := args[0]
			if version != "" {
				label += "@" + version
			}
			return writeOutput(cmd.OutOrStdout(), root.output, "removed "+label, map[string]string{"removed": label})
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "只移除指定版本；为空时移除整个包目录")
	return cmd
}

func newUpgradeCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade [package]",
		Short: "检查本地提示词包升级状态",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			repo := NewRepository(cfg.Repository.Path)
			packages, err := repo.ListPackages()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				filtered := packages[:0]
				for _, pkg := range packages {
					if pkg.Meta.Name == args[0] {
						filtered = append(filtered, pkg)
					}
				}
				packages = filtered
			}
			if cfg.Local {
				return writeOutput(cmd.OutOrStdout(), root.output, "local packages are up to date", packages)
			}
			// 远程：逐个比对 server 最新版本并拉取新版本。
			client, err := newRemoteClient(cfg)
			if err != nil {
				return err
			}
			if err := client.requireToken(); err != nil {
				return err
			}
			type upgraded struct {
				Name    string `json:"name"`
				From    string `json:"from"`
				To      string `json:"to"`
				Skipped bool   `json:"skipped,omitempty"`
			}
			results := make([]upgraded, 0, len(packages))
			visited := map[string]bool{}
			for _, pkg := range packages {
				detail, err := client.packageDetail(cmd.Context(), pkg.Meta.Name)
				if err != nil {
					return fmt.Errorf("upgrade %s: %w", pkg.Meta.Name, err)
				}
				if detail.LatestVersion == "" || detail.LatestVersion == pkg.Meta.Version {
					results = append(results, upgraded{Name: pkg.Meta.Name, From: pkg.Meta.Version, To: pkg.Meta.Version, Skipped: true})
					continue
				}
				if _, err := remoteFetchPackage(cmd.Context(), client, repo, pkg.Meta.Name, "latest", visited, 0); err != nil {
					return fmt.Errorf("upgrade %s: %w", pkg.Meta.Name, err)
				}
				results = append(results, upgraded{Name: pkg.Meta.Name, From: pkg.Meta.Version, To: detail.LatestVersion})
			}
			return writeOutput(cmd.OutOrStdout(), root.output, "upgrade completed", results)
		},
	}
	return cmd
}

func formatPackageList(packages []StoredPackage) string {
	if len(packages) == 0 {
		return "no packages"
	}
	var b strings.Builder
	for _, pkg := range packages {
		fmt.Fprintf(&b, "%s@%s %s\n", pkg.Meta.Name, pkg.Meta.Version, pkg.Meta.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}
