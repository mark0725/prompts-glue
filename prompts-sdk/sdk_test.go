package promptssdk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestParseExtractsDependenciesAndAST(t *testing.T) {
	sdk := New()
	result, err := sdk.Parse(ParseRequest{Expression: `base-rules@^1.2.3 + summary(@team/backend@~2.0.0){"保留关键约束"}`})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if result.AST == nil {
		t.Fatal("expected AST")
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(result.Dependencies))
	}
	if result.Dependencies[0].Name != "base-rules" || result.Dependencies[0].VersionRule != "^1.2.3" {
		t.Fatalf("unexpected first dependency: %+v", result.Dependencies[0])
	}
	if result.Dependencies[1].Name != "@team/backend" || result.Dependencies[1].VersionRule != "~2.0.0" {
		t.Fatalf("unexpected second dependency: %+v", result.Dependencies[1])
	}
}

func TestRunLoadsContentAndExecutesInstruction(t *testing.T) {
	sdk := New()
	loader := ContentLoaderFunc(func(ctx context.Context, ref DependencyRef) (Content, error) {
		switch ref.Name {
		case "base-rules":
			return Content{Name: ref.Name, Version: "1.0.0", Text: "基础规则"}, nil
		case "team-rules":
			return Content{Name: ref.Name, Version: "1.0.0", Text: "团队规则"}, nil
		default:
			return Content{}, errors.New("missing prompt")
		}
	})
	executor := InstructionExecutorFunc(func(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
		switch req.Name {
		case "summary":
			return InstructionResult{Content: "摘要:" + strings.Join(req.Inputs, ",") + ":" + req.Option, TokenUsage: TokenUsage{Input: 2, Output: 1, Total: 3}}, nil
		case "merge":
			// 运算符 | 现在路由到 merge 指令，按顺序合并输入。
			return InstructionResult{Content: strings.Join(req.Inputs, "\n")}, nil
		default:
			t.Fatalf("unexpected instruction %s", req.Name)
			return InstructionResult{}, nil
		}
	})
	result, err := sdk.Run(context.Background(), RunRequest{
		Expression:          `base-rules | summary(team-rules){"短句"}`,
		ContentLoader:       loader,
		InstructionExecutor: executor,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	want := "基础规则\n摘要:团队规则:短句"
	if result.Content != want {
		t.Fatalf("unexpected content\nwant: %q\n got: %q", want, result.Content)
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("expected 2 runtime dependencies, got %d", len(result.Dependencies))
	}
	// summary 执行器贡献 3 token；运算符 | 同样走执行器路径。
	if result.Calculation.TokenUsage.Total < 3 {
		t.Fatalf("expected token usage to include executor contribution, got %+v", result.Calculation.TokenUsage)
	}
	if len(result.Trace) == 0 {
		t.Fatal("expected trace steps")
	}
}

func TestRunUsesContextVariables(t *testing.T) {
	sdk := New()
	executor := InstructionExecutorFunc(func(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
		return InstructionResult{Content: strings.Join(req.Inputs, "\n")}, nil
	})
	result, err := sdk.Run(context.Background(), RunRequest{
		Expression:          `"你好" + $name`,
		Context:             map[string]string{"name": "Ada"},
		InstructionExecutor: executor,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Content != "你好\nAda" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestParseRejectsUnknownInstruction(t *testing.T) {
	sdk := New()
	_, err := sdk.Parse(ParseRequest{Expression: `unknown(base-rules)`})
	if err == nil {
		t.Fatal("expected error")
	}
	sdkErr, ok := err.(*SDKError)
	if !ok {
		t.Fatalf("expected SDKError, got %T", err)
	}
	if sdkErr.Code != ErrSemanticInvalid {
		t.Fatalf("expected semantic error, got %s", sdkErr.Code)
	}
}

func TestRunMissingContextCanBeLenient(t *testing.T) {
	sdk := New()
	executor := InstructionExecutorFunc(func(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
		return InstructionResult{Content: strings.Join(req.Inputs, "\n")}, nil
	})
	result, err := sdk.Run(context.Background(), RunRequest{
		Expression: `"hello" + $missing`,
		Options: RuntimeOptions{
			ErrorStrategy: ErrorLenient,
		},
		InstructionExecutor: executor,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected warning, got %+v", result.Warnings)
	}
}

func TestRunMissingDependencyReturnsSDKError(t *testing.T) {
	sdk := New()
	_, err := sdk.Run(context.Background(), RunRequest{
		Expression: `missing-prompt`,
		ContentLoader: ContentLoaderFunc(func(ctx context.Context, ref DependencyRef) (Content, error) {
			return Content{}, errors.New("not found")
		}),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	sdkErr, ok := err.(*SDKError)
	if !ok {
		t.Fatalf("expected SDKError, got %T", err)
	}
	if sdkErr.Code != ErrDependencyLoadFailed {
		t.Fatalf("expected dependency load error, got %s", sdkErr.Code)
	}
}

func TestParseAcceptsNewInstructions(t *testing.T) {
	sdk := New()
	for _, expr := range []string{
		`format("文本")`,
		`translate("文本"){"英文"}`,
		`redact("文本")`,
		`refine("文本")`,
	} {
		if _, err := sdk.Parse(ParseRequest{Expression: expr}); err != nil {
			t.Fatalf("expected %q to parse, got error: %v", expr, err)
		}
	}
}

func TestRunInstructionWithoutExecutorReturnsError(t *testing.T) {
	sdk := New()
	_, err := sdk.Run(context.Background(), RunRequest{Expression: `summary("text")`})
	if err == nil {
		t.Fatal("expected error when no executor is configured")
	}
	sdkErr, ok := err.(*SDKError)
	if !ok {
		t.Fatalf("expected SDKError, got %T", err)
	}
	if sdkErr.Code != ErrInstructionFailed {
		t.Fatalf("expected ErrInstructionFailed, got %s", sdkErr.Code)
	}
}

// 指令选项 {} 可不带引号；带引号时向后兼容；引号内的 } 不提前结束；支持内层花括号。
func TestParseInstructionOptionForms(t *testing.T) {
	sdk := New()
	cases := []struct {
		expr string
		want string
	}{
		{`summary(x){输出为英文}`, "输出为英文"},
		{`summary(x){"输出为英文"}`, "输出为英文"},
		{`summary(x){  输出为英文  }`, "输出为英文"},
		{`template(x){"问候: {{content}}"}`, "问候: {{content}}"},
		{`summary(x){按 {a,b} 分组}`, "按 {a,b} 分组"},
		{`summary(x){}`, ""},
	}
	for _, tc := range cases {
		result, err := sdk.Parse(ParseRequest{Expression: tc.expr})
		if err != nil {
			t.Fatalf("Parse(%q) returned error: %v", tc.expr, err)
		}
		call, ok := result.AST.(*CallNode)
		if !ok {
			t.Fatalf("Parse(%q) expected CallNode, got %T", tc.expr, result.AST)
		}
		if call.Option != tc.want {
			t.Fatalf("Parse(%q) option\nwant: %q\n got: %q", tc.expr, tc.want, call.Option)
		}
	}
}

// 无执行器时，运算符（路由到 synthesis/merge/tags）同样应报错。
func TestRunOperatorWithoutExecutorReturnsError(t *testing.T) {
	sdk := New()
	_, err := sdk.Run(context.Background(), RunRequest{Expression: `"a" + "b"`})
	if err == nil {
		t.Fatal("expected error when no executor is configured")
	}
	sdkErr, ok := err.(*SDKError)
	if !ok {
		t.Fatalf("expected SDKError, got %T", err)
	}
	if sdkErr.Code != ErrInstructionFailed {
		t.Fatalf("expected ErrInstructionFailed, got %s", sdkErr.Code)
	}
}

// 运算符 + / | / # 应与对应指令一致：走 InstructionExecutor，并累计 token。
func TestRunOperatorsRouteThroughExecutor(t *testing.T) {
	sdk := New()
	loader := ContentLoaderFunc(func(ctx context.Context, ref DependencyRef) (Content, error) {
		return Content{Name: ref.Name, Version: "1.0.0", Text: ref.Name}, nil
	})
	seen := map[string]bool{}
	executor := InstructionExecutorFunc(func(ctx context.Context, req InstructionRequest) (InstructionResult, error) {
		seen[req.Name] = true
		return InstructionResult{Content: strings.Join(req.Inputs, "+"), TokenUsage: TokenUsage{Total: 1}}, nil
	})
	for _, op := range []string{"+", "|", "#"} {
		if _, err := sdk.Run(context.Background(), RunRequest{
			Expression:          fmt.Sprintf("a %s b", op),
			ContentLoader:       loader,
			InstructionExecutor: executor,
		}); err != nil {
			t.Fatalf("Run with operator %q returned error: %v", op, err)
		}
	}
	for _, name := range []string{"synthesis", "merge", "tags"} {
		if !seen[name] {
			t.Fatalf("operator did not route to instruction %q (seen: %v)", name, seen)
		}
	}
}
