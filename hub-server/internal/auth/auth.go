package auth

import (
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

const (
	RoleAdmin = "ADMIN"
	RoleUser  = "USER"
)

type Service struct {
	db  *gorm.DB
	cfg config.Config
}

func NewService(db *gorm.DB, cfg config.Config) *Service {
	return &Service{db: db, cfg: cfg}
}

func (s *Service) EnsureAdmin() error {
	if s.cfg.Auth.AdminPassword == "" {
		return nil
	}
	var count int64
	if err := s.db.Model(&model.User{}).Where("username = ?", s.cfg.Auth.AdminUsername).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.cfg.Auth.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Create(&model.User{Username: s.cfg.Auth.AdminUsername, DisplayName: s.cfg.Auth.AdminUsername, PasswordHash: string(hash), Status: "ACTIVE", Role: RoleAdmin}).Error
}

func (s *Service) Login(username string, password string) (string, time.Time, model.User, error) {
	var user model.User
	if err := s.db.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
		return "", time.Time{}, model.User{}, errors.New("invalid credentials")
	}
	if user.Status != "ACTIVE" {
		return "", time.Time{}, model.User{}, errors.New("user is not active")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", time.Time{}, model.User{}, errors.New("invalid credentials")
	}
	token, tokenHash, err := newToken()
	if err != nil {
		return "", time.Time{}, model.User{}, err
	}
	expires := time.Now().UTC().Add(s.cfg.Auth.TokenTTL)
	access := model.AccessToken{UserID: user.ID, TokenHash: tokenHash, Name: "login", ExpiresAt: &expires}
	if err := s.db.Create(&access).Error; err != nil {
		return "", time.Time{}, model.User{}, err
	}
	now := time.Now().UTC()
	_ = s.db.Model(&user).Updates(map[string]any{"last_login_at": &now}).Error
	user.LastLoginAt = &now
	return token, expires, user, nil
}

func (s *Service) Authenticate(token string) (*model.User, error) {
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
	var user model.User
	if err := s.db.First(&user, "id = ?", access.UserID).Error; err != nil {
		return nil, errors.New("invalid token user")
	}
	if user.Status != "ACTIVE" {
		return nil, errors.New("user is not active")
	}
	now := time.Now().UTC()
	_ = s.db.Model(&access).Update("last_used_at", &now).Error
	return &user, nil
}

func IsAdmin(user *model.User) bool {
	return user != nil && user.Role == RoleAdmin
}

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
