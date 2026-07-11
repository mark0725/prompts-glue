package model

import "time"

type Base struct {
	ID        string     `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

type User struct {
	Base
	Username     string     `gorm:"uniqueIndex;size:128;not null" json:"username"`
	Email        string     `gorm:"uniqueIndex;size:255" json:"email,omitempty"`
	DisplayName  string     `gorm:"size:255" json:"display_name,omitempty"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	Status       string     `gorm:"index;size:32;not null" json:"status"`
	Role         string     `gorm:"index;size:32;not null" json:"role"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

type AccessToken struct {
	ID         string     `gorm:"primaryKey;size:36" json:"id"`
	UserID     string     `gorm:"index;size:36;not null" json:"user_id"`
	TokenHash  string     `gorm:"uniqueIndex;size:128;not null" json:"-"`
	Name       string     `gorm:"size:255" json:"name,omitempty"`
	Scopes     string     `gorm:"type:text" json:"scopes,omitempty"`
	ExpiresAt  *time.Time `gorm:"index" json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Namespace struct {
	Base
	Name        string `gorm:"uniqueIndex;size:128;not null" json:"name"`
	DisplayName string `gorm:"size:255" json:"display_name,omitempty"`
	OwnerUserID string `gorm:"index;size:36;not null" json:"owner_user_id"`
	Visibility  string `gorm:"index;size:32;not null" json:"visibility"`
	Description string `gorm:"type:text" json:"description,omitempty"`
}

type NamespaceMember struct {
	Base
	NamespaceID string `gorm:"uniqueIndex:idx_namespace_user;index;size:36;not null" json:"namespace_id"`
	UserID      string `gorm:"uniqueIndex:idx_namespace_user;index;size:36;not null" json:"user_id"`
	Role        string `gorm:"index;size:32;not null" json:"role"`
}

type Package struct {
	Base
	NamespaceID   string `gorm:"index;size:36" json:"namespace_id,omitempty"`
	Name          string `gorm:"uniqueIndex;size:214;not null" json:"name"`
	ShortName     string `gorm:"index;size:214;not null" json:"short_name"`
	Description   string `gorm:"type:text;not null" json:"description"`
	Category      string `gorm:"index;size:64;not null" json:"category"`
	Visibility    string `gorm:"index;size:32;not null" json:"visibility"`
	LatestVersion string `gorm:"size:64" json:"latest_version,omitempty"`
	OwnerUserID   string `gorm:"index;size:36;not null" json:"owner_user_id"`
	StarCount     int64  `gorm:"index" json:"star_count"`
}

type PackageVersion struct {
	Base
	PackageID          string     `gorm:"uniqueIndex:idx_package_version;index;size:36;not null" json:"package_id"`
	Version            string     `gorm:"uniqueIndex:idx_package_version;size:64;not null" json:"version"`
	Status             string     `gorm:"index;size:32;not null" json:"status"`
	Type               string     `gorm:"index;size:32;not null" json:"type"`
	Description        string     `gorm:"type:text;not null" json:"description"`
	Expression         string     `gorm:"type:text" json:"expression,omitempty"`
	ContentChecksum    string     `gorm:"size:128;not null" json:"content_checksum"`
	DependencyChecksum string     `gorm:"size:128" json:"dependency_checksum,omitempty"`
	ComputeTiming      string     `gorm:"size:32" json:"compute_timing,omitempty"`
	CacheStrategy      string     `gorm:"size:32" json:"cache_strategy,omitempty"`
	TokenInput         int        `json:"token_input,omitempty"`
	TokenOutput        int        `json:"token_output,omitempty"`
	TokenTotal         int        `json:"token_total,omitempty"`
	CreatedBy          string     `gorm:"index;size:36;not null" json:"created_by"`
	PublishedBy        string     `gorm:"size:36" json:"published_by,omitempty"`
	PublishedAt        *time.Time `gorm:"index" json:"published_at,omitempty"`
	DeprecatedAt       *time.Time `json:"deprecated_at,omitempty"`
	DeprecatedReason   string     `gorm:"type:text" json:"deprecated_reason,omitempty"`
	Replacement        string     `gorm:"size:255" json:"replacement,omitempty"`
}

type PackageContent struct {
	Base
	PackageVersionID string `gorm:"uniqueIndex;size:36;not null" json:"package_version_id"`
	Content          string `gorm:"type:text;not null" json:"content"`
	Readme           string `gorm:"type:text" json:"readme,omitempty"`
	Checksum         string `gorm:"index;size:128;not null" json:"checksum"`
	SizeBytes        int64  `json:"size_bytes"`
}

type PackageTag struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	PackageID     string    `gorm:"uniqueIndex:idx_package_tag;index;size:36;not null" json:"package_id"`
	Tag           string    `gorm:"size:64;not null" json:"tag"`
	NormalizedTag string    `gorm:"uniqueIndex:idx_package_tag;index;size:64;not null" json:"normalized_tag"`
	CreatedAt     time.Time `json:"created_at"`
}

type PackageDependency struct {
	Base
	PackageVersionID         string `gorm:"index;size:36;not null" json:"package_version_id"`
	DependencyPackageName    string `gorm:"index;size:214;not null" json:"dependency_package_name"`
	DependencyPackageID      string `gorm:"index;size:36" json:"dependency_package_id,omitempty"`
	VersionRule              string `gorm:"size:64" json:"version_rule,omitempty"`
	ResolvedVersion          string `gorm:"size:64" json:"resolved_version,omitempty"`
	ResolvedPackageVersionID string `gorm:"index;size:36" json:"resolved_package_version_id,omitempty"`
	Optional                 bool   `json:"optional"`
	AffectsCompute           bool   `json:"affects_compute"`
	PositionStart            int    `json:"position_start,omitempty"`
	PositionEnd              int    `json:"position_end,omitempty"`
}

type PackageVersionEvent struct {
	ID               string    `gorm:"primaryKey;size:36" json:"id"`
	PackageVersionID string    `gorm:"index;size:36;not null" json:"package_version_id"`
	EventType        string    `gorm:"index;size:64;not null" json:"event_type"`
	ActorUserID      string    `gorm:"index;size:36" json:"actor_user_id,omitempty"`
	Message          string    `gorm:"type:text" json:"message,omitempty"`
	Metadata         string    `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

type GenerationRecord struct {
	ID               string    `gorm:"primaryKey;size:36" json:"id"`
	RequestUserID    string    `gorm:"index;size:36" json:"request_user_id,omitempty"`
	PackageVersionID string    `gorm:"index;size:36" json:"package_version_id,omitempty"`
	Expression       string    `gorm:"type:text;not null" json:"expression"`
	ExpressionHash   string    `gorm:"index;size:128;not null" json:"expression_hash"`
	InputHash        string    `gorm:"size:128" json:"input_hash,omitempty"`
	ResultChecksum   string    `gorm:"size:128" json:"result_checksum,omitempty"`
	CacheHit         bool      `json:"cache_hit"`
	CacheKey         string    `gorm:"index;size:128" json:"cache_key,omitempty"`
	ErrorCode        string    `gorm:"index;size:64" json:"error_code,omitempty"`
	ErrorMessage     string    `gorm:"type:text" json:"error_message,omitempty"`
	TokenInput       int       `json:"token_input,omitempty"`
	TokenOutput      int       `json:"token_output,omitempty"`
	TokenTotal       int       `json:"token_total,omitempty"`
	ModelName        string    `gorm:"size:128" json:"model_name,omitempty"`
	DurationMS       int64     `json:"duration_ms,omitempty"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

type CalculationCache struct {
	Base
	CacheKey           string     `gorm:"uniqueIndex;size:128;not null" json:"cache_key"`
	CacheStrategy      string     `gorm:"size:32;not null" json:"cache_strategy"`
	ExpressionHash     string     `gorm:"index;size:128;not null" json:"expression_hash"`
	DependencyChecksum string     `gorm:"index;size:128;not null" json:"dependency_checksum"`
	ContextHash        string     `gorm:"size:128" json:"context_hash,omitempty"`
	ResultContent      string     `gorm:"type:text;not null" json:"result_content"`
	ResultChecksum     string     `gorm:"size:128;not null" json:"result_checksum"`
	TokenInput         int        `json:"token_input,omitempty"`
	TokenOutput        int        `json:"token_output,omitempty"`
	TokenTotal         int        `json:"token_total,omitempty"`
	HitCount           int64      `json:"hit_count"`
	ExpiresAt          *time.Time `gorm:"index" json:"expires_at,omitempty"`
	LastHitAt          *time.Time `gorm:"index" json:"last_hit_at,omitempty"`
}

type DictItem struct {
	Base
	Group       string `gorm:"uniqueIndex:idx_dict_group_value;index;size:64;not null" json:"group"`
	Value       string `gorm:"uniqueIndex:idx_dict_group_value;size:128;not null" json:"value"`
	Label       string `gorm:"size:255;not null" json:"label"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	SortOrder   int    `json:"sort_order"`
	Enabled     bool   `gorm:"index" json:"enabled"`
	System      bool   `json:"system"`
}

type ServiceConfig struct {
	Base
	Key         string `gorm:"uniqueIndex;size:128;not null" json:"key"`
	Value       string `gorm:"type:text;not null" json:"value"`
	ValueType   string `gorm:"size:32;not null" json:"value_type"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	Encrypted   bool   `json:"encrypted"`
	UpdatedBy   string `gorm:"index;size:36" json:"updated_by,omitempty"`
}

type AuditLog struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	ActorUserID  string    `gorm:"index;size:36" json:"actor_user_id,omitempty"`
	Action       string    `gorm:"index;size:64;not null" json:"action"`
	ResourceType string    `gorm:"index;size:64;not null" json:"resource_type"`
	ResourceID   string    `gorm:"index;size:36" json:"resource_id,omitempty"`
	RequestID    string    `gorm:"index;size:64" json:"request_id,omitempty"`
	IPAddress    string    `gorm:"size:64" json:"ip_address,omitempty"`
	UserAgent    string    `gorm:"size:512" json:"user_agent,omitempty"`
	BeforeData   string    `gorm:"type:text" json:"before_data,omitempty"`
	AfterData    string    `gorm:"type:text" json:"after_data,omitempty"`
	Result       string    `gorm:"size:32;not null" json:"result"`
	Message      string    `gorm:"type:text" json:"message,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

type DownloadRecord struct {
	ID               string    `gorm:"primaryKey;size:36" json:"id"`
	PackageVersionID string    `gorm:"index;size:36;not null" json:"package_version_id"`
	UserID           string    `gorm:"index;size:36" json:"user_id,omitempty"`
	Channel          string    `gorm:"index;size:32;not null" json:"channel"`
	IPAddress        string    `gorm:"size:64" json:"ip_address,omitempty"`
	UserAgent        string    `gorm:"size:512" json:"user_agent,omitempty"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

// PackageStar 记录用户对包的收藏关系；同一用户对同一包仅一条（复合唯一索引保证）。
type PackageStar struct {
	Base
	PackageID string `gorm:"uniqueIndex:idx_star_user_pkg;index;size:36;not null" json:"package_id"`
	UserID    string `gorm:"uniqueIndex:idx_star_user_pkg;index;size:36;not null" json:"user_id"`
}

type RiskMark struct {
	Base
	ResourceType   string     `gorm:"index;size:64;not null" json:"resource_type"`
	ResourceID     string     `gorm:"index;size:36;not null" json:"resource_id"`
	RiskType       string     `gorm:"index;size:64;not null" json:"risk_type"`
	Severity       string     `gorm:"index;size:32;not null" json:"severity"`
	Reason         string     `gorm:"type:text;not null" json:"reason"`
	Status         string     `gorm:"index;size:32;not null" json:"status"`
	ReporterUserID string     `gorm:"size:36" json:"reporter_user_id,omitempty"`
	HandledBy      string     `gorm:"size:36" json:"handled_by,omitempty"`
	HandledAt      *time.Time `json:"handled_at,omitempty"`
}
