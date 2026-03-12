// EdgeFabric API client — wraps fetch with auth, error handling, and envelope unwrapping.

const TOKEN_KEY = 'edgefabric-token';

// --- Token helpers ---

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

// --- Error types ---

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

// --- Envelope types ---

interface ApiResponse<T> {
  data?: T;
  error?: { code: string; message: string };
}

export interface ListResult<T> {
  items: T[];
  total: number;
  offset: number;
  limit: number;
}

interface ApiListResponse<T> {
  data: T[];
  total: number;
  offset: number;
  limit: number;
}

// --- Core fetch wrapper ---

async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...((options.headers as Record<string, string>) || {}),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(path, {
    ...options,
    headers,
  });

  // Handle 401 — redirect to login
  if (response.status === 401) {
    clearToken();
    // Only redirect if we're not already on the login page
    if (window.location.pathname !== '/login') {
      window.location.href = '/login';
    }
    throw new ApiError(401, 'unauthorized', 'Authentication required');
  }

  // Handle 204 No Content
  if (response.status === 204) {
    return undefined as T;
  }

  const body = await response.json();

  if (!response.ok || body.error) {
    const err = body.error || { code: 'unknown', message: 'Request failed' };
    throw new ApiError(response.status, err.code, err.message);
  }

  return body as T;
}

// --- Convenience methods ---

/** GET a single resource — unwraps { data: T } envelope. */
export async function apiGet<T>(path: string): Promise<T> {
  const resp = await apiFetch<ApiResponse<T>>(path);
  return resp.data as T;
}

/** GET a list of resources — unwraps { data: T[], total, offset, limit } envelope. */
export async function apiList<T>(path: string, params?: Record<string, string | number | undefined>): Promise<ListResult<T>> {
  const url = new URL(path, window.location.origin);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined && value !== '') {
        url.searchParams.set(key, String(value));
      }
    }
  }

  const resp = await apiFetch<ApiListResponse<T>>(url.pathname + url.search);
  return {
    items: resp.data || [],
    total: resp.total || 0,
    offset: resp.offset || 0,
    limit: resp.limit || 0,
  };
}

/** POST — sends JSON body, unwraps { data: T } envelope. */
export async function apiPost<T>(path: string, body?: unknown): Promise<T> {
  const resp = await apiFetch<ApiResponse<T>>(path, {
    method: 'POST',
    body: body ? JSON.stringify(body) : undefined,
  });
  return resp.data as T;
}

/** PUT — sends JSON body, unwraps { data: T } envelope. */
export async function apiPut<T>(path: string, body?: unknown): Promise<T> {
  const resp = await apiFetch<ApiResponse<T>>(path, {
    method: 'PUT',
    body: body ? JSON.stringify(body) : undefined,
  });
  return resp.data as T;
}

/** DELETE — no response body expected. */
export async function apiDelete(path: string): Promise<void> {
  await apiFetch<void>(path, { method: 'DELETE' });
}

/** POST for login — returns raw response (not envelope-wrapped). */
export async function apiLogin(email: string, password: string): Promise<{ token: string; totp_required: boolean }> {
  const resp = await apiFetch<ApiResponse<{ token: string; totp_required: boolean }>>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });
  return resp.data as { token: string; totp_required: boolean };
}

/** POST for TOTP verification. */
export async function apiVerifyTotp(code: string): Promise<{ token: string }> {
  const resp = await apiFetch<ApiResponse<{ token: string }>>('/api/v1/auth/totp/verify', {
    method: 'POST',
    body: JSON.stringify({ code }),
  });
  return resp.data as { token: string };
}

/** POST for TOTP verification using an explicit pending token (not stored in localStorage). */
export async function apiVerifyTotpWithToken(code: string, pendingToken: string): Promise<{ token: string }> {
  const response = await fetch('/api/v1/auth/totp/verify', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${pendingToken}`,
    },
    body: JSON.stringify({ code }),
  });

  if (!response.ok) {
    const body = await response.json();
    const err = body.error || { code: 'unknown', message: 'Request failed' };
    throw new ApiError(response.status, err.code, err.message);
  }

  const body = await response.json();
  return body.data as { token: string };
}
