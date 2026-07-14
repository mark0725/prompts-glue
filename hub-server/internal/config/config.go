package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// envPrefix 是环境变量公共前缀，层级点号用 envSep（双下划线）表示。
const (
	envPrefix = "HUB_"
	envSep    = "__"
)

// Duration 是 time.Duration 的配置包装类型，统一以文本形式（如 "24h"、"30m"）
// 出现在 YAML / JSON / TOML 文件、环境变量和命令行参数中。
type Duration time.Duration

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

// UnmarshalYAML 让 yaml.v3 直接按字符串解析，避免依赖 TextUnmarshaler 的行为差异。
func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var raw string
	if err := node.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

type Config struct {
	Server   ServerConfig   `yaml:"server" json:"server" toml:"server"`
	Database DatabaseConfig `yaml:"database" json:"database" toml:"database"`
	Auth     AuthConfig     `yaml:"auth" json:"auth" toml:"auth"`
	LLM      LLMConfig      `yaml:"llm" json:"llm" toml:"llm"`
	Cache    CacheConfig    `yaml:"cache" json:"cache" toml:"cache"`
	Limits   LimitsConfig   `yaml:"limits" json:"limits" toml:"limits"`
	Logging  LoggingConfig  `yaml:"logging" json:"logging" toml:"logging"`
	LDAP     LDAPConfig     `yaml:"ldap" json:"ldap" toml:"ldap"`
}

type ServerConfig struct {
	Address       string `yaml:"address" json:"address" toml:"address"`
	PublicBaseURL string `yaml:"public_base_url" json:"public_base_url" toml:"public_base_url"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver" json:"driver" toml:"driver"`
	DSN    string `yaml:"dsn" json:"dsn" toml:"dsn"`
}

type AuthConfig struct {
	TokenTTL      Duration `yaml:"token_ttl" json:"token_ttl" toml:"token_ttl"`
	AdminUsername string   `yaml:"admin_username" json:"admin_username" toml:"admin_username"`
	AdminPassword string   `yaml:"admin_password" json:"-" toml:"admin_password"`
}

type LLMConfig struct {
	Model   string   `yaml:"model" json:"model" toml:"model"`
	APIBase string   `yaml:"api_base" json:"api_base" toml:"api_base"`
	APIKey  string   `yaml:"api_key" json:"-" toml:"api_key"`
	Proxy   string   `yaml:"proxy" json:"proxy" toml:"proxy"`
	Timeout Duration `yaml:"timeout" json:"timeout" toml:"timeout"`
}

type CacheConfig struct {
	DefaultStrategy string   `yaml:"default_strategy" json:"default_strategy" toml:"default_strategy"`
	TTL             Duration `yaml:"ttl" json:"ttl" toml:"ttl"`
}

type LimitsConfig struct {
	RequestBodyBytes int64 `yaml:"request_body_bytes" json:"request_body_bytes" toml:"request_body_bytes"`
	PageSizeMax      int   `yaml:"page_size_max" json:"page_size_max" toml:"page_size_max"`
	ExpressionLength int   `yaml:"expression_length" json:"expression_length" toml:"expression_length"`
	DependencyDepth  int   `yaml:"dependency_depth" json:"dependency_depth" toml:"dependency_depth"`
}

type LoggingConfig struct {
	Level string `yaml:"level" json:"level" toml:"level"`
}

// LDAPConfig 描述 LDAP 连接与用户同步配置。LDAP 作为企业外部身份源，
// 默认将用户同步到 manager_users 表；在 sync_targets 中加入 "web" 时也同步到 users（web）表。
// LDAP 来源用户登录时以 external_id（DN）+ 密码走 LDAP bind 验证，不使用本地密码摘要。
type LDAPConfig struct {
	Enable               bool     `yaml:"enable" json:"enable" toml:"enable"`
	Host                 string   `yaml:"host" json:"host" toml:"host"`
	Port                 int      `yaml:"port" json:"port" toml:"port"`
	BindDN               string   `yaml:"bind_dn" json:"bind_dn" toml:"bind_dn"`
	BindPassword         string   `yaml:"bind_password" json:"-" toml:"bind_password"`
	SearchDN             string   `yaml:"search_dn" json:"search_dn" toml:"search_dn"`
	UserFilter           string   `yaml:"user_filter" json:"user_filter" toml:"user_filter"`
	Attribute            string   `yaml:"attribute" json:"attribute" toml:"attribute"`
	DisplayNameAttribute string   `yaml:"display_name_attribute" json:"display_name_attribute" toml:"display_name_attribute"`
	StartTLS             bool     `yaml:"start_tls" json:"start_tls" toml:"start_tls"`
	LDAPS                bool     `yaml:"ldaps" json:"ldaps" toml:"ldaps"`
	VerifyHost           bool     `yaml:"verify_host" json:"verify_host" toml:"verify_host"`
	Timeout              Duration `yaml:"timeout" json:"timeout" toml:"timeout"`
	SyncTargets          []string `yaml:"sync_targets" json:"sync_targets" toml:"sync_targets"`
	SyncInterval         Duration `yaml:"sync_interval" json:"sync_interval" toml:"sync_interval"`
}

func Default() Config {
	return Config{
		Server:   ServerConfig{Address: ":8080"},
		Database: DatabaseConfig{Driver: "sqlite", DSN: "hub-server.db"},
		Auth:     AuthConfig{TokenTTL: Duration(24 * time.Hour), AdminUsername: "admin"},
		Cache:    CacheConfig{DefaultStrategy: "VERSIONED", TTL: Duration(time.Hour)},
		Limits:   LimitsConfig{RequestBodyBytes: 1 << 20, PageSizeMax: 100, ExpressionLength: 20000, DependencyDepth: 16},
		Logging:  LoggingConfig{Level: "info"},
		LDAP: LDAPConfig{
			Attribute:            "uid",
			DisplayNameAttribute: "givenName",
			UserFilter:           "(objectClass=person)",
			VerifyHost:           true,
			Timeout:              Duration(10 * time.Second),
			SyncTargets:          []string{"manager"},
			SyncInterval:         Duration(30 * time.Minute),
		},
	}
}

// Load 按优先级 合并配置：代码默认值 < 配置文件 < 环境变量 < 命令行参数。
// filePath 按扩展名识别 yaml/yml/json/toml；fs 为注册了配置 flag 的 FlagSet，可为 nil。
func Load(filePath string, fs *pflag.FlagSet) (Config, error) {
	cfg := Default()
	if filePath != "" {
		if err := loadFile(filePath, &cfg); err != nil {
			return Config{}, fmt.Errorf("load config file %q: %w", filePath, err)
		}
	}
	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	if fs != nil {
		if err := applyFlags(fs, &cfg); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func loadFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".yaml", ".yml":
		return yaml.Unmarshal(data, cfg)
	case ".json":
		return json.Unmarshal(data, cfg)
	case ".toml":
		return toml.Unmarshal(data, cfg)
	default:
		return fmt.Errorf("unsupported config extension %q (use yaml/yml/json/toml)", ext)
	}
}

// RegisterFlags 为每个配置叶子字段注册命令行 flag（横杠表示层级点号，段内下划线转横杠）。
func RegisterFlags(fs *pflag.FlagSet) {
	if fs == nil {
		return
	}
	for _, d := range describe() {
		if d.isSlice {
			fs.StringArray(d.flag, nil, "覆盖配置项 "+d.path+"（可重复）")
			continue
		}
		fs.String(d.flag, "", "覆盖配置项 "+d.path)
	}
}

func applyEnv(cfg *Config) error {
	root := reflect.ValueOf(cfg).Elem()
	for _, d := range describe() {
		field := root.FieldByIndex(d.idx)
		if d.isSlice {
			setSliceFromEnv(field, d.env)
			continue
		}
		raw, ok := os.LookupEnv(d.env)
		if !ok || raw == "" {
			continue
		}
		if err := setScalar(field, raw); err != nil {
			return fmt.Errorf("env %s: %w", d.env, err)
		}
	}
	return nil
}

func applyFlags(fs *pflag.FlagSet, cfg *Config) error {
	root := reflect.ValueOf(cfg).Elem()
	for _, d := range describe() {
		fl := fs.Lookup(d.flag)
		if fl == nil || !fl.Changed {
			continue
		}
		field := root.FieldByIndex(d.idx)
		if d.isSlice {
			values, err := flagStringSlice(fl)
			if err != nil {
				return fmt.Errorf("flag --%s: %w", d.flag, err)
			}
			if err := setSlice(field, values); err != nil {
				return fmt.Errorf("flag --%s: %w", d.flag, err)
			}
			continue
		}
		if err := setScalar(field, fl.Value.String()); err != nil {
			return fmt.Errorf("flag --%s: %w", d.flag, err)
		}
	}
	return nil
}

func flagStringSlice(fl *pflag.Flag) ([]string, error) {
	if sv, ok := fl.Value.(pflag.SliceValue); ok {
		return sv.GetSlice(), nil
	}
	return nil, fmt.Errorf("flag %q is not a slice", fl.Name)
}

// fieldDesc 描述一个叶子配置字段的路径、env/flag 命名与反射定位信息。
type fieldDesc struct {
	path       string
	env        string
	flag       string
	idx        []int
	isDuration bool
	isSlice    bool
}

var configDescs = describe()

func describe() []fieldDesc {
	var out []fieldDesc
	walkStruct(reflect.TypeOf(Config{}), nil, "", &out)
	seenEnv := map[string]bool{}
	seenFlag := map[string]bool{}
	for _, d := range out {
		if d.flag == "config" {
			panic("config: derived flag name collides with reserved --config")
		}
		if seenEnv[d.env] {
			panic("config: duplicate env name " + d.env)
		}
		if seenFlag[d.flag] {
			panic("config: duplicate flag name " + d.flag)
		}
		seenEnv[d.env] = true
		seenFlag[d.flag] = true
	}
	return out
}

func walkStruct(t reflect.Type, idx []int, parent string, out *[]fieldDesc) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := fieldName(f)
		if name == "" {
			continue
		}
		path := name
		if parent != "" {
			path = parent + "." + name
		}
		fidx := append(append([]int{}, idx...), i)
		switch {
		case isDurationType(f.Type):
			*out = append(*out, fieldDesc{path: path, env: envName(path), flag: flagName(path), idx: fidx, isDuration: true})
		case f.Type.Kind() == reflect.Struct:
			walkStruct(f.Type, fidx, path, out)
		case f.Type.Kind() == reflect.Slice:
			*out = append(*out, fieldDesc{path: path, env: envName(path), flag: flagName(path), idx: fidx, isSlice: true})
		default:
			*out = append(*out, fieldDesc{path: path, env: envName(path), flag: flagName(path), idx: fidx})
		}
	}
}

func fieldName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" {
		tag = f.Tag.Get("json")
	}
	if tag == "" || tag == "-" {
		return strings.ToLower(f.Name)
	}
	return strings.SplitN(tag, ",", 2)[0]
}

func envName(path string) string {
	return envPrefix + strings.ToUpper(strings.ReplaceAll(path, ".", envSep))
}

func flagName(path string) string {
	return strings.NewReplacer(".", "-", "_", "-").Replace(path)
}

var (
	durationType    = reflect.TypeOf(Duration(0))
	timeDurationTyp = reflect.TypeOf(time.Duration(0))
)

func isDurationType(t reflect.Type) bool {
	return t == durationType || t == timeDurationTyp
}

func setScalar(field reflect.Value, raw string) error {
	if isDurationType(field.Type()) {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(d))
		return nil
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		field.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return err
		}
		field.SetFloat(f)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

func setSlice(field reflect.Value, values []string) error {
	elemType := field.Type().Elem()
	slice := reflect.MakeSlice(field.Type(), 0, len(values))
	for _, raw := range values {
		elem := reflect.New(elemType).Elem()
		if err := setScalar(elem, raw); err != nil {
			return err
		}
		slice = reflect.Append(slice, elem)
	}
	field.Set(slice)
	return nil
}

// setSliceFromEnv 收集 HUB_<base>__<i> 或 HUB_<base>__<i>__ 形式的环境变量为数组元素（i 为下标段）。
// 项目规范为数组元素下标带尾 __（如 HUB_LDAP__SYNC_TARGETS__0__），此处同时兼容不带尾 __ 的写法。
func setSliceFromEnv(field reflect.Value, baseEnv string) {
	prefix := baseEnv + envSep
	idxVals := map[int]string{}
	maxIdx := -1
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, prefix) {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			continue
		}
		// 下标段位于 prefix 之后、'=' 之前；去掉规范要求的尾随 __ 再解析为整数。
		idxSeg := strings.TrimSuffix(kv[len(prefix):eq], envSep)
		n, err := strconv.Atoi(idxSeg)
		if err != nil {
			continue
		}
		if v := kv[eq+1:]; v != "" {
			idxVals[n] = v
			if n > maxIdx {
				maxIdx = n
			}
		}
	}
	if maxIdx < 0 {
		return
	}
	values := make([]string, 0, maxIdx+1)
	for i := 0; i <= maxIdx; i++ {
		if v, ok := idxVals[i]; ok {
			values = append(values, v)
		}
	}
	_ = setSlice(field, values)
}
