"use client";

import { useCallback, useEffect, useState } from "react";
import { ApiError, SessionExpiredError, type RefreshCallbacks, type Session } from "@/lib/api";
import { useAuth } from "@/lib/auth";

/**
 * The dashboard's standard authenticated read.
 *
 * Every screen here needs the same four things — load once the session
 * exists, keep a first-load skeleton distinct from an empty result, end the
 * session when a refresh can no longer save it, and re-fetch in place after
 * a mutation without flashing back to skeletons. Writing that out per page
 * is how the pages drift apart; this is the one copy.
 */

export interface Resource<T> {
  /** null until the first load resolves — the skeleton state. */
  data: T | null;
  error: string | null;
  /** Re-fetches in place, keeping the current data on screen. */
  refresh: () => void;
  /**
   * Updates the loaded value locally, for optimistic updates.
   *
   * The updater form is the one to reach for whenever two writes can be in
   * flight at once. Passing a value computed from a render's snapshot means
   * the slower write wins with stale data — which is how a deleted row
   * comes back because a status change that started earlier landed later.
   */
  set: (update: T | ((previous: T | null) => T | null)) => void;
}

export function useResource<T>(
  load: (session: Session, callbacks: RefreshCallbacks) => Promise<T>,
  deps: unknown[] = [],
): Resource<T> {
  const { session, refreshCallbacks, logout } = useAuth();
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  // The loader is supplied inline by each caller and would otherwise change
  // identity every render; the caller's own deps are the real signal.
  // Both rules want a dependency literal here, which is exactly what a
  // shared hook taking a caller-supplied loader cannot have.
  // eslint-disable-next-line react-hooks/exhaustive-deps, react-hooks/use-memo
  const run = useCallback(load, deps);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;

    run(session, refreshCallbacks)
      .then((value) => {
        if (cancelled) return;
        setData(value);
        setError(null);
      })
      .catch((failure: unknown) => {
        if (cancelled) return;
        // A session that can no longer be refreshed ends here, with the
        // branded message handed to the login screen.
        if (failure instanceof SessionExpiredError) {
          void logout(failure.message);
          return;
        }
        setError(describe(failure));
      });

    return () => {
      cancelled = true;
    };
  }, [session, refreshCallbacks, logout, run, attempt]);

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

/** Turns an unknown failure into something worth showing a practitioner. */
export function describe(error: unknown): string {
  if (error instanceof ApiError) {
    return error.message;
  }
  return "Something went wrong. Try again.";
}

/**
 * The mutation counterpart: runs a write, surfaces its error, and reports
 * whether it is in flight. Session expiry is handled the same way as on
 * reads, so a write never leaves a dead session on screen.
 */
export function useAction() {
  const { session, refreshCallbacks, logout } = useAuth();
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
        return await action(session, refreshCallbacks);
      } catch (failure) {
        if (failure instanceof SessionExpiredError) {
          void logout(failure.message);
          return undefined;
        }
        setError(describe(failure));
        return undefined;
      } finally {
        setPending(null);
      }
    },
    [session, refreshCallbacks, logout],
  );

  return { run, pending, error, clearError: useCallback(() => setError(null), []) };
}
