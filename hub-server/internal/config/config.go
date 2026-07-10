package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Auth     AuthConfig     `yaml:"auth" json:"auth"`
	LLM      LLMConfig      `yaml:"llm" json:"llm"`
	Cache    CacheConfig    `yaml:"cache" json:"cache"`
	Limits   LimitsConfig   `yaml:"limits" json:"limits"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging"`
}

type ServerConfig struct {
	Address       string `yaml:"address" json:"address"`
	PublicBaseURL string `yaml:"public_base_url" json:"public_base_url"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver" json:"driver"`
	DSN    string `yaml:"dsn" json:"dsn"`
}

type AuthConfig struct {
	TokenTTL      time.Duration `yaml:"token_ttl" json:"token_ttl"`
	AdminUsername string        `yaml:"admin_username" json:"admin_username"`
	AdminPassword string        `yaml:"admin_password" json:"-"`
}

type LLMConfig struct {
	Model   string `yaml:"model" json:"model"`
	APIBase string `yaml:"api_base" json:"api_base"`
	APIKey  string `yaml:"api_key" json:"-"`
	Proxy   string `yaml:"proxy" json:"proxy"`
}

type CacheConfig struct {
	DefaultStrategy string        `yaml:"default_strategy" json:"default_strategy"`
	TTL             time.Duration `yaml:"ttl" json:"ttl"`
}

type LimitsConfig struct {
	RequestBodyBytes int64 `yaml:"request_body_bytes" json:"request_body_bytes"`
	PageSizeMax      int   `yaml:"page_size_max" json:"page_size_max"`
	ExpressionLength int   `yaml:"expression_length" json:"expression_length"`
	DependencyDepth  int   `yaml:"dependency_depth" json:"dependency_depth"`
}

type LoggingConfig struct {
	Level string `yaml:"level" json:"level"`
}

func Default() Config {
	return Config{
		Server:   ServerConfig{Address: ":8080"},
		Database: DatabaseConfig{Driver: "sqlite", DSN: "hub-server.db"},
		Auth:     AuthConfig{TokenTTL: 24 * time.Hour, AdminUsername: "admin"},
		Cache:    CacheConfig{DefaultStrategy: "VERSIONED", TTL: time.Hour},
		Limits:   LimitsConfig{RequestBodyBytes: 1 << 20, PageSizeMax: 100, ExpressionLength: 20000, DependencyDepth: 16},
		Logging:  LoggingConfig{Level: "info"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, err
		}
	}
	applyEnv(&cfg)
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("HUB_SERVER_ADDRESS"); v != "" {
		cfg.Server.Address = v
	}
	if v := os.Getenv("HUB_DATABASE_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("HUB_DATABASE_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("HUB_AUTH_ADMIN_USERNAME"); v != "" {
		cfg.Auth.AdminUsername = v
	}
	if v := os.Getenv("HUB_AUTH_ADMIN_PASSWORD"); v != "" {
		cfg.Auth.AdminPassword = v
	}
	if v := os.Getenv("HUB_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("HUB_LLM_API_BASE"); v != "" {
		cfg.LLM.APIBase = v
	}
	if v := os.Getenv("HUB_LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("HUB_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("HUB_LIMIT_PAGE_SIZE_MAX"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Limits.PageSizeMax = parsed
		}
	}
}
