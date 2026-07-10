package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"hub-server/internal/config"
	"hub-server/internal/model"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch strings.ToLower(cfg.Database.Driver) {
	case "", "sqlite":
		dialector = sqlite.Open(cfg.Database.DSN)
	case "mysql":
		dialector = mysql.Open(cfg.Database.DSN)
	case "postgres", "postgresql":
		dialector = postgres.Open(cfg.Database.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
	level := logger.Warn
	if strings.ToLower(cfg.Logging.Level) == "debug" {
		level = logger.Info
	}
	return gorm.Open(dialector, &gorm.Config{Logger: logger.Default.LogMode(level)})
}

func Ping(ctx context.Context, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.AccessToken{},
		&model.Namespace{},
		&model.NamespaceMember{},
		&model.Package{},
		&model.PackageVersion{},
		&model.PackageContent{},
		&model.PackageTag{},
		&model.PackageDependency{},
		&model.PackageVersionEvent{},
		&model.GenerationRecord{},
		&model.CalculationCache{},
		&model.DictItem{},
		&model.ServiceConfig{},
		&model.AuditLog{},
		&model.DownloadRecord{},
		&model.RiskMark{},
	)
}

func BeforeCreate(db *gorm.DB) {
	now := time.Now().UTC()
	db.Callback().Create().Before("gorm:create").Register("hub:set_ids", func(tx *gorm.DB) {
		if tx.Statement.Schema == nil {
			return
		}
		if field := tx.Statement.Schema.LookUpField("ID"); field != nil {
			value, _ := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue)
			if value == "" {
				_ = field.Set(tx.Statement.Context, tx.Statement.ReflectValue, uuid.NewString())
			}
		}
		if field := tx.Statement.Schema.LookUpField("CreatedAt"); field != nil {
			value, _ := field.ValueOf(tx.Statement.Context, tx.Statement.ReflectValue)
			if t, ok := value.(time.Time); ok && t.IsZero() {
				_ = field.Set(tx.Statement.Context, tx.Statement.ReflectValue, now)
			}
		}
	})
}
