package command

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newContentCommand(root *rootOptions) *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "content <package>",
		Short: "输出提示词包正文内容",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			var content string
			if cfg.Local {
				pkg, err := NewRepository(cfg.Repository.Path).ReadPackage(args[0], version)
				if err != nil {
					return err
				}
				content = pkg.Content
			} else {
				client, err := newRemoteClient(cfg)
				if err != nil {
					return err
				}
				if err := client.requireToken(); err != nil {
					return err
				}
				name, ver := splitNameVersion(args[0])
				if ver == "" {
					ver = version
				}
				if ver == "" {
					ver = "latest"
				}
				resp, err := client.content(cmd.Context(), name, ver)
				if err != nil {
					return err
				}
				content = resp.Content
			}
			content = strings.TrimPrefix(content, "\n")
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), content)
			return err
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "指定版本，默认 latest")
	return cmd
}
