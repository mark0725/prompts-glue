package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type versionOutput struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Protocol   string `json:"protocol"`
	Repository string `json:"repository"`
}

func newVersionCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "查看 CLI 版本和运行环境信息",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			out := versionOutput{Name: "glue", Version: Version, Protocol: "local", Repository: cfg.Repository.Path}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("glue %s", Version), out)
		},
	}
}

func newPublishCommand(root *rootOptions) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "publish <package>",
		Short: "发布提示词包到远程 server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			if cfg.Local {
				return fmt.Errorf("publish is unavailable in local mode")
			}
			client, err := newRemoteClient(cfg)
			if err != nil {
				return err
			}
			if err := client.requireToken(); err != nil {
				return err
			}
			name, version := splitNameVersion(args[0])
			pkg, err := NewRepository(cfg.Repository.Path).ReadPackage(name, version)
			if err != nil {
				return err
			}
			resp, err := client.publish(cmd.Context(), publishRequest{Package: cliToServerMeta(pkg.Meta), Content: pkg.Content, DryRun: dryRun})
			if err != nil {
				return err
			}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("published %s@%s", resp.Package.Name, resp.Package.Version), map[string]any{"package": resp.Package, "dependencies": resp.Dependencies, "warnings": resp.Warnings})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "仅校验与预览，不实际发布")
	return cmd
}

func newLoginCommand(root *rootOptions) *cobra.Command {
	var username string
	var password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "登录远程 server，保存访问令牌",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			if cfg.Local {
				return writeOutput(cmd.OutOrStdout(), root.output, "local mode does not require login", map[string]bool{"local": true})
			}
			user := username
			if user == "" {
				user = os.Getenv("GLUE_USERNAME")
			}
			pass := password
			if pass == "" {
				pass = os.Getenv("GLUE_PASSWORD")
			}
			if user == "" || pass == "" {
				var err error
				user, pass, err = promptCredentials(user)
				if err != nil {
					return err
				}
			}
			client, err := newRemoteClient(cfg)
			if err != nil {
				return err
			}
			token, err := client.login(cmd.Context(), user, pass)
			if err != nil {
				return err
			}
			cfg.Server.Token = token
			if err := saveConfig(cfg); err != nil {
				return fmt.Errorf("save token: %w", err)
			}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("logged in as %s", user), map[string]string{"username": user})
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "登录用户名（未提供则交互输入或读 GLUE_USERNAME）")
	cmd.Flags().StringVar(&password, "password", "", "登录密码（未提供则交互输入或读 GLUE_PASSWORD）")
	return cmd
}

// promptCredentials 交互式读取用户名与密码（密码无回显）。prefillUser 为已通过 flag/env 提供的用户名，可为空。
func promptCredentials(prefillUser string) (string, string, error) {
	username := prefillUser
	if username == "" {
		fmt.Fprint(os.Stderr, "username: ")
		var u string
		if _, err := fmt.Fscanln(os.Stdin, &u); err != nil {
			return "", "", err
		}
		username = strings.TrimSpace(u)
	}
	fmt.Fprint(os.Stderr, "password: ")
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		bytes, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", "", err
		}
		return username, string(bytes), nil
	}
	// 非终端环境（如管道）回退为明文读取。
	var p string
	if _, err := fmt.Fscanln(os.Stdin, &p); err != nil {
		return "", "", err
	}
	return username, p, nil
}

// splitNameVersion 把 "name@version" 拆为 name 与 version（version 可空）。
func splitNameVersion(s string) (string, string) {
	if idx := strings.IndexByte(s, '@'); idx > 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}
