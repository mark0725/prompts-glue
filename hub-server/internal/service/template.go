package service

import (
	"context"
	"errors"
	"strings"

	promptssdk "prompts-sdk"

	"hub-server/internal/model"
)

// TemplateRequest 为模板渲染请求。模板正文由请求体承载（纯文本），
// 上下文变量经查询参数传入；功能与 glue CLI 的 template 命令等价。
type TemplateRequest struct {
	Template string
	Inputs   map[string]string
}

// TemplateResponse 聚合所有占位符的渲染内容、依赖与 token，
// 字段与 glue CLI template 的 JSON 输出一致；文本输出仅取 Content。
type TemplateResponse struct {
	Content      string                `json:"content"`
	Dependencies []DependencyMeta      `json:"dependencies"`
	Tokens       promptssdk.TokenUsage `json:"tokens"`
}

// registryTemplateEvaluator 实现 promptssdk.TemplateEvaluator：每个占位符独立调用 Generate，
// 复用既有缓存命中、generation 记录与 LLM 执行（与 CLI 远程模式逐次调 /api/registry/generate 等价）。
type registryTemplateEvaluator struct {
	svc  *RegistryService
	user *model.Principal
}

func (e registryTemplateEvaluator) Evaluate(ctx context.Context, expr string, vars map[string]string) (promptssdk.TemplateEvalResult, error) {
	resp, err := e.svc.Generate(ctx, GenerateRequest{Expression: expr, Inputs: vars}, e.user)
	if err != nil {
		return promptssdk.TemplateEvalResult{}, err
	}
	deps := make([]promptssdk.DependencyRef, 0, len(resp.Dependencies))
	for _, d := range resp.Dependencies {
		deps = append(deps, promptssdk.DependencyRef{Name: d.Name, VersionRule: d.VersionRule})
	}
	return promptssdk.TemplateEvalResult{Content: resp.Content, Dependencies: deps, TokenUsage: resp.Calculation.TokenUsage}, nil
}

// Template 渲染模板：扫描 {{...}} 占位符，逐个作为 glue 表达式生成并组装为完整内容。
// 任一占位符解析或求值失败即返回错误（严格模式）。每个占位符各自计缓存与 generation 记录。
func (s *RegistryService) Template(ctx context.Context, req TemplateRequest, user *model.Principal) (TemplateResponse, error) {
	if strings.TrimSpace(req.Template) == "" {
		return TemplateResponse{}, errors.New("template is required")
	}
	if s.cfg.Limits.TemplateLength > 0 && len(req.Template) > s.cfg.Limits.TemplateLength {
		return TemplateResponse{}, errors.New("template is too long")
	}
	res, err := promptssdk.RenderTemplate(ctx, req.Template, req.Inputs, registryTemplateEvaluator{svc: s, user: user})
	if err != nil {
		return TemplateResponse{}, err
	}
	deps := make([]DependencyMeta, 0, len(res.Dependencies))
	for _, d := range res.Dependencies {
		deps = append(deps, DependencyMeta{Name: d.Name, VersionRule: d.VersionRule, AffectsCompute: true})
	}
	return TemplateResponse{Content: res.Content, Dependencies: deps, Tokens: res.TokenUsage}, nil
}
