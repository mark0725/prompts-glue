package command

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// executeWithStdin 运行根命令并把 stdin 注入 template 等从标准输入读取的命令。
func executeWithStdin(stdin string, args ...string) (string, error) {
	cmd := NewRootCommand()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if stdin != "" {
		cmd.SetIn(strings.NewReader(stdin))
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// createPackageForTest 在本地 repository 创建一个普通提示词包，供 template 引用。
func createPackageForTest(t *testing.T, repo, name, content string) {
	t.Helper()
	_, err := executeCommand(testArgs(repo, "create",
		"--name", name, "--version", "1.0.0", "--content", content)...)
	if err != nil {
		t.Fatalf("create %s returned error: %v", name, err)
	}
}

func TestTemplateSubstitutesPromptRef(t *testing.T) {
	repo := testRepo(t)
	createPackageForTest(t, repo, "base-rules", "基础规则")
	out, err := executeCommand(testArgs(repo, "template", "--content", "前缀 {{ base-rules }} 后缀")...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if want := "前缀 基础规则 后缀"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestTemplateLiteralAndVariable(t *testing.T) {
	repo := testRepo(t)
	out, err := executeCommand(testArgs(repo, "template",
		"--content", `Hello {{ "World" }} - {{ $who }}`,
		"--var", "who=glue")...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if want := "Hello World - glue"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// 字符串字面量中的 }} 不应触发占位符闭合，验证扫描器对字符串边界的正确处理。
func TestTemplateStringLiteralContainsClosingBraces(t *testing.T) {
	repo := testRepo(t)
	out, err := executeCommand(testArgs(repo, "template", "--content", `{{ "a}}b" }}`)...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if want := "a}}b"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// 指令 option 的花括号不应让占位符提前闭合。
func TestTemplatePlaceholderWithOptionBraces(t *testing.T) {
	repo := testRepo(t)
	createPackageForTest(t, repo, "base-rules", "基础内容")
	server := startMockLLM(t, "组合结果")
	defer server.Close()
	t.Setenv("GLUE_LLM__MODEL", "test-model")
	t.Setenv("GLUE_LLM__API_BASE", server.URL)
	out, err := executeCommand(testArgs(repo, "template",
		"--content", "{{ merge(base-rules){按规则合并} }}")...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if !strings.Contains(out, "组合结果") {
		t.Fatalf("expected instruction output, got %q", out)
	}
}

func TestTemplateMultiplePlaceholders(t *testing.T) {
	repo := testRepo(t)
	out, err := executeCommand(testArgs(repo, "template",
		"--content", `A {{ "X" }} B {{ "Y" }} C`)...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if want := "A X B Y C"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

func TestTemplateReadsStdin(t *testing.T) {
	repo := testRepo(t)
	out, err := executeWithStdin(`X {{ "Y" }} Z`, testArgs(repo, "template")...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if want := "X Y Z"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}

// 严格模式：缺失上下文变量应报错终止。
func TestTemplateStrictErrorOnMissingVariable(t *testing.T) {
	repo := testRepo(t)
	_, err := executeCommand(testArgs(repo, "template", "--content", `{{ $missing }}`)...)
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
}

// 严格模式：占位符内非法表达式应报错终止。
func TestTemplateStrictErrorOnInvalidExpression(t *testing.T) {
	repo := testRepo(t)
	_, err := executeCommand(testArgs(repo, "template", "--content", `{{ !!! }}`)...)
	if err == nil {
		t.Fatal("expected error for invalid expression, got nil")
	}
}

// 未闭合的占位符应报错。
func TestTemplateUnclosedPlaceholder(t *testing.T) {
	repo := testRepo(t)
	_, err := executeCommand(testArgs(repo, "template", "--content", `{{ base-rules`)...)
	if err == nil {
		t.Fatal("expected error for unclosed placeholder, got nil")
	}
}

func TestTemplateJSONOutput(t *testing.T) {
	repo := testRepo(t)
	out, err := executeCommand(testArgs(repo, "--output", "json", "template", "--content", `{{ "hi" }}`)...)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	var got templateOutput
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, out)
	}
	if got.Content != "hi" {
		t.Fatalf("unexpected content: %q", got.Content)
	}
}

// TestTemplateInstructionWithMockLLM 验证含制作指令的占位符经 LLM 求值后被替换。
func TestTemplateInstructionWithMockLLM(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLUE_HOME", home)
	repo := home + "/repository"
	server := startMockLLM(t, "摘要结果")
	defer server.Close()
	t.Setenv("GLUE_LLM__MODEL", "test-model")
	t.Setenv("GLUE_LLM__API_BASE", server.URL)
	out, err := executeCommand("--local", "--repository", repo, "template",
		"--content", `{{ summary("一段需要摘要的较长文本") }}`)
	if err != nil {
		t.Fatalf("template returned error: %v", err)
	}
	if !strings.Contains(out, "摘要结果") {
		t.Fatalf("expected mocked instruction output, got %q", out)
	}
}

// startMockLLM 启动一个模拟 OpenAI 兼容 chat/completions 的测试服务，返回固定内容。
func startMockLLM(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": content}}},
			"usage":   map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
}
