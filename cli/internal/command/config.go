package command

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Home       string           `yaml:"home" json:"home"`
	Local      bool             `yaml:"local" json:"local"`
	Repository RepositoryConfig `yaml:"repository" json:"repository"`
	Server     ServerConfig     `yaml:"server" json:"server"`
	LLM        LLMConfig        `yaml:"llm" json:"llm"`
	Cache      CacheConfig      `yaml:"cache" json:"cache"`
}

type RepositoryConfig struct {
	Path string `yaml:"path" json:"path"`
}

type ServerConfig struct {
	Base  string `yaml:"base" json:"base"`
	Token string `yaml:"token,omitempty" json:"token,omitempty"`
}

type LLMConfig struct {
	Model   string `yaml:"model" json:"model"`
	APIBase string `yaml:"api_base" json:"api_base"`
	APIKey  string `yaml:"api_key" json:"api_key"`
	Proxy   string `yaml:"proxy" json:"proxy"`
}

type CacheConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Strategy string `yaml:"strategy" json:"strategy"`
}

func defaultConfig() Config {
	home := os.Getenv("GLUE_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".glue")
		} else {
			home = ".glue"
		}
	}
	return Config{
		Home:       home,
		Local:      false,
		Repository: RepositoryConfig{Path: filepath.Join(home, "repository")},
		Server:     ServerConfig{},
		LLM:        LLMConfig{},
		Cache:      CacheConfig{Enabled: true, Strategy: "VERSIONED"},
	}
}

func loadConfigFile(home string) (Config, bool, error) {
	cfg := Config{}
	path := filepath.Join(home, "config.yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, false, err
	}
	return cfg, true, nil
}

func loadEffectiveConfig(cmd *cobra.Command, opts *rootOptions) (Config, error) {
	cfg := defaultConfig()
	if err := ensureSampleConfig(cfg); err != nil {
		return Config{}, err
	}
	if fileCfg, ok, err := loadConfigFile(cfg.Home); err != nil {
		return Config{}, err
	} else if ok {
		cfg = mergeConfig(cfg, fileCfg)
		if cfg.Home == "" {
			cfg.Home = defaultConfig().Home
		}
		if cfg.Repository.Path == "" {
			cfg.Repository.Path = filepath.Join(cfg.Home, "repository")
		}
	}
	applyEnv(&cfg)
	if changed(cmd, "local") {
		cfg.Local = opts.local
	}
	if opts.llmModel != "" {
		cfg.LLM.Model = opts.llmModel
	}
	if opts.llmAPIBase != "" {
		cfg.LLM.APIBase = opts.llmAPIBase
	}
	if opts.llmAPIKey != "" {
		cfg.LLM.APIKey = opts.llmAPIKey
	}
	if opts.llmProxy != "" {
		cfg.LLM.Proxy = opts.llmProxy
	}
	if opts.repositoryPath != "" {
		cfg.Repository.Path = opts.repositoryPath
	}
	return cfg, nil
}

func mergeConfig(base Config, override Config) Config {
	if override.Home != "" {
		base.Home = override.Home
	}
	base.Local = override.Local
	if override.Repository.Path != "" {
		base.Repository.Path = override.Repository.Path
	}
	if override.Server.Base != "" {
		base.Server.Base = override.Server.Base
	}
	if override.Server.Token != "" {
		base.Server.Token = override.Server.Token
	}
	if override.LLM.Model != "" {
		base.LLM.Model = override.LLM.Model
	}
	if override.LLM.APIBase != "" {
		base.LLM.APIBase = override.LLM.APIBase
	}
	if override.LLM.APIKey != "" {
		base.LLM.APIKey = override.LLM.APIKey
	}
	if override.LLM.Proxy != "" {
		base.LLM.Proxy = override.LLM.Proxy
	}
	if override.Cache.Strategy != "" {
		base.Cache.Strategy = override.Cache.Strategy
	}
	base.Cache.Enabled = override.Cache.Enabled || base.Cache.Enabled
	return base
}

func applyEnv(cfg *Config) {
	if value := os.Getenv("GLUE_LOCAL"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Local = parsed
		}
	}
	if value := os.Getenv("GLUE_REPOSITORY__PATH"); value != "" {
		cfg.Repository.Path = value
	}
	if value := os.Getenv("GLUE_SERVER__BASE"); value != "" {
		cfg.Server.Base = value
	}
	if value := os.Getenv("GLUE_SERVER__TOKEN"); value != "" {
		cfg.Server.Token = value
	}
	if value := os.Getenv("GLUE_LLM__MODEL"); value != "" {
		cfg.LLM.Model = value
	}
	if value := os.Getenv("GLUE_LLM__API_BASE"); value != "" {
		cfg.LLM.APIBase = value
	}
	if value := os.Getenv("GLUE_LLM__API_KEY"); value != "" {
		cfg.LLM.APIKey = value
	}
	if value := os.Getenv("GLUE_LLM__PROXY"); value != "" {
		cfg.LLM.Proxy = value
	}
	if value := os.Getenv("GLUE_CACHE__ENABLED"); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Cache.Enabled = parsed
		}
	}
	if value := os.Getenv("GLUE_CACHE__STRATEGY"); value != "" {
		cfg.Cache.Strategy = value
	}
}

func ensureSampleConfig(cfg Config) error {
	path := filepath.Join(cfg.Home, "config.yaml")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(cfg.Home, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(sampleConfigTemplate()), 0o600)
}

func sampleConfigTemplate() string {
	return `# 示例: /home/user/.glue
home: ""
# 示例: false
local: false
repository:
    # 示例: /home/user/.glue/repository
    path: ""
server:
    # 示例: https://prompts.example.com
    base: ""
    # 个人访问令牌或登录令牌（glue login 自动写入；切勿泄露）
    token: ""
llm:
    # 示例: deepseek-v4-flash
    model: ""
    # 示例: https://api.example.com/v1
    api_base: ""
    # 示例: sk-example
    api_key: ""
    # 示例: http://127.0.0.1:7890
    proxy: ""
cache:
    # 示例: true
    enabled: true
    # 示例: VERSIONED
    strategy: VERSIONED
`
}

func saveConfig(cfg Config) error {
	if err := os.MkdirAll(cfg.Home, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.Home, "config.yaml"), data, 0o600)
}

func changed(cmd *cobra.Command, name string) bool {
	flag := cmd.Flags().Lookup(name)
	if flag != nil && flag.Changed {
		return true
	}
	flag = cmd.InheritedFlags().Lookup(name)
	return flag != nil && flag.Changed
}

func setConfigValue(cfg *Config, key string, value string) error {
	switch strings.ToLower(key) {
	case "home":
		cfg.Home = value
	case "repository.path":
		cfg.Repository.Path = value
	case "server.base":
		cfg.Server.Base = value
	case "server.token":
		cfg.Server.Token = value
	case "local":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Local = parsed
	case "llm.model":
		cfg.LLM.Model = value
	case "llm.api_base":
		cfg.LLM.APIBase = value
	case "llm.api_key":
		cfg.LLM.APIKey = value
	case "llm.proxy":
		cfg.LLM.Proxy = value
	case "cache.enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		cfg.Cache.Enabled = parsed
	case "cache.strategy":
		cfg.Cache.Strategy = value
	default:
		return errors.New("unknown config key")
	}
	return nil
}
