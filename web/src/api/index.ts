import { request } from "./client";
import type {
  AccessToken,
  ASTNode,
  ContentResponse,
  CreateTokenResponse,
  Dependency,
  GenerateResponse,
  GenerationRecord,
  LoginResponse,
  NamespaceSummary,
  Page,
  PackageDetail,
  PackageSummary,
  PublishResponse,
  User,
  VersionDetail,
} from "@/types";

export interface SearchParams {
  q?: string;
  category?: string;
  tag?: string;
  status?: string;
  type?: string;
  sort?: string;
  page?: number;
  page_size?: number;
}

function buildQuery(params: Record<string, unknown>): string {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value !== undefined && value !== null && value !== "") {
      query.set(key, String(value));
    }
  });
  const string = query.toString();
  return string ? `?${string}` : "";
}

export const api = {
  health: () => request<{ status: string; version: string; time: string }>("/api/registry/health"),
  login: (username: string, password: string) =>
    request<LoginResponse>("/api/registry/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password, client: "web" }),
    }),
  me: () => request<User>("/api/registry/auth/me"),
  searchPackages: (params: SearchParams = {}) =>
    request<Page<PackageSummary>>(`/api/registry/packages${buildQuery({ ...params })}`),
  package: (name: string) => request<PackageDetail>(`/api/registry/packages/${encodeURIComponent(name)}`),
  version: (name: string, version: string) =>
    request<VersionDetail>(`/api/registry/packages/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}`),
  content: (name: string, version: string, view = false) =>
    request<ContentResponse>(`/api/registry/packages/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}/content${view ? "?view=true" : ""}`),
  resolveDependencies: (expression: string) =>
    request<{ dependencies: Dependency[] }>("/api/registry/dependencies/resolve", {
      method: "POST",
      body: JSON.stringify({ expression }),
    }),
  parseExpression: (expression: string) =>
    request<{ ast: ASTNode; dependencies: Dependency[] }>("/api/registry/parse", {
      method: "POST",
      body: JSON.stringify({ expression }),
    }),
  generate: (opts: { expression: string; inputs?: Record<string, string>; cache_strategy?: string; error_strategy?: string; sensitive?: boolean }) =>
    request<GenerateResponse>("/api/registry/generate", {
      method: "POST",
      body: JSON.stringify({
        expression: opts.expression,
        inputs: opts.inputs ?? {},
        cache_strategy: opts.cache_strategy,
        error_strategy: opts.error_strategy,
        sensitive: opts.sensitive,
      }),
    }),
  publish: (packageMeta: Record<string, unknown>, content: string, dryRun = false) =>
    request<PublishResponse>("/api/registry/packages", {
      method: "POST",
      body: JSON.stringify({ package: packageMeta, content, dry_run: dryRun }),
    }),
  deprecate: (name: string, version: string, reason: string, replacement?: string) =>
    request<{ status: string }>(`/api/registry/packages/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}/deprecate`, {
      method: "POST",
      body: JSON.stringify({ reason, replacement }),
    }),
  dicts: () => request<Record<string, import("@/types").DictItem[]>>("/api/dicts"),
  dictGroup: (group: string) => request<import("@/types").DictItem[]>(`/api/dicts/${group}`),
  myPackages: (page = 1, pageSize = 20) =>
    request<Page<PackageSummary>>(`/api/registry/my-packages${buildQuery({ page, page_size: pageSize })}`),
  myStars: (page = 1, pageSize = 20) =>
    request<Page<PackageSummary>>(`/api/registry/my-stars${buildQuery({ page, page_size: pageSize })}`),
  star: (name: string) =>
    request<{ star_count: number; starred: boolean }>(`/api/registry/packages/${encodeURIComponent(name)}/star`, { method: "POST" }),
  unstar: (name: string) =>
    request<{ star_count: number; starred: boolean }>(`/api/registry/packages/${encodeURIComponent(name)}/star`, { method: "DELETE" }),
  myGenerations: (page = 1, pageSize = 20) =>
    request<Page<GenerationRecord>>(`/api/registry/my-generations${buildQuery({ page, page_size: pageSize })}`),
  myNamespaces: () => request<NamespaceSummary[]>("/api/registry/my-namespaces"),
  listTokens: () => request<AccessToken[]>("/api/registry/tokens"),
  createToken: (name: string, expires_at?: string) =>
    request<CreateTokenResponse>("/api/registry/tokens", {
      method: "POST",
      body: JSON.stringify({ name, expires_at: expires_at || undefined }),
    }),
  revokeToken: (id: string) =>
    request<{ revoked: boolean }>(`/api/registry/tokens/${encodeURIComponent(id)}`, { method: "DELETE" }),
};
