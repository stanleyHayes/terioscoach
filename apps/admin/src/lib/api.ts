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
  mfaEnabled: boolean;
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
  login(email: string, password: string, code?: string): Promise<AuthResponse> {
    return request<AuthResponse>("/v1/auth/login", {
      method: "POST",
      body: { email, password, ...(code ? { code } : {}) },
    });
  },

  forgotPassword(email: string): Promise<void> {
    return request<void>("/v1/auth/forgot-password", { method: "POST", body: { email } });
  },

  resetPassword(token: string, password: string): Promise<void> {
    return request<void>("/v1/auth/reset-password", { method: "POST", body: { token, password } });
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

  beginMfa(accessToken: string): Promise<{ secret: string; otpAuthUrl: string }> {
    return request("/v1/auth/mfa/enroll", { method: "POST", accessToken });
  },

  confirmMfa(accessToken: string, code: string): Promise<void> {
    return request("/v1/auth/mfa/confirm", { method: "POST", accessToken, body: { code } });
  },

  disableMfa(accessToken: string, code: string): Promise<void> {
    return request("/v1/auth/mfa/disable", { method: "POST", accessToken, body: { code } });
  },
};

export type Session = AuthTokens;

export interface RefreshCallbacks {
  /** Called with the rotated tokens after a successful refresh — persist them. */
  onTokensRefreshed: (tokens: AuthTokens) => void;
}

/* ---------------------------------------------------------------------------
 * Single-flight refresh.
 *
 * A refresh token is one-time: the API rotates it and, on seeing a revoked one
 * presented again, treats it as a stolen token and revokes the whole session
 * family (auth.Service.Refresh). That is the right rule against theft, and it
 * makes a naive per-request refresh actively dangerous here, because expiry is
 * never observed by one request alone. `/forms` mounts two authenticated reads
 * at once and `/content` mounts four; fifteen minutes after sign-in they all
 * 401 together. Refreshing per request would send the same token two, three,
 * four times over — the first rotates it, the rest look exactly like a replay,
 * and the practitioner is signed out for the crime of opening a page.
 *
 * So the rotation is shared. Concurrent callers await one in-flight refresh,
 * and a caller whose snapshot has already been rotated past takes the newer
 * tokens rather than presenting its dead one.
 * ------------------------------------------------------------------------- */

/** The refresh currently in progress, if any — concurrent 401s await this. */
let refreshInFlight: Promise<AuthTokens> | null = null;
/** The newest tokens this module has issued, for callers holding a stale set. */
let latestTokens: AuthTokens | null = null;

/**
 * Forgets the rotation state. The auth provider calls this whenever it
 * establishes or ends a session, so tokens from a previous sign-in are never
 * handed to a caller in the next one.
 */
export function resetTokenRotation(tokens: AuthTokens | null = null): void {
  refreshInFlight = null;
  latestTokens = tokens;
}

/** Rotates `session`'s refresh token, joining an in-flight rotation if there
 * is one and skipping it entirely if this caller is already behind. */
function rotate(session: Session, callbacks: RefreshCallbacks): Promise<AuthTokens> {
  // Someone else already rotated past this caller's snapshot. Presenting the
  // token it holds is precisely the replay the server revokes for.
  if (latestTokens !== null && latestTokens.refreshToken !== session.refreshToken) {
    return Promise.resolve(latestTokens);
  }
  if (refreshInFlight !== null) {
    return refreshInFlight;
  }

  refreshInFlight = authApi
    .refresh(session.refreshToken)
    // refresh returns the full AuthResponse (user included); the callback
    // contract is AuthTokens only, so strip the user before reporting.
    .then(({ accessToken, refreshToken }) => {
      const tokens: AuthTokens = { accessToken, refreshToken };
      latestTokens = tokens;
      callbacks.onTokensRefreshed(tokens);
      return tokens;
    })
    .finally(() => {
      refreshInFlight = null;
    });
  return refreshInFlight;
}

/**
 * Authenticated request with the contract's 401 recovery: try the request once;
 * on 401 refresh the token exactly once and retry exactly once; if the refresh
 * or the retry is still unauthorized, throw SessionExpiredError so the caller
 * can force a logout.
 *
 * "Exactly once" is per session, not per request — see the single-flight note
 * above. Ten simultaneous 401s produce one rotation and ten retries.
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
    tokens = await rotate(session, callbacks);
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      throw new SessionExpiredError();
    }
    throw error;
  }

  try {
    return await request<T>(path, { ...options, accessToken: tokens.accessToken });
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      throw new SessionExpiredError();
    }
    throw error;
  }
}
