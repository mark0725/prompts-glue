import type { ApiError, ApiResponse } from "@/types";

const TOKEN_STORAGE_KEY = "prompts-glue.web.token";

export class ApiRequestError extends Error {
  code: string;
  requestId?: string;
  status: number;
  details?: Record<string, unknown>;

  constructor(error: ApiError, status: number, requestId?: string) {
    super(error.message);
    this.code = error.code;
    this.status = status;
    this.requestId = requestId;
    this.details = error.details;
  }
}

export function getToken(): string {
  return localStorage.getItem(TOKEN_STORAGE_KEY) ?? "";
}

export function setToken(token: string): void {
  if (token) {
    localStorage.setItem(TOKEN_STORAGE_KEY, token);
  } else {
    localStorage.removeItem(TOKEN_STORAGE_KEY);
  }
}

export async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers ?? {});
  if (options.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const token = getToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const response = await fetch(path, { ...options, headers });
  const payload: ApiResponse<T> = await response.json().catch(() => ({
    success: false,
    error: { code: "SERVER_ERROR", message: "无法解析服务端响应" },
    request_id: "",
  }));
  if (!payload.success) {
    throw new ApiRequestError(payload.error ?? { code: "SERVER_ERROR", message: "未知错误" }, response.status, payload.request_id);
  }
  return payload.data as T;
}

export function apiError(err: unknown): ApiRequestError {
  if (err instanceof ApiRequestError) {
    return err;
  }
  if (err instanceof Error) {
    return new ApiRequestError({ code: "CLIENT_ERROR", message: err.message }, 0);
  }
  return new ApiRequestError({ code: "CLIENT_ERROR", message: "发生未知错误" }, 0);
}
