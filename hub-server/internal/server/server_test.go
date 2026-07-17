package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/model"
	"hub-server/internal/server"
)

func newApp(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()
	cfg := config.Default()
	cfg.Database.DSN = ":memory:"
	cfg.Auth.AdminPassword = "secret-pass"
	cfg.Auth.TokenTTL = config.Duration(time.Hour)
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.BeforeCreate(db)
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	app := server.New(cfg, db, slog.Default())
	if err := app.Init(); err != nil {
		t.Fatal(err)
	}
	return app.Handler(), db
}

func TestHealthLoginAndDicts(t *testing.T) {
	handler, _ := newApp(t)
	requireOK(t, handler, http.MethodGet, "/api/registry/health", nil, "")
	// 引导 admin 现在进 manager_users，必须走管理端登录端点。
	managerToken := loginOK(t, handler, "/api/admin/auth/login", "admin", "secret-pass")
	requireOK(t, handler, http.MethodGet, "/api/admin/manager-users", nil, managerToken)
	requireOK(t, handler, http.MethodGet, "/api/dicts/prompt_type", nil, "")
}

// TestManagerLoginRoleGate 验证管理端登录门禁：属于任意角色的用户可登录，无任何角色的用户被拒（403）。
func TestManagerLoginRoleGate(t *testing.T) {
	handler, db := newApp(t)
	adminToken := loginOK(t, handler, "/api/admin/auth/login", "admin", "secret-pass")
	// 创建 operator 角色 + 一个属于该角色的 manager 用户 → 可登录（属于某角色）。
	requireOK(t, handler, http.MethodPost, "/api/admin/roles", []byte(`{"code":"operator","name":"运营","enabled":true}`), adminToken)
	requireOK(t, handler, http.MethodPost, "/api/admin/manager-users", []byte(`{"username":"ops","password":"opspass","role":"operator"}`), adminToken)
	loginOK(t, handler, "/api/admin/auth/login", "ops", "opspass")
	// 创建一个 manager 用户后，清除其所有角色成员关系 → 无任何角色 → 不能登录（403）。
	requireOK(t, handler, http.MethodPost, "/api/admin/manager-users", []byte(`{"username":"norole","password":"p"}`), adminToken)
	var norole model.ManagerUser
	if err := db.Where("username = ?", "norole").First(&norole).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("user_id = ?", norole.ID).Delete(&model.RoleMember{}).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, handler, http.MethodPost, "/api/admin/auth/login", []byte(`{"username":"norole","password":"p"}`), "", http.StatusForbidden)
	// bootstrap admin 仍可登录。
	loginOK(t, handler, "/api/admin/auth/login", "admin", "secret-pass")
}

// TestRBACPATAndGenerate 验证 RBAC 种子（角色/权限）、PAT 管理、generate 需登录。
func TestRBACPATAndGenerate(t *testing.T) {
	handler, _ := newApp(t)
	// 1) manager 体系：RBAC 种子。admin 角色与权限列表可读。
	managerToken := loginOK(t, handler, "/api/admin/auth/login", "admin", "secret-pass")
	roles := requireOK(t, handler, http.MethodGet, "/api/admin/roles", nil, managerToken)
	if data, _ := roles["data"].([]any); len(data) == 0 {
		t.Fatalf("expected seeded roles")
	}
	perms := requireOK(t, handler, http.MethodGet, "/api/admin/permissions", nil, managerToken)
	if data, _ := perms["data"].([]any); len(data) == 0 {
		t.Fatalf("expected seeded permissions")
	}
	// 2) manager 创建本地 web 用户，web 登录后可创建/列出/吊销 PAT。
	body := []byte(`{"username":"webu","password":"webpass"}`)
	requireOK(t, handler, http.MethodPost, "/api/admin/users", body, managerToken)
	webToken := loginOK(t, handler, "/api/registry/auth/login", "webu", "webpass")
	created := requireOK(t, handler, http.MethodPost, "/api/registry/tokens", []byte(`{"name":"ci-token"}`), webToken)
	if created["data"] == nil {
		t.Fatalf("expected PAT created")
	}
	tokenMap, _ := created["data"].(map[string]any)
	pat, _ := tokenMap["access_token"].(string)
	if pat == "" {
		t.Fatalf("expected plaintext token returned once")
	}
	tokList := requireOK(t, handler, http.MethodGet, "/api/registry/tokens", nil, webToken)
	if items, _ := tokList["data"].([]any); len(items) == 0 {
		t.Fatalf("expected tokens listed (login + PAT)")
	}
	// 3) generate 未登录 401，带 token 200（字面量表达式不需 LLM）。
	requireStatus(t, handler, http.MethodPost, "/api/registry/generate", []byte(`{"expression":"\"hi\""}`), "", http.StatusUnauthorized)
	requireOK(t, handler, http.MethodPost, "/api/registry/generate", []byte(`{"expression":"\"hi\""}`), pat)
}

// TestRealmIsolation 验证 WEB 令牌与 MANAGER 令牌不能跨域访问。
func TestRealmIsolation(t *testing.T) {
	handler, _ := newApp(t)

	managerToken := loginOK(t, handler, "/api/admin/auth/login", "admin", "secret-pass")

	// manager 创建一个本地 web 用户。
	body := []byte(`{"username":"webu","password":"webpass","display_name":"Web User"}`)
	requireOK(t, handler, http.MethodPost, "/api/admin/users", body, managerToken)

	// web 用户走 web 登录端点。
	webToken := loginOK(t, handler, "/api/registry/auth/login", "webu", "webpass")

	// web 用户可以访问 registry 鉴权路由。
	requireOK(t, handler, http.MethodGet, "/api/registry/auth/me", nil, webToken)
	// web 用户不能访问管理端（403）。
	requireStatus(t, handler, http.MethodGet, "/api/admin/users", nil, webToken, http.StatusForbidden)
	// manager 不能访问 web 身份接口（403，/me 是 web 域；manager 用 /api/admin/auth/me）。
	requireStatus(t, handler, http.MethodGet, "/api/registry/auth/me", nil, managerToken, http.StatusForbidden)
	// manager 可以浏览 registry 数据路由（manager 界面需读 packages 等）。
	requireOK(t, handler, http.MethodGet, "/api/registry/packages?page=1&page_size=8", nil, managerToken)
	// admin 同名账号不在 web 表中：web 登录 admin 失败。
	requireStatus(t, handler, http.MethodPost, "/api/registry/auth/login", []byte(`{"username":"admin","password":"secret-pass"}`), "", http.StatusUnauthorized)
}

func loginOK(t *testing.T, handler http.Handler, path, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp := requireOK(t, handler, http.MethodPost, path, body, "")
	data := resp["data"].(map[string]any)
	token, _ := data["access_token"].(string)
	if token == "" {
		t.Fatalf("expected access token from %s", path)
	}
	return token
}

func requireOK(t *testing.T, handler http.Handler, method string, path string, body []byte, token string) map[string]any {
	t.Helper()
	resp := requireStatus(t, handler, method, path, body, token, 0)
	if resp == nil {
		t.Fatalf("expected success response with data")
	}
	return resp
}

func requireStatus(t *testing.T, handler http.Handler, method string, path string, body []byte, token string, wantStatus int) map[string]any {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if wantStatus == 0 {
		if rr.Code < 200 || rr.Code >= 300 {
			t.Fatalf("expected success, got %d: %s", rr.Code, rr.Body.String())
		}
	} else if rr.Code != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if rr.Body.Len() > 0 {
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v: %s", err, rr.Body.String())
		}
	}
	return resp
}
