package command

import (
	"fmt"

	"github.com/spf13/cobra"
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
	return &cobra.Command{
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
			return fmt.Errorf("publish requires Registry API implementation")
		},
	}
}

func newLoginCommand(root *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "登录远程 server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			if cfg.Local {
				return writeOutput(cmd.OutOrStdout(), root.output, "local mode does not require login", map[string]bool{"local": true})
			}
			return fmt.Errorf("login requires Registry API implementation")
		},
	}
}
