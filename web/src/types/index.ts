export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: ApiError;
  request_id: string;
}

export interface Page<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface DictItem {
  group: string;
  value: string;
  label: string;
  description?: string;
  sort_order: number;
  enabled: boolean;
  system: boolean;
}

export interface User {
  id: string;
  username: string;
  email?: string;
  display_name?: string;
  status: string;
  source?: string;
  last_login_at?: string;
}

export interface LoginResponse {
  access_token: string;
  expires_at?: string | null;
  user: User;
}

export interface AccessToken {
  id: string;
  name: string;
  scopes?: string;
  created_at: string;
  last_used_at?: string;
  expires_at?: string | null;
}

export interface CreateTokenResponse {
  access_token: string;
  token: AccessToken;
}

export interface Dependency {
  name: string;
  version_rule?: string;
  resolved_version?: string;
  optional?: boolean;
  affects_compute?: boolean;
}

export interface PackageMeta {
  name: string;
  version: string;
  description: string;
  status: string;
  category: string;
  tags?: string[];
  author?: string;
  type: string;
  expression?: string;
  dependencies?: Dependency[];
  created_at?: string;
}

export interface PackageSummary {
  name: string;
  description: string;
  latest_version?: string;
  status: string;
  category: string;
  tags?: string[];
  author?: string;
  updated_at: string;
  download_count: number;
  reference_count: number;
  star_count: number;
  starred?: boolean;
}

export interface VersionSummary {
  version: string;
  status: string;
  type: string;
  published_at?: string;
  deprecated_at?: string;
}

export interface PackageDetail {
  name: string;
  description: string;
  latest_version?: string;
  versions: VersionSummary[];
  tags?: string[];
  category: string;
  visibility: string;
  star_count: number;
  starred?: boolean;
}

export interface VersionEvent {
  event_type: string;
  actor_name?: string;
  message?: string;
  version?: string;
  created_at: string;
}

export interface VersionDetail {
  package: PackageMeta;
  dependencies: Dependency[];
  published_by?: string;
  published_at?: string;
  events?: VersionEvent[];
}

export interface ContentResponse {
  package: PackageMeta;
  content: string;
  readme?: string;
  checksum: string;
}

export interface ASTNode {
  kind: string;
  name?: string;
  value?: string;
  version_rule?: string;
  op?: string;
  option?: string;
  args?: ASTNode[];
  left?: ASTNode;
  right?: ASTNode;
  expr?: ASTNode;
}

export interface GenerationRecord {
  id: string;
  expression: string;
  cache_hit: boolean;
  token_input: number;
  token_output: number;
  token_total: number;
  model_name?: string;
  duration_ms?: number;
  error_code?: string;
  created_at: string;
}

export interface NamespaceSummary {
  id: string;
  name: string;
  display_name?: string;
  visibility: string;
  role: string;
}

export interface TokenUsage {
  input: number;
  output: number;
  total: number;
}

export interface CalculationInfo {
  expression: string;
  dependencies: Dependency[];
  cache_hit: boolean;
  cache_key?: string;
  token_usage?: TokenUsage;
  computed_at: string;
}

export interface GenerateResponse {
  content: string;
  dependencies: Dependency[];
  calculation: CalculationInfo;
  package?: PackageMeta;
  warnings?: string[];
}

export interface PublishResponse {
  package: PackageMeta;
  dependencies: Dependency[];
  calculation?: CalculationInfo;
  warnings?: string[];
}
