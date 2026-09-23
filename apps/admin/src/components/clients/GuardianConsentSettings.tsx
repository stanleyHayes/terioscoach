"use client";
import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth";
import { authedRequest } from "@/lib/api";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
export function GuardianConsentSettings() {
  const { user, session, refreshCallbacks } = useAuth();
  const [body, setBody] = useState("");
  const [version, setVersion] = useState<number | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  useEffect(() => {
    if (!session || user?.role !== "practitioner") return;
    let cancelled = false;
    authedRequest<{ agreement: { body: string; version: number } }>(
      "/v1/guardian-consent",
      session,
      refreshCallbacks,
    )
      .then(({ agreement }) => {
        if (!cancelled) {
          setBody(agreement.body);
          setVersion(agreement.version);
        }
      })
      .catch((e) => {
        if (!cancelled) setMessage(e.message);
      });
    return () => {
      cancelled = true;
    };
  }, [session, user?.role, refreshCallbacks]);
  async function save() {
    if (!session) return;
    setBusy(true);
    setMessage("");
    try {
      const { agreement } = await authedRequest<{
        agreement: { body: string; version: number };
      }>("/v1/guardian-consent", session, refreshCallbacks, {
        method: "PATCH",
        body: { body },
      });
      setBody(agreement.body);
      setVersion(agreement.version);
      setMessage(
        "Consent wording saved. Existing signed wording is preserved.",
      );
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Could not save wording.");
    } finally {
      setBusy(false);
    }
  }
  if (user?.role !== "practitioner") return null;
  return (
    <Card>
      <h2 className="font-display text-xl font-semibold">
        Parent / guardian consent
      </h2>
      <p className="mt-2 text-sm text-ink-muted">
        Shown when an appointment is for someone under 18. Each electronic
        signature preserves the exact wording and version. Changes require a new
        guardian consent execution for upcoming sessions.
      </p>
      <label className="mt-4 block text-sm font-medium">
        Consent wording
        <textarea
          className="mt-2 min-h-40 w-full rounded-xl border border-border-strong bg-surface-raised p-3 text-ink focus-visible:outline-primary"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          maxLength={120000}
          disabled={busy || version === null}
        />
      </label>
      <p className="mt-2 text-xs text-ink-muted">
        {version === null ? "Loading wording…" : `Current version: ${version}`}
      </p>
      <Button
        className="mt-4"
        loading={busy}
        disabled={!body.trim() || version === null}
        onClick={() => void save()}
      >
        Save consent wording
      </Button>
      {message && (
        <p role="status" className="mt-3 text-sm">
          {message}
        </p>
      )}
    </Card>
  );
}
