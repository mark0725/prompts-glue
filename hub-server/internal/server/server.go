package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gorm.io/gorm"

	"hub-server/internal/api"
	"hub-server/internal/auth"
	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/ldap"
	"hub-server/internal/model"
	"hub-server/internal/rbac"
	"hub-server/internal/service"
)

var (
	errUnauthorized = errors.New("authentication required")
	errForbidden    = errors.New("access forbidden")
)

type Server struct {
	cfg        config.Config
	db         *gorm.DB
	logger     *slog.Logger
	auth       *auth.Service
	registry   *service.RegistryService
	dicts      *service.DictService
	syncer     *ldap.Service
	rbacSvc    *rbac.Service
	handler    http.Handler
	httpServer *http.Server
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

func New(cfg config.Config, db *gorm.DB, logger *slog.Logger) *Server {
	syncer := ldap.NewService(cfg.LDAP, db, logger)
	s := &Server{
		cfg:      cfg,
		db:       db,
		logger:   logger,
		auth:     auth.NewService(db, cfg, syncer),
		registry: service.NewRegistryService(db, cfg),
		dicts:    service.NewDictService(db),
		syncer:   syncer,
		rbacSvc:  rbac.NewService(db, logger),
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.handler = s.routes()
	return s
}

func (s *Server) Start() error {
	if err := s.Init(); err != nil {
		return err
	}
	s.startLDAPSync()
	s.httpServer = &http.Server{Addr: s.cfg.Server.Address, Handler: s.handler}
	s.logger.Info("hub-server listening", "address", s.cfg.Server.Address)
	errCh := make(chan error, 1)
	go func() { errCh <- s.httpServer.ListenAndServe() }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		s.cancel()
		return err
	case <-sigCh:
		s.logger.Info("hub-server shutting down")
	}
	s.cancel()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("http shutdown error", "error", err)
	}
	s.wg.Wait()
	return nil
}

// startLDAPSync 在 LDAP 启用时启动同步：先立即同步一次，再按周期定时同步。
func (s *Server) startLDAPSync() {
	if !s.cfg.LDAP.Enable {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 启动后立即同步一次，便于配置变更后即时生效。
		if stats, err := s.syncer.Sync(s.ctx); err != nil {
			s.logger.Warn("ldap initial sync failed", "error", err)
		} else {
			s.logger.Info("ldap initial sync done", "manager", stats.SyncedManager, "web", stats.SyncedWeb, "errors", len(stats.Errors))
		}
		interval := time.Duration(s.cfg.LDAP.SyncInterval)
		if interval <= 0 {
			return
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		s.logger.Info("ldap periodic sync started", "interval", interval.String())
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				if stats, err := s.syncer.Sync(s.ctx); err != nil {
					s.logger.Warn("ldap periodic sync failed", "error", err)
				} else {
					s.logger.Debug("ldap periodic sync done", "manager", stats.SyncedManager, "web", stats.SyncedWeb)
				}
			}
		}
	}()
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
	s.logger.Info("seeding rbac permissions and admin role")
	if err := s.rbacSvc.SeedDefaults(); err != nil {
		s.logger.Error("rbac seeding failed", "error", err)
		return err
	}
	// users 表旧的 role 列已从模型移除但 AutoMigrate 不删；其为 NOT NULL 无默认值会阻塞
	// LDAP 同步向 web 用户表插入，故显式删除（失败仅告警，不阻断启动）。
	if err := database.DropLegacyUsersRole(s.db); err != nil {
		s.logger.Warn("drop legacy users.role column failed", "error", err)
	}
	// 引导管理员账号绑定 admin 角色（拥有全部权限）。
	if s.cfg.Auth.AdminPassword != "" {
		var admin model.ManagerUser
		if err := s.db.Where("username = ?", s.cfg.Auth.AdminUsername).First(&admin).Error; err == nil {
			if err := s.rbacSvc.AssignAdminRole(admin.ID); err != nil {
				s.logger.Warn("assign admin role failed", "error", err)
			}
		}
	}
	return nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/registry/health", s.health)
	mux.HandleFunc("/api/registry/auth/login", s.webLogin)
	mux.Handle("/api/registry/auth/me", s.requireRealm(model.RealmWeb, http.HandlerFunc(s.me)))
	mux.Handle("/api/registry/packages", s.optionalAuth(http.HandlerFunc(s.packages)))
	mux.Handle("/api/registry/packages/", s.optionalAuth(http.HandlerFunc(s.packageRoutes)))
	mux.HandleFunc("/api/registry/resolve", s.resolve)
	mux.HandleFunc("/api/registry/dependencies/resolve", s.resolveDependencies)
	mux.HandleFunc("/api/registry/parse", s.parseExpr)
	mux.Handle("/api/registry/generate", s.requireAuth(http.HandlerFunc(s.generate)))
	mux.Handle("/api/registry/cache", s.requireAuth(http.HandlerFunc(s.cache)))
	mux.Handle("/api/registry/my-packages", s.requireAuth(http.HandlerFunc(s.myPackages)))
	mux.Handle("/api/registry/my-generations", s.requireAuth(http.HandlerFunc(s.myGenerations)))
	mux.Handle("/api/registry/my-namespaces", s.requireAuth(http.HandlerFunc(s.myNamespaces)))
	mux.Handle("/api/registry/my-stars", s.requireAuth(http.HandlerFunc(s.myStars)))
	mux.Handle("/api/registry/tokens", s.requireRealm(model.RealmWeb, http.HandlerFunc(s.tokenRoutes)))
	mux.Handle("/api/registry/tokens/", s.requireRealm(model.RealmWeb, http.HandlerFunc(s.tokenRoutes)))
	mux.HandleFunc("/api/dicts", s.dictAll)
	mux.HandleFunc("/api/dicts/", s.dictGroup)
	mux.HandleFunc("/api/admin/auth/login", s.managerLogin)
	mux.Handle("/api/admin/auth/me", s.requireRealm(model.RealmManager, http.HandlerFunc(s.managerMe)))
	mux.Handle("/api/admin/", s.requireRealm(model.RealmManager, http.HandlerFunc(s.adminRoutes)))
	var handler http.Handler = mux
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

func (s *Server) webLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Client   string `json:"client"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	accessToken, p, err := s.auth.WebLogin(r.Context(), req.Username, req.Password, req.Client)
	if err != nil {
		api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err)
		return
	}
	api.WriteJSON(w, r, http.StatusOK, map[string]any{"access_token": accessToken, "user": publicUser(p)})
}

func (s *Server) managerLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Client   string `json:"client"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	accessToken, p, err := s.auth.ManagerLogin(r.Context(), req.Username, req.Password, req.Client)
	if err != nil {
		api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", err)
		return
	}
	api.WriteJSON(w, r, http.StatusOK, map[string]any{"access_token": accessToken, "user": publicManagerUser(p)})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, r, http.StatusOK, publicUser(CurrentUser(r)))
}

func (s *Server) managerMe(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, r, http.StatusOK, publicManagerUser(CurrentUser(r)))
}

func (s *Server) packages(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
		userID := ""
		if user := CurrentUser(r); user != nil {
			userID = user.ID
		}
		result, err := s.registry.Search(r.URL.Query().Get("q"), r.URL.Query().Get("category"), r.URL.Query().Get("tag"), r.URL.Query().Get("status"), r.URL.Query().Get("type"), r.URL.Query().Get("sort"), userID, page, pageSize)
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
		userID := ""
		if user := CurrentUser(r); user != nil {
			userID = user.ID
		}
		result, err := s.registry.Package(parts[0], userID)
		writeRegistryResult(w, r, result, err)
		return
	}
	if len(parts) == 2 && parts[1] == "star" {
		user := CurrentUser(r)
		if user == nil {
			api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", errors.New("authentication required"))
			return
		}
		switch r.Method {
		case http.MethodPost:
			count, err := s.registry.Star(parts[0], user)
			if err != nil {
				api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
				return
			}
			api.WriteJSON(w, r, http.StatusOK, map[string]any{"star_count": count, "starred": true})
		case http.MethodDelete:
			count, err := s.registry.Unstar(parts[0], user)
			if err != nil {
				api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
				return
			}
			api.WriteJSON(w, r, http.StatusOK, map[string]any{"star_count": count, "starred": false})
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
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
			if err != nil {
				writeRegistryResult(w, r, result, err)
				return
			}
			if strings.EqualFold(r.URL.Query().Get("format"), "raw") {
				w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(result.Content))
				return
			}
			writeRegistryResult(w, r, result, nil)
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
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	user := CurrentUser(r)
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	result, err := s.registry.MyPackages(user.ID, page, pageSize)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myStars(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	user := CurrentUser(r)
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	result, err := s.registry.MyStars(user.ID, page, pageSize)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myGenerations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
	user := CurrentUser(r)
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	result, err := s.registry.MyGenerations(user.ID, page, pageSize)
	writeRegistryResult(w, r, result, err)
}

func (s *Server) myNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
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
	segments := strings.Split(path, "/")
	if len(segments) == 0 || segments[0] == "" {
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
		return
	}
	switch segments[0] {
	case "manager-users":
		s.managerUserRoutes(w, r, segments[1:])
	case "users":
		s.webUserRoutes(w, r, segments[1:])
	case "ldap":
		if len(segments) == 2 && segments[1] == "sync" && r.Method == http.MethodPost {
			s.ldapSync(w, r)
			return
		}
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
	case "dicts":
		if r.Method != http.MethodGet {
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
			return
		}
		if !s.checkPermission(w, r, "dict:read") {
			return
		}
		items, err := s.dicts.All(false)
		writeRegistryResult(w, r, items, err)
	case "audit-logs":
		if !s.checkPermission(w, r, "audit:read") {
			return
		}
		s.listAuditLogs(w, r)
	case "generation-records":
		if !s.checkPermission(w, r, "generation:read") {
			return
		}
		s.listGenerationRecords(w, r)
	case "roles":
		s.roleRoutes(w, r, segments[1:])
	case "permissions":
		if r.Method != http.MethodGet {
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
			return
		}
		s.listPermissions(w, r)
	default:
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
	}
}

// managerUserRoutes 处理 manager_users 的增删改查与重置密码。
func (s *Server) managerUserRoutes(w http.ResponseWriter, r *http.Request, rest []string) {
	actor := CurrentUser(r)
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			if !s.checkPermission(w, r, "user:read") {
				return
			}
			s.listUsers(w, r, &[]model.ManagerUser{}, "manager_users")
		case http.MethodPost:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.createManagerUser(w, r, actor)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	id := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodPatch:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.updateManagerUser(w, r, id, actor)
		case http.MethodDelete:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.deleteUser(w, r, "manager_users", id, "MANAGER_USER_DELETE", actor)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	if len(rest) == 2 && rest[1] == "reset-password" && r.Method == http.MethodPost {
		if !s.checkPermission(w, r, "user:write") {
			return
		}
		s.resetPassword(w, r, "manager_users", id, "MANAGER_USER_RESET_PASSWORD", actor)
		return
	}
	if len(rest) == 2 && rest[1] == "tokens" && r.Method == http.MethodGet {
		if !s.checkPermission(w, r, "user:token") {
			return
		}
		s.listUserTokens(w, r, id, model.RealmManager)
		return
	}
	if len(rest) == 3 && rest[1] == "tokens" && r.Method == http.MethodDelete {
		if !s.checkPermission(w, r, "user:token") {
			return
		}
		s.adminRevokeToken(w, r, id, rest[2], model.RealmManager, actor)
		return
	}
	api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
}

// webUserRoutes 处理 web 用户（users 表）的增删改查与重置密码。
func (s *Server) webUserRoutes(w http.ResponseWriter, r *http.Request, rest []string) {
	actor := CurrentUser(r)
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			if !s.checkPermission(w, r, "user:read") {
				return
			}
			s.listUsers(w, r, &[]model.User{}, "users")
		case http.MethodPost:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.createWebUser(w, r, actor)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	id := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodPatch:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.updateWebUser(w, r, id, actor)
		case http.MethodDelete:
			if !s.checkPermission(w, r, "user:write") {
				return
			}
			s.deleteUser(w, r, "users", id, "WEB_USER_DELETE", actor)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	if len(rest) == 2 && rest[1] == "reset-password" && r.Method == http.MethodPost {
		if !s.checkPermission(w, r, "user:write") {
			return
		}
		s.resetPassword(w, r, "users", id, "WEB_USER_RESET_PASSWORD", actor)
		return
	}
	if len(rest) == 2 && rest[1] == "tokens" && r.Method == http.MethodGet {
		if !s.checkPermission(w, r, "user:token") {
			return
		}
		s.listUserTokens(w, r, id, model.RealmWeb)
		return
	}
	if len(rest) == 3 && rest[1] == "tokens" && r.Method == http.MethodDelete {
		if !s.checkPermission(w, r, "user:token") {
			return
		}
		s.adminRevokeToken(w, r, id, rest[2], model.RealmWeb, actor)
		return
	}
	api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
}

// listUsers 分页列出指定表的未删除用户，支持 q（用户名/姓名/邮箱模糊）与 status 过滤。
func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, dest any, table string) {
	page, pageSize := pageParams(r, s.cfg.Limits.PageSizeMax)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	status := r.URL.Query().Get("status")
	apply := func(db *gorm.DB) *gorm.DB {
		db = db.Table(table).Where("deleted_at IS NULL")
		if q != "" {
			like := "%" + q + "%"
			db = db.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?", like, like, like)
		}
		if status != "" {
			db = db.Where("status = ?", status)
		}
		return db
	}
	var total int64
	if err := apply(s.db.Session(&gorm.Session{})).Count(&total).Error; err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	if err := apply(s.db.Session(&gorm.Session{})).Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(dest).Error; err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	api.WriteJSON(w, r, http.StatusOK, map[string]any{"items": dest, "page": page, "page_size": pageSize, "total": total, "has_more": int64(page*pageSize) < total})
}

func (s *Server) createManagerUser(w http.ResponseWriter, r *http.Request, actor *model.Principal) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if req.Username == "" || req.Password == "" {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("username and password are required"))
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	role := req.Role
	if role == "" {
		role = model.RoleAdmin
	}
	mu := model.ManagerUser{Username: req.Username, Email: model.StrPtr(req.Email), DisplayName: req.DisplayName, PasswordHash: hash, Status: "ACTIVE", Source: model.SourceLocal, Role: role}
	if err := s.db.Create(&mu).Error; err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	s.recordAudit(r, actor, "MANAGER_USER_CREATE", "manager_user", mu.ID, "SUCCESS", "create local manager user "+req.Username)
	mu.PasswordHash = ""
	api.WriteJSON(w, r, http.StatusCreated, mu)
}

func (s *Server) createWebUser(w http.ResponseWriter, r *http.Request, actor *model.Principal) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if req.Username == "" || req.Password == "" {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("username and password are required"))
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	u := model.User{Username: req.Username, Email: model.StrPtr(req.Email), DisplayName: req.DisplayName, PasswordHash: hash, Status: "ACTIVE", Source: model.SourceLocal}
	if err := s.db.Create(&u).Error; err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	s.recordAudit(r, actor, "WEB_USER_CREATE", "web_user", u.ID, "SUCCESS", "create local web user "+req.Username)
	u.PasswordHash = ""
	api.WriteJSON(w, r, http.StatusCreated, u)
}

func (s *Server) updateManagerUser(w http.ResponseWriter, r *http.Request, id string, actor *model.Principal) {
	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
		Status      *string `json:"status"`
		Role        *string `json:"role"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	updates := map[string]any{}
	if req.Email != nil {
		updates["email"] = model.StrPtr(*req.Email)
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if len(updates) == 0 {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("no fields to update"))
		return
	}
	if err := s.db.Model(&model.ManagerUser{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "MANAGER_USER_UPDATE", "manager_user", id, "SUCCESS", "update manager user")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"updated": true})
}

func (s *Server) updateWebUser(w http.ResponseWriter, r *http.Request, id string, actor *model.Principal) {
	var req struct {
		Email       *string `json:"email"`
		DisplayName *string `json:"display_name"`
		Status      *string `json:"status"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	updates := map[string]any{}
	if req.Email != nil {
		updates["email"] = model.StrPtr(*req.Email)
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if len(updates) == 0 {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("no fields to update"))
		return
	}
	if err := s.db.Model(&model.User{}).Where("id = ? AND deleted_at IS NULL", id).Updates(updates).Error; err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "WEB_USER_UPDATE", "web_user", id, "SUCCESS", "update web user")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"updated": true})
}

// deleteUser 软删除指定表的用户（设置 deleted_at）。
func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request, table, id, action string, actor *model.Principal) {
	now := time.Now().UTC()
	res := s.db.Table(table).Where("id = ? AND deleted_at IS NULL", id).Update("deleted_at", &now)
	if res.Error != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", res.Error)
		return
	}
	if res.RowsAffected == 0 {
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("user not found"))
		return
	}
	s.recordAudit(r, actor, action, "user", id, "SUCCESS", "delete user")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"deleted": true})
}

// resetPassword 重置本地来源用户密码；LDAP 来源用户返回 400（凭据由目录管理）。
func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request, table, id, action string, actor *model.Principal) {
	var req struct {
		Password string `json:"password"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if req.Password == "" {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("password is required"))
		return
	}
	var found struct{ Source string }
	if err := s.db.Table(table).Select("source").Where("id = ? AND deleted_at IS NULL", id).Take(&found).Error; err != nil {
		api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("user not found"))
		return
	}
	if found.Source == model.SourceLDAP {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", errors.New("cannot reset password for ldap user"))
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	if err := s.db.Table(table).Where("id = ? AND deleted_at IS NULL", id).Update("password_hash", hash).Error; err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, action, "user", id, "SUCCESS", "reset password")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"reset": true})
}

func (s *Server) ldapSync(w http.ResponseWriter, r *http.Request) {
	if !s.checkPermission(w, r, "ldap:sync") {
		return
	}
	actor := CurrentUser(r)
	stats, err := s.syncer.Sync(r.Context())
	if err != nil {
		s.recordAudit(r, actor, "LDAP_SYNC", "ldap", "", "FAILURE", err.Error())
		api.WriteError(w, r, http.StatusInternalServerError, "LDAP_SYNC_FAILED", err)
		return
	}
	s.recordAudit(r, actor, "LDAP_SYNC", "ldap", "", "SUCCESS", "sync completed")
	api.WriteJSON(w, r, http.StatusOK, stats)
}

func (s *Server) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
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
}

func (s *Server) listGenerationRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		return
	}
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
}

// requirePermission 校验当前 manager 用户是否拥有指定权限，否则 403。
func (s *Server) requirePermission(r *http.Request, code string) bool {
	p := CurrentUser(r)
	if p == nil || p.Realm != model.RealmManager {
		return false
	}
	return s.rbacSvc.HasPermission(p.ID, code)
}

// checkPermission 校验权限，不通过时写 403 并返回 false。
func (s *Server) checkPermission(w http.ResponseWriter, r *http.Request, code string) bool {
	if !s.requirePermission(r, code) {
		api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
		return false
	}
	return true
}

// roleRoutes 处理角色集合（列表/新建）与单项（详情/更新/删除/权限/成员）。
func (s *Server) roleRoutes(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			if !s.requirePermission(r, "role:read") {
				api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
				return
			}
			roles, err := s.rbacSvc.ListRoles()
			writeRegistryResult(w, r, roles, err)
		case http.MethodPost:
			if !s.requirePermission(r, "role:write") {
				api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
				return
			}
			s.createRole(w, r)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	id := rest[0]
	if len(rest) == 1 {
		switch r.Method {
		case http.MethodGet:
			if !s.requirePermission(r, "role:read") {
				api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
				return
			}
			detail, err := s.rbacSvc.GetRole(id)
			writeRegistryResult(w, r, detail, err)
		case http.MethodPut:
			if !s.requirePermission(r, "role:write") {
				api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
				return
			}
			s.updateRole(w, r, id)
		case http.MethodDelete:
			if !s.requirePermission(r, "role:write") {
				api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
				return
			}
			s.deleteRole(w, r, id)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	if len(rest) == 2 && rest[1] == "permissions" && r.Method == http.MethodPut {
		if !s.requirePermission(r, "role:write") {
			api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
			return
		}
		s.setRolePermissions(w, r, id)
		return
	}
	if len(rest) == 2 && rest[1] == "members" && r.Method == http.MethodGet {
		if !s.requirePermission(r, "role:read") {
			api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
			return
		}
		members, err := s.rbacSvc.ListRoleMembers(id)
		writeRegistryResult(w, r, members, err)
		return
	}
	if len(rest) == 2 && rest[1] == "members" && r.Method == http.MethodPut {
		if !s.requirePermission(r, "role:write") {
			api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
			return
		}
		s.setRoleMembers(w, r, id)
		return
	}
	api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
}

func (s *Server) listPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := s.rbacSvc.ListPermissions()
	writeRegistryResult(w, r, perms, err)
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	actor := CurrentUser(r)
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     bool   `json:"enabled"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	role, err := s.rbacSvc.CreateRole(req.Code, req.Name, req.Description, req.Enabled)
	if err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	s.recordAudit(r, actor, "ROLE_CREATE", "role", role.ID, "SUCCESS", "create role "+req.Code)
	api.WriteJSON(w, r, http.StatusCreated, role)
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request, id string) {
	actor := CurrentUser(r)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if err := s.rbacSvc.UpdateRole(id, req.Name, req.Description, req.Enabled); err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "ROLE_UPDATE", "role", id, "SUCCESS", "update role")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"updated": true})
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request, id string) {
	actor := CurrentUser(r)
	if err := s.rbacSvc.DeleteRole(id); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	s.recordAudit(r, actor, "ROLE_DELETE", "role", id, "SUCCESS", "delete role")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) setRolePermissions(w http.ResponseWriter, r *http.Request, id string) {
	actor := CurrentUser(r)
	var req struct {
		PermissionIDs []string `json:"permission_ids"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if err := s.rbacSvc.SetRolePermissions(id, req.PermissionIDs); err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "ROLE_PERMISSIONS", "role", id, "SUCCESS", "set role permissions")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"updated": true})
}

func (s *Server) setRoleMembers(w http.ResponseWriter, r *http.Request, id string) {
	actor := CurrentUser(r)
	var req struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	if err := s.rbacSvc.SetRoleMembers(id, req.UserIDs); err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "ROLE_MEMBERS", "role", id, "SUCCESS", "set role members")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"updated": true})
}

// recordAudit 记录一条审计日志。失败只记日志，不影响业务流程。
func (s *Server) recordAudit(r *http.Request, actor *model.Principal, action, resourceType, resourceID, result, message string) {
	log := model.AuditLog{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IPAddress:    r.RemoteAddr,
		UserAgent:    r.UserAgent(),
		Result:       result,
		Message:      message,
	}
	if actor != nil {
		log.ActorUserID = actor.ID
	}
	if err := s.db.Create(&log).Error; err != nil {
		s.logger.Warn("record audit failed", "error", err, "action", action)
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

func publicUser(p *model.Principal) map[string]any {
	if p == nil {
		return nil
	}
	return map[string]any{
		"id":            p.ID,
		"username":      p.Username,
		"email":         p.Email,
		"display_name":  p.DisplayName,
		"status":        p.Status,
		"source":        p.Source,
		"last_login_at": p.LastLoginAt,
	}
}

func publicManagerUser(p *model.Principal) map[string]any {
	if p == nil {
		return nil
	}
	return map[string]any{
		"id":            p.ID,
		"username":      p.Username,
		"email":         p.Email,
		"display_name":  p.DisplayName,
		"status":        p.Status,
		"source":        p.Source,
		"role":          p.Role,
		"realm":         p.Realm,
		"last_login_at": p.LastLoginAt,
	}
}

// tokenRoutes 处理当前 web 用户的个人访问令牌（含登录令牌）的创建、列表与吊销。
func (s *Server) tokenRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/registry/tokens")
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		switch r.Method {
		case http.MethodPost:
			s.createToken(w, r)
		case http.MethodGet:
			s.listTokens(w, r)
		default:
			api.WriteError(w, r, http.StatusMethodNotAllowed, "INVALID_REQUEST", errors.New("method not allowed"))
		}
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.revokeToken(w, r, parts[0])
		return
	}
	api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", errors.New("not found"))
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	var req struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := api.DecodeJSON(r, &req); err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	plaintext, access, err := s.auth.CreatePAT(user.ID, model.RealmWeb, req.Name, req.ExpiresAt)
	if err != nil {
		api.WriteError(w, r, http.StatusBadRequest, "INVALID_REQUEST", err)
		return
	}
	s.recordAudit(r, user, "PAT_CREATE", "access_token", access.ID, "SUCCESS", "create personal access token "+req.Name)
	api.WriteJSON(w, r, http.StatusCreated, map[string]any{"access_token": plaintext, "token": publicTokenView(access)})
}

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	user := CurrentUser(r)
	items, err := s.auth.ListPATs(user.ID, model.RealmWeb)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		out = append(out, publicTokenView(t))
	}
	api.WriteJSON(w, r, http.StatusOK, out)
}

func (s *Server) revokeToken(w http.ResponseWriter, r *http.Request, id string) {
	user := CurrentUser(r)
	if err := s.auth.RevokePAT(user.ID, id); err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", err)
			return
		}
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, user, "PAT_REVOKE", "access_token", id, "SUCCESS", "revoke personal access token")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"revoked": true})
}

// listUserTokens 管理端列出指定用户的令牌（web 用户=WEB 域，管理端用户=MANAGER 域）。
func (s *Server) listUserTokens(w http.ResponseWriter, r *http.Request, userID, realm string) {
	items, err := s.auth.ListPATs(userID, realm)
	if err != nil {
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		out = append(out, publicTokenView(t))
	}
	api.WriteJSON(w, r, http.StatusOK, out)
}

// adminRevokeToken 管理端吊销指定用户的令牌（按目标 user_id 限定）。
func (s *Server) adminRevokeToken(w http.ResponseWriter, r *http.Request, userID, tokenID, realm string, actor *model.Principal) {
	if err := s.auth.RevokePAT(userID, tokenID); err != nil {
		if errors.Is(err, auth.ErrTokenNotFound) {
			api.WriteError(w, r, http.StatusNotFound, "INVALID_REQUEST", err)
			return
		}
		api.WriteError(w, r, http.StatusInternalServerError, "SERVER_ERROR", err)
		return
	}
	s.recordAudit(r, actor, "ADMIN_TOKEN_REVOKE", "access_token", tokenID, "SUCCESS", "admin revoke user token")
	api.WriteJSON(w, r, http.StatusOK, map[string]bool{"revoked": true})
}

// publicTokenView 返回令牌的安全视图（不含摘要与明文）。
func publicTokenView(t model.AccessToken) map[string]any {
	return map[string]any{
		"id":           t.ID,
		"name":         t.Name,
		"scopes":       t.Scopes,
		"created_at":   t.CreatedAt,
		"last_used_at": t.LastUsedAt,
		"expires_at":   t.ExpiresAt,
	}
}
