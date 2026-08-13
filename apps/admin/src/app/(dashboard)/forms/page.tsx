"use client";

import { ClipboardList, FileSignature, Pencil, Send, Trash2 } from "lucide-react";
import { useState } from "react";
import { EmptyState, ErrorBanner, LoadFailure, Skeletons } from "@/components/content/states";
import { AssignFormModal } from "@/components/forms/AssignFormModal";
import { FormBuilder } from "@/components/forms/FormBuilder";
import { SubmissionModal } from "@/components/forms/SubmissionView";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Switch } from "@/components/ui/Switch";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  formsApi,
  type FormDefinition,
  type FormDraft,
  type FormSubmission,
  type SubmissionView,
} from "@/lib/forms";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * Forms (ADM-08) — the definitions on one tab, what's come back on the other.
 *
 * They are separated because they are different jobs. Building a form is
 * occasional and deliberate; checking who has signed one is a thing you do
 * before a session. Mixing them means the second is always buried under the
 * first.
 */

const TABS = [
  { id: "forms", label: "Forms" },
  { id: "submissions", label: "Responses" },
] as const;

type TabId = (typeof TABS)[number]["id"];

export default function FormsPage() {
  const [tab, setTab] = useState<TabId>("forms");

  return (
    <div data-admin-page="forms" className="flex flex-col gap-6">
      <header>
        <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
          Forms
        </h1>
        <p className="mt-1.5 text-sm text-ink-muted">
          Intake and consent forms your clients fill in and sign before a session.
        </p>
      </header>

      <div
        role="tablist"
        aria-label="Forms view"
        className="flex flex-wrap gap-1 border-b border-border"
      >
        {TABS.map(({ id, label }, index) => (
          <button
            key={id}
            type="button"
            role="tab"
            id={`forms-tab-${id}`}
            aria-selected={tab === id}
            aria-controls={`forms-panel-${id}`}
            tabIndex={tab === id ? 0 : -1}
            onClick={() => setTab(id)}
            onKeyDown={(event) => {
              const step = event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
              if (step === 0) return;
              event.preventDefault();
              const next = TABS[(index + step + TABS.length) % TABS.length]!;
              setTab(next.id);
              document.getElementById(`forms-tab-${next.id}`)?.focus();
            }}
            className={cn(
              "-mb-px border-b-2 px-4 py-2.5 text-sm font-medium transition-colors duration-instant ease-out",
              tab === id
                ? "border-primary text-ink"
                : "border-transparent text-ink-muted hover:text-ink",
            )}
          >
            {label}
          </button>
        ))}
      </div>

      <div
        role="tabpanel"
        id={`forms-panel-${tab}`}
        aria-labelledby={`forms-tab-${tab}`}
        tabIndex={0}
      >
        {tab === "forms" ? <FormsList /> : <SubmissionsList />}
      </div>
    </div>
  );
}

function FormsList() {
  const forms = useResource<FormDefinition[]>(
    (session, callbacks) => formsApi.list(session, callbacks),
    [],
  );
  // Loaded alongside so the builder can warn before a form with signed
  // responses is reworded.
  const submissions = useResource<FormSubmission[]>(
    (session, callbacks) => formsApi.listSubmissions(session, callbacks),
    [],
  );
  const action = useAction();

  const [building, setBuilding] = useState<FormDefinition | null | undefined>(undefined);
  const [assigning, setAssigning] = useState<FormDefinition | null>(null);
  const [confirming, setConfirming] = useState<FormDefinition | null>(null);

  const items = [...(forms.data ?? [])].sort((a, b) => a.sortOrder - b.sortOrder);
  const signedFormIds = new Set(
    (submissions.data ?? []).filter((s) => s.status === "submitted").map((s) => s.formId),
  );

  async function save(draft: FormDraft) {
    const existing = building ?? null;
    const saved = await action.run("form", (session, callbacks) =>
      existing
        ? formsApi.update(session, callbacks, existing.id, draft)
        : formsApi.create(session, callbacks, { ...draft, sortOrder: items.length }),
    );
    if (!saved) throw new ApiError(500, "write_failed", "It didn't save. Try again.");
    forms.set((current) => {
      const list = current ?? [];
      return existing ? list.map((f) => (f.id === saved.id ? saved : f)) : [...list, saved];
    });
  }

  async function toggleActive(form: FormDefinition, active: boolean) {
    const updated = await action.run(form.id, (session, callbacks) =>
      formsApi.update(session, callbacks, form.id, { active }),
    );
    if (updated) {
      forms.set((current) => (current ?? []).map((f) => (f.id === updated.id ? updated : f)));
    }
  }

  async function assign(form: FormDefinition, clientId: string) {
    const created = await action.run("assign", (session, callbacks) =>
      formsApi.assign(session, callbacks, { formId: form.id, clientId }),
    );
    if (!created) throw new ApiError(500, "assign_failed", "It wasn't sent. Try again.");
    submissions.set((current) => [created, ...(current ?? [])]);
  }

  async function remove(form: FormDefinition) {
    const done = await action.run(form.id, (session, callbacks) =>
      formsApi.remove(session, callbacks, form.id).then(() => true),
    );
    if (done) {
      forms.set((current) => (current ?? []).filter((f) => f.id !== form.id));
      setConfirming(null);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-ink-muted">
          {items.length === 0
            ? "Build the forms you need signed before a first session."
            : `${items.filter((f) => f.active).length} of ${items.length} in use`}
        </p>
        <Button size="sm" onClick={() => setBuilding(null)}>
          New form
        </Button>
      </div>

      <ErrorBanner message={action.error} />

      {forms.error ? (
        <LoadFailure message={forms.error} onRetry={forms.refresh} />
      ) : forms.data === null ? (
        <Skeletons label="Loading forms…" />
      ) : items.length === 0 ? (
        <EmptyState
          icon={<ClipboardList size={26} aria-hidden="true" className="text-ink-faint" />}
          title="No forms yet"
          body="A health intake and a consent form are the usual two. Both can carry a signature the client draws and types."
          action={
            <Button size="sm" onClick={() => setBuilding(null)}>
              New form
            </Button>
          }
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((form) => {
            const busy = action.pending === form.id;
            const signable = form.fields.some((field) => field.type === "signature");
            return (
              <li
                key={form.id}
                className="flex flex-wrap items-start justify-between gap-4 rounded-lg border border-border bg-surface-raised p-5"
              >
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="text-sm font-medium text-ink">{form.title}</h3>
                    {signable ? <Badge variant="info">Signed</Badge> : null}
                    {!form.active ? <Badge variant="warning">Not in use</Badge> : null}
                  </div>
                  {form.description ? (
                    <p className="mt-1.5 max-w-[68ch] text-[13px] leading-[1.55] text-ink-muted">
                      {form.description}
                    </p>
                  ) : null}
                  <p className="mt-2 text-[13px] text-ink-faint">
                    {form.fields.length} question{form.fields.length === 1 ? "" : "s"}
                    {signedFormIds.has(form.id) ? " · has signed responses" : ""}
                  </p>
                  <div className="mt-3">
                    <Switch
                      checked={form.active}
                      disabled={busy}
                      label={`Use "${form.title}"`}
                      onChange={(next) => void toggleActive(form, next)}
                    />
                  </div>
                </div>

                <div className="flex flex-wrap gap-2">
                  <Button size="sm" disabled={busy} onClick={() => setAssigning(form)}>
                    <Send size={14} aria-hidden="true" className="mr-1.5" />
                    Send to a client
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setBuilding(form)}
                  >
                    <Pencil size={14} aria-hidden="true" className="mr-1.5" />
                    Edit
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setConfirming(form)}
                  >
                    <Trash2 size={14} aria-hidden="true" className="mr-1.5" />
                    Delete
                  </Button>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {building !== undefined ? (
        <FormBuilder
          form={building}
          hasSubmissions={building !== null && signedFormIds.has(building.id)}
          onClose={() => setBuilding(undefined)}
          onSubmit={save}
        />
      ) : null}

      {assigning ? (
        <AssignFormModal
          form={assigning}
          onClose={() => setAssigning(null)}
          onAssign={(clientId) => assign(assigning, clientId)}
        />
      ) : null}

      {confirming ? (
        <Modal
          open
          onClose={() => setConfirming(null)}
          title="Delete this form?"
          description={
            signedFormIds.has(confirming.id)
              ? "Responses already signed against it are kept — but you won't be able to read them back against the questions. Turning it off instead keeps everything."
              : "This can't be undone."
          }
          footer={
            <>
              <Button variant="secondary" onClick={() => setConfirming(null)}>
                Keep it
              </Button>
              <Button
                variant="danger"
                loading={action.pending === confirming.id}
                onClick={() => void remove(confirming)}
              >
                Delete
              </Button>
            </>
          }
        >
          <p className="text-sm leading-[1.55] text-ink-muted">{confirming.title}</p>
        </Modal>
      ) : null}
    </div>
  );
}

function SubmissionsList() {
  const [filter, setFilter] = useState<"submitted" | "assigned" | "all">("submitted");
  const submissions = useResource<FormSubmission[]>(
    (session, callbacks) => formsApi.listSubmissions(session, callbacks),
    [],
  );
  const action = useAction();
  const [open, setOpen] = useState<SubmissionView | null>(null);

  const items = submissions.data ?? [];
  const visible = filter === "all" ? items : items.filter((s) => s.status === filter);
  const waiting = items.filter((s) => s.status === "assigned").length;

  async function view(submission: FormSubmission) {
    const loaded = await action.run(submission.id, (session, callbacks) =>
      formsApi.getSubmission(session, callbacks, submission.id),
    );
    if (loaded) setOpen(loaded);
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-ink-muted">
          {waiting > 0 ? `${waiting} still waiting on a client` : "Nothing outstanding"}
        </p>
        <div className="flex flex-wrap gap-2" role="group" aria-label="Filter responses">
          {(["submitted", "assigned", "all"] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
              className={cn(
                "rounded-full px-3 py-1.5 text-[13px] font-medium transition-colors duration-instant ease-out",
                filter === value
                  ? "bg-primary text-on-primary"
                  : "bg-surface-sunken text-ink-muted hover:text-ink",
              )}
            >
              {value === "submitted" ? "Signed" : value === "assigned" ? "Waiting" : "All"}
            </button>
          ))}
        </div>
      </div>

      <ErrorBanner message={action.error} />

      {submissions.error ? (
        <LoadFailure message={submissions.error} onRetry={submissions.refresh} />
      ) : submissions.data === null ? (
        <Skeletons label="Loading responses…" />
      ) : visible.length === 0 ? (
        <EmptyState
          icon={<FileSignature size={26} aria-hidden="true" className="text-ink-faint" />}
          title={filter === "submitted" ? "Nothing signed yet" : "Nothing here"}
          body="Send a form to a client from the Forms tab. What comes back appears here, with the signature it was signed with."
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {visible.map((submission) => (
            <li
              key={submission.id}
              className="flex flex-wrap items-center justify-between gap-4 rounded-lg border border-border bg-surface-raised p-5"
            >
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-medium text-ink">{submission.formTitle}</h3>
                  <Badge variant={submission.status === "submitted" ? "success" : "warning"}>
                    {submission.status === "submitted" ? "Signed" : "Waiting"}
                  </Badge>
                </div>
                <p className="mt-1.5 text-[13px] tabular-nums text-ink-muted">
                  {submission.submittedAt
                    ? `Signed ${formatDate(submission.submittedAt)}`
                    : `Sent ${formatDate(submission.assignedAt)}`}
                </p>
              </div>
              <Button
                variant="secondary"
                size="sm"
                disabled={action.pending === submission.id}
                onClick={() => void view(submission)}
              >
                {submission.status === "submitted" ? "Read it" : "See what was sent"}
              </Button>
            </li>
          ))}
        </ul>
      )}

      {open ? <SubmissionModal view={open} onClose={() => setOpen(null)} /> : null}
    </div>
  );
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}
