package command

import (
	"context"
	"fmt"
	"os"
	"strings"

	promptssdk "prompts-sdk"

	"github.com/spf13/cobra"
)

func newCreateCommand(root *rootOptions) *cobra.Command {
	var meta PackageMeta
	var content string
	var contentFile string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "创建普通提示词或复合提示词包",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			if meta.Name == "" {
				return fmt.Errorf("--name is required")
			}
			if meta.Version == "" {
				meta.Version = "0.1.0"
			}
			if meta.Description == "" {
				meta.Description = meta.Name
			}
			if meta.Status == "" {
				meta.Status = "DRAFT"
			}
			if meta.Category == "" {
				meta.Category = "DEV"
			}
			if meta.Expression != "" {
				meta.Type = "COMPOSITE"
			} else if meta.Type == "" {
				meta.Type = "PLAIN"
			}
			if meta.CreatedAt == "" {
				meta.CreatedAt = nowUTC()
			}
			if contentFile != "" {
				data, err := os.ReadFile(contentFile)
				if err != nil {
					return err
				}
				content = string(data)
			}
			if meta.Type == "COMPOSITE" {
				result, err := promptssdk.New().Parse(promptssdk.ParseRequest{Expression: meta.Expression})
				if err != nil {
					return err
				}
				meta.Dependencies = dependenciesFromSDK(result.Dependencies)
				if content == "" {
					run, err := promptssdk.New().Run(context.Background(), promptssdk.RunRequest{Expression: meta.Expression, InstructionExecutor: llmExecutor(cfg), ContentLoader: NewRepository(cfg.Repository.Path).Loader()})
					if err != nil {
						return err
					}
					content = run.Content
				}
			}
			if content == "" {
				return fmt.Errorf("content is required; use --content, --content-file, or a composite expression that can run")
			}
			repo := NewRepository(cfg.Repository.Path)
			path, err := repo.WritePackage(meta, content)
			if err != nil {
				return err
			}
			pkg, err := repo.ReadPackage(meta.Name, meta.Version)
			if err != nil {
				return err
			}
			return writeOutput(cmd.OutOrStdout(), root.output, fmt.Sprintf("created %s", path), pkg)
		},
	}
	cmd.Flags().StringVar(&meta.Name, "name", "", "提示词包名")
	cmd.Flags().StringVar(&meta.Version, "version", "0.1.0", "提示词版本")
	cmd.Flags().StringVar(&meta.Description, "description", "", "提示词用途说明")
	cmd.Flags().StringVar(&meta.Status, "status", "DRAFT", "版本状态")
	cmd.Flags().StringVar(&meta.Category, "category", "DEV", "提示词分类")
	cmd.Flags().StringVar(&meta.Type, "type", "", "提示词类型: PLAIN 或 COMPOSITE；提供 --expression 时自动为 COMPOSITE")
	cmd.Flags().StringVar(&meta.Expression, "expression", "", "复合提示词表达式")
	cmd.Flags().StringVar(&meta.Author, "author", "", "作者或维护者")
	cmd.Flags().StringVar(&meta.License, "license", "", "授权协议")
	cmd.Flags().StringVar(&content, "content", "", "提示词正文")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "从文件读取提示词正文")
	cmd.Flags().StringSliceVar(&meta.Tags, "tag", nil, "标签，可重复或逗号分隔")
	return cmd
}

func newInfoCommand(root *rootOptions) *cobra.Command {
	var version string
	var deps bool
	cmd := &cobra.Command{
		Use:   "info [package]",
		Short: "查看提示词包、配置、依赖或本地 repository 信息",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			repo := NewRepository(cfg.Repository.Path)
			if len(args) == 0 {
				packages, err := repo.ListPackages()
				if err != nil {
					return err
				}
				return writeOutput(cmd.OutOrStdout(), root.output, formatPackageList(packages), map[string]any{"repository": cfg.Repository.Path, "packages": packages})
			}
			pkg, err := repo.ReadPackage(args[0], version)
			if err != nil {
				return err
			}
			text := fmt.Sprintf("%s@%s\n%s", pkg.Meta.Name, pkg.Meta.Version, pkg.Meta.Description)
			if deps {
				lines := []string{text, "dependencies:"}
				for _, dep := range pkg.Meta.Dependencies {
					lines = append(lines, "- "+dep.Name+formatVersionRule(dep.VersionRule))
				}
				text = strings.Join(lines, "\n")
			}
			return writeOutput(cmd.OutOrStdout(), root.output, text, pkg)
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "指定版本，默认 latest")
	cmd.Flags().BoolVar(&deps, "deps", false, "展示依赖信息")
	return cmd
}

func formatVersionRule(rule string) string {
	if rule == "" {
		return ""
	}
	return "@" + rule
}

// llmExecutor 根据配置构建 SDK 的 LLM 指令执行器；未配置 LLM 时返回 nil，
// SDK 在命中指令时会返回 ErrInstructionFailed（本地模式复合提示词需配置 LLM）。
func llmExecutor(cfg Config) promptssdk.InstructionExecutor {
	if cfg.LLM.Model == "" || cfg.LLM.APIBase == "" {
		return nil
	}
	executor, err := promptssdk.NewLLMExecutor(promptssdk.LLMConfig{
		Model:   cfg.LLM.Model,
		APIBase: cfg.LLM.APIBase,
		APIKey:  cfg.LLM.APIKey,
		Proxy:   cfg.LLM.Proxy,
	})
	if err != nil {
		return nil
	}
	return executor
}
