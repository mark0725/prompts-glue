package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"hub-server/internal/api"
	"hub-server/internal/auth"
	"hub-server/internal/config"
	"hub-server/internal/model"
	"hub-server/internal/service"
)

type Server struct {
	cfg      config.Config
	db       *gorm.DB
	logger   *slog.Logger
	auth     *auth.Service
	registry *service.RegistryService
	dicts    *service.DictService
	handler  http.Handler
}

func New(cfg config.Config, db *gorm.DB, logger *slog.Logger) *Server {
	s := &Server{cfg: cfg, db: db, logger: logger}
	s.auth = auth.NewService(db, cfg)
	s.registry = service.NewRegistryService(db, cfg)
	s.dicts = service.NewDictService(db)
	s.handler = s.routes()
	return s
}

func (s *Server) Start() error {
	if err := s.Init(); err != nil {
		return err
	}
	s.logger.Info("hub-server listening", "address", s.cfg.Server.Address)
	return http.ListenAndServe(s.cfg.Server.Address, s.handler)
}

func (s *Server) Init() error {
	s.logger.Info("initializing admin account")
	if err := s.auth.EnsureAdmin(); err != nil {
		s.logger.Error("admin initialization failed", "error", err)
		return err
	}
	s.logger.Info("seeding data dictionary")
	if err := s.dicts.SeedDefaults(); err != nil {
		s.logger.Error("data dictionary seeding failed", "error", err)
		return err
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/registry/health", s.health)
	mux.HandleFunc("/api/registry/auth/login", s.login)
	mux.Handle("/api/registry/auth/me", s.requireAuth(http.HandlerFunc(s.me)))
	mux.Handle("/api/registry/packages", s.optionalAuth(http.HandlerFunc(s.packages)))
	mux.Handle("/api/registry/packages/", s.optionalAuth(http.HandlerFunc(s.packageRoutes)))
	mux.HandleFunc("/api/registry/resolve", s.resolve)
	mux.HandleFunc("/api/registry/dependencies/resolve", s.resolveDependencies)
	mux.HandleFunc("/api/registry/parse", s.parseExpr)
	mux.Handle("/api/registry/generate", s.optionalAuth(http.HandlerFunc(s.generate)))
	mux.Handle("/api/registry/cache", s.requireAuth(http.HandlerFunc(s.cache)))
	mux.Handle("/api/registry/my-packages", s.requireAuth(http.HandlerFunc(s.myPackages)))
	mux.Handle("/api/registry/my-generations", s.requireAuth(http.HandlerFunc(s.myGenerations)))
	mux.Handle("/api/registry/my-namespaces", s.requireAuth(http.HandlerFunc(s.myNamespaces)))
	mux.HandleFunc("/api/dicts", s.dictAll)
	mux.HandleFunc("/api/dicts/", s.dictGroup)
	mux.Handle("/api/admin/", s.requireAdmin(http.HandlerFunc(s.adminRoutes)))
	var handler http.Handler = mux
	handler = s.optionalAuth(handler)
	handler = bodyLimitMiddleware(s.cfg.Limits.RequestBodyBytes, handler)
	handler = loggingMiddleware(s.logger, handler)
	handler = requestIDMiddleware(handler)
	return handler
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	api.WriteJSON(w, r, http.StatusOK, map[string]any{"status": "OK", "version": "0.1.0", "time": time.Now().UTC()})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Token    string `json:"token"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	accessToken, expires, user, err := s.auth.Login(req.Username, req.Password)
	if err != nil {
		api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err)
		return
	}
	api.WriteJSON(w, r, http.StatusOK, map[string]any{"access_token": accessToken, "expires_at": expires, "user": publicUser(user)})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, r, http.StatusOK, publicUser(*CurrentUser(r)))
}

func (s *Server) packages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
		result, err := s.registry.Search(r.URL.Query().Get("q"), r.URL.Query().Get("category"), r.URL.Query().Get("tag"), r.URL.Query().Get("status"), r.URL.Query().Get("type"), page, pageSize)
		if err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, result)
	case http.MethodPost:
		user := CurrentUser(r)
		if user == nil {
			api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", errors.New("authentication required"))
			return
		}
		var req service.PublishRequest
		if err := api.DecodeJSON(r, &req); err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
			return
		}
		result, err := s.registry.Publish(r.Context(), req, user)
		if err != nil {
			api.WriteError(w, r, http.StatusBadRequest, "INVALID_PACKAGE", err)
			return
		}
		api.WriteJSON(w, r, http.StatusCreated, result)
	default:
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
	}
}

func (s *Server) packageRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/registry/packages/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] != "" {
		if r.Method != http.MethodGet {
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
			return
		}
		result, err := s.registry.Package(parts[0])
		writeRegistryResult(w, r, result, err)
		return
	}
	if len(parts) >= 3 && parts[1] == "versions" {
		name := parts[0]
		version := parts[2]
		if len(parts) == 3 && r.Method == http.MethodGet {
			result, err := s.registry.Version(name, version)
			writeRegistryResult(w, r, result, err)
			return
		}
		if len(parts) == 4 && parts[3] == "content" && r.Method == http.MethodGet {
			user := CurrentUser(r)
			userID := ""
			if user != nil {
				userID = user.ID
			}
					view := r.URL.Query().Get("view") == "true"
			result, err := s.registry.Content(name, version, userID, r.RemoteAddr, r.UserAgent(), !view)
			writeRegistryResult(w, r, result, err)
			return
		}
		if len(parts) == 4 && parts[3] == "deprecate" && r.Method == http.MethodPost {
			user := CurrentUser(r)
			if user == nil {
				api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", errors.New("authentication required"))
				return
			}
			var req struct {
				Reason      string `json:"reason"`
				Replacement string `json:"replacement"`
			}
			if err := api.DecodeJSON(r, &req); err != nil {
				api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
				return
			}
			if err := s.registry.Deprecate(name, version, req.Reason, req.Replacement, user); err != nil {
				api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
				return
			}
			api.WriteJSON(w, r, http.StatusOK, map[string]string{"status": "DEPRECATED"})
			return
		}
	}
	api.WriteError(w, r, http.StatusNotFound, "PACKAGE_NOT_FOUND", errors.New("not found"))
}

func (s *Server) resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Name              string `json:"name"`
		VersionRule       string `json:"version_rule"`
		IncludeDeprecated bool   `json:"include_deprecated"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	result, err := s.registry.Resolve(req.Name, req.VersionRule, req.IncludeDeprecated)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) resolveDependencies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Expression string `json:"expression"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	result, err := s.registry.ResolveDependencies(req.Expression)
	writeRegistryResult(w, r, map[string]any{"dependencies": result}, err)
}

func (s *Server) generate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req service.GenerateRequest
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	result, err := s.registry.Generate(r.Context(), req, CurrentUser(r))
	writeRegistryResult(w, r, result, err)
}

func (s *Server) cache(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
		result, err := s.registry.Cache(page, pageSize)
		writeRegistryResult(w, r, result, err)
	case http.MethodDelete:
		var req struct {
			ExpiredOnly bool `json:"expired_only"`
		}
		_ = api.DecodeJSON(r, &req)
		if err := s.registry.ClearCache(req.ExpiredOnly); err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "CACHE_UNAVAILABLE", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, map[string]bool{"cleared": true})
	default:
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
	}
}

func (s *Server) parseExpr(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Expression string `json:"expression"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	result, err := s.registry.ParseExpression(req.Expression)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myPackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed")); return }
	user := CurrentUser(r)
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	result, err := s.registry.MyPackages(user.ID, page, pageSize)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myGenerations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed")); return }
	user := CurrentUser(r)
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	result, err := s.registry.MyGenerations(user.ID, page, pageSize)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed")); return }
	user := CurrentUser(r)
	result, err := s.registry.MyNamespaces(user.ID)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) dictAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	items, err := s.dicts.All(true)
	writeRegistryResult(w, r, items, err)
}

func (s *Server) dictGroup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	group := strings.TrimPrefix(r.URL.Path, "/api/dicts/")
	items, err := s.dicts.Group(group, true)
	writeRegistryResult(w, r, items, err)
}

func (s *Server) adminRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/")
	switch {
	case path == "users" && r.Method == http.MethodGet:
		var users []map[string]any
		if err := s.db.Model(&struct{}{}).Raw("SELECT id, username, email, display_name, status, role, last_login_at, created_at FROM users ORDER BY created_at DESC").Scan(&users).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, users)
	case path == "dicts" && r.Method == http.MethodGet:
		items, err := s.dicts.All(false)
		writeRegistryResult(w, r, items, err)
	case path == "audit-logs" && r.Method == http.MethodGet:
		page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
		var total int64
		if err := s.db.Model(&model.AuditLog{}).Count(&total).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		var items []model.AuditLog
		if err := s.db.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, map[string]any{"items": items, "page": page, "page_size": pageSize, "total": total, "has_more": int64(page*pageSize) < total})
	case path == "generation-records" && r.Method == http.MethodGet:
		page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
		var total int64
		if err := s.db.Model(&model.GenerationRecord{}).Count(&total).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		var items []model.GenerationRecord
		if err := s.db.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, map[string]any{"items": items, "page": page, "page_size": pageSize, "total": total, "has_more": int64(page*pageSize) < total})
	case path == "roles" && r.Method == http.MethodGet:
		type roleCount struct {
			Code  string `json:"code"`
			Count int64  `json:"count"`
		}
		account := []roleCount{}
		if err := s.db.Model(&model.User{}).Select("role as code, count(*) as count").Where("deleted_at IS NULL").Group("role").Scan(&account).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		namespace := []roleCount{}
		if err := s.db.Model(&model.NamespaceMember{}).Select("role as code, count(*) as count").Where("deleted_at IS NULL").Group("role").Scan(&namespace).Error; err != nil {
			api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
			return
		}
		api.WriteJSON(w, r, http.StatusOK, map[string]any{"account_roles": account, "namespace_roles": namespace})
	default:
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
	}
}

func writeRegistryResult(w http.ResponseWriter, r *http.Request, data any, err error) {
	if err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	api.WriteJSON(w, r, http.StatusOK, data)
}

func pageParams(r *http.Request, max int) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > max {
		pageSize = max
	}
	return page, pageSize
}

func publicUser(user model.User) map[string]any {
	return map[string]any{
		"id":            user.ID,
		"username":      user.Username,
		"email":         user.Email,
		"display_name":  user.DisplayName,
		"status":        user.Status,
		"role":          user.Role,
		"last_login_at": user.LastLoginAt,
	}
}
