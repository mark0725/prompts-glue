package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	promptssdk "prompts-sdk"

	"hub-server/internal/config"
	"hub-server/internal/model"
)

var packageNameRegexp = regexp.MustCompile(`^(@[a-z0-9][a-z0-9_-]*/)?[a-z0-9][a-z0-9_-]*$`)

type RegistryService struct {
	db       *gorm.DB
	cfg      config.Config
	sdk      *promptssdk.SDK
	executor promptssdk.InstructionExecutor
}

type PackageMeta struct {
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Description  string           `json:"description"`
	Status       string           `json:"status"`
	Category     string           `json:"category"`
	Tags         []string         `json:"tags,omitempty"`
	Author       string           `json:"author,omitempty"`
	Repository   string           `json:"repository,omitempty"`
	Homepage     string           `json:"homepage,omitempty"`
	License      string           `json:"license,omitempty"`
	Type         string           `json:"type"`
	Visibility   string           `json:"visibility,omitempty"`
	Expression   string           `json:"expression,omitempty"`
	Dependencies []DependencyMeta `json:"dependencies,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
}

type DependencyMeta struct {
	Name            string `json:"name"`
	VersionRule     string `json:"version_rule,omitempty"`
	ResolvedVersion string `json:"resolved_version,omitempty"`
	Optional        bool   `json:"optional,omitempty"`
	AffectsCompute  bool   `json:"affects_compute,omitempty"`
}

type PublishRequest struct {
	Package PackageMeta `json:"package"`
	Content string      `json:"content"`
	Compute bool        `json:"compute"`
	DryRun  bool        `json:"dry_run"`
}

type PublishResponse struct {
	Package      PackageMeta      `json:"package"`
	Dependencies []DependencyMeta `json:"dependencies"`
	Calculation  *CalculationInfo `json:"calculation,omitempty"`
	Warnings     []string         `json:"warnings,omitempty"`
}

type CalculationInfo struct {
	Expression   string                `json:"expression"`
	Dependencies []DependencyMeta      `json:"dependencies"`
	CacheHit     bool                  `json:"cache_hit"`
	CacheKey     string                `json:"cache_key,omitempty"`
	TokenUsage   promptssdk.TokenUsage `json:"token_usage,omitempty"`
	ComputedAt   time.Time             `json:"computed_at"`
}

type GenerateRequest struct {
	Expression    string            `json:"expression"`
	Inputs        map[string]string `json:"inputs,omitempty"`
	CacheStrategy string            `json:"cache_strategy,omitempty"`
	ErrorStrategy string            `json:"error_strategy,omitempty"`
	SaveAsPackage bool              `json:"save_as_package,omitempty"`
	Package       PackageMeta       `json:"package,omitempty"`
	Sensitive     bool              `json:"sensitive,omitempty"`
}

type GenerateResponse struct {
	Content      string           `json:"content"`
	Dependencies []DependencyMeta `json:"dependencies"`
	Calculation  CalculationInfo  `json:"calculation"`
	Package      *PackageMeta     `json:"package,omitempty"`
	Warnings     []string         `json:"warnings,omitempty"`
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
	HasMore  bool  `json:"has_more"`
}

type PackageSummary struct {
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	LatestVersion string    `json:"latest_version,omitempty"`
	Status        string    `json:"status"`
	Category      string    `json:"category"`
	Tags          []string  `json:"tags,omitempty"`
	Author        string    `json:"author,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	DownloadCount int64     `json:"download_count"`
	ReferenceCount int64    `json:"reference_count"`
	StarCount     int64     `json:"star_count"`
	Starred       bool      `json:"starred"`
}

type PackageDetail struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	LatestVersion string           `json:"latest_version,omitempty"`
	Versions      []VersionSummary `json:"versions"`
	Tags          []string         `json:"tags,omitempty"`
	Category      string           `json:"category"`
	Visibility    string           `json:"visibility"`
	StarCount     int64            `json:"star_count"`
	Starred       bool             `json:"starred"`
}

type VersionSummary struct {
	Version      string     `json:"version"`
	Status       string     `json:"status"`
	Type         string     `json:"type"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	DeprecatedAt *time.Time `json:"deprecated_at,omitempty"`
}

type VersionDetail struct {
	Package      PackageMeta    `json:"package"`
	Dependencies []DependencyMeta `json:"dependencies"`
	PublishedBy  string         `json:"published_by,omitempty"`
	PublishedAt  *time.Time     `json:"published_at,omitempty"`
	Events       []VersionEvent `json:"events,omitempty"`
}

type VersionEvent struct {
	EventType   string    `json:"event_type"`
	ActorName   string    `json:"actor_name,omitempty"`
	Message     string    `json:"message,omitempty"`
	Version     string    `json:"version,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type ContentResponse struct {
	Package  PackageMeta `json:"package"`
	Content  string      `json:"content"`
	Readme   string      `json:"readme,omitempty"`
	Checksum string      `json:"checksum"`
}

func NewRegistryService(db *gorm.DB, cfg config.Config) *RegistryService {
	return &RegistryService{db: db, cfg: cfg, sdk: promptssdk.New(), executor: buildLLMExecutor(cfg)}
}

// buildLLMExecutor 根据配置构建 SDK 的 LLM 指令执行器；未配置 LLM 时返回 nil，
// SDK 在命中指令时会返回 ErrInstructionFailed。
func buildLLMExecutor(cfg config.Config) promptssdk.InstructionExecutor {
	if cfg.LLM.Model == "" || cfg.LLM.APIBase == "" {
		return nil
	}
	executor, err := promptssdk.NewLLMExecutor(promptssdk.LLMConfig{
		Model:   cfg.LLM.Model,
		APIBase: cfg.LLM.APIBase,
		APIKey:  cfg.LLM.APIKey,
		Proxy:   cfg.LLM.Proxy,
		Timeout: time.Duration(cfg.LLM.Timeout),
	})
	if err != nil {
		return nil
	}
	return executor
}

func (s *RegistryService) Search(q string, category string, tag string, status string, typ string, sort string, userID string, page int, pageSize int) (Page[PackageSummary], error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > s.cfg.Limits.PageSizeMax {
		pageSize = s.cfg.Limits.PageSizeMax
	}
	query := s.db.Model(&model.Package{}).Where("visibility = ?", "PUBLIC")
	if q != "" {
		like := "%" + q + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if tag != "" {
		query = query.Joins("JOIN package_tags ON package_tags.package_id = packages.id").Where("package_tags.normalized_tag = ?", strings.ToLower(tag))
	}
	if status != "" || typ != "" {
		query = query.Joins("JOIN package_versions ON package_versions.package_id = packages.id")
		if status != "" {
			query = query.Where("package_versions.status = ?", status)
		}
		if typ != "" {
			query = query.Where("package_versions.type = ?", typ)
		}
	}
	var total int64
	if err := query.Distinct("packages.id").Count(&total).Error; err != nil {
		return Page[PackageSummary]{}, err
	}
	var packages []model.Package
	if err := query.Distinct("packages.*").Order(searchOrderClause(sort)).Limit(pageSize).Offset((page - 1) * pageSize).Find(&packages).Error; err != nil {
		return Page[PackageSummary]{}, err
	}
	starred := s.starredSet(userID, packageIDs(packages))
	downloads := s.downloadCounts(packageIDs(packages))
	references := s.referenceCounts(packageNames(packages))
	items := make([]PackageSummary, 0, len(packages))
	for _, pkg := range packages {
		tags, _ := s.tags(pkg.ID)
		latestStatus := ""
		if pkg.LatestVersion != "" {
			var version model.PackageVersion
			if s.db.Where("package_id = ? AND version = ?", pkg.ID, pkg.LatestVersion).First(&version).Error == nil {
				latestStatus = version.Status
			}
		}
		items = append(items, PackageSummary{Name: pkg.Name, Description: pkg.Description, LatestVersion: pkg.LatestVersion, Status: latestStatus, Category: pkg.Category, Tags: tags, UpdatedAt: pkg.UpdatedAt, DownloadCount: downloads[pkg.ID], ReferenceCount: references[pkg.Name], StarCount: pkg.StarCount, Starred: starred[pkg.ID]})
	}
	return Page[PackageSummary]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (s *RegistryService) Package(name string, userID string) (PackageDetail, error) {
	pkg, err := s.findPackage(name)
	if err != nil {
		return PackageDetail{}, err
	}
	var versions []model.PackageVersion
	if err := s.db.Where("package_id = ?", pkg.ID).Order("published_at desc, version desc").Find(&versions).Error; err != nil {
		return PackageDetail{}, err
	}
	outVersions := make([]VersionSummary, 0, len(versions))
	for _, v := range versions {
		outVersions = append(outVersions, VersionSummary{Version: v.Version, Status: v.Status, Type: v.Type, PublishedAt: v.PublishedAt, DeprecatedAt: v.DeprecatedAt})
	}
	tags, _ := s.tags(pkg.ID)
	starred := false
	if userID != "" {
		var star model.PackageStar
		if s.db.Where("package_id = ? AND user_id = ?", pkg.ID, userID).First(&star).Error == nil {
			starred = true
		}
	}
	return PackageDetail{Name: pkg.Name, Description: pkg.Description, LatestVersion: pkg.LatestVersion, Versions: outVersions, Tags: tags, Category: pkg.Category, Visibility: pkg.Visibility, StarCount: pkg.StarCount, Starred: starred}, nil
}

func (s *RegistryService) Version(name string, versionRule string) (VersionDetail, error) {
	pkg, version, err := s.resolvePackageVersion(name, versionRule, false)
	if err != nil {
		return VersionDetail{}, err
	}
	tags, _ := s.tags(pkg.ID)
	deps, err := s.dependencies(version.ID)
	if err != nil {
		return VersionDetail{}, err
	}
	publishedByName := s.userName(version.PublishedBy)
	// 事件按包聚合：包含该包所有版本的生命周期事件（创建、发布、计算、废弃等），
	// 而非仅当前选中版本，便于在详情页展示完整版本演进时间线。
	var rows []struct {
		EventType   string
		ActorUserID string
		Message     string
		CreatedAt   time.Time
		Version     string
	}
	s.db.Table("package_version_events").
		Select("package_version_events.event_type, package_version_events.actor_user_id, package_version_events.message, package_version_events.created_at, package_versions.version").
		Joins("JOIN package_versions ON package_versions.id = package_version_events.package_version_id").
		Where("package_versions.package_id = ?", pkg.ID).
		Order("package_version_events.created_at desc").
		Limit(200).
		Scan(&rows)
	events := make([]VersionEvent, 0, len(rows))
	for _, r := range rows {
		events = append(events, VersionEvent{EventType: r.EventType, ActorName: s.userName(r.ActorUserID), Message: r.Message, Version: r.Version, CreatedAt: r.CreatedAt})
	}
	return VersionDetail{Package: PackageMeta{Name: pkg.Name, Version: version.Version, Description: version.Description, Status: version.Status, Category: pkg.Category, Tags: tags, Type: version.Type, Expression: version.Expression, Dependencies: deps, CreatedAt: version.CreatedAt}, Dependencies: deps, PublishedBy: publishedByName, PublishedAt: version.PublishedAt, Events: events}, nil
}

func (s *RegistryService) userName(userID string) string {
	if userID == "" {
		return ""
	}
	var user model.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return ""
	}
	if user.DisplayName != "" {
		return user.DisplayName
	}
	return user.Username
}

func (s *RegistryService) Content(name string, versionRule string, userID string, ip string, userAgent string, trackDownload bool) (ContentResponse, error) {
	_, version, err := s.resolvePackageVersion(name, versionRule, false)
	if err != nil {
		return ContentResponse{}, err
	}
	var content model.PackageContent
	if err := s.db.Where("package_version_id = ?", version.ID).First(&content).Error; err != nil {
		return ContentResponse{}, err
	}
	if trackDownload {
		_ = s.db.Create(&model.DownloadRecord{PackageVersionID: version.ID, UserID: userID, Channel: "URL", IPAddress: ip, UserAgent: userAgent}).Error
	}
	detail, err := s.Version(name, version.Version)
	if err != nil {
		return ContentResponse{}, err
	}
	return ContentResponse{Package: detail.Package, Content: content.Content, Readme: content.Readme, Checksum: content.Checksum}, nil
}

func (s *RegistryService) Publish(ctx context.Context, req PublishRequest, user *model.User) (PublishResponse, error) {
	if user == nil {
		return PublishResponse{}, errors.New("authentication required")
	}
	if err := validateMeta(req.Package); err != nil {
		return PublishResponse{}, err
	}
	if req.Content == "" {
		return PublishResponse{}, errors.New("content is required")
	}
	deps, err := s.extractDependencies(req.Package.Expression)
	if err != nil {
		return PublishResponse{}, err
	}
	resp := PublishResponse{Package: req.Package, Dependencies: deps}
	if req.Package.Type == "COMPOSITE" && req.Compute {
		generated, err := s.runExpression(ctx, GenerateRequest{Expression: req.Package.Expression, CacheStrategy: "NONE"}, user)
		if err != nil {
			return PublishResponse{}, err
		}
		req.Content = generated.Content
		resp.Calculation = &generated.Calculation
	}
	if req.DryRun {
		resp.Package.Dependencies = deps
		return resp, nil
	}
	now := time.Now().UTC()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Package
		if err := tx.Where("name = ?", req.Package.Name).First(&existing).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		pkg := existing
		if pkg.ID == "" {
			visibility := req.Package.Visibility
			if visibility == "" {
				visibility = "PUBLIC"
			}
			pkg = model.Package{Name: req.Package.Name, ShortName: shortName(req.Package.Name), Description: req.Package.Description, Category: req.Package.Category, Visibility: visibility, OwnerUserID: user.ID}
			if err := tx.Create(&pkg).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{"description": req.Package.Description, "category": req.Package.Category}
			if req.Package.Visibility != "" {
				updates["visibility"] = req.Package.Visibility
			}
			if err := tx.Model(&pkg).Updates(updates).Error; err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&model.PackageVersion{}).Where("package_id = ? AND version = ?", pkg.ID, req.Package.Version).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("version already exists")
		}
		contentChecksum := checksum(req.Content)
		depChecksum := checksum(fmt.Sprintf("%v", deps))
		version := model.PackageVersion{PackageID: pkg.ID, Version: req.Package.Version, Status: "PUBLISHED", Type: req.Package.Type, Description: req.Package.Description, Expression: req.Package.Expression, ContentChecksum: contentChecksum, DependencyChecksum: depChecksum, CacheStrategy: s.cfg.Cache.DefaultStrategy, CreatedBy: user.ID, PublishedBy: user.ID, PublishedAt: &now}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.PackageContent{PackageVersionID: version.ID, Content: req.Content, Checksum: contentChecksum, SizeBytes: int64(len(req.Content))}).Error; err != nil {
			return err
		}
		for _, tag := range req.Package.Tags {
			normalized := strings.ToLower(strings.TrimSpace(tag))
			if normalized == "" {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.PackageTag{PackageID: pkg.ID, Tag: tag, NormalizedTag: normalized}).Error; err != nil {
				return err
			}
		}
		for _, dep := range deps {
			if err := tx.Create(&model.PackageDependency{PackageVersionID: version.ID, DependencyPackageName: dep.Name, VersionRule: dep.VersionRule, ResolvedVersion: dep.ResolvedVersion, Optional: dep.Optional, AffectsCompute: true}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&pkg).Update("latest_version", req.Package.Version).Error; err != nil {
			return err
		}
		return tx.Create(&model.PackageVersionEvent{PackageVersionID: version.ID, EventType: "PUBLISHED", ActorUserID: user.ID, Message: "version published"}).Error
	})
	if err != nil {
		return PublishResponse{}, err
	}
	resp.Package.Status = "PUBLISHED"
	resp.Package.Dependencies = deps
	resp.Package.CreatedAt = now
	return resp, nil
}

func (s *RegistryService) Deprecate(name string, versionRule string, reason string, replacement string, user *model.User) error {
	if user == nil {
		return errors.New("authentication required")
	}
	_, version, err := s.resolvePackageVersion(name, versionRule, true)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&version).Updates(map[string]any{"status": "DEPRECATED", "deprecated_at": &now, "deprecated_reason": reason, "replacement": replacement}).Error; err != nil {
			return err
		}
		return tx.Create(&model.PackageVersionEvent{PackageVersionID: version.ID, EventType: "DEPRECATED", ActorUserID: user.ID, Message: reason}).Error
	})
}

func (s *RegistryService) Resolve(name string, versionRule string, includeDeprecated bool) (DependencyMeta, error) {
	_, version, err := s.resolvePackageVersion(name, versionRule, includeDeprecated)
	if err != nil {
		return DependencyMeta{}, err
	}
	return DependencyMeta{Name: name, VersionRule: normalizeRule(versionRule), ResolvedVersion: version.Version, AffectsCompute: true}, nil
}

func (s *RegistryService) ResolveDependencies(expression string) ([]DependencyMeta, error) {
	deps, err := s.extractDependencies(expression)
	if err != nil {
		return nil, err
	}
	for i := range deps {
		resolved, err := s.Resolve(deps[i].Name, deps[i].VersionRule, false)
		if err == nil {
			deps[i].ResolvedVersion = resolved.ResolvedVersion
		}
	}
	return deps, nil
}

func (s *RegistryService) Generate(ctx context.Context, req GenerateRequest, user *model.User) (GenerateResponse, error) {
	return s.runExpression(ctx, req, user)
}

func (s *RegistryService) Cache(page int, pageSize int) (Page[model.CalculationCache], error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > s.cfg.Limits.PageSizeMax {
		pageSize = s.cfg.Limits.PageSizeMax
	}
	var total int64
	if err := s.db.Model(&model.CalculationCache{}).Count(&total).Error; err != nil {
		return Page[model.CalculationCache]{}, err
	}
	var items []model.CalculationCache
	if err := s.db.Order("updated_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		return Page[model.CalculationCache]{}, err
	}
	return Page[model.CalculationCache]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (s *RegistryService) ClearCache(expiredOnly bool) error {
	query := s.db.Model(&model.CalculationCache{})
	if expiredOnly {
		query = query.Where("expires_at IS NOT NULL AND expires_at < ?", time.Now().UTC())
	}
	return query.Delete(&model.CalculationCache{}).Error
}

func (s *RegistryService) runExpression(ctx context.Context, req GenerateRequest, user *model.User) (GenerateResponse, error) {
	if req.Expression == "" {
		return GenerateResponse{}, errors.New("expression is required")
	}
	if s.cfg.Limits.ExpressionLength > 0 && len(req.Expression) > s.cfg.Limits.ExpressionLength {
		return GenerateResponse{}, errors.New("expression is too long")
	}
	strategy := req.CacheStrategy
	if strategy == "" {
		strategy = s.cfg.Cache.DefaultStrategy
	}
	expressionHash := checksum(req.Expression)
	contextHash := checksum(fmt.Sprintf("%v", req.Inputs))
	deps, err := s.ResolveDependencies(req.Expression)
	if err != nil {
		return GenerateResponse{}, err
	}
	depChecksum := checksum(fmt.Sprintf("%v", deps))
	cacheKey := checksum(expressionHash + depChecksum + contextHash + strategy)
	start := time.Now()
	if strategy != "NONE" && !req.Sensitive {
		var cached model.CalculationCache
		if err := s.db.Where("cache_key = ?", cacheKey).First(&cached).Error; err == nil && (cached.ExpiresAt == nil || cached.ExpiresAt.After(time.Now().UTC())) {
			now := time.Now().UTC()
			_ = s.db.Model(&cached).Updates(map[string]any{"hit_count": gorm.Expr("hit_count + ?", 1), "last_hit_at": &now}).Error
			calc := CalculationInfo{Expression: req.Expression, Dependencies: deps, CacheHit: true, CacheKey: cacheKey, TokenUsage: promptssdk.TokenUsage{Input: cached.TokenInput, Output: cached.TokenOutput, Total: cached.TokenTotal}, ComputedAt: now}
			hit := model.GenerationRecord{RequestUserID: userID(user), Expression: req.Expression, ExpressionHash: expressionHash, InputHash: contextHash, ResultChecksum: checksum(cached.ResultContent), CacheHit: true, CacheKey: cacheKey, TokenInput: cached.TokenInput, TokenOutput: cached.TokenOutput, TokenTotal: cached.TokenTotal, DurationMS: time.Since(start).Milliseconds()}
			_ = s.db.Create(&hit).Error
			return GenerateResponse{Content: cached.ResultContent, Dependencies: deps, Calculation: calc}, nil
		}
	}
	result, err := s.sdk.Run(ctx, promptssdk.RunRequest{Expression: req.Expression, Context: req.Inputs, InstructionExecutor: s.executor, ContentLoader: promptssdk.ContentLoaderFunc(func(ctx context.Context, ref promptssdk.DependencyRef) (promptssdk.Content, error) {
		_, version, err := s.resolvePackageVersion(ref.Name, ref.VersionRule, false)
		if err != nil {
			return promptssdk.Content{}, err
		}
		var content model.PackageContent
		if err := s.db.Where("package_version_id = ?", version.ID).First(&content).Error; err != nil {
			return promptssdk.Content{}, err
		}
		return promptssdk.Content{Name: ref.Name, Version: version.Version, Text: content.Content}, nil
	}), Options: promptssdk.RuntimeOptions{MaxDepth: s.cfg.Limits.DependencyDepth}})
	calc := CalculationInfo{Expression: req.Expression, Dependencies: deps, CacheHit: false, CacheKey: cacheKey, TokenUsage: result.Calculation.TokenUsage, ComputedAt: time.Now().UTC()}
	generation := model.GenerationRecord{RequestUserID: userID(user), Expression: req.Expression, ExpressionHash: expressionHash, InputHash: contextHash, ResultChecksum: checksum(result.Content), CacheHit: false, CacheKey: cacheKey, TokenInput: result.Calculation.TokenUsage.Input, TokenOutput: result.Calculation.TokenUsage.Output, TokenTotal: result.Calculation.TokenUsage.Total, DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		generation.ErrorCode = "GENERATION_FAILED"
		generation.ErrorMessage = err.Error()
		_ = s.db.Create(&generation).Error
		return GenerateResponse{}, err
	}
	_ = s.db.Create(&generation).Error
	if strategy != "NONE" && !req.Sensitive {
		expires := time.Now().UTC().Add(time.Duration(s.cfg.Cache.TTL))
		_ = s.db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "cache_key"}}, DoUpdates: clause.AssignmentColumns([]string{"result_content", "result_checksum", "token_input", "token_output", "token_total", "expires_at", "updated_at"})}).Create(&model.CalculationCache{CacheKey: cacheKey, CacheStrategy: strategy, ExpressionHash: expressionHash, DependencyChecksum: depChecksum, ContextHash: contextHash, ResultContent: result.Content, ResultChecksum: checksum(result.Content), TokenInput: result.Calculation.TokenUsage.Input, TokenOutput: result.Calculation.TokenUsage.Output, TokenTotal: result.Calculation.TokenUsage.Total, ExpiresAt: &expires}).Error
	}
	return GenerateResponse{Content: result.Content, Dependencies: deps, Calculation: calc, Warnings: result.Warnings}, nil
}

func (s *RegistryService) extractDependencies(expression string) ([]DependencyMeta, error) {
	if expression == "" {
		return nil, nil
	}
	parsed, err := s.sdk.Parse(promptssdk.ParseRequest{Expression: expression})
	if err != nil {
		return nil, err
	}
	out := make([]DependencyMeta, 0, len(parsed.Dependencies))
	for _, dep := range parsed.Dependencies {
		out = append(out, DependencyMeta{Name: dep.Name, VersionRule: dep.VersionRule, AffectsCompute: true})
	}
	return out, nil
}

func (s *RegistryService) dependencies(versionID string) ([]DependencyMeta, error) {
	var deps []model.PackageDependency
	if err := s.db.Where("package_version_id = ?", versionID).Find(&deps).Error; err != nil {
		return nil, err
	}
	out := make([]DependencyMeta, 0, len(deps))
	for _, dep := range deps {
		out = append(out, DependencyMeta{Name: dep.DependencyPackageName, VersionRule: dep.VersionRule, ResolvedVersion: dep.ResolvedVersion, Optional: dep.Optional, AffectsCompute: dep.AffectsCompute})
	}
	return out, nil
}

func (s *RegistryService) findPackage(name string) (model.Package, error) {
	var pkg model.Package
	return pkg, s.db.Where("name = ?", name).First(&pkg).Error
}

func (s *RegistryService) resolvePackageVersion(name string, versionRule string, includeDeprecated bool) (model.Package, model.PackageVersion, error) {
	pkg, err := s.findPackage(name)
	if err != nil {
		return model.Package{}, model.PackageVersion{}, err
	}
	var versions []model.PackageVersion
	query := s.db.Where("package_id = ?", pkg.ID)
	if !includeDeprecated {
		query = query.Where("status <> ?", "DEPRECATED")
	}
	if err := query.Find(&versions).Error; err != nil {
		return model.Package{}, model.PackageVersion{}, err
	}
	if len(versions) == 0 {
		return model.Package{}, model.PackageVersion{}, errors.New("version not found")
	}
	sort.Slice(versions, func(i, j int) bool { return compareSemver(versions[i].Version, versions[j].Version) > 0 })
	rule := normalizeRule(versionRule)
	for _, version := range versions {
		if matchesRule(version.Version, rule) {
			return pkg, version, nil
		}
	}
	return model.Package{}, model.PackageVersion{}, errors.New("version not found")
}

func (s *RegistryService) tags(packageID string) ([]string, error) {
	var tags []model.PackageTag
	if err := s.db.Where("package_id = ?", packageID).Order("normalized_tag asc").Find(&tags).Error; err != nil {
		return nil, err
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		out = append(out, tag.Tag)
	}
	return out, nil
}

func validateMeta(meta PackageMeta) error {
	if !packageNameRegexp.MatchString(meta.Name) {
		return errors.New("invalid package name")
	}
	if meta.Version == "" || compareSemver(meta.Version, "0.0.0") < 0 {
		return errors.New("invalid package version")
	}
	if meta.Description == "" {
		return errors.New("package description is required")
	}
	if meta.Category == "" {
		return errors.New("package category is required")
	}
	if meta.Type != "PLAIN" && meta.Type != "COMPOSITE" {
		return errors.New("package type must be PLAIN or COMPOSITE")
	}
	if meta.Type == "COMPOSITE" && meta.Expression == "" {
		return errors.New("composite package expression is required")
	}
	return nil
}

func checksum(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func shortName(name string) string {
	if strings.HasPrefix(name, "@") {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return name
}

func userID(user *model.User) string {
	if user == nil {
		return ""
	}
	return user.ID
}

func normalizeRule(rule string) string {
	if rule == "" || rule == "*" {
		return "latest"
	}
	return rule
}

func matchesRule(version string, rule string) bool {
	if rule == "latest" {
		return true
	}
	if strings.HasPrefix(rule, "^") {
		base := strings.TrimPrefix(rule, "^")
		return major(version) == major(base) && compareSemver(version, base) >= 0
	}
	if strings.HasPrefix(rule, "~") {
		base := strings.TrimPrefix(rule, "~")
		return major(version) == major(base) && minor(version) == minor(base) && compareSemver(version, base) >= 0
	}
	if strings.HasPrefix(rule, ">=") {
		return compareSemver(version, strings.TrimPrefix(rule, ">=")) >= 0
	}
	if strings.HasPrefix(rule, "<") {
		return compareSemver(version, strings.TrimPrefix(rule, "<")) < 0
	}
	return version == rule
}

func compareSemver(a string, b string) int {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	parts := strings.Split(v, ".")
	out := [3]int{-1, -1, -1}
	for i := 0; i < len(parts) && i < 3; i++ {
		var n int
		if _, err := fmt.Sscanf(parts[i], "%d", &n); err != nil {
			return out
		}
		out[i] = n
	}
	return out
}

func major(v string) int { return parseSemver(v)[0] }
func minor(v string) int { return parseSemver(v)[1] }

// ParseExpression 解析表达式为 JSON 友好的 AST 结构，供前端反向渲染为图形块
func (s *RegistryService) ParseExpression(expression string) (map[string]any, error) {
	parsed, err := s.sdk.Parse(promptssdk.ParseRequest{Expression: expression})
	if err != nil {
		return nil, err
	}
	return map[string]any{"ast": toASTJSON(parsed.AST), "dependencies": toDepJSON(parsed.Dependencies)}, nil
}

func toASTJSON(node promptssdk.Node) map[string]any {
	switch n := node.(type) {
	case *promptssdk.PromptRefNode:
		return map[string]any{"kind": "prompt_ref", "name": n.Name, "version_rule": n.VersionRule}
	case *promptssdk.StringNode:
		return map[string]any{"kind": "string", "value": n.Value}
	case *promptssdk.VariableNode:
		return map[string]any{"kind": "variable", "name": n.Name}
	case *promptssdk.CallNode:
		args := make([]any, 0, len(n.Args))
		for _, arg := range n.Args {
			args = append(args, toASTJSON(arg))
		}
		return map[string]any{"kind": "call", "name": n.Name, "option": n.Option, "args": args}
	case *promptssdk.BinaryNode:
		return map[string]any{"kind": "binary", "op": n.Op, "left": toASTJSON(n.Left), "right": toASTJSON(n.Right)}
	case *promptssdk.GroupNode:
		return map[string]any{"kind": "group", "expr": toASTJSON(n.Expr)}
	}
	return map[string]any{"kind": "unknown"}
}

func toDepJSON(deps []promptssdk.DependencyRef) []map[string]any {
	out := make([]map[string]any, 0, len(deps))
	for _, d := range deps {
		out = append(out, map[string]any{"name": d.Name, "version_rule": d.VersionRule})
	}
	return out
}

// --- 用户维度查询 ---

// searchOrderClause 根据排序键返回白名单化的 ORDER BY 子句。键为固定枚举，安全拼接。
func searchOrderClause(sort string) string {
	dl := "(SELECT COUNT(*) FROM download_records dr JOIN package_versions pv ON pv.id = dr.package_version_id WHERE pv.package_id = packages.id)"
	ref := "(SELECT COUNT(*) FROM package_dependencies pd WHERE pd.dependency_package_name = packages.name)"
	switch sort {
	case "stars":
		return "packages.star_count DESC, " + dl + " DESC, packages.updated_at DESC, packages.name"
	case "downloads":
		return dl + " DESC, packages.star_count DESC, packages.updated_at DESC, packages.name"
	case "references":
		return ref + " DESC, " + dl + " DESC, packages.updated_at DESC, packages.name"
	default: // updated / 最新
		return "packages.updated_at DESC, packages.name"
	}
}

// packageIDs 提取包的 ID 列表，供批量查询收藏/下载状态使用。
func packageIDs(packages []model.Package) []string {
	out := make([]string, 0, len(packages))
	for _, p := range packages {
		out = append(out, p.ID)
	}
	return out
}

// packageNames 提取包名列表，供按名聚合引用数使用。
func packageNames(packages []model.Package) []string {
	out := make([]string, 0, len(packages))
	for _, p := range packages {
		out = append(out, p.Name)
	}
	return out
}

// downloadCounts 按包聚合各版本下载记录的总和，返回 packageID -> 下载总数。
func (s *RegistryService) downloadCounts(ids []string) map[string]int64 {
	out := map[string]int64{}
	if len(ids) == 0 {
		return out
	}
	type row struct {
		PackageID string
		Count     int64
	}
	var rows []row
	s.db.Table("download_records").
		Select("package_versions.package_id as package_id, count(*) as count").
		Joins("JOIN package_versions ON package_versions.id = download_records.package_version_id").
		Where("package_versions.package_id IN ?", ids).
		Group("package_versions.package_id").
		Scan(&rows)
	for _, r := range rows {
		out[r.PackageID] = r.Count
	}
	return out
}

// referenceCounts 按包名聚合被复合提示词引用的次数（distinct 引用方版本数），返回 包名 -> 引用数。
func (s *RegistryService) referenceCounts(names []string) map[string]int64 {
	out := map[string]int64{}
	if len(names) == 0 {
		return out
	}
	type row struct {
		Name  string
		Count int64
	}
	var rows []row
	s.db.Table("package_dependencies").
		Select("dependency_package_name as name, count(distinct package_version_id) as count").
		Where("dependency_package_name IN ?", names).
		Group("dependency_package_name").
		Scan(&rows)
	for _, r := range rows {
		out[r.Name] = r.Count
	}
	return out
}

// starredSet 查询当前用户对给定包集合中已收藏的包 ID，返回集合映射。userID 为空时返回空集。
func (s *RegistryService) starredSet(userID string, ids []string) map[string]bool {
	out := map[string]bool{}
	if userID == "" || len(ids) == 0 {
		return out
	}
	var stars []model.PackageStar
	s.db.Where("user_id = ? AND package_id IN ?", userID, ids).Find(&stars)
	for _, st := range stars {
		out[st.PackageID] = true
	}
	return out
}

// Star 收藏包；同一用户对同一包仅计一次（唯一索引保证幂等）。
func (s *RegistryService) Star(name string, user *model.User) (int64, error) {
	if user == nil {
		return 0, errors.New("authentication required")
	}
	pkg, err := s.findPackage(name)
	if err != nil {
		return 0, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing model.PackageStar
		if err := tx.Where("package_id = ? AND user_id = ?", pkg.ID, user.ID).First(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			// 未收藏：插入。并发下若唯一索引冲突，视为已收藏。
			if err := tx.Create(&model.PackageStar{PackageID: pkg.ID, UserID: user.ID}).Error; err != nil {
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return s.syncStarCount(pkg.ID), nil
}

// Unstar 取消收藏；幂等。
func (s *RegistryService) Unstar(name string, user *model.User) (int64, error) {
	if user == nil {
		return 0, errors.New("authentication required")
	}
	pkg, err := s.findPackage(name)
	if err != nil {
		return 0, err
	}
	if err := s.db.Where("package_id = ? AND user_id = ?", pkg.ID, user.ID).Delete(&model.PackageStar{}).Error; err != nil {
		return 0, err
	}
	return s.syncStarCount(pkg.ID), nil
}

// syncStarCount 按实际收藏行数重算并回写包的 star_count，返回最新值，避免缓存计数漂移。
func (s *RegistryService) syncStarCount(packageID string) int64 {
	var n int64
	s.db.Model(&model.PackageStar{}).Where("package_id = ?", packageID).Count(&n)
	s.db.Model(&model.Package{}).Where("id = ?", packageID).UpdateColumn("star_count", n)
	return n
}

// MyStars 返回当前用户收藏的包列表。
func (s *RegistryService) MyStars(userID string, page, pageSize int) (Page[PackageSummary], error) {
	if page < 1 { page = 1 }
	if pageSize <= 0 || pageSize > s.cfg.Limits.PageSizeMax { pageSize = s.cfg.Limits.PageSizeMax }
	var total int64
	s.db.Model(&model.PackageStar{}).Where("user_id = ?", userID).Count(&total)
	var packages []model.Package
	s.db.Joins("JOIN package_stars ON package_stars.package_id = packages.id").
		Where("package_stars.user_id = ?", userID).
		Order("packages.updated_at desc").
		Limit(pageSize).Offset((page - 1) * pageSize).Find(&packages)
	items := make([]PackageSummary, 0, len(packages))
	downloads := s.downloadCounts(packageIDs(packages))
	references := s.referenceCounts(packageNames(packages))
	for _, pkg := range packages {
		tags, _ := s.tags(pkg.ID)
		latestStatus := ""
		if pkg.LatestVersion != "" {
			var v model.PackageVersion
			if s.db.Where("package_id = ? AND version = ?", pkg.ID, pkg.LatestVersion).First(&v).Error == nil { latestStatus = v.Status }
		}
		items = append(items, PackageSummary{Name: pkg.Name, Description: pkg.Description, LatestVersion: pkg.LatestVersion, Status: latestStatus, Category: pkg.Category, Tags: tags, UpdatedAt: pkg.UpdatedAt, DownloadCount: downloads[pkg.ID], ReferenceCount: references[pkg.Name], StarCount: pkg.StarCount, Starred: true})
	}
	return Page[PackageSummary]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (s *RegistryService) MyPackages(userID string, page, pageSize int) (Page[PackageSummary], error) {
	if page < 1 { page = 1 }
	if pageSize <= 0 || pageSize > s.cfg.Limits.PageSizeMax { pageSize = s.cfg.Limits.PageSizeMax }
	var total int64
	s.db.Model(&model.Package{}).Where("owner_user_id = ? AND deleted_at IS NULL", userID).Count(&total)
	var packages []model.Package
	s.db.Where("owner_user_id = ? AND deleted_at IS NULL", userID).Order("updated_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&packages)
	items := make([]PackageSummary, 0, len(packages))
	pkgDownloads := s.downloadCounts(packageIDs(packages))
	pkgReferences := s.referenceCounts(packageNames(packages))
	for _, pkg := range packages {
		tags, _ := s.tags(pkg.ID)
		latestStatus := ""
		if pkg.LatestVersion != "" {
			var v model.PackageVersion
			if s.db.Where("package_id = ? AND version = ?", pkg.ID, pkg.LatestVersion).First(&v).Error == nil { latestStatus = v.Status }
		}
		items = append(items, PackageSummary{Name: pkg.Name, Description: pkg.Description, LatestVersion: pkg.LatestVersion, Status: latestStatus, Category: pkg.Category, Tags: tags, UpdatedAt: pkg.UpdatedAt, DownloadCount: pkgDownloads[pkg.ID], ReferenceCount: pkgReferences[pkg.Name]})
	}
	return Page[PackageSummary]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (s *RegistryService) MyGenerations(userID string, page, pageSize int) (Page[model.GenerationRecord], error) {
	if page < 1 { page = 1 }
	if pageSize <= 0 || pageSize > s.cfg.Limits.PageSizeMax { pageSize = s.cfg.Limits.PageSizeMax }
	var total int64
	s.db.Model(&model.GenerationRecord{}).Where("request_user_id = ?", userID).Count(&total)
	var items []model.GenerationRecord
	s.db.Where("request_user_id = ?", userID).Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&items)
	return Page[model.GenerationRecord]{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

type NamespaceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Visibility  string `json:"visibility"`
	Role        string `json:"role"`
}

func (s *RegistryService) MyNamespaces(userID string) ([]NamespaceSummary, error) {
	out := []NamespaceSummary{}
	// owned
	var owned []model.Namespace
	s.db.Where("owner_user_id = ? AND deleted_at IS NULL", userID).Find(&owned)
	for _, ns := range owned {
		out = append(out, NamespaceSummary{ID: ns.ID, Name: ns.Name, DisplayName: ns.DisplayName, Visibility: ns.Visibility, Role: "OWNER"})
	}
	// member
	var members []model.NamespaceMember
	s.db.Where("user_id = ? AND deleted_at IS NULL", userID).Find(&members)
	for _, m := range members {
		// skip if already in out (owned)
		skip := false
		for _, o := range out { if o.ID == m.NamespaceID { skip = true; break } }
		if skip { continue }
		var ns model.Namespace
		if s.db.First(&ns, "id = ?", m.NamespaceID).Error == nil {
			out = append(out, NamespaceSummary{ID: ns.ID, Name: ns.Name, DisplayName: ns.DisplayName, Visibility: ns.Visibility, Role: m.Role})
		}
	}
	return out, nil
}
