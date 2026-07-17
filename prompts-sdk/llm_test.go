package promptssdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewLLMExecutorValidation(t *testing.T) {
	if _, err := NewLLMExecutor(LLMConfig{APIBase: "https://x"}); err == nil {
		t.Fatal("expected error when model is empty")
	}
	if _, err := NewLLMExecutor(LLMConfig{Model: "m"}); err == nil {
		t.Fatal("expected error when api_base is empty")
	}
	if _, err := NewLLMExecutor(LLMConfig{Model: "m", APIBase: "https://x", Proxy: "://bad"}); err == nil {
		t.Fatal("expected error for invalid proxy URL")
	}
}

func TestLLMExecutorHappyPath(t *testing.T) {
	var gotBody struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "  摘要结果  "}},
			},
			"usage": map[string]int{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18},
		})
	}))
	defer server.Close()

	executor, err := NewLLMExecutor(LLMConfig{Model: "gpt-x", APIBase: server.URL, APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("NewLLMExecutor: %v", err)
	}

	res, err := executor.ExecuteInstruction(context.Background(), InstructionRequest{
		Name: "summary", Inputs: []string{"第一段", "第二段"}, Option: "不超过两句",
	})
	if err != nil {
		t.Fatalf("ExecuteInstruction: %v", err)
	}
	if res.Content != "摘要结果" {
		t.Fatalf("expected trimmed content, got %q", res.Content)
	}
	if res.TokenUsage != (TokenUsage{Input: 11, Output: 7, Total: 18}) {
		t.Fatalf("unexpected token usage %+v", res.TokenUsage)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if gotBody.Model != "gpt-x" || gotBody.Stream {
		t.Fatalf("unexpected request model/stream: %+v", gotBody)
	}
	if len(gotBody.Messages) != 2 || gotBody.Messages[0].Role != "system" || gotBody.Messages[1].Role != "user" {
		t.Fatalf("unexpected messages: %+v", gotBody.Messages)
	}
	if gotBody.Messages[0].Content != globalSystemPromptPrefix+defaultSystemPrompts["summary"] {
		t.Fatalf("system prompt not from bundled defaults (with global prefix)")
	}
	wantUser := "额外要求：不超过两句\n\n第一段\n第二段"
	if gotBody.Messages[1].Content != wantUser {
		t.Fatalf("user message\nwant: %q\n got: %q", wantUser, gotBody.Messages[1].Content)
	}
}

func TestLLMExecutorOmitsAuthWhenNoKey(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	defer server.Close()
	executor, err := NewLLMExecutor(LLMConfig{Model: "local", APIBase: server.URL})
	if err != nil {
		t.Fatalf("NewLLMExecutor: %v", err)
	}
	if _, err := executor.ExecuteInstruction(context.Background(), InstructionRequest{Name: "merge", Inputs: []string{"a"}}); err != nil {
		t.Fatalf("ExecuteInstruction: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no auth header, got %q", gotAuth)
	}
}

func TestLLMExecutorHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer server.Close()
	executor, err := NewLLMExecutor(LLMConfig{Model: "m", APIBase: server.URL})
	if err != nil {
		t.Fatalf("NewLLMExecutor: %v", err)
	}
	_, err = executor.ExecuteInstruction(context.Background(), InstructionRequest{Name: "summary", Inputs: []string{"x"}})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestLLMExecutorUsesOverriddenSystemPrompt(t *testing.T) {
	var gotBody struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	defer server.Close()
	executor, err := NewLLMExecutor(LLMConfig{
		Model: "m", APIBase: server.URL,
		SystemPrompts: map[string]string{"summary": "自定义系统提示词"},
	})
	if err != nil {
		t.Fatalf("NewLLMExecutor: %v", err)
	}
	if _, err := executor.ExecuteInstruction(context.Background(), InstructionRequest{Name: "summary", Inputs: []string{"x"}}); err != nil {
		t.Fatalf("ExecuteInstruction: %v", err)
	}
	if gotBody.Messages[0].Content != globalSystemPromptPrefix+"自定义系统提示词" {
		t.Fatalf("expected overridden system prompt (with global prefix), got %q", gotBody.Messages[0].Content)
	}
}
