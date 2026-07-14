package auth

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/model"
)

type stubVerifier struct {
	failDN string
	err    error
}

func (s *stubVerifier) BindVerify(ctx context.Context, dn, password string) error {
	if s.err != nil {
		return s.err
	}
	if dn == s.failDN {
		return errors.New("invalid credentials")
	}
	return nil
}

func newAuthDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	database.BeforeCreate(db)
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// LOCAL 来源走本地 bcrypt 校验。
func TestManagerLoginLocalSource(t *testing.T) {
	db := newAuthDB(t)
	cfg := config.Default()
	svc := NewService(db, cfg, &stubVerifier{})

	hash, _ := HashPassword("pass123")
	mu := model.ManagerUser{Username: "ops", PasswordHash: hash, Status: "ACTIVE", Source: model.SourceLocal, Role: model.RoleAdmin}
	if err := db.Create(&mu).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ManagerLogin(context.Background(), "ops", "wrong", "test"); err == nil {
		t.Fatal("expected error for wrong password")
	}
	if _, p, err := svc.ManagerLogin(context.Background(), "ops", "pass123", "test"); err != nil {
		t.Fatalf("expected login success: %v", err)
	} else if p.Realm != model.RealmManager || p.Role != model.RoleAdmin {
		t.Fatalf("unexpected principal: %+v", p)
	}
}

// LDAP 来源走 BindVerify；验证成功与失败两条路径。
func TestManagerLoginLDAPSource(t *testing.T) {
	db := newAuthDB(t)
	cfg := config.Default()
	verifier := &stubVerifier{failDN: "uid=blocked,dc=test,dc=com"}
	svc := NewService(db, cfg, verifier)

	mu := model.ManagerUser{Username: "ldapu", Source: model.SourceLDAP, ExternalID: "uid=ldapu,dc=test,dc=com", Status: "ACTIVE", Role: model.RoleAdmin}
	if err := db.Create(&mu).Error; err != nil {
		t.Fatal(err)
	}
	blocked := model.ManagerUser{Username: "ldapa", Source: model.SourceLDAP, ExternalID: "uid=blocked,dc=test,dc=com", Status: "ACTIVE", Role: model.RoleAdmin}
	if err := db.Create(&blocked).Error; err != nil {
		t.Fatal(err)
	}
	// 正常 LDAP 用户：BindVerify 成功 → 登录成功。
	if _, p, err := svc.ManagerLogin(context.Background(), "ldapu", "anything", "test"); err != nil {
		t.Fatalf("expected ldap login success: %v", err)
	} else if p.Source != model.SourceLDAP {
		t.Fatalf("expected source LDAP, got %q", p.Source)
	}
	// 被拒绝的 DN：BindVerify 失败 → 登录失败。
	if _, _, err := svc.ManagerLogin(context.Background(), "ldapa", "x", "test"); err == nil {
		t.Fatal("expected login failure when bind verify fails")
	}
}

// Authenticate 按令牌 realm 回查对应表。
func TestAuthenticateRealmDispatch(t *testing.T) {
	db := newAuthDB(t)
	cfg := config.Default()
	svc := NewService(db, cfg, &stubVerifier{})

	hash, _ := HashPassword("p")
	wu := model.User{Username: "webu", PasswordHash: hash, Status: "ACTIVE", Source: model.SourceLocal}
	if err := db.Create(&wu).Error; err != nil {
		t.Fatal(err)
	}
	mu := model.ManagerUser{Username: "ops", PasswordHash: hash, Status: "ACTIVE", Source: model.SourceLocal, Role: model.RoleAdmin}
	if err := db.Create(&mu).Error; err != nil {
		t.Fatal(err)
	}

	webToken, _, err := svc.WebLogin(context.Background(), "webu", "p", "test")
	if err != nil {
		t.Fatal(err)
	}
	mgrToken, _, err := svc.ManagerLogin(context.Background(), "ops", "p", "test")
	if err != nil {
		t.Fatal(err)
	}

	wp, err := svc.Authenticate(webToken)
	if err != nil || wp.Realm != model.RealmWeb || wp.ID != wu.ID {
		t.Fatalf("web token auth mismatch: %+v %v", wp, err)
	}
	mp, err := svc.Authenticate(mgrToken)
	if err != nil || mp.Realm != model.RealmManager || mp.ID != mu.ID {
		t.Fatalf("manager token auth mismatch: %+v %v", mp, err)
	}
}

func init() { slog.SetDefault(slog.Default()) }
