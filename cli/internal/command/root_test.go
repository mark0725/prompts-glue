package command

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func testRepo(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GLUE_HOME", home)
	return filepath.Join(home, "repository")
}

func testArgs(repo string, args ...string) []string {
	base := []string{"--local", "--repository", repo}
	return append(base, args...)
}

func TestCreatePlainPackageAndInfo(t *testing.T) {
	repo := testRepo(t)
	args := testArgs(repo,
		"create",
		"--name", "base-rules",
		"--version", "1.0.0",
		"--category", "DEV",
		"--type", "PLAIN",
		"--content", "基础内容",
	)
	out, err := executeCommand(args...)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Fatalf("unexpected create output: %q", out)
	}
	infoArgs := testArgs(repo, "info", "base-rules")
	out, err = executeCommand(infoArgs...)
	if err != nil {
		t.Fatalf("info returned error: %v", err)
	}
	if !strings.Contains(out, "base-rules@1.0.0") || !strings.Contains(out, "base-rules") {
		t.Fatalf("unexpected info output: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(repo, "base-rules", "1.0.0.md"))
	if err != nil {
		t.Fatalf("read package file: %v", err)
	}
	text := string(data)
	if strings.Contains(text, "package:\n") {
		t.Fatalf("frontmatter should not contain package wrapper:\n%s", text)
	}
	if !strings.Contains(text, "name: base-rules") || !strings.Contains(text, "type: PLAIN") {
		t.Fatalf("frontmatter missing top-level fields:\n%s", text)
	}
	if !strings.Contains(text, "---\n\n基础内容") {
		t.Fatalf("frontmatter and content should be separated by one blank line:\n%s", text)
	}
}

func TestCreateCompositePackageUsesSDK(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLUE_HOME", home)
	repo := filepath.Join(home, "repository")

	// 模拟 OpenAI 兼容的 LLM 服务，避免依赖真实网络。
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "组合后的内容"}}},
			"usage":   map[string]int{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		})
	}))
	defer server.Close()
	t.Setenv("GLUE_LLM__MODEL", "test-model")
	t.Setenv("GLUE_LLM__API_BASE", server.URL)

	_, err := executeCommand("--local", "--repository", repo,
		"create",
		"--name", "base-rules",
		"--version", "1.0.0",
		"--description", "基础规则",
		"--category", "DEV",
		"--type", "PLAIN",
		"--content", "基础内容",
	)
	if err != nil {
		t.Fatalf("create base returned error: %v", err)
	}
	_, err = executeCommand("--local", "--repository", repo,
		"create",
		"--name", "combined-rules",
		"--version", "1.0.0",
		"--description", "组合规则",
		"--category", "DEV",
		"--expression", `base-rules | template("补充内容"){按模板输出}`,
	)
	if err != nil {
		t.Fatalf("create composite returned error: %v", err)
	}
	out, err := executeCommand("--local", "--repository", repo, "info", "combined-rules", "--deps")
	if err != nil {
		t.Fatalf("info composite returned error: %v", err)
	}
	if !strings.Contains(out, "combined-rules@1.0.0") || !strings.Contains(out, "base-rules") {
		t.Fatalf("expected dependency output, got %q", out)
	}
}

func TestContentCommandOutputsBodyOnly(t *testing.T) {
	repo := testRepo(t)
	_, err := executeCommand(testArgs(repo,
		"create",
		"--name", "claude-rules",
		"--version", "1.0.0",
		"--content", "第一行\n第二行",
	)...)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	out, err := executeCommand(testArgs(repo, "content", "claude-rules")...)
	if err != nil {
		t.Fatalf("content returned error: %v", err)
	}
	if out != "第一行\n第二行\n" {
		t.Fatalf("unexpected content output: %q", out)
	}
	if strings.Contains(out, "---") || strings.Contains(out, "name:") {
		t.Fatalf("content output should not include frontmatter: %q", out)
	}
}

func TestStartupCreatesSampleConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLUE_HOME", home)
	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	if !strings.Contains(out, "glue") {
		t.Fatalf("unexpected version output: %q", out)
	}
	data, err := os.ReadFile(filepath.Join(home, "config.yaml"))
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	text := string(data)
	checks := []string{
		"# 示例: /home/user/.glue\nhome: \"\"",
		"# 示例: /home/user/.glue/repository\n    path: \"\"",
		"# 示例: https://prompts.example.com\n    base: \"\"",
		"# 示例: deepseek-v4-flash\n    model: \"\"",
		"# 示例: https://api.example.com/v1\n    api_base: \"\"",
		"# 示例: sk-example\n    api_key: \"\"",
	}
	for _, check := range checks {
		if !strings.Contains(text, check) {
			t.Fatalf("sample config missing %q:\n%s", check, text)
		}
	}
}

func TestStartupDoesNotOverwriteExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GLUE_HOME", home)
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, []byte("local: true\nrepository:\n    path: /tmp/custom-repo\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	_, err := executeCommand("version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	if string(data) != "local: true\nrepository:\n    path: /tmp/custom-repo\n" {
		t.Fatalf("config file should not be overwritten:\n%s", string(data))
	}
}

func TestConfigSetAndGet(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repository")
	sets := [][]string{
		{"llm.model", "deepseek-v4-flash"},
		{"llm.api_base", "https://api.example.com/v1"},
		{"llm.api_key", "sk-example"},
	}
	for _, set := range sets {
		_, err := executeCommand("--repository", repo, "config", "set", set[0], set[1])
		if err != nil {
			t.Fatalf("config set %s returned error: %v", set[0], err)
		}
	}
	out, err := executeCommand("--repository", repo, "config", "get")
	if err != nil {
		t.Fatalf("config get returned error: %v", err)
	}
	for _, want := range []string{"deepseek-v4-flash", "https://api.example.com/v1", "llm.api_key: ****"} {
		if !strings.Contains(out, want) {
			t.Fatalf("config output missing %q: %q", want, out)
		}
	}
}

func TestLLMConfigOverrides(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repository")
	t.Setenv("GLUE_HOME", home)
	t.Setenv("GLUE_LLM__API_BASE", "https://env.example.com/v1")
	t.Setenv("GLUE_LLM__API_KEY", "env-key")
	out, err := executeCommand("--repository", repo, "--llm-api-base", "https://flag.example.com/v1", "--llm-api-key", "flag-key", "config", "get")
	if err != nil {
		t.Fatalf("config get returned error: %v", err)
	}
	for _, want := range []string{"https://flag.example.com/v1", "llm.api_key: ****"} {
		if !strings.Contains(out, want) {
			t.Fatalf("config output missing %q: %q", want, out)
		}
	}
	if strings.Contains(out, "https://env.example.com/v1") || strings.Contains(out, "env-key") {
		t.Fatalf("command line options should override env values: %q", out)
	}
}

func TestAddImportsPackageFile(t *testing.T) {
	home := t.TempDir()
	sourceRepo := filepath.Join(home, "source")
	targetRepo := filepath.Join(home, "target")
	_, err := executeCommand("--local", "--repository", sourceRepo,
		"create", "--name", "import-me", "--version", "1.0.0", "--description", "导入包", "--category", "DEV", "--content", "导入内容")
	if err != nil {
		t.Fatalf("create source returned error: %v", err)
	}
	file := filepath.Join(sourceRepo, "import-me", "1.0.0.md")
	out, err := executeCommand("--local", "--repository", targetRepo, "add", "--file", file)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}
	if !strings.Contains(out, "added import-me@1.0.0") {
		t.Fatalf("unexpected add output: %q", out)
	}
}

func TestCacheAndVersionJSON(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repository")
	out, err := executeCommand("--local", "--repository", repo, "--output", "json", "version")
	if err != nil {
		t.Fatalf("version returned error: %v", err)
	}
	var version versionOutput
	if err := json.Unmarshal([]byte(out), &version); err != nil {
		t.Fatalf("invalid version json: %v", err)
	}
	if version.Name != "glue" || version.Version == "" {
		t.Fatalf("unexpected version: %+v", version)
	}
	out, err = executeCommand("--local", "--repository", repo, "cache", "info")
	if err != nil {
		t.Fatalf("cache info returned error: %v", err)
	}
	if !strings.Contains(out, "cache entries") {
		t.Fatalf("unexpected cache output: %q", out)
	}
}

func TestPublishLocalModeRejected(t *testing.T) {
	_, err := executeCommand(testArgs(testRepo(t), "publish", "base-rules")...)
	if err == nil {
		t.Fatal("expected publish error")
	}
	if !strings.Contains(err.Error(), "local mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}
