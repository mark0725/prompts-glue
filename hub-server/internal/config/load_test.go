package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Server.Address != ":8080" {
		t.Fatalf("server.address default = %q", cfg.Server.Address)
	}
	if time.Duration(cfg.Auth.TokenTTL) != 24*time.Hour {
		t.Fatalf("auth.token_ttl default = %v", cfg.Auth.TokenTTL)
	}
	if cfg.Limits.RequestBodyBytes != 1<<20 {
		t.Fatalf("limits.request_body_bytes default = %d", cfg.Limits.RequestBodyBytes)
	}
	if cfg.Cache.DefaultStrategy != "VERSIONED" {
		t.Fatalf("cache.default_strategy default = %q", cfg.Cache.DefaultStrategy)
	}
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadYAML(t *testing.T) {
	path := writeFile(t, "c.yaml", `
server: {address: ":9090"}
auth: {token_ttl: 12h}
llm: {model: "deepseek"}
limits: {request_body_bytes: 2048}
`)
	cfg, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Address != ":9090" {
		t.Fatalf("server.address = %q", cfg.Server.Address)
	}
	if time.Duration(cfg.Auth.TokenTTL) != 12*time.Hour {
		t.Fatalf("auth.token_ttl = %v", cfg.Auth.TokenTTL)
	}
	if cfg.LLM.Model != "deepseek" {
		t.Fatalf("llm.model = %q", cfg.LLM.Model)
	}
	if cfg.Limits.RequestBodyBytes != 2048 {
		t.Fatalf("limits.request_body_bytes = %d", cfg.Limits.RequestBodyBytes)
	}
}

func TestLoadJSON(t *testing.T) {
	path := writeFile(t, "c.json", `{"auth":{"token_ttl":"30m"},"llm":{"api_base":"https://x/v1"}}`)
	cfg, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if time.Duration(cfg.Auth.TokenTTL) != 30*time.Minute {
		t.Fatalf("auth.token_ttl = %v", cfg.Auth.TokenTTL)
	}
	if cfg.LLM.APIBase != "https://x/v1" {
		t.Fatalf("llm.api_base = %q", cfg.LLM.APIBase)
	}
}

func TestLoadTOML(t *testing.T) {
	path := writeFile(t, "c.toml", `
llm.model = "toml-model"
cache.ttl = "45m"
`)
	cfg, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLM.Model != "toml-model" {
		t.Fatalf("llm.model = %q", cfg.LLM.Model)
	}
	if time.Duration(cfg.Cache.TTL) != 45*time.Minute {
		t.Fatalf("cache.ttl = %v", cfg.Cache.TTL)
	}
}

func TestLoadUnknownExtension(t *testing.T) {
	path := writeFile(t, "c.ini", "k=v")
	if _, err := Load(path, nil); err == nil {
		t.Fatal("expected error for unknown extension")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("HUB_LLM__MODEL", "env-model")
	t.Setenv("HUB_AUTH__TOKEN_TTL", "15m")
	t.Setenv("HUB_LIMITS__REQUEST_BODY_BYTES", "999")
	t.Setenv("HUB_LIMITS__PAGE_SIZE_MAX", "7")
	cfg, err := Load("", nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLM.Model != "env-model" {
		t.Fatalf("llm.model = %q", cfg.LLM.Model)
	}
	if time.Duration(cfg.Auth.TokenTTL) != 15*time.Minute {
		t.Fatalf("auth.token_ttl = %v", cfg.Auth.TokenTTL)
	}
	if cfg.Limits.RequestBodyBytes != 999 {
		t.Fatalf("limits.request_body_bytes = %d", cfg.Limits.RequestBodyBytes)
	}
	if cfg.Limits.PageSizeMax != 7 {
		t.Fatalf("limits.page_size_max = %d", cfg.Limits.PageSizeMax)
	}
}

func TestEnvBeatsFile(t *testing.T) {
	path := writeFile(t, "c.yaml", `llm: {model: "file-model"}`)
	t.Setenv("HUB_LLM__MODEL", "env-model")
	cfg, err := Load(path, nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLM.Model != "env-model" {
		t.Fatalf("expected env to beat file, got %q", cfg.LLM.Model)
	}
}

func TestFlagBeatsEnv(t *testing.T) {
	t.Setenv("HUB_LLM__MODEL", "env-model")
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	if err := fs.Set("llm-model", "flag-model"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	cfg, err := Load("", fs)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LLM.Model != "flag-model" {
		t.Fatalf("expected flag to beat env, got %q", cfg.LLM.Model)
	}
}

func TestRegisterFlagsContains(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(fs)
	for _, name := range []string{"llm-model", "llm-api-base", "auth-token-ttl", "limits-request-body-bytes", "logging-level"} {
		if fs.Lookup(name) == nil {
			t.Fatalf("flag --%s not registered", name)
		}
	}
}

func TestNamingTable(t *testing.T) {
	want := map[string]struct{ env, flag string }{
		"llm.model":                  {"HUB_LLM__MODEL", "llm-model"},
		"llm.api_base":               {"HUB_LLM__API_BASE", "llm-api-base"},
		"auth.token_ttl":             {"HUB_AUTH__TOKEN_TTL", "auth-token-ttl"},
		"cache.ttl":                  {"HUB_CACHE__TTL", "cache-ttl"},
		"limits.request_body_bytes":  {"HUB_LIMITS__REQUEST_BODY_BYTES", "limits-request-body-bytes"},
		"limits.page_size_max":       {"HUB_LIMITS__PAGE_SIZE_MAX", "limits-page-size-max"},
		"logging.level":              {"HUB_LOGGING__LEVEL", "logging-level"},
		"server.public_base_url":     {"HUB_SERVER__PUBLIC_BASE_URL", "server-public-base-url"},
	}
	got := map[string]fieldDesc{}
	for _, d := range describe() {
		got[d.path] = d
	}
	for path, w := range want {
		d, ok := got[path]
		if !ok {
			t.Fatalf("no descriptor for %s", path)
		}
		if d.env != w.env || d.flag != w.flag {
			t.Fatalf("%s: want env=%s flag=%s; got env=%s flag=%s", path, w.env, w.flag, d.env, d.flag)
		}
	}
}

// tSliceStruct 仅用于验证数组类型的环境变量/命令行收集机制（Config 当前无数组字段）。
type tSliceStruct struct {
	Tags []string `yaml:"tags"`
}

func TestSliceFromEnv(t *testing.T) {
	t.Setenv("HUB_TESTTAGS__0", "a")
	t.Setenv("HUB_TESTTAGS__1", "b")
	t.Setenv("HUB_TESTTAGS__3", "d")
	var s tSliceStruct
	field := reflect.ValueOf(&s).Elem().FieldByName("Tags")
	setSliceFromEnv(field, "HUB_TESTTAGS")
	if len(s.Tags) != 3 || s.Tags[0] != "a" || s.Tags[1] != "b" || s.Tags[2] != "d" {
		t.Fatalf("unexpected slice %#v", s.Tags)
	}
}

// TestSliceFromEnvTrailingSep 验证项目规范的数组下标带尾 __ 形式（如 HUB_LDAP__SYNC_TARGETS__0__）。
func TestSliceFromEnvTrailingSep(t *testing.T) {
	t.Setenv("HUB_TESTTAGS__0__", "manager")
	t.Setenv("HUB_TESTTAGS__1__", "web")
	var s tSliceStruct
	field := reflect.ValueOf(&s).Elem().FieldByName("Tags")
	setSliceFromEnv(field, "HUB_TESTTAGS")
	if len(s.Tags) != 2 || s.Tags[0] != "manager" || s.Tags[1] != "web" {
		t.Fatalf("unexpected slice %#v", s.Tags)
	}
}

// TestSliceFromEnvOnlyHighIndex 验证仅设置高位下标（无 __0）时，数组整体替换默认值且不留空位。
// 对应 HUB_LDAP__SYNC_TARGETS__1=web（不含 __0）应得到 ["web"]，而非 ["", "web"] 或保留默认。
func TestSliceFromEnvOnlyHighIndex(t *testing.T) {
	t.Setenv("HUB_TESTTAGS__1", "web")
	var s tSliceStruct
	field := reflect.ValueOf(&s).Elem().FieldByName("Tags")
	setSliceFromEnv(field, "HUB_TESTTAGS")
	if len(s.Tags) != 1 || s.Tags[0] != "web" {
		t.Fatalf("expected [web], got %#v", s.Tags)
	}
}

func TestSliceFromFlag(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.StringArray("tags", nil, "")
	if err := fs.Parse([]string{"--tags=x", "--tags=y"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	values, err := flagStringSlice(fs.Lookup("tags"))
	if err != nil {
		t.Fatalf("flagStringSlice: %v", err)
	}
	var s tSliceStruct
	if err := setSlice(reflect.ValueOf(&s).Elem().FieldByName("Tags"), values); err != nil {
		t.Fatalf("setSlice: %v", err)
	}
	if len(s.Tags) != 2 || s.Tags[0] != "x" || s.Tags[1] != "y" {
		t.Fatalf("unexpected slice %#v", s.Tags)
	}
}
