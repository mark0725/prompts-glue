package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"hub-server/internal/config"
	"hub-server/internal/database"
	"hub-server/internal/server"
)

func main() {
	var configPath string
	root := &cobra.Command{
		Use:          "hub-server",
		Short:        "prompts-glue hub server",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath, cmd.Flags())
			if err != nil {
				return err
			}
			logger := newLogger(cfg.Logging.Level)
			logger.Info("hub-server starting", "version", "0.1.0", "address", cfg.Server.Address, "driver", cfg.Database.Driver)
			db, err := database.Open(cfg)
			if err != nil {
				logger.Error("database open failed", "error", err)
				return err
			}
			pingCtx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()
			if err := database.Ping(pingCtx, db); err != nil {
				logger.Error("database ping failed", "driver", cfg.Database.Driver, "error", err)
				return err
			}
			logger.Info("database connected")
			database.BeforeCreate(db)
			logger.Info("running database migration")
			if err := database.Migrate(db); err != nil {
				logger.Error("database migration failed", "error", err)
				return err
			}
			logger.Info("database migration completed")
			return server.New(cfg, db, logger).Start()
		},
	}
	root.PersistentFlags().StringVar(&configPath, "config", "", "配置文件路径（按扩展名识别 yaml/yml/json/toml）")
	config.RegisterFlags(root.PersistentFlags())

	healthcheck := &cobra.Command{
		Use:          "healthcheck",
		Short:        "检查服务健康状态，供容器健康检查使用",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath, cmd.Flags())
			if err != nil {
				return err
			}
			return checkHealth(cfg)
		},
	}
	root.AddCommand(healthcheck)

	if err := root.Execute(); err != nil {
		slog.Error("hub-server failed", "error", err)
		os.Exit(1)
	}
}

func checkHealth(cfg config.Config) error {
	address := cfg.Server.Address
	if address == "" {
		address = ":8080"
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid server address %q: %w", address, err)
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	url := "http://" + net.JoinHostPort(host, port) + "/api/registry/health"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %s", resp.Status)
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}
