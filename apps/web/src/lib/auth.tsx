"use client";

/**
 * Auth context for the customer portal (apps/web).
 *
 * Token storage: the access token lives in memory (React state) only — it is
 * short-lived (15m) and never written to web storage, which limits the window
 * for XSS exfiltration. The refresh token is persisted in localStorage so a
 * session survives a page reload; on every reload we immediately rotate it
 * via POST /v1/auth/refresh. (The API contract returns tokens in the response
 * body, so an httpOnly cookie session is not available without a backend
 * change; localStorage is the accepted trade-off for v1.)
 *
 * Role: portal accounts are role "client" (registration always yields a
 * client). The role is exposed on `user` for future gating but v1 does not
 * reject practitioner sign-ins — the practice dashboard (apps/admin) owns
 * the inverse check.
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
import { authApi, type AuthTokens, type User } from "@/lib/api";

export const REFRESH_TOKEN_KEY = "terios.web.refreshToken";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export interface AuthContextValue {
  status: AuthStatus;
  user: User | null;
  accessToken: string | null;
  /** The full token set, for `authedRequest` calls (booking APIs). Null when
   * signed out. */
  session: AuthTokens | null;
  /** Persists rotated tokens after an automatic refresh — the
   * `RefreshCallbacks` half of `authedRequest`. */
  onTokensRefreshed: (tokens: AuthTokens) => void;
  /** Throws ApiError on failure (e.g. code "invalid_credentials"). */
  login: (email: string, password: string) => Promise<void>;
  /** Registers and signs in (201 returns tokens). Throws ApiError on failure
   * (e.g. code "email_taken", "validation_error"). */
  register: (name: string, email: string, password: string) => Promise<void>;
  /** Ends the session locally and invalidates the refresh token server-side. */
  logout: () => Promise<void>;
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
    try {
      localStorage.setItem(REFRESH_TOKEN_KEY, nextTokens.refreshToken);
    } catch {
      /* storage unavailable — session ends on reload */
    }
  }, []);

  // Restore the session on mount: rotate the persisted refresh token, then
  // load the profile via GET /v1/auth/me.
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
        if (!cancelled) applySession(nextTokens, nextUser);
      } catch {
        if (cancelled) return;
        clearSession();
      }
    }

    void restore();
    return () => {
      cancelled = true;
    };
  }, [applySession, clearSession]);

  const login = useCallback(
    async (email: string, password: string) => {
      const response = await authApi.login(email, password);
      applySession(response, response.user);
    },
    [applySession],
  );

  const register = useCallback(
    async (name: string, email: string, password: string) => {
      const response = await authApi.register(name, email, password);
      applySession(response, response.user);
    },
    [applySession],
  );

  const logout = useCallback(async () => {
    const refreshToken = tokens?.refreshToken;
    clearSession();
    if (refreshToken) {
      // Idempotent per contract — unknown tokens also 204. Best-effort.
      await authApi.logout(refreshToken).catch(() => {});
    }
  }, [tokens, clearSession]);

  /* authedRequest rotated the access token after a 401 — persist the new set
   * (same user, so only the tokens + stored refresh token change). */
  const onTokensRefreshed = useCallback((nextTokens: AuthTokens) => {
    setTokens(nextTokens);
    try {
      localStorage.setItem(REFRESH_TOKEN_KEY, nextTokens.refreshToken);
    } catch {
      /* storage unavailable — session ends on reload */
    }
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      user,
      accessToken: tokens?.accessToken ?? null,
      session: tokens,
      onTokensRefreshed,
      login,
      register,
      logout,
    }),
    [status, user, tokens, onTokensRefreshed, login, register, logout],
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
