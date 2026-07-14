package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"hub-server/internal/api"
	"hub-server/internal/model"
)

type contextKey string

const userKey contextKey = "user"

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", requestID)
		next.ServeHTTP(w, api.WithRequestID(r, requestID))
	})
}

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("request completed", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func bodyLimitMiddleware(limit int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limit > 0 {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// requireRealm 要求请求携带有效令牌且其 realm 与入参一致，否则 401（缺令牌/无效）或 403（域不匹配）。
// 用于管理 API：仅 MANAGER 域令牌可访问。
func (s *Server) requireRealm(realm string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.authenticate(r)
		if err != nil || p == nil {
			api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", errUnauthorized)
			return
		}
		if p.Realm != realm {
			api.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", errForbidden)
			return
		}
		next.ServeHTTP(w, withUser(r, p))
	})
}

// requireAuth 要求请求携带任意域的有效令牌（WEB 或 MANAGER），用于 registry 鉴权路由。
// manager 界面需浏览/管理 registry 资源，故 registry 不按域隔离，仅管理 API 限 MANAGER。
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := s.authenticate(r)
		if err != nil || p == nil {
			api.WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", errUnauthorized)
			return
		}
		next.ServeHTTP(w, withUser(r, p))
	})
}

// optionalAuth 允许匿名访问；若携带令牌则须为任意域的有效令牌，无效令牌按匿名处理。
func (s *Server) optionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token := bearerToken(r); token != "" {
			if p, err := s.auth.Authenticate(token); err == nil {
				next.ServeHTTP(w, withUser(r, p))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// authenticate 解析并校验令牌，返回当前身份。无令牌时返回 (nil, nil)。
func (s *Server) authenticate(r *http.Request) (*model.Principal, error) {
	token := bearerToken(r)
	if token == "" {
		return nil, nil
	}
	return s.auth.Authenticate(token)
}

func withUser(r *http.Request, p *model.Principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, p))
}

// CurrentUser 返回当前请求已认证的身份，未认证返回 nil。
func CurrentUser(r *http.Request) *model.Principal {
	if value := r.Context().Value(userKey); value != nil {
		if p, ok := value.(*model.Principal); ok {
			return p
		}
	}
	return nil
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
