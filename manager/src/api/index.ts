import { request } from "./client";
import type { AuditLog, CacheEntry, DictItem, GenerationRecord, LoginResponse, PackageSummary, Page, RoleSummary, User } from "@/types";

export interface PackageQuery {
  q?: string;
  category?: string;
  type?: string;
  status?: string;
  page?: number;
  page_size?: number;
}

function qs(params: Record<string, unknown>): string {
  const sp = new URLSearchParams();
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== "") sp.set(k, String(v));
  });
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export const api = {
  login: (username: string, password: string) =>
    request<LoginResponse>("/api/registry/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  me: () => request<User>("/api/registry/auth/me"),
  dictsAll: () => request<Record<string, DictItem[]>>("/api/admin/dicts"),
  dictGroup: (group: string) => request<DictItem[]>(`/api/dicts/${group}`),
  searchPackages: (query: PackageQuery = {}) =>
    request<Page<PackageSummary>>(`/api/registry/packages${qs({ ...query })}`),
  deprecate: (name: string, version: string, reason: string) =>
    request<{ status: string }>(`/api/registry/packages/${encodeURIComponent(name)}/versions/${encodeURIComponent(version)}/deprecate`, {
      method: "POST",
      body: JSON.stringify({ reason }),
    }),
  cacheList: (page = 1, pageSize = 20) =>
    request<Page<CacheEntry>>(`/api/registry/cache${qs({ page, page_size: pageSize })}`),
  clearCache: (expiredOnly = false) =>
    request<{ cleared: boolean }>("/api/registry/cache", {
      method: "DELETE",
      body: JSON.stringify({ expired_only: expiredOnly }),
    }),
  users: () => request<User[]>("/api/admin/users"),
  roles: () => request<RoleSummary>("/api/admin/roles"),
  auditLogs: (page = 1, pageSize = 20) =>
    request<Page<AuditLog>>(`/api/admin/audit-logs${qs({ page, page_size: pageSize })}`),
  generationRecords: (page = 1, pageSize = 20) =>
    request<Page<GenerationRecord>>(`/api/admin/generation-records${qs({ page, page_size: pageSize })}`),
};
