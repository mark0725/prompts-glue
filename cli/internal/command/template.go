package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	promptssdk "prompts-sdk"

	"github.com/spf13/cobra"
)

// 以下结构仅用于 template 命令的 JSON 输出，保持与 CLI 其它命令一致的小写键风格
// （SDK 的 DependencyRef / TokenUsage 未携带 json tag）。

type templateDependency struct {
	Name        string `json:"name"`
	VersionRule string `json:"versionRule,omitempty"`
}

type templateTokens struct {
	Input  int `json:"input"`
	Output int `json:"output"`
	Total  int `json:"total"`
}

type templateOutput struct {
	Content      string               `json:"content"`
	Dependencies []templateDependency `json:"dependencies,omitempty"`
	Tokens       templateTokens       `json:"tokens"`
}

func newTemplateCommand(root *rootOptions) *cobra.Command {
	var content string
	var contentFile string
	var rawVars []string
	cmd := &cobra.Command{
		Use:   "template",
		Short: "渲染模板，替换 {{...}} 占位符并输出",
		Long: "读取模板内容（优先级 --content-file > --content > stdin），对其中的 {{...}} 占位符按 glue 表达式" +
			"求值并替换，输出渲染后的完整内容。\n\n" +
			"占位符内部为 glue 表达式：提示词引用（如 base-rules）、字面量（\"...\"）、变量（$name）、制作指令" +
			"（如 summary($x){...}）与组合运算符（| + #）。\n" +
			"变量通过 --var key=value 提供；含制作指令或运算符的占位符需配置 LLM（--llm-model / --llm-api-base / --llm-api-key）。\n" +
			"任一占位符解析或求值失败即报错终止。暂不支持转义字面 {{。",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadEffectiveConfig(cmd, root)
			if err != nil {
				return err
			}
			input, err := readTemplateInput(cmd.InOrStdin(), contentFile, content)
			if err != nil {
				return err
			}
			vars, err := parseTemplateVars(rawVars)
			if err != nil {
				return err
			}
			ev, err := newTemplateEvaluator(cfg)
			if err != nil {
				return err
			}
			res, err := promptssdk.RenderTemplate(cmd.Context(), input, vars, ev)
			if err != nil {
				return err
			}
			result := templateOutput{Content: res.Content, Tokens: templateTokens{Input: res.TokenUsage.Input, Output: res.TokenUsage.Output, Total: res.TokenUsage.Total}}
			for _, dep := range res.Dependencies {
				result.Dependencies = append(result.Dependencies, templateDependency{Name: dep.Name, VersionRule: dep.VersionRule})
			}
			return writeTemplateOutput(cmd.OutOrStdout(), root.output, result)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "模板内容（内联文本）")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "从文件读取模板内容")
	cmd.Flags().StringSliceVar(&rawVars, "var", nil, "上下文变量 key=value，可重复或逗号分隔")
	return cmd
}

func readTemplateInput(stdin io.Reader, contentFile, content string) (string, error) {
	if contentFile != "" {
		data, err := os.ReadFile(contentFile)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if content != "" {
		return content, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func parseTemplateVars(raw []string) (map[string]string, error) {
	vars := make(map[string]string, len(raw))
	for _, item := range raw {
		idx := strings.Index(item, "=")
		if idx <= 0 {
			return nil, fmt.Errorf("invalid --var %q, expected key=value", item)
		}
		vars[item[:idx]] = item[idx+1:]
	}
	return vars, nil
}

func writeTemplateOutput(w io.Writer, format string, result templateOutput) error {
	switch format {
	case "", "text":
		_, err := fmt.Fprint(w, result.Content)
		return err
	case "json":
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

// localEvaluator 用 SDK 在本地求值表达式（本地 repository 加载与本地 LLM 执行）。
type localEvaluator struct {
	loader   promptssdk.ContentLoader
	executor promptssdk.InstructionExecutor
}

func (e localEvaluator) Evaluate(ctx context.Context, expr string, vars map[string]string) (promptssdk.TemplateEvalResult, error) {
	sdk := promptssdk.New()
	run, err := sdk.Run(ctx, promptssdk.RunRequest{Expression: expr, Context: vars, ContentLoader: e.loader, InstructionExecutor: e.executor})
	if err != nil {
		return promptssdk.TemplateEvalResult{}, err
	}
	return promptssdk.TemplateEvalResult{Content: run.Content, Dependencies: run.Dependencies, TokenUsage: run.Calculation.TokenUsage}, nil
}

// remoteEvaluator 委托 server 端 generate 求值表达式（远程模式不需本地 LLM 与本地 repository）。
type remoteEvaluator struct {
	client *RemoteClient
}

func (e remoteEvaluator) Evaluate(ctx context.Context, expr string, vars map[string]string) (promptssdk.TemplateEvalResult, error) {
	resp, err := e.client.generate(ctx, expr, vars)
	if err != nil {
		return promptssdk.TemplateEvalResult{}, err
	}
	deps := make([]promptssdk.DependencyRef, 0, len(resp.Dependencies))
	for _, dep := range resp.Dependencies {
		deps = append(deps, promptssdk.DependencyRef{Name: dep.Name, VersionRule: dep.VersionRule})
	}
	return promptssdk.TemplateEvalResult{Content: resp.Content, Dependencies: deps, TokenUsage: resp.Calculation.TokenUsage}, nil
}

// newTemplateEvaluator 按本地/远程模式构造模板求值器。远程模式需已配置访问令牌（glue login 或 server.token）。
func newTemplateEvaluator(cfg Config) (promptssdk.TemplateEvaluator, error) {
	if cfg.Local {
		return localEvaluator{loader: NewRepository(cfg.Repository.Path).Loader(), executor: llmExecutor(cfg)}, nil
	}
	client, err := newRemoteClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.requireToken(); err != nil {
		return nil, err
	}
	return remoteEvaluator{client: client}, nil
}
