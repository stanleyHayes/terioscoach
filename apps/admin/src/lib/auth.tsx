"use client";

/**
 * Auth context for the practice dashboard.
 *
 * Token storage: the access token lives in memory (React state) only — it is
 * short-lived and never written to web storage, which limits the window for
 * XSS exfiltration. The refresh token is persisted in localStorage so a
 * session survives a page reload; on every reload we immediately rotate it
 * via POST /v1/auth/refresh. (The API contract returns tokens in the response
 * body, so an httpOnly cookie session is not available without a backend
 * change; localStorage is the accepted trade-off for v1.)
 *
 * Role enforcement: only practitioners and permission-scoped staff may use this app —
 * enforced both at login and when restoring a session via GET /v1/auth/me.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ApiError,
  authApi,
  resetTokenRotation,
  type AuthTokens,
  type RefreshCallbacks,
  type User,
} from "@/lib/api";

export const PRACTITIONER_ROLE = "practitioner";
export const STAFF_ROLE = "staff";
export const REFRESH_TOKEN_KEY = "terios.admin.refreshToken";
export const SIGN_OUT_MESSAGE_KEY = "terios.admin.signOutMessage";

export const NOT_PRACTITIONER_MESSAGE =
  "This account doesn't have practice-dashboard access. Sign in with an owner or staff account for this practice.";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export interface AuthContextValue {
  status: AuthStatus;
  user: User | null;
  accessToken: string | null;
  /** Live token pair for authedRequest call sites; null until authenticated. */
  session: AuthTokens | null;
  /** Persists rotated tokens — hand this to authedRequest as its RefreshCallbacks. */
  refreshCallbacks: RefreshCallbacks;
  /** Throws ApiError on failure (code "not_a_practitioner" for wrong role). */
  login: (email: string, password: string, code?: string) => Promise<void>;
  /** Updates the local profile after a verified MFA enrollment. */
  setMfaEnabled: (enabled: boolean) => void;
  /** Ends the session. `message` is handed to the login screen to display. */
  logout: (message?: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<AuthStatus>("loading");
  const [user, setUser] = useState<User | null>(null);
  const [tokens, setTokens] = useState<AuthTokens | null>(null);
  // Guards against the mount effect running twice in React StrictMode.
  const restoredRef = useRef(false);

  const clearSession = useCallback(() => {
    setTokens(null);
    setUser(null);
    setStatus("unauthenticated");
    // Drop the shared rotation state with the session, so the next sign-in
    // never inherits tokens belonging to the account that just left.
    resetTokenRotation(null);
    try {
      localStorage.removeItem(REFRESH_TOKEN_KEY);
    } catch {
      /* storage unavailable — session simply won't persist */
    }
  }, []);

  const applySession = useCallback((nextTokens: AuthTokens, nextUser: User) => {
    setTokens(nextTokens);
    setUser(nextUser);
    setStatus("authenticated");
    // These are now the newest tokens in play; tell the shared rotation so a
    // request still holding the pre-sign-in set doesn't present it.
    resetTokenRotation(nextTokens);
    try {
      localStorage.setItem(REFRESH_TOKEN_KEY, nextTokens.refreshToken);
    } catch {
      /* storage unavailable — session ends on reload */
    }
  }, []);

  // Restore the session on mount: rotate the persisted refresh token, then
  // load the profile and re-check the practitioner role.
  useEffect(() => {
    if (restoredRef.current) return;
    restoredRef.current = true;

    let cancelled = false;

    async function restore() {
      let refreshToken: string | null = null;
      try {
        refreshToken = localStorage.getItem(REFRESH_TOKEN_KEY);
      } catch {
        /* storage unavailable */
      }
      if (!refreshToken) {
        if (!cancelled) setStatus("unauthenticated");
        return;
      }
      try {
        const nextTokens = await authApi.refresh(refreshToken);
        const { user: nextUser } = await authApi.me(nextTokens.accessToken);
        if (![PRACTITIONER_ROLE, STAFF_ROLE].includes(nextUser.role)) {
          // Best-effort server-side invalidation of the rotated token.
          await authApi.logout(nextTokens.refreshToken).catch(() => {});
          throw new ApiError(
            403,
            "not_a_practitioner",
            NOT_PRACTITIONER_MESSAGE,
          );
        }
        if (!cancelled) applySession(nextTokens, nextUser);
      } catch (error) {
        if (cancelled) return;
        clearSession();
        if (error instanceof ApiError && error.code === "not_a_practitioner") {
          try {
            sessionStorage.setItem(SIGN_OUT_MESSAGE_KEY, error.message);
          } catch {
            /* storage unavailable */
          }
        }
      }
    }

    void restore();
    return () => {
      cancelled = true;
    };
  }, [applySession, clearSession]);

  const login = useCallback(
    async (email: string, password: string, code?: string) => {
      const response = await authApi.login(email, password, code);
      if (![PRACTITIONER_ROLE, STAFF_ROLE].includes(response.user.role)) {
        // Invalidate the just-issued refresh token; this account may not hold
        // a session on the practice dashboard.
        await authApi.logout(response.refreshToken).catch(() => {});
        throw new ApiError(403, "not_a_practitioner", NOT_PRACTITIONER_MESSAGE);
      }
      applySession(response, response.user);
    },
    [applySession],
  );

  const logout = useCallback(
    async (message?: string) => {
      const refreshToken = tokens?.refreshToken;
      clearSession();
      if (message) {
        try {
          sessionStorage.setItem(SIGN_OUT_MESSAGE_KEY, message);
        } catch {
          /* storage unavailable */
        }
      }
      if (refreshToken) {
        await authApi.logout(refreshToken).catch(() => {});
      }
    },
    [tokens, clearSession],
  );

  const setMfaEnabled = useCallback((enabled: boolean) => {
    setUser((current) =>
      current ? { ...current, mfaEnabled: enabled } : current,
    );
  }, []);

  // Handed to authedRequest callers: keeps the in-memory token pair (and the
  // persisted refresh token) current when a request triggers a rotation.
  const refreshCallbacks = useMemo<RefreshCallbacks>(
    () => ({
      onTokensRefreshed: (next: AuthTokens) => {
        setTokens(next);
        try {
          localStorage.setItem(REFRESH_TOKEN_KEY, next.refreshToken);
        } catch {
          /* storage unavailable — session ends on reload */
        }
      },
    }),
    [],
  );

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      accessToken: tokens?.accessToken ?? null,
      session: tokens,
      refreshCallbacks,
      login,
      setMfaEnabled,
      logout,
    }),
    [status, user, tokens, refreshCallbacks, login, logout, setMfaEnabled],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
