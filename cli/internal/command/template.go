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
			result, err := renderTemplate(cmd.Context(), input, vars, ev)
			if err != nil {
				return err
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

// expressionEvaluator 求值单个 glue 表达式，返回内容、依赖与 token 消耗。
// 本地模式用 SDK（本地 repository 加载与本地 LLM 执行）；远程模式委托 server 端 generate。
type expressionEvaluator interface {
	evaluate(ctx context.Context, expr string, vars map[string]string) (evalResult, error)
}

type evalResult struct {
	Content      string
	Dependencies []templateDependency
	Tokens       templateTokens
}

// localEvaluator 用 SDK 在本地求值表达式。
type localEvaluator struct {
	loader   promptssdk.ContentLoader
	executor promptssdk.InstructionExecutor
}

func (e localEvaluator) evaluate(ctx context.Context, expr string, vars map[string]string) (evalResult, error) {
	sdk := promptssdk.New()
	run, err := sdk.Run(ctx, promptssdk.RunRequest{Expression: expr, Context: vars, ContentLoader: e.loader, InstructionExecutor: e.executor})
	if err != nil {
		return evalResult{}, err
	}
	out := evalResult{Content: run.Content, Tokens: templateTokens{Input: run.Calculation.TokenUsage.Input, Output: run.Calculation.TokenUsage.Output, Total: run.Calculation.TokenUsage.Total}}
	for _, dep := range run.Dependencies {
		out.Dependencies = append(out.Dependencies, templateDependency{Name: dep.Name, VersionRule: dep.VersionRule})
	}
	return out, nil
}

// remoteEvaluator 委托 server 端 generate 求值表达式（远程模式不需本地 LLM 与本地 repository）。
type remoteEvaluator struct {
	client *RemoteClient
}

func (e remoteEvaluator) evaluate(ctx context.Context, expr string, vars map[string]string) (evalResult, error) {
	resp, err := e.client.generate(ctx, expr, vars)
	if err != nil {
		return evalResult{}, err
	}
	out := evalResult{Content: resp.Content, Tokens: templateTokens{Input: resp.Calculation.TokenUsage.Input, Output: resp.Calculation.TokenUsage.Output, Total: resp.Calculation.TokenUsage.Total}}
	for _, dep := range resp.Dependencies {
		out.Dependencies = append(out.Dependencies, templateDependency{Name: dep.Name, VersionRule: dep.VersionRule})
	}
	return out, nil
}

// newTemplateEvaluator 按本地/远程模式构造模板求值器。远程模式需已配置访问令牌（glue login 或 server.token）。
func newTemplateEvaluator(cfg Config) (expressionEvaluator, error) {
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

// renderTemplate 扫描 input，把每个 {{...}} 占位符内部作为 glue 表达式交给求值器求值并替换。
// 扫描正确处理占位符内的字符串字面量（其中的 }} 不触发闭合）与指令 option 的花括号深度，
// 因此不使用正则（正则会误截断字面量内的 }} 或 option 的 }）。
// 任一占位符解析或求值失败即返回错误（严格模式）。
func renderTemplate(ctx context.Context, input string, vars map[string]string, ev expressionEvaluator) (templateOutput, error) {
	var out strings.Builder
	result := templateOutput{}
	i := 0
	for i < len(input) {
		if i+1 < len(input) && input[i] == '{' && input[i+1] == '{' {
			start := i
			i += 2
			var expr strings.Builder
			braceDepth := 0
			inString := false
			closed := false
			for i < len(input) {
				ch := input[i]
				if inString {
					// 字符串字面量内保留原文，处理 \" 转义使其中的 " 不结束字符串。
					if ch == '\\' && i+1 < len(input) {
						expr.WriteByte(ch)
						expr.WriteByte(input[i+1])
						i += 2
						continue
					}
					expr.WriteByte(ch)
					if ch == '"' {
						inString = false
					}
					i++
					continue
				}
				switch ch {
				case '"':
					inString = true
					expr.WriteByte(ch)
					i++
				case '{':
					braceDepth++
					expr.WriteByte(ch)
					i++
				case '}':
					switch {
					case braceDepth > 0:
						braceDepth--
						expr.WriteByte(ch)
						i++
					case i+1 < len(input) && input[i+1] == '}':
						i += 2
						closed = true
					default:
						return templateOutput{}, fmt.Errorf("template placeholder at %d has unmatched '}'", start)
					}
				default:
					expr.WriteByte(ch)
					i++
				}
				if closed {
					break
				}
			}
			if !closed {
				return templateOutput{}, fmt.Errorf("template placeholder at %d not closed, expected }}", start)
			}
			exprStr := strings.TrimSpace(expr.String())
			if exprStr == "" {
				return templateOutput{}, fmt.Errorf("template placeholder at %d is empty", start)
			}
			run, err := ev.evaluate(ctx, exprStr, vars)
			if err != nil {
				return templateOutput{}, fmt.Errorf("template placeholder {{ %s }} at %d: %w", exprStr, start, err)
			}
			out.WriteString(run.Content)
			result.Dependencies = append(result.Dependencies, run.Dependencies...)
			result.Tokens.Input += run.Tokens.Input
			result.Tokens.Output += run.Tokens.Output
			result.Tokens.Total += run.Tokens.Total
			continue
		}
		out.WriteByte(input[i])
		i++
	}
	result.Content = out.String()
	return result, nil
}
