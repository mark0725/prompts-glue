package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"hub-server/internal/config"
	"hub-server/internal/model"
)

// LDAPVerifier 校验 LDAP 来源用户的凭据。由 internal/ldap.Service 实现，便于测试注入桩实现。
type LDAPVerifier interface {
	BindVerify(ctx context.Context, dn, password string) error
}

// RoleChecker 检查用户是否具备管理端登录所需的角色，用于管理端登录门禁。由 internal/rbac.Service 实现。
type RoleChecker interface {
	HasAnyRole(userID string) bool
}

// ErrRoleRequired 表示管理端登录被拒：用户不属于任何角色（不在 manager 角色列表中）。
var ErrRoleRequired = errors.New("manager login requires at least one role")

type Service struct {
	db   *gorm.DB
	cfg  config.Config
	ldap LDAPVerifier
	rbac RoleChecker
}

func NewService(db *gorm.DB, cfg config.Config, verifier LDAPVerifier, rbac RoleChecker) *Service {
	return &Service{db: db, cfg: cfg, ldap: verifier, rbac: rbac}
}

// EnsureAdmin 初始化管理端引导账号到 manager_users（本地来源、ADMIN 角色）。
// 已存在同名账号时跳过。
func (s *Service) EnsureAdmin() error {
	if s.cfg.Auth.AdminPassword == "" {
		return nil
	}
	var count int64
	if err := s.db.Model(&model.ManagerUser{}).Where("username = ?", s.cfg.Auth.AdminUsername).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.Auth.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&model.ManagerUser{
		Username:     s.cfg.Auth.AdminUsername,
		DisplayName:  s.cfg.Auth.AdminUsername,
		PasswordHash: string(hash),
		Status:       "ACTIVE",
		Source:       model.SourceLocal,
		Role:         model.RoleAdmin,
	}).Error
}

// loginTokenName 按登录客户端返回令牌名称，使登录令牌在个人中心/管理端的令牌列表中可识别。
func loginTokenName(client string) string {
	switch client {
	case "cli":
		return "Cli登录创建"
	case "web":
		return "Web登录创建"
	case "manager":
		return "Manager登录创建"
	default:
		return "登录创建"
	}
}

// WebLogin 校验 web 域用户并签发 realm=WEB 的访问令牌（不过期）。client 标识登录来源，用于令牌命名。
func (s *Service) WebLogin(ctx context.Context, username, password, client string) (string, *model.Principal, error) {
	var u model.User
	if err := s.db.Where("username = ? OR email = ?", username, username).First(&u).Error; err != nil {
		return "", nil, errors.New("invalid credentials")
	}
	if u.Status != "ACTIVE" {
		return "", nil, errors.New("user is not active")
	}
	if err := s.verifyPassword(ctx, u.Source, u.ExternalID, u.PasswordHash, password); err != nil {
		return "", nil, errors.New("invalid credentials")
	}
	token, err := s.issueToken(u.ID, model.RealmWeb, loginTokenName(client))
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	_ = s.db.Model(&model.User{}).Where("id = ?", u.ID).Update("last_login_at", &now).Error
	u.LastLoginAt = &now
	return token, model.PrincipalFromUser(u), nil
}

// ManagerLogin 校验 manager 域用户并签发 realm=MANAGER 的访问令牌（不过期）。
func (s *Service) ManagerLogin(ctx context.Context, username, password, client string) (string, *model.Principal, error) {
	var mu model.ManagerUser
	if err := s.db.Where("username = ? OR email = ?", username, username).First(&mu).Error; err != nil {
		return "", nil, errors.New("invalid credentials")
	}
	if mu.Status != "ACTIVE" {
		return "", nil, errors.New("user is not active")
	}
	if err := s.verifyPassword(ctx, mu.Source, mu.ExternalID, mu.PasswordHash, password); err != nil {
		return "", nil, errors.New("invalid credentials")
	}
	// 管理端登录门禁：用户必须属于至少一个角色（不在 manager 角色列表中的用户不能登录）。
	if s.rbac != nil && !s.rbac.HasAnyRole(mu.ID) {
		return "", nil, ErrRoleRequired
	}
	token, err := s.issueToken(mu.ID, model.RealmManager, loginTokenName(client))
	if err != nil {
		return "", nil, err
	}
	now := time.Now().UTC()
	_ = s.db.Model(&model.ManagerUser{}).Where("id = ?", mu.ID).Update("last_login_at", &now).Error
	mu.LastLoginAt = &now
	return token, model.PrincipalFromManagerUser(mu), nil
}

// verifyPassword 按来源分派凭据校验：LOCAL 用本地 bcrypt 摘要；LDAP 用 LDAP bind；
// WEB 为预留的自注册来源，当前不可登录。
func (s *Service) verifyPassword(ctx context.Context, source, externalID, passwordHash, password string) error {
	switch source {
	case model.SourceLDAP:
		if s.ldap == nil {
			return errors.New("ldap verifier unavailable")
		}
		return s.ldap.BindVerify(ctx, externalID, password)
	case model.SourceWeb:
		return errors.New("login not supported for web-registered account")
	default:
		if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
			return errors.New("invalid credentials")
		}
		return nil
	}
}

// ErrTokenNotFound 表示令牌不存在或不属于该用户。
var ErrTokenNotFound = errors.New("token not found")

// issueToken 生成登录访问令牌并落库（只保存摘要，携带 realm，登录令牌不过期）。
func (s *Service) issueToken(userID, realm, name string) (string, error) {
	token, tokenHash, err := newToken()
	if err != nil {
		return "", err
	}
	access := model.AccessToken{UserID: userID, Realm: realm, TokenHash: tokenHash, Name: name}
	if err := s.db.Create(&access).Error; err != nil {
		return "", err
	}
	return token, nil
}

// CreatePAT 创建个人访问令牌（默认不过期，可指定 expiresAt）。返回明文（仅此一次）与记录。
func (s *Service) CreatePAT(userID, realm, name string, expiresAt *time.Time) (string, model.AccessToken, error) {
	if name == "" {
		return "", model.AccessToken{}, errors.New("token name is required")
	}
	if expiresAt != nil && expiresAt.Before(time.Now().UTC()) {
		return "", model.AccessToken{}, errors.New("expires_at must be in the future")
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return "", model.AccessToken{}, err
	}
	access := model.AccessToken{UserID: userID, Realm: realm, TokenHash: tokenHash, Name: name, ExpiresAt: expiresAt}
	if err := s.db.Create(&access).Error; err != nil {
		return "", model.AccessToken{}, err
	}
	return token, access, nil
}

// ListPATs 列出指定用户某域的未吊销且未过期的令牌（不含令牌摘要与明文）。
func (s *Service) ListPATs(userID, realm string) ([]model.AccessToken, error) {
	var items []model.AccessToken
	if err := s.db.Where("user_id = ? AND realm = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at >= ?)", userID, realm, time.Now().UTC()).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// RevokePAT 吊销指定用户的令牌。按 user_id 限定，确保用户只能吊销自己的令牌；不存在则返回 ErrTokenNotFound。
func (s *Service) RevokePAT(userID, id string) error {
	now := time.Now().UTC()
	res := s.db.Model(&model.AccessToken{}).Where("id = ? AND user_id = ? AND revoked_at IS NULL", id, userID).Update("revoked_at", &now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// Authenticate 校验访问令牌，按令牌 realm 回查对应表并返回统一身份 Principal。
func (s *Service) Authenticate(token string) (*model.Principal, error) {
	if token == "" {
		return nil, errors.New("missing token")
	}
	tokenHash := HashToken(token)
	var access model.AccessToken
	if err := s.db.Where("token_hash = ? AND revoked_at IS NULL", tokenHash).First(&access).Error; err != nil {
		return nil, errors.New("invalid token")
	}
	if access.ExpiresAt != nil && access.ExpiresAt.Before(time.Now().UTC()) {
		return nil, errors.New("token expired")
	}
	var principal *model.Principal
	switch access.Realm {
	case model.RealmWeb:
		var u model.User
		if err := s.db.First(&u, "id = ?", access.UserID).Error; err != nil {
			return nil, errors.New("invalid token user")
		}
		principal = model.PrincipalFromUser(u)
	case model.RealmManager:
		var mu model.ManagerUser
		if err := s.db.First(&mu, "id = ?", access.UserID).Error; err != nil {
			return nil, errors.New("invalid token user")
		}
		principal = model.PrincipalFromManagerUser(mu)
	default:
		return nil, errors.New("invalid token realm")
	}
	if principal.Status != "ACTIVE" {
		return nil, errors.New("user is not active")
	}
	now := time.Now().UTC()
	_ = s.db.Model(&model.AccessToken{}).Where("id = ?", access.ID).Update("last_used_at", &now).Error
	return principal, nil
}

// HashPassword 返回密码的 bcrypt 摘要，供引导账号与管理端创建/重置密码使用。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// HashToken 返回访问令牌的 sha256 摘要，落库时不保存明文令牌。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, HashToken(token), nil
}
