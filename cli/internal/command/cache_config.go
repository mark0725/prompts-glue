package command

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newCacheCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache <info|clean>",
		Short: "查看或清理提示词计算缓存",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			cacheDir := filepath.Join(cfg.Home, "cache")
			switch args[0] {
			case "info":
				entries, err := os.ReadDir(cacheDir)
				if os.IsNotExist(err) {
					entries = nil
				} else if err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("cache entries: %d", len(entries)), map[string]any{"path": cacheDir, "entries": len(entries), "enabled": cfg.Cache.Enabled, "strategy": cfg.Cache.Strategy})
			case "clean":
				if err := os.RemoveAll(cacheDir); err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, "cache cleaned", map[string]string{"cleaned": cacheDir})
			default:
				return fmt.Errorf("unknown cache action %q", args[0])
			}
		},
	}
	return cmd
}

func newConfigCommand(root *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <get|set|path>",
		Short: "查看和修改 CLI 配置",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			switch args[0] {
			case "get":
				if len(args) != 1 {
					return fmt.Errorf("config get does not accept extra arguments")
				}
				return writeOutput(cmd.OutOrStdout(), root.output, formatConfigText(cfg), cfg)
			case "set":
				if len(args) != 3 {
					return fmt.Errorf("usage: glue config set <key> <value>")
				}
				if err := setConfigValue(&cfg, args[1], args[2]); err != nil {
					return err
				}
				if err := saveConfig(cfg); err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, "config updated", cfg)
			case "path":
				return writeOutput(cmd.OutOrStdout(), root.output, filepath.Join(cfg.Home, "config.yaml"), map[string]string{"path": filepath.Join(cfg.Home, "config.yaml")})
			default:
				return fmt.Errorf("unknown config action %q", args[0])
			}
		},
	}
	return cmd
}

// mask 返回密钥的掩码形式：未设置显示空，已设置显示 "****"。
func mask(v string) string {
	if v == "" {
		return ""
	}
	return "****"
}

func formatConfigText(cfg Config) string {
	return fmt.Sprintf("home: %s\nrepository: %s\nlocal: %t\nserver.base: %s\nserver.token: %s\nllm.model: %s\nllm.api_base: %s\nllm.api_key: %s\ncache: %t %s", cfg.Home, cfg.Repository.Path, cfg.Local, cfg.Server.Base, mask(cfg.Server.Token), cfg.LLM.Model, cfg.LLM.APIBase, mask(cfg.LLM.APIKey), cfg.Cache.Enabled, cfg.Cache.Strategy)
}
