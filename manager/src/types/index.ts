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
  role: string;
  last_login_at?: string;
  created_at?: string;
}

export interface LoginResponse {
  access_token: string;
  expires_at: string;
  user: User;
}

export interface PackageSummary {
  name: string;
  description: string;
  latest_version?: string;
  status: string;
  category: string;
  download_count: number;
  updated_at: string;
}

export interface Page<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

export interface CacheEntry {
  id: string;
  cache_key: string;
  cache_strategy: string;
  expression_hash: string;
  hit_count: number;
  expires_at?: string;
  updated_at: string;
}

export interface GenerationRecord {
  id: string;
  request_user_id?: string;
  package_version_id?: string;
  expression: string;
  expression_hash: string;
  cache_hit: boolean;
  cache_key?: string;
  error_code?: string;
  error_message?: string;
  token_input: number;
  token_output: number;
  token_total: number;
  model_name?: string;
  duration_ms?: number;
  created_at: string;
}

export interface AuditLog {
  id: string;
  actor_user_id?: string;
  action?: string;
  resource_type?: string;
  resource_id?: string;
  result?: string;
  message?: string;
  created_at: string;
}

export interface RoleAggregate {
  code: string;
  count: number;
}

export interface RoleSummary {
  account_roles: RoleAggregate[];
  namespace_roles: RoleAggregate[];
}
