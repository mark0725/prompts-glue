package promptssdk

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// echoEvaluator 把表达式原样（包在 [] 中）作为内容返回，仅用于验证扫描器的拆分/替换行为，
// 不做真实求值（真实求值路径由 CLI 本地模式的集成测试覆盖）。
type echoEvaluator struct {
	err error
}

func (e echoEvaluator) Evaluate(ctx context.Context, expr string, vars map[string]string) (TemplateEvalResult, error) {
	if e.err != nil {
		return TemplateEvalResult{}, e.err
	}
	return TemplateEvalResult{
		Content:      "[" + expr + "]",
		Dependencies: []DependencyRef{{Name: expr}},
		TokenUsage:   TokenUsage{Total: 1},
	}, nil
}

func TestRenderTemplatePromptRef(t *testing.T) {
	res, err := RenderTemplate(context.Background(), "前缀 {{ base-rules }} 后缀", nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := "前缀 [base-rules] 后缀"; res.Content != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
	if len(res.Dependencies) != 1 || res.Dependencies[0].Name != "base-rules" {
		t.Fatalf("unexpected dependencies: %+v", res.Dependencies)
	}
}

func TestRenderTemplateMultiple(t *testing.T) {
	res, err := RenderTemplate(context.Background(), `A {{ "X" }} B {{ "Y" }} C`, nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `A ["X"] B ["Y"] C`; res.Content != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
	// 多占位符的依赖与 token 应累加。
	if len(res.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(res.Dependencies))
	}
	if res.TokenUsage.Total != 2 {
		t.Fatalf("expected aggregated total token 2, got %d", res.TokenUsage.Total)
	}
}

// 字符串字面量中的 }} 不应触发占位符闭合。
func TestRenderTemplateStringLiteralContainsClosingBraces(t *testing.T) {
	res, err := RenderTemplate(context.Background(), `{{ "a}}b" }}`, nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `["a}}b"]`; res.Content != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
}

// 指令 option 的花括号深度不应让占位符提前闭合。
func TestRenderTemplatePlaceholderWithOptionBraces(t *testing.T) {
	res, err := RenderTemplate(context.Background(), `{{ merge(base-rules){按规则合并} }}`, nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `[merge(base-rules){按规则合并}]`; res.Content != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
}

// 多层配对花括号应正确平衡。
func TestRenderTemplateNestedBalancedBraces(t *testing.T) {
	res, err := RenderTemplate(context.Background(), `{{ a{b}{c} }}`, nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := `[a{b}{c}]`; res.Content != want {
		t.Fatalf("content = %q, want %q", res.Content, want)
	}
}

func TestRenderTemplateNoPlaceholder(t *testing.T) {
	res, err := RenderTemplate(context.Background(), "纯文本无占位符", nil, echoEvaluator{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "纯文本无占位符" {
		t.Fatalf("content = %q", res.Content)
	}
	if len(res.Dependencies) != 0 || res.TokenUsage.Total != 0 {
		t.Fatalf("expected empty aggregation, got %+v / %+v", res.Dependencies, res.TokenUsage)
	}
}

// vars 应透传给求值器。
func TestRenderTemplateVarsPassthrough(t *testing.T) {
	ev := &varCapturingEvaluator{}
	_, err := RenderTemplate(context.Background(), `{{ $who }}`, map[string]string{"who": "glue"}, ev)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.got["who"] != "glue" {
		t.Fatalf("vars not passed through: %+v", ev.got)
	}
}

type varCapturingEvaluator struct {
	got map[string]string
}

func (e *varCapturingEvaluator) Evaluate(ctx context.Context, expr string, vars map[string]string) (TemplateEvalResult, error) {
	e.got = vars
	return TemplateEvalResult{Content: expr}, nil
}

func TestRenderTemplateUnclosedPlaceholder(t *testing.T) {
	_, err := RenderTemplate(context.Background(), `{{ base-rules`, nil, echoEvaluator{})
	if err == nil || !strings.Contains(err.Error(), "not closed") {
		t.Fatalf("expected not-closed error, got %v", err)
	}
}

func TestRenderTemplateEmptyPlaceholder(t *testing.T) {
	_, err := RenderTemplate(context.Background(), `{{    }}`, nil, echoEvaluator{})
	if err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Fatalf("expected empty error, got %v", err)
	}
}

// 占位符内出现无法配对的 } 应报错。
func TestRenderTemplateUnmatchedClosingBrace(t *testing.T) {
	_, err := RenderTemplate(context.Background(), `{{ a}b }}`, nil, echoEvaluator{})
	if err == nil || !strings.Contains(err.Error(), "unmatched") {
		t.Fatalf("expected unmatched error, got %v", err)
	}
}

// 严格模式：求值器返回错误时应终止并包装错误。
func TestRenderTemplateStrictEvaluatorError(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := RenderTemplate(context.Background(), `{{ "x" }}`, nil, echoEvaluator{err: sentinel})
	if err == nil || !strings.Contains(err.Error(), "template placeholder") || !errors.Is(err, sentinel) {
		t.Fatalf("expected wrapped evaluator error, got %v", err)
	}
}
