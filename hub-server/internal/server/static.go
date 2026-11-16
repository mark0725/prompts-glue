package server

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// assetsCacheControl 用于 Vite 输出的带哈希静态资源，可长期缓存。
const assetsCacheControl = "public, max-age=604800, immutable"

// spaHandler 托管一个前端构建产物目录，并把 /api 请求交给后端 API 处理。
// 非 API 请求命中文件时直接返回；未命中时回退 index.html，支持前端路由。
type spaHandler struct {
	root *os.Root
	api  http.Handler
}

// newSPAHandler 打开静态目录并校验 index.html 存在。使用 os.Root 限制访问范围，
// 避免通过符号链接或路径穿越访问目录之外的文件。
func newSPAHandler(dir string, api http.Handler) (*spaHandler, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve directory %q: %w", dir, err)
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, fmt.Errorf("open static directory %q: %w", abs, err)
	}
	info, err := root.Stat("index.html")
	if err == nil && info.IsDir() {
		err = fmt.Errorf("index.html is a directory")
	}
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("static directory %q has no usable index.html: %w", abs, err)
	}
	return &spaHandler{root: root, api: api}, nil
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
		h.api.ServeHTTP(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "." {
		name = ""
	}
	if name != "" && !fs.ValidPath(name) {
		http.NotFound(w, r)
		return
	}
	// 构建资源缺失时返回 404，避免把 index.html 当作 JS/CSS 返回。
	if strings.HasPrefix(name, "assets/") {
		if !h.serveFile(w, r, name) {
			http.NotFound(w, r)
		}
		return
	}
	if name != "" && h.serveFile(w, r, name) {
		return
	}
	h.serveIndex(w, r)
}

func (h *spaHandler) serveFile(w http.ResponseWriter, r *http.Request, name string) bool {
	file, err := h.root.Open(name)
	if err != nil {
		return false
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", assetsCacheControl)
	} else if name == "index.html" {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, name, info.ModTime(), file)
	return true
}

func (h *spaHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := h.root.Open("index.html")
	if err != nil {
		http.Error(w, "web UI not available", http.StatusServiceUnavailable)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "web UI not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", info.ModTime(), file)
}
