package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// errLoginRequired 表示请求未认证（401），命令层应提示用户执行 glue login。
var errLoginRequired = errors.New("authentication required: run `glue login`")

// remoteEnvelope 镜像 server 的 api.Response 信封，CLI 不 import server 模块。
type remoteEnvelope struct {
	Success   bool            `json:"success"`
	Data      json.RawMessage `json:"data,omitempty"`
	Error     *remoteError    `json:"error,omitempty"`
	RequestID string          `json:"request_id"`
}

type remoteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RemoteClient 是与远程 server 交互的 HTTP 客户端。
type RemoteClient struct {
	base  string
	token string
	http  *http.Client
}

// newRemoteClient 构造远程客户端。base 必填；token 可空（登录命令无 token 时调用）。
func newRemoteClient(cfg Config) (*RemoteClient, error) {
	if cfg.Server.Base == "" {
		return nil, errors.New("server.base is not configured; set it via `glue config set server.base <url>` or GLUE_SERVER__BASE")
	}
	return &RemoteClient{
		base:  cfg.Server.Base,
		token: cfg.Server.Token,
		http:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// hasToken 返回是否已配置访问令牌。
func (c *RemoteClient) hasToken() bool { return c.token != "" }

// requireToken 在需要鉴权的命令前检查令牌是否已配置。
func (c *RemoteClient) requireToken() error {
	if !c.hasToken() {
		return errLoginRequired
	}
	return nil
}

// do 发送请求并解码信封。401 返回 errLoginRequired；非 success 返回错误。
func (c *RemoteClient) do(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("server unreachable: %w", err)
	}
	defer resp.Body.Close()
	var env remoteEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errLoginRequired
	}
	if !env.Success {
		msg := "request failed"
		if env.Error != nil && env.Error.Message != "" {
			msg = env.Error.Message
		}
		return errors.New(msg)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode data: %w", err)
		}
	}
	return nil
}

// ---- 远程响应结构（snake_case，对齐 server 输出）----

type serverDependencyMeta struct {
	Name            string `json:"name"`
	VersionRule     string `json:"version_rule,omitempty"`
	ResolvedVersion string `json:"resolved_version,omitempty"`
	Optional        bool   `json:"optional,omitempty"`
	AffectsCompute  bool   `json:"affects_compute,omitempty"`
}

type serverPackageMeta struct {
	Name         string                 `json:"name"`
	Version      string                 `json:"version"`
	Description  string                 `json:"description"`
	Status       string                 `json:"status"`
	Category     string                 `json:"category"`
	Type         string                 `json:"type"`
	Expression   string                 `json:"expression,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
	Author       string                 `json:"author,omitempty"`
	Repository   string                 `json:"repository,omitempty"`
	Homepage     string                 `json:"homepage,omitempty"`
	License      string                 `json:"license,omitempty"`
	Visibility   string                 `json:"visibility,omitempty"`
	Dependencies []serverDependencyMeta `json:"dependencies,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
}

type generateRequest struct {
	Expression    string            `json:"expression"`
	Inputs        map[string]string `json:"inputs,omitempty"`
	CacheStrategy string            `json:"cache_strategy,omitempty"`
	Sensitive     bool              `json:"sensitive,omitempty"`
}

type generateResponse struct {
	Content      string                 `json:"content"`
	Dependencies []serverDependencyMeta `json:"dependencies"`
	Calculation  struct {
		TokenUsage struct {
			Input  int `json:"input"`
			Output int `json:"output"`
			Total  int `json:"total"`
		} `json:"token_usage"`
	} `json:"calculation"`
	Warnings []string `json:"warnings,omitempty"`
}

type versionSummary struct {
	Version      string     `json:"version"`
	Status       string     `json:"status"`
	Type         string     `json:"type"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
}

type packageDetail struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	LatestVersion string           `json:"latest_version,omitempty"`
	Versions      []versionSummary `json:"versions"`
	Tags          []string         `json:"tags,omitempty"`
	Category      string           `json:"category"`
	Visibility    string           `json:"visibility"`
	StarCount     int64            `json:"star_count"`
	Starred       bool             `json:"starred"`
}

type versionDetail struct {
	Package      serverPackageMeta      `json:"package"`
	Dependencies []serverDependencyMeta `json:"dependencies"`
	PublishedBy  string                 `json:"published_by,omitempty"`
	PublishedAt  *time.Time             `json:"published_at,omitempty"`
}

type contentResponse struct {
	Package  serverPackageMeta `json:"package"`
	Content  string            `json:"content"`
	Readme   string            `json:"readme,omitempty"`
	Checksum string            `json:"checksum"`
}

type publishRequest struct {
	Package serverPackageMeta `json:"package"`
	Content string            `json:"content"`
	Compute bool              `json:"compute"`
	DryRun  bool              `json:"dry_run"`
}

type publishResponse struct {
	Package      serverPackageMeta      `json:"package"`
	Dependencies []serverDependencyMeta `json:"dependencies"`
	Warnings     []string               `json:"warnings,omitempty"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Client   string `json:"client"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

// ---- 便捷方法 ----

func (c *RemoteClient) login(ctx context.Context, username, password string) (string, error) {
	var resp loginResponse
	if err := c.do(ctx, http.MethodPost, "/api/registry/auth/login", loginRequest{Username: username, Password: password, Client: "cli"}, &resp); err != nil {
		return "", err
	}
	return resp.AccessToken, nil
}

func (c *RemoteClient) generate(ctx context.Context, expr string, vars map[string]string) (generateResponse, error) {
	var resp generateResponse
	if err := c.do(ctx, http.MethodPost, "/api/registry/generate", generateRequest{Expression: expr, Inputs: vars}, &resp); err != nil {
		return generateResponse{}, err
	}
	return resp, nil
}

func (c *RemoteClient) packageDetail(ctx context.Context, name string) (packageDetail, error) {
	var resp packageDetail
	if err := c.do(ctx, http.MethodGet, "/api/registry/packages/"+url.PathEscape(name), nil, &resp); err != nil {
		return packageDetail{}, err
	}
	return resp, nil
}

func (c *RemoteClient) versionDetail(ctx context.Context, name, version string) (versionDetail, error) {
	var resp versionDetail
	if err := c.do(ctx, http.MethodGet, "/api/registry/packages/"+url.PathEscape(name)+"/versions/"+url.PathEscape(version), nil, &resp); err != nil {
		return versionDetail{}, err
	}
	return resp, nil
}

func (c *RemoteClient) content(ctx context.Context, name, version string) (contentResponse, error) {
	var resp contentResponse
	if err := c.do(ctx, http.MethodGet, "/api/registry/packages/"+url.PathEscape(name)+"/versions/"+url.PathEscape(version)+"/content", nil, &resp); err != nil {
		return contentResponse{}, err
	}
	return resp, nil
}

func (c *RemoteClient) publish(ctx context.Context, req publishRequest) (publishResponse, error) {
	var resp publishResponse
	if err := c.do(ctx, http.MethodPost, "/api/registry/packages", req, &resp); err != nil {
		return publishResponse{}, err
	}
	return resp, nil
}

// ---- CLI ↔ server 映射 ----

// serverToCLIMeta 把 server 包元数据映射为 CLI 本地 PackageMeta（snake→camel，丢弃 visibility）。
func serverToCLIMeta(m serverPackageMeta) PackageMeta {
	return PackageMeta{
		Name:         m.Name,
		Version:      m.Version,
		Description:  m.Description,
		Status:       m.Status,
		Category:     m.Category,
		Tags:         m.Tags,
		Author:       m.Author,
		Repository:   m.Repository,
		Homepage:     m.Homepage,
		License:      m.License,
		Type:         m.Type,
		Expression:   m.Expression,
		Dependencies: serverToCLIDeps(m.Dependencies),
		CreatedAt:    m.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func serverToCLIDeps(deps []serverDependencyMeta) []DependencyMeta {
	out := make([]DependencyMeta, 0, len(deps))
	for _, d := range deps {
		out = append(out, DependencyMeta{
			Name:            d.Name,
			VersionRule:     d.VersionRule,
			ResolvedVersion: d.ResolvedVersion,
			Optional:        d.Optional,
			AffectsCompute:  d.AffectsCompute,
		})
	}
	return out
}

// cliToServerMeta 把 CLI 本地 PackageMeta 映射为 server 包元数据；visibility 默认 PUBLIC。
func cliToServerMeta(m PackageMeta) serverPackageMeta {
	return serverPackageMeta{
		Name:         m.Name,
		Version:      m.Version,
		Description:  m.Description,
		Status:       m.Status,
		Category:     m.Category,
		Type:         m.Type,
		Expression:   m.Expression,
		Tags:         m.Tags,
		Author:       m.Author,
		Repository:   m.Repository,
		Homepage:     m.Homepage,
		License:      m.License,
		Visibility:   "PUBLIC",
		Dependencies: cliToServerDeps(m.Dependencies),
	}
}

func cliToServerDeps(deps []DependencyMeta) []serverDependencyMeta {
	out := make([]serverDependencyMeta, 0, len(deps))
	for _, d := range deps {
		out = append(out, serverDependencyMeta{
			Name:            d.Name,
			VersionRule:     d.VersionRule,
			ResolvedVersion: d.ResolvedVersion,
			Optional:        d.Optional,
			AffectsCompute:  d.AffectsCompute,
		})
	}
	return out
}
