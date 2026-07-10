package promptssdk

import (
	"context"
	"errors"
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
		if req.Name != "summary" {
			t.Fatalf("unexpected instruction %s", req.Name)
		}
		return InstructionResult{Content: "摘要:" + strings.Join(req.Inputs, ",") + ":" + req.Option, TokenUsage: TokenUsage{Input: 2, Output: 1, Total: 3}}, nil
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
	if result.Calculation.TokenUsage.Total != 3 {
		t.Fatalf("expected token usage from executor, got %+v", result.Calculation.TokenUsage)
	}
	if len(result.Trace) == 0 {
		t.Fatal("expected trace steps")
	}
}

func TestRunUsesContextVariables(t *testing.T) {
	sdk := New()
	result, err := sdk.Run(context.Background(), RunRequest{
		Expression: `template("你好" + $name){"问候: {{content}}"}`,
		Context:    map[string]string{"name": "Ada"},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Content != "问候: 你好\nAda" {
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
	result, err := sdk.Run(context.Background(), RunRequest{
		Expression: `"hello" + $missing`,
		Options: RuntimeOptions{
			ErrorStrategy: ErrorLenient,
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Content != "hello" {
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
