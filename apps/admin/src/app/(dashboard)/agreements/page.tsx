"use client";

import { useCallback, useEffect, useState } from "react";
import { CircleAlert, FileText, Save } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/TextInput";
import { agreementsApi, type Agreement } from "@/lib/agreements";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/cn";
import { describe } from "@/lib/use-resource";

/**
 * Service agreements (ADM-14).
 *
 * The practice's contracts, editable in place. Two rules shape the screen:
 *
 *   - Editing the wording bumps the version, and the clients who signed the
 *     previous wording stay recorded against *that* wording. Nothing is
 *     rewritten under anyone, which is why the version is shown next to the
 *     save button rather than hidden.
 *   - Retiring an agreement stops it being asked for; it does not delete the
 *     signatures already given, which remain on each client's file.
 *
 * Which services each agreement covers is set on the Services screen, not
 * here — a service has one agreement, an agreement has many services, and
 * the choice belongs with the service being offered.
 */
export default function AgreementsPage() {
  const { session, refreshCallbacks } = useAuth();
  const [agreements, setAgreements] = useState<Agreement[] | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    if (!session) return;
    agreementsApi
      .list(session, refreshCallbacks)
      .then((items) => {
        setAgreements(items);
        setSelectedId((current) => current ?? items[0]?.id ?? null);
      })
      .catch((failure) => setError(describe(failure)));
  }, [refreshCallbacks, session]);

  useEffect(() => load(), [load]);

  const selected = agreements?.find((item) => item.id === selectedId) ?? null;

  const applyUpdate = useCallback((updated: Agreement) => {
    setAgreements((current) =>
      (current ?? []).map((item) => (item.id === updated.id ? updated : item)),
    );
  }, []);

  return (
    <div data-admin-page="agreements" className="flex flex-col gap-6">
      <div>
        <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
          Service agreements
        </h1>
        <p className="mt-2 max-w-[70ch] text-sm leading-relaxed text-ink-muted">
          The contracts a client signs before their first session. A client
          signs each one once, and it then covers every service that uses it —
          including repeat bookings. Which services use which agreement is set
          on the Services screen.
        </p>
      </div>

      {error ? (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
        >
          <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
          {error}
        </div>
      ) : null}

      {agreements === null ? (
        <div className="h-64 animate-pulse rounded-xl bg-surface-sunken" />
      ) : agreements.length === 0 ? (
        <p className="text-sm text-ink-muted">
          No agreements yet. They are created the first time the API starts
          against this practice.
        </p>
      ) : (
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start">
          <nav
            aria-label="Agreements"
            className="flex gap-2 overflow-x-auto lg:w-64 lg:shrink-0 lg:flex-col lg:overflow-visible"
          >
            {agreements.map((item) => (
              <button
                key={item.id}
                type="button"
                aria-current={item.id === selectedId ? "true" : undefined}
                onClick={() => setSelectedId(item.id)}
                className={cn(
                  "flex shrink-0 items-start gap-2 rounded-xl border p-3 text-left transition-colors lg:shrink",
                  item.id === selectedId
                    ? "border-primary bg-surface-raised"
                    : "border-border bg-surface-raised hover:bg-surface-sunken",
                )}
              >
                <FileText
                  size={15}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0 text-ink-faint"
                />
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium text-ink">
                    {item.title}
                  </span>
                  <span className="mt-0.5 block text-[11px] text-ink-muted">
                    Version {item.version}
                    {item.active ? "" : " · retired"}
                  </span>
                </span>
              </button>
            ))}
          </nav>

          {selected ? (
            // Keyed on the agreement: switching selection remounts the
            // editor with the new text, rather than an effect copying it in
            // a render late.
            <AgreementEditor
              key={selected.id}
              agreement={selected}
              onSaved={applyUpdate}
              onError={setError}
            />
          ) : null}
        </div>
      )}
    </div>
  );
}

/** The editor for one agreement. Its state is the draft, and it is thrown
 * away by the remount when another agreement is selected. */
function AgreementEditor({
  agreement,
  onSaved,
  onError,
}: {
  agreement: Agreement;
  onSaved: (updated: Agreement) => void;
  onError: (message: string | null) => void;
}) {
  const { session, refreshCallbacks } = useAuth();
  const [title, setTitle] = useState(agreement.title);
  const [body, setBody] = useState(agreement.body);
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState<string | null>(null);

  const dirty = title !== agreement.title || body !== agreement.body;

  async function save() {
    if (!session) return;
    setSaving(true);
    onError(null);
    try {
      const updated = await agreementsApi.update(session, refreshCallbacks, agreement.id, {
        title: title.trim(),
        body: body.trim(),
      });
      onSaved(updated);
      setSavedAt(
        new Date().toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit" }),
      );
    } catch (failure) {
      onError(describe(failure));
    } finally {
      setSaving(false);
    }
  }

  async function toggleActive() {
    if (!session) return;
    onError(null);
    try {
      onSaved(
        await agreementsApi.update(session, refreshCallbacks, agreement.id, {
          active: !agreement.active,
        }),
      );
    } catch (failure) {
      onError(describe(failure));
    }
  }

  return (
    <section className="min-w-0 flex-1 rounded-xl border border-border bg-surface-raised p-5">
      <TextInput
        label="Title"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />

      <label className="mt-4 flex flex-col gap-1.5">
        <span className="text-sm font-medium text-ink">Agreement text</span>
        <textarea
          value={body}
          onChange={(event) => setBody(event.target.value)}
          rows={26}
          spellCheck={false}
          className="rounded-lg border border-border bg-surface px-3 py-2 font-mono text-[13px] leading-relaxed text-ink outline-none transition-[border-color,box-shadow] focus:border-primary focus:ring-2 focus:ring-primary/20"
        />
        <span className="text-xs leading-relaxed text-ink-muted">
          Blank line between paragraphs. A line starting with
          <code className="mx-1 rounded bg-surface-sunken px-1">##</code>
          is a heading. Plain text only — nothing here is rendered as HTML, on
          the page or in the signed PDF.
        </span>
      </label>

      <div className="mt-5 flex flex-wrap items-center gap-3">
        <Button loading={saving} disabled={!dirty || saving} onClick={() => void save()}>
          <Save size={15} aria-hidden="true" />
          Save changes
        </Button>
        <Button variant="secondary" onClick={() => void toggleActive()}>
          {agreement.active ? "Retire this agreement" : "Put back in use"}
        </Button>
        <p className="text-xs text-ink-muted">
          {savedAt
            ? `Saved at ${savedAt} · now version ${agreement.version}`
            : dirty
              ? `Saving will make this version ${agreement.version + 1}`
              : `Version ${agreement.version}`}
        </p>
      </div>

      <p className="mt-3 text-xs leading-relaxed text-ink-faint">
        Clients who have already signed keep the wording they signed. Editing
        here applies to everyone who signs from now on.
      </p>
    </section>
  );
}
