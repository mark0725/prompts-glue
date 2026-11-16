package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeStaticFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestSPAHandler(t *testing.T) {
	dir := t.TempDir()
	writeStaticFile(t, dir, "index.html", "<html>index</html>")
	writeStaticFile(t, dir, "assets/app-abc.js", "console.log(1)")
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "api:"+r.URL.Path)
	})
	handler, err := newSPAHandler(dir, api)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		method      string
		path        string
		wantStatus  int
		wantBody    string
		wantCache   string
		bodyContain bool
	}{
		{name: "index", method: http.MethodGet, path: "/", wantStatus: http.StatusOK, wantBody: "<html>index</html>", wantCache: "no-cache"},
		{name: "spa fallback", method: http.MethodGet, path: "/packages/foo", wantStatus: http.StatusOK, wantBody: "<html>index</html>", wantCache: "no-cache"},
		{name: "asset", method: http.MethodGet, path: "/assets/app-abc.js", wantStatus: http.StatusOK, wantBody: "console.log(1)", wantCache: "immutable"},
		{name: "missing asset", method: http.MethodGet, path: "/assets/missing.js", wantStatus: http.StatusNotFound, wantBody: "404 page not found"},
		{name: "api", method: http.MethodGet, path: "/api/registry/health", wantStatus: http.StatusOK, wantBody: "api:/api/registry/health"},
		{name: "method not allowed", method: http.MethodPost, path: "/packages", wantStatus: http.StatusMethodNotAllowed, wantBody: "method not allowed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tc.wantBody) {
				t.Fatalf("body = %q, want contains %q", body, tc.wantBody)
			}
			if tc.wantCache != "" {
				if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, tc.wantCache) {
					t.Fatalf("Cache-Control = %q, want contains %q", got, tc.wantCache)
				}
			}
		})
	}
}

func TestNewSPAHandlerRequiresIndex(t *testing.T) {
	if _, err := newSPAHandler(t.TempDir(), http.NotFoundHandler()); err == nil {
		t.Fatal("expected error for directory without index.html")
	}
}

func TestWebMuxExcludesAdminRoutes(t *testing.T) {
	mux := (&Server{}).newMux(false)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/auth/me", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("web mux admin route status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/registry/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("web mux registry health status = %d, want 200", rec.Code)
	}
}
