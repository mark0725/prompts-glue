package command

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const Version = "0.1.0"

type rootOptions struct {
	output         string
	local          bool
	llmModel       string
	llmAPIBase     string
	llmAPIKey      string
	llmProxy       string
	repositoryPath string
}

func NewRootCommand() *cobra.Command {
	opts := &rootOptions{}
	cmd := &cobra.Command{
		Use:           "glue [options] <add|remove|upgrade|create|content|template|cache|info|config|version|publish|login>",
		Short:         "prompts-glue command line client",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVarP(&opts.output, "output", "o", "text", "输出格式: text 或 json")
	cmd.PersistentFlags().BoolVar(&opts.local, "local", false, "本地操作模式，不连接远程 server")
	cmd.PersistentFlags().StringVar(&opts.llmModel, "llm-model", "", "指定提示词制作和计算使用的语言模型")
	cmd.PersistentFlags().StringVar(&opts.llmAPIBase, "llm-api-base", "", "指定语言模型服务的访问入口")
	cmd.PersistentFlags().StringVar(&opts.llmAPIKey, "llm-api-key", "", "指定语言模型服务的 API Key")
	cmd.PersistentFlags().StringVar(&opts.llmProxy, "llm-proxy", "", "指定访问语言模型服务时使用的代理")
	cmd.PersistentFlags().StringVar(&opts.repositoryPath, "repository", "", "覆盖本地 repository 路径")
	cmd.AddCommand(newAddCommand(opts))
	cmd.AddCommand(newRemoveCommand(opts))
	cmd.AddCommand(newUpgradeCommand(opts))
	cmd.AddCommand(newCreateCommand(opts))
	cmd.AddCommand(newContentCommand(opts))
	cmd.AddCommand(newTemplateCommand(opts))
	cmd.AddCommand(newCacheCommand(opts))
	cmd.AddCommand(newInfoCommand(opts))
	cmd.AddCommand(newConfigCommand(opts))
	cmd.AddCommand(newVersionCommand(opts))
	cmd.AddCommand(newPublishCommand(opts))
	cmd.AddCommand(newLoginCommand(opts))
	return cmd
}

func writeOutput(w io.Writer, format string, text string, value any) error {
	switch format {
	case "", "text":
		_, err := fmt.Fprintln(w, text)
		return err
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}
