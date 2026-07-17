package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/model"
)

func newCacheDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Default()
	cfg.Database.DSN = ":memory:"
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	database.BeforeCreate(db)
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedCache(t *testing.T, db *gorm.DB, key string, expires *time.Time) {
	t.Helper()
	c := model.CalculationCache{CacheKey: key, CacheStrategy: "VERSIONED", ExpressionHash: "h", DependencyChecksum: "d", ResultContent: "r", ResultChecksum: "rc", ExpiresAt: expires}
	if err := db.Create(&c).Error; err != nil {
		t.Fatal(err)
	}
}

func TestClearCacheAll(t *testing.T) {
	db := newCacheDB(t)
	svc := NewRegistryService(db, config.Default())
	seedCache(t, db, "k1", nil)
	seedCache(t, db, "k2", nil)
	if err := svc.ClearCache(false); err != nil {
		t.Fatalf("ClearCache(false): %v", err)
	}
	var n int64
	db.Model(&model.CalculationCache{}).Count(&n)
	if n != 0 {
		t.Fatalf("expected 0 rows after clear all, got %d", n)
	}
}

func TestClearCacheExpiredOnly(t *testing.T) {
	db := newCacheDB(t)
	svc := NewRegistryService(db, config.Default())
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)
	seedCache(t, db, "expired", &past)
	seedCache(t, db, "live", &future)
	if err := svc.ClearCache(true); err != nil {
		t.Fatalf("ClearCache(true): %v", err)
	}
	var n int64
	db.Model(&model.CalculationCache{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected 1 (live) row after expired-only clear, got %d", n)
	}
}
