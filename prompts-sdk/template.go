package promptssdk

import (
	"context"
	"fmt"
	"strings"
)

// TemplateEvaluator 求值单个 glue 表达式，返回内容、依赖与 token 消耗。
// CLI 本地模式用 SDK 本地求值；远程模式与 server 端各自提供委托实现
// （远程模式委托 server generate，server 端委托 RegistryService.Generate）。
type TemplateEvaluator interface {
	Evaluate(ctx context.Context, expr string, vars map[string]string) (TemplateEvalResult, error)
}

// TemplateEvalResult 为单个占位符的求值结果，复用 SDK 既有的依赖与 token 类型。
type TemplateEvalResult struct {
	Content      string
	Dependencies []DependencyRef
	TokenUsage   TokenUsage
}

// TemplateResult 为整段模板渲染后的聚合结果。
type TemplateResult struct {
	Content      string
	Dependencies []DependencyRef
	TokenUsage   TokenUsage
}

// RenderTemplate 扫描 input，把每个 {{...}} 占位符内部作为 glue 表达式交给 ev 求值并替换。
// 扫描正确处理占位符内的字符串字面量（其中的 }} 不触发闭合）与指令 option 的花括号深度，
// 因此不使用正则（正则会误截断字面量内的 }} 或 option 的 }）。
// 任一占位符解析或求值失败即返回错误（严格模式）。
func RenderTemplate(ctx context.Context, input string, vars map[string]string, ev TemplateEvaluator) (TemplateResult, error) {
	var out strings.Builder
	result := TemplateResult{}
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
						return TemplateResult{}, fmt.Errorf("template placeholder at %d has unmatched '}'", start)
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
				return TemplateResult{}, fmt.Errorf("template placeholder at %d not closed, expected }}", start)
			}
			exprStr := strings.TrimSpace(expr.String())
			if exprStr == "" {
				return TemplateResult{}, fmt.Errorf("template placeholder at %d is empty", start)
			}
			run, err := ev.Evaluate(ctx, exprStr, vars)
			if err != nil {
				return TemplateResult{}, fmt.Errorf("template placeholder {{ %s }} at %d: %w", exprStr, start, err)
			}
			out.WriteString(run.Content)
			result.Dependencies = append(result.Dependencies, run.Dependencies...)
			result.TokenUsage.Input += run.TokenUsage.Input
			result.TokenUsage.Output += run.TokenUsage.Output
			result.TokenUsage.Total += run.TokenUsage.Total
			continue
		}
		out.WriteByte(input[i])
		i++
	}
	result.Content = out.String()
	return result, nil
}
