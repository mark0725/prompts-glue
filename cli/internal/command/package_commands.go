package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

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
			if !cfg.Local {
				return fmt.Errorf("remote add requires Registry API implementation; use --local --file to add a local package")
			}
			pkg, err := repo.ReadPackage(args[0], "")
			if err != nil {
				return err
			}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("already present %s@%s", pkg.Meta.Name, pkg.Meta.Version), pkg)
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
			if !cfg.Local {
				return fmt.Errorf("remote upgrade requires Registry API implementation; local repository has no remote source to compare")
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
			return writeOutput(cmd.OutOrStdout(), root.output, "local packages are up to date", packages)
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
