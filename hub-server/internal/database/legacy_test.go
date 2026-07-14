package database

import (
	"testing"

	"hub-server/internal/config"
	"hub-server/internal/model"
)

// TestDropLegacyUsersRole 模拟旧库 users 表遗留 NOT NULL 无默认的 role 列：
// 删除该列后，LDAP 同步插入 web 用户（不带 role）应成功。
func TestDropLegacyUsersRole(t *testing.T) {
	cfg := config.Default()
	cfg.Database.DSN = ":memory:"
	db, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	BeforeCreate(db)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	// 模拟旧库：手工添加 NOT NULL 无默认的 role 列。
	if err := db.Exec("ALTER TABLE users ADD COLUMN role VARCHAR(32) NOT NULL").Error; err != nil {
		t.Fatalf("add legacy role column: %v", err)
	}
	if !db.Migrator().HasColumn(&model.User{}, "role") {
		t.Fatal("expected legacy role column to exist")
	}
	// 删除前：插入（不设 role）应失败（NOT NULL 无默认）。
	if err := db.Create(&model.User{Username: "ldapu", Source: model.SourceLDAP, ExternalID: "dn=ldapu", Status: "ACTIVE"}).Error; err == nil {
		t.Fatal("expected insert to fail before dropping legacy role column")
	}
	if err := DropLegacyUsersRole(db); err != nil {
		t.Fatalf("DropLegacyUsersRole: %v", err)
	}
	if db.Migrator().HasColumn(&model.User{}, "role") {
		t.Fatal("expected legacy role column to be dropped")
	}
	// 删除后：插入 web 用户应成功。
	if err := db.Create(&model.User{Username: "ldapu2", Source: model.SourceLDAP, ExternalID: "dn=ldapu2", Status: "ACTIVE"}).Error; err != nil {
		t.Fatalf("insert after drop failed: %v", err)
	}
	// 二次调用应幂等（列已不存在，不报错）。
	if err := DropLegacyUsersRole(db); err != nil {
		t.Fatalf("idempotent drop failed: %v", err)
	}
}
