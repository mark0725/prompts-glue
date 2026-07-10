package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/server"
)

func TestHealthLoginAndDicts(t *testing.T) {
	cfg := config.Default()
	cfg.Database.DSN = ":memory:"
	cfg.Auth.AdminPassword = "secret-pass"
	cfg.Auth.TokenTTL = time.Hour
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
	requireOK(t, app.Handler(), http.MethodGet, "/api/registry/health", nil, "")
	loginBody := []byte(`{"username":"admin","password":"secret-pass"}`)
	resp := requireOK(t, app.Handler(), http.MethodPost, "/api/registry/auth/login", loginBody, "")
	data := resp["data"].(map[string]any)
	if data["access_token"] == "" {
		t.Fatal("expected access token")
	}
	requireOK(t, app.Handler(), http.MethodGet, "/api/dicts/prompt_type", nil, "")
}

func requireOK(t *testing.T, handler http.Handler, method string, path string, body []byte, token string) map[string]any {
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
	if rr.Code < 200 || rr.Code >= 300 {
		t.Fatalf("expected success, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success response: %v", resp)
	}
	return resp
}
