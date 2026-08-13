"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ApiError,
  SessionExpiredError,
  type RefreshCallbacks,
  type Session,
} from "./api";
import { useAuth } from "./auth";

/**
 * The portal's standard authenticated read.
 *
 * Same shape as the dashboard's: load once a session exists, keep the
 * first-load skeleton distinct from an empty result, and end the session
 * when a refresh can no longer save it. Written once here so every portal
 * screen behaves identically when the network misbehaves.
 */

export interface PortalResource<T> {
  /** null until the first load resolves — the skeleton state. */
  data: T | null;
  error: string | null;
  refresh: () => void;
  set: (update: T | ((previous: T | null) => T | null)) => void;
}

export function usePortalData<T>(
  load: (session: Session, callbacks: RefreshCallbacks) => Promise<T>,
  deps: unknown[] = [],
): PortalResource<T> {
  const { session, onTokensRefreshed, logout } = useAuth();
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  // The loader is written inline by each caller and would otherwise change
  // identity every render; the caller's own deps are the real signal. Both
  // rules want a literal here, which is exactly what a shared hook cannot
  // have.
  // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/use-memo
  const run = useCallback(load, deps);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;

    run(session, { onTokensRefreshed })
      .then((value) => {
        if (cancelled) return;
        setData(value);
        setError(null);
      })
      .catch((failure: unknown) => {
        if (cancelled) return;
        if (failure instanceof SessionExpiredError) {
          void logout();
          return;
        }
        setError(explain(failure));
      });

    return () => {
      cancelled = true;
    };
  }, [session, onTokensRefreshed, logout, run, attempt]);

  return {
    data,
    error,
    refresh: useCallback(() => setAttempt((n) => n + 1), []),
    set: useCallback(
      (update: T | ((previous: T | null) => T | null)) =>
        setData((previous) =>
          typeof update === "function"
            ? (update as (p: T | null) => T | null)(previous)
            : update,
        ),
      [],
    ),
  };
}

/** The write counterpart, with the same session-expiry handling. */
export function usePortalAction() {
  const { session, onTokensRefreshed, logout } = useAuth();
  const [pending, setPending] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(
    async <T,>(
      key: string,
      action: (session: Session, callbacks: RefreshCallbacks) => Promise<T>,
    ): Promise<T | undefined> => {
      if (!session) return undefined;
      setPending(key);
      setError(null);
      try {
        return await action(session, { onTokensRefreshed });
      } catch (failure) {
        if (failure instanceof SessionExpiredError) {
          void logout();
          return undefined;
        }
        setError(explain(failure));
        return undefined;
      } finally {
        setPending(null);
      }
    },
    [session, onTokensRefreshed, logout],
  );

  return { run, pending, error };
}

/** Turns a failure into something a client should read. */
export function explain(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 0) {
      return "We can't reach the practice right now. Check your connection and try again.";
    }
    return error.message;
  }
  return "Something went wrong. Please try again in a moment.";
}
