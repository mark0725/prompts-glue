import { request } from "./client";
import type { AccessToken, AuditLog, CacheEntry, DictItem, GenerationRecord, LoginResponse, PackageSummary, Page, Permission, Role, RoleDetail, RoleMember, User } from "@/types";

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
    request<LoginResponse>("/api/admin/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  me: () => request<User>("/api/admin/auth/me"),
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
  users: (q?: string, status?: string, page = 1, pageSize = 20) =>
    request<Page<User>>(`/api/admin/users${qs({ q, status, page, page_size: pageSize })}`),
  managerUsers: (q?: string, status?: string, page = 1, pageSize = 20) =>
    request<Page<User>>(`/api/admin/manager-users${qs({ q, status, page, page_size: pageSize })}`),
  createWebUser: (body: { username: string; password: string; email?: string; display_name?: string }) =>
    request<User>("/api/admin/users", { method: "POST", body: JSON.stringify(body) }),
  createManagerUser: (body: { username: string; password: string; email?: string; display_name?: string; role?: string }) =>
    request<User>("/api/admin/manager-users", { method: "POST", body: JSON.stringify(body) }),
  updateWebUser: (id: string, body: { display_name?: string; email?: string; status?: string }) =>
    request<{ updated: boolean }>(`/api/admin/users/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  updateManagerUser: (id: string, body: { display_name?: string; email?: string; status?: string; role?: string }) =>
    request<{ updated: boolean }>(`/api/admin/manager-users/${id}`, { method: "PATCH", body: JSON.stringify(body) }),
  deleteWebUser: (id: string) => request<{ deleted: boolean }>(`/api/admin/users/${id}`, { method: "DELETE" }),
  deleteManagerUser: (id: string) => request<{ deleted: boolean }>(`/api/admin/manager-users/${id}`, { method: "DELETE" }),
  resetWebPassword: (id: string, password: string) =>
    request<{ reset: boolean }>(`/api/admin/users/${id}/reset-password`, { method: "POST", body: JSON.stringify({ password }) }),
  resetManagerPassword: (id: string, password: string) =>
    request<{ reset: boolean }>(`/api/admin/manager-users/${id}/reset-password`, { method: "POST", body: JSON.stringify({ password }) }),
  syncLDAP: () => request<{ synced_manager: number; synced_web: number; errors: string[] }>("/api/admin/ldap/sync", { method: "POST" }),
  roles: () => request<Role[]>("/api/admin/roles"),
  rbacRole: (id: string) => request<RoleDetail>(`/api/admin/roles/${id}`),
  createRole: (body: { code: string; name: string; description?: string; enabled: boolean }) =>
    request<Role>("/api/admin/roles", { method: "POST", body: JSON.stringify(body) }),
  updateRole: (id: string, body: { name?: string; description?: string; enabled?: boolean }) =>
    request<{ updated: boolean }>(`/api/admin/roles/${id}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteRole: (id: string) => request<{ deleted: boolean }>(`/api/admin/roles/${id}`, { method: "DELETE" }),
  setRolePermissions: (id: string, permission_ids: string[]) =>
    request<{ updated: boolean }>(`/api/admin/roles/${id}/permissions`, { method: "PUT", body: JSON.stringify({ permission_ids }) }),
  roleMembers: (id: string) => request<RoleMember[]>(`/api/admin/roles/${id}/members`),
  setRoleMembers: (id: string, user_ids: string[]) =>
    request<{ updated: boolean }>(`/api/admin/roles/${id}/members`, { method: "PUT", body: JSON.stringify({ user_ids }) }),
  permissions: () => request<Permission[]>("/api/admin/permissions"),
  userTokens: (scope: "users" | "manager-users", id: string) =>
    request<AccessToken[]>(`/api/admin/${scope}/${id}/tokens`),
  revokeUserToken: (scope: "users" | "manager-users", id: string, tid: string) =>
    request<{ revoked: boolean }>(`/api/admin/${scope}/${id}/tokens/${tid}`, { method: "DELETE" }),
  auditLogs: (page = 1, pageSize = 20) =>
    request<Page<AuditLog>>(`/api/admin/audit-logs${qs({ page, page_size: pageSize })}`),
  generationRecords: (page = 1, pageSize = 20) =>
    request<Page<GenerationRecord>>(`/api/admin/generation-records${qs({ page, page_size: pageSize })}`),
};
