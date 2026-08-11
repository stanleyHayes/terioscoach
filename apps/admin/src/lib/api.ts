/**
 * Typed fetch client for the Terios API.
 *
 * Contract (fixed with the backend):
 *   POST /v1/auth/login    {email, password}   → 200 {accessToken, refreshToken, user}
 *   POST /v1/auth/refresh  {refreshToken}      → 200 {accessToken, refreshToken} (rotation)
 *   POST /v1/auth/logout   {refreshToken}      → 204
 *   GET  /v1/auth/me       Authorization: Bearer <accessToken> → 200 {user}
 * Errors always arrive as {error: {code, message}}.
 */

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface User {
  id: string;
  email: string;
  role: string;
  name: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
}

export interface AuthResponse extends AuthTokens {
  user: User;
}

/** API error carrying the contract's {error: {code, message}} shape. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

/** Thrown when a request is still unauthorized after a single token refresh:
 * the session is over and the caller must force a logout. */
export class SessionExpiredError extends Error {
  constructor() {
    super("Your session has expired. Sign in again.");
    this.name = "SessionExpiredError";
  }
}

interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  accessToken?: string;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, accessToken } = options;

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method,
      headers: {
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError(
      0,
      "network_error",
      "Can't reach the server. Check your connection and try again.",
    );
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const data: unknown = await response.json().catch(() => null);

  if (!response.ok) {
    const err =
      data !== null && typeof data === "object" && "error" in data
        ? (data as { error?: { code?: unknown; message?: unknown } }).error
        : undefined;
    throw new ApiError(
      response.status,
      typeof err?.code === "string" ? err.code : "unknown_error",
      typeof err?.message === "string"
        ? err.message
        : "Something went wrong. Try again.",
    );
  }

  return data as T;
}

/** Auth endpoints per the contract. Registration is portal-only and intentionally omitted. */
export const authApi = {
  login(email: string, password: string): Promise<AuthResponse> {
    return request<AuthResponse>("/v1/auth/login", {
      method: "POST",
      body: { email, password },
    });
  },

  refresh(refreshToken: string): Promise<AuthTokens> {
    return request<AuthTokens>("/v1/auth/refresh", {
      method: "POST",
      body: { refreshToken },
    });
  },

  logout(refreshToken: string): Promise<void> {
    return request<void>("/v1/auth/logout", {
      method: "POST",
      body: { refreshToken },
    });
  },

  me(accessToken: string): Promise<{ user: User }> {
    return request<{ user: User }>("/v1/auth/me", { accessToken });
  },
};

export type Session = AuthTokens;

export interface RefreshCallbacks {
  /** Called with the rotated tokens after a successful refresh — persist them. */
  onTokensRefreshed: (tokens: AuthTokens) => void;
}

/**
 * Authenticated request with the contract's 401 recovery: try the request once;
 * on 401 refresh the token exactly once and retry exactly once; if the refresh
 * or the retry is still unauthorized, throw SessionExpiredError so the caller
 * can force a logout.
 */
export async function authedRequest<T>(
  path: string,
  session: Session,
  callbacks: RefreshCallbacks,
  options: Omit<RequestOptions, "accessToken"> = {},
): Promise<T> {
  try {
    return await request<T>(path, { ...options, accessToken: session.accessToken });
  } catch (error) {
    if (!(error instanceof ApiError) || error.status !== 401) {
      throw error;
    }
  }

  let tokens: AuthTokens;
  try {
    tokens = await authApi.refresh(session.refreshToken);
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      throw new SessionExpiredError();
    }
    throw error;
  }
  callbacks.onTokensRefreshed(tokens);

  try {
    return await request<T>(path, { ...options, accessToken: tokens.accessToken });
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      throw new SessionExpiredError();
    }
    throw error;
  }
}
