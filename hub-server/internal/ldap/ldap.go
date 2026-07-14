// Package ldap 提供基于 LDAP 目录的用户同步与登录验证能力。
//
// 同步：以服务账号绑定后按 search_dn 与 user_filter 检索用户，按 external_id（DN）
// 幂等 upsert 到 sync_targets 指定的表（manager_users 和/或 users）。默认仅同步到
// manager_users；配置加入 "web" 时也同步到 web 用户表。显示名取 display_name_attribute
// （默认 givenName），缺失时回退到 attribute（默认 uid）。
//
// 登录验证：LDAP 来源用户登录时，以其 external_id（DN）+ 密码发起 LDAP bind，
// 成功即视为凭据有效，不使用本地密码摘要。
package ldap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"gorm.io/gorm"

	"hub-server/internal/config"
	"hub-server/internal/model"
)

// ErrDisabled 表示 LDAP 未启用。BindVerify 在此情况下无法验证 LDAP 来源用户凭据。
var ErrDisabled = errors.New("ldap is disabled")

// SyncStats 记录一次同步的统计结果。
type SyncStats struct {
	SyncedManager int      `json:"synced_manager"`
	SyncedWeb     int      `json:"synced_web"`
	Errors        []string `json:"errors"`
}

// Service 封装 LDAP 连接、用户同步与登录验证。
type Service struct {
	cfg config.LDAPConfig
	db  *gorm.DB
	log *slog.Logger
}

// NewService 构造 LDAP 服务。即使 enable=false 也允许构造，调用方按需调用 Sync/BindVerify。
func NewService(cfg config.LDAPConfig, db *gorm.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{cfg: cfg, db: db, log: log}
}

// Enabled 返回 LDAP 是否启用。
func (s *Service) Enabled() bool { return s.cfg.Enable }

// Sync 执行一次用户同步。enable=false 时直接返回空统计。每个目标表各自一个事务，
// 单条记录失败只记录到 stats.Errors 并继续，不影响其余记录。
func (s *Service) Sync(ctx context.Context) (SyncStats, error) {
	stats := SyncStats{}
	if !s.cfg.Enable {
		s.log.Debug("ldap sync skipped: disabled")
		return stats, nil
	}
	if s.cfg.Host == "" || s.cfg.SearchDN == "" {
		return stats, errors.New("ldap host or search_dn is empty")
	}

	conn, err := s.connect()
	if err != nil {
		return stats, fmt.Errorf("connect ldap: %w", err)
	}
	defer conn.Close()
	if err := conn.Bind(s.cfg.BindDN, s.cfg.BindPassword); err != nil {
		return stats, fmt.Errorf("bind service account: %w", err)
	}

	entries, err := s.search(conn)
	if err != nil {
		return stats, fmt.Errorf("search users: %w", err)
	}

	hasManager, hasWeb := s.targets()
	if !hasManager && !hasWeb {
		hasManager = true // 默认同步到 manager
	}
	s.log.Info("ldap sync starting", "sync_targets", s.cfg.SyncTargets, "manager", hasManager, "web", hasWeb, "entries", len(entries))

	for _, e := range entries {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}
		parsed, ok := parseEntry(e, s.cfg.Attribute, s.cfg.DisplayNameAttribute)
		if !ok {
			stats.Errors = append(stats.Errors, fmt.Sprintf("skip entry without %q: dn=%s", s.cfg.Attribute, e.DN))
			continue
		}
		if hasManager {
			if perr := s.upsertManager(parsed); perr != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("manager upsert %s: %v", parsed.DN, perr))
				continue
			}
			stats.SyncedManager++
		}
		if hasWeb {
			if perr := s.upsertWeb(parsed); perr != nil {
				stats.Errors = append(stats.Errors, fmt.Sprintf("web upsert %s: %v", parsed.DN, perr))
				continue
			}
			stats.SyncedWeb++
		}
	}
	s.log.Info("ldap sync completed", "manager", stats.SyncedManager, "web", stats.SyncedWeb, "errors", len(stats.Errors))
	return stats, nil
}

// BindVerify 以用户 DN + 密码发起一次 LDAP bind，验证凭据。成功返回 nil。
// enable=false 时返回 ErrDisabled。
func (s *Service) BindVerify(ctx context.Context, dn, password string) error {
	if !s.cfg.Enable {
		return ErrDisabled
	}
	conn, err := s.connect()
	if err != nil {
		return fmt.Errorf("connect ldap: %w", err)
	}
	defer conn.Close()
	if err := conn.Bind(dn, password); err != nil {
		return err
	}
	return nil
}

// connect 建立到 LDAP 的连接，按配置应用 LDAPS / StartTLS / 主机校验。不执行 bind。
func (s *Service) connect() (*ldap.Conn, error) {
	scheme := "ldap"
	if s.cfg.LDAPS {
		scheme = "ldaps"
	}
	addr := fmt.Sprintf("%s://%s:%d", scheme, s.cfg.Host, s.cfg.Port)
	tlsCfg := &tls.Config{ServerName: s.cfg.Host, InsecureSkipVerify: !s.cfg.VerifyHost}
	timeout := time.Duration(s.cfg.Timeout)
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout}
	var opts []ldap.DialOpt
	if s.cfg.LDAPS {
		opts = append(opts, ldap.DialWithTLSConfig(tlsCfg))
	}
	opts = append(opts, ldap.DialWithDialer(dialer))
	conn, err := ldap.DialURL(addr, opts...)
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(timeout)
	if s.cfg.StartTLS && !s.cfg.LDAPS {
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func (s *Service) search(conn *ldap.Conn) ([]*ldap.Entry, error) {
	filter := s.cfg.UserFilter
	if strings.TrimSpace(filter) == "" {
		filter = "(objectClass=person)"
	}
	attrs := []string{s.cfg.Attribute, s.cfg.DisplayNameAttribute, "mail"}
	req := ldap.NewSearchRequest(
		s.cfg.SearchDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		attrs,
		nil,
	)
	res, err := conn.Search(req)
	if err != nil {
		return nil, err
	}
	return res.Entries, nil
}

// targets 解析 sync_targets 配置。
func (s *Service) targets() (manager, web bool) {
	for _, t := range s.cfg.SyncTargets {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "manager":
			manager = true
		case "web":
			web = true
		}
	}
	return manager, web
}

type ldapEntry struct {
	DN          string
	Username    string
	DisplayName string
	Email       string
}

// parseEntry 从 LDAP 条目解析出用户字段。username 为空时返回 ok=false。
// 显示名取 displayAttr，为空时回退到 username（即 attr，默认 uid）。
func parseEntry(entry *ldap.Entry, attr, displayAttr string) (ldapEntry, bool) {
	if entry == nil {
		return ldapEntry{}, false
	}
	username := entry.GetAttributeValue(attr)
	if username == "" {
		return ldapEntry{}, false
	}
	display := entry.GetAttributeValue(displayAttr)
	if display == "" {
		display = username
	}
	return ldapEntry{
		DN:          entry.DN,
		Username:    username,
		DisplayName: display,
		Email:       entry.GetAttributeValue("mail"),
	}, true
}

func (s *Service) upsertManager(e ldapEntry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var mu model.ManagerUser
		err := tx.Where("source = ? AND external_id = ?", model.SourceLDAP, e.DN).First(&mu).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&model.ManagerUser{
				Username: e.Username, DisplayName: e.DisplayName, Email: model.StrPtr(e.Email),
				Source: model.SourceLDAP, ExternalID: e.DN, Status: "ACTIVE", Role: model.RoleAdmin,
			}).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&mu).Updates(map[string]any{
			"username": e.Username, "display_name": e.DisplayName, "email": model.StrPtr(e.Email), "status": "ACTIVE",
		}).Error
	})
}

func (s *Service) upsertWeb(e ldapEntry) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var u model.User
		err := tx.Where("source = ? AND external_id = ?", model.SourceLDAP, e.DN).First(&u).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&model.User{
				Username: e.Username, DisplayName: e.DisplayName, Email: model.StrPtr(e.Email),
				Source: model.SourceLDAP, ExternalID: e.DN, Status: "ACTIVE",
			}).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&u).Updates(map[string]any{
			"username": e.Username, "display_name": e.DisplayName, "email": model.StrPtr(e.Email), "status": "ACTIVE",
		}).Error
	})
}
