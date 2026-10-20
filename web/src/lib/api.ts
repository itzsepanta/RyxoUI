/**
 * Thin client for the panel API.
 *
 * The session lives in an HttpOnly cookie, so every request sends
 * credentials. Errors are normalised into ApiError so the pages can show a
 * single message shape.
 */

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: unknown;
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || "request failed");
    this.status = status;
    this.code = body.code;
  }
}

export interface ListMeta {
  count: number;
  limit: number;
  offset: number;
  total?: number;
}

export interface ListResponse<T> {
  data: T[];
  meta: ListMeta;
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
    ...init,
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const payload = text ? JSON.parse(text) : {};

  if (!response.ok) {
    const body = (payload as { error?: ApiErrorBody }).error ?? {
      code: "unknown",
      message: response.statusText,
    };
    throw new ApiError(response.status, body);
  }
  return payload as T;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "POST", body: body === undefined ? undefined : JSON.stringify(body) }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: "PUT", body: body === undefined ? undefined : JSON.stringify(body) }),
  delete: <T>(path: string, body?: unknown) =>
    request<T>(path, {
      method: "DELETE",
      body: body === undefined ? undefined : JSON.stringify(body),
    }),
};

export interface Operator {
  id: string;
  username: string;
  roleId: string;
  roleName: string;
  isOwner: boolean;
  status: string;
  lastLoginAt: number;
  lastLoginIp: string;
  totpEnabled: boolean;
  createdAt: number;
  updatedAt: number;
}

export interface Role {
  id: string;
  name: string;
  base: string;
  builtin: boolean;
  description: string;
  scopes: string[];
  userLimit: number;
  maxDeviceLimit: number;
  maxDuration: number;
  createdAt: number;
  updatedAt: number;
}

export interface ScopeInfo {
  name: string;
  description: string;
  group: string;
}

export interface AuditEntry {
  id: string;
  actorId: string;
  actorName: string;
  action: string;
  entity: string;
  entityId: string;
  before?: unknown;
  after?: unknown;
  ip: string;
  userAgent: string;
  requestId: string;
  createdAt: number;
}

export interface ApiToken {
  id: string;
  name: string;
  scopes: string[];
  lastUsedAt: number;
  expiresAt: number;
  createdAt: number;
  plaintext?: string;
}

export interface Me {
  operator: Operator;
  role: Role;
  scopes: string[];
  viaToken: boolean;
}

export interface SetupState {
  initialized: boolean;
  operators: number;
}
