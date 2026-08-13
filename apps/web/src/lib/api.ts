/**
 * Typed fetch client for the Terios API — customer portal (apps/web).
 *
 * Contract (design/api-contract.md, Auth section — final and live):
 *   POST /v1/auth/register  {email, password, name} → 201 {accessToken, accessTokenExpiresAt, refreshToken, user}
 *   POST /v1/auth/login     {email, password}       → 200 (same shape)
 *   POST /v1/auth/refresh   {refreshToken}          → 200 (rotated, includes user)
 *   POST /v1/auth/logout    {refreshToken}          → 204
 *   GET  /v1/auth/me        Authorization: Bearer <accessToken> → 200 {user}
 * Errors always arrive as {error: {code, message}}.
 *
 * Services (BE-03, final):
 *   GET  /v1/services       public, no auth → 200 {items: [service]}
 */

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export interface User {
  id: string;
  email: string;
  /** "client" for portal accounts; registration always yields "client". */
  role: string;
  name: string;
}

export interface AuthTokens {
  accessToken: string;
  /** RFC 3339 UTC timestamp. */
  accessTokenExpiresAt: string;
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
  /** Fetch cache mode. Public catalog reads pass "no-store" — prices are
   * edited in the dashboard and must always be served fresh. */
  cache?: RequestCache;
}

/** Low-level contract request. Exported for public (unauthenticated) reads
 * such as availability slots; authed endpoints should go through
 * `authedRequest` instead so 401s get the refresh-and-retry treatment. */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, accessToken, cache } = options;

  let response: Response;
  try {
    response = await fetch(`${API_BASE_URL}${path}`, {
      method,
      headers: {
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
      ...(cache !== undefined ? { cache } : {}),
    });
  } catch {
    throw new ApiError(
      0,
      "network_error",
      "We can't reach the server. Check your connection and try again.",
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
        : "Something went wrong on our side. Try again in a moment.",
    );
  }

  return data as T;
}

/** Auth endpoints per the contract. */
export const authApi = {
  register(name: string, email: string, password: string): Promise<AuthResponse> {
    return request<AuthResponse>("/v1/auth/register", {
      method: "POST",
      body: { email, password, name },
    });
  },

  login(email: string, password: string): Promise<AuthResponse> {
    return request<AuthResponse>("/v1/auth/login", {
      method: "POST",
      body: { email, password },
    });
  },

  forgotPassword(email: string): Promise<void> {
    return request<void>("/v1/auth/forgot-password", { method: "POST", body: { email } });
  },

  resetPassword(token: string, password: string): Promise<void> {
    return request<void>("/v1/auth/reset-password", { method: "POST", body: { token, password } });
  },

  refresh(refreshToken: string): Promise<AuthResponse> {
    return request<AuthResponse>("/v1/auth/refresh", {
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

/** Public catalog entry (contract §Services). The full server record carries
 * practitionerId/active/timestamps as well; the public pages only need these. */
export interface ServiceSummary {
  id: string;
  name: string;
  description: string;
  durationMinutes: number;
  /** Integer minor units, e.g. 45000 = GH₵450.00. */
  priceKobo: number;
  /** ISO 4217, "GHS" unless the practitioner set otherwise. */
  currency: string;
  sortOrder: number;
}

/**
 * GET /v1/services — public, no auth. Server-side safe (plain fetch).
 * Always fetched with cache: "no-store": practitioners edit prices and the
 * catalog in the dashboard, and the marketing pages must never serve a
 * stale cached render. ApiError propagates (e.g. 503 service_unavailable
 * when the database is not configured).
 */
export async function listServices(): Promise<ServiceSummary[]> {
  const { items } = await request<{ items: ServiceSummary[] }>("/v1/services", {
    cache: "no-store",
  });
  return items;
}

export interface RefreshCallbacks {
  /** Called with the rotated tokens after a successful refresh — persist them. */
  onTokensRefreshed: (tokens: AuthTokens) => void;
}

/**
 * Authenticated request with the contract's 401 recovery: try the request once;
 * on 401 (token_expired / token_invalid / unauthorized) refresh the token
 * exactly once and retry exactly once; if the refresh or the retry is still
 * unauthorized, throw SessionExpiredError so the caller can force a logout.
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
    // refresh returns the full AuthResponse (user included); the callback
    // contract is AuthTokens only, so strip the user before reporting.
    const { accessToken, accessTokenExpiresAt, refreshToken } = await authApi.refresh(
      session.refreshToken,
    );
    tokens = { accessToken, accessTokenExpiresAt, refreshToken };
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
