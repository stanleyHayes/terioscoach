"use client";

import { useEffect, useState } from "react";
import { FileText, Plus } from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { BrandedSelect } from "@/components/ui/ChoiceControls";
import { SubmissionModal } from "@/components/forms/SubmissionView";
import {
  formsApi,
  type FormDefinition,
  type FormSubmission,
  type SubmissionView,
} from "@/lib/forms";
import { useAuth } from "@/lib/auth";

export function ClientForms({
  clientId,
  clientName,
}: {
  clientId: string;
  clientName: string;
}) {
  const { session, refreshCallbacks } = useAuth();
  const [submissions, setSubmissions] = useState<FormSubmission[] | null>(null);
  const [forms, setForms] = useState<FormDefinition[]>([]);
  const [assignOpen, setAssignOpen] = useState(false);
  const [selectedFormId, setSelectedFormId] = useState("");
  const [assigning, setAssigning] = useState(false);
  const [assignError, setAssignError] = useState<string | null>(null);

  const [activeView, setActiveView] = useState<SubmissionView | null>(null);
  const [loadingViewId, setLoadingViewId] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    formsApi
      .listSubmissions(session, refreshCallbacks, { clientId })
      .then((items) => {
        if (!cancelled) setSubmissions(items);
      })
      .catch(() => {
        if (!cancelled) setSubmissions([]);
      });

    formsApi
      .list(session, refreshCallbacks)
      .then((items) => {
        if (!cancelled) setForms(items.filter((f) => f.active));
      })
      .catch(() => {
        if (!cancelled) setForms([]);
      });

    return () => {
      cancelled = true;
    };
  }, [clientId, refreshCallbacks, session]);

  async function openSubmission(sub: FormSubmission) {
    if (!session) return;
    setLoadingViewId(sub.id);
    try {
      const view = await formsApi.getSubmission(
        session,
        refreshCallbacks,
        sub.id,
      );
      setActiveView(view);
    } catch {
      // ignore
    } finally {
      setLoadingViewId(null);
    }
  }

  async function handleAssignSubmit() {
    if (!session || !selectedFormId) return;
    setAssigning(true);
    setAssignError(null);
    try {
      const created = await formsApi.assign(session, refreshCallbacks, {
        formId: selectedFormId,
        clientId,
      });
      setSubmissions((prev) => [created, ...(prev ?? [])]);
      setAssignOpen(false);
      setSelectedFormId("");
    } catch (err: unknown) {
      setAssignError(
        err instanceof Error ? err.message : "Failed to assign form.",
      );
    } finally {
      setAssigning(false);
    }
  }

  if (submissions === null) {
    return <div className="h-10 animate-pulse rounded-lg bg-surface-sunken" />;
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <h3 className="text-xs font-semibold tracking-[0.04em] text-ink-muted uppercase">
          Intake & Consent Forms
        </h3>
        {forms.length > 0 ? (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setAssignOpen(true)}
            className="text-xs h-7 px-2 text-primary"
          >
            <Plus size={13} className="mr-1" aria-hidden="true" />
            Assign form
          </Button>
        ) : null}
      </div>

      {submissions.length === 0 ? (
        <p className="text-sm text-ink-muted">
          No forms assigned or submitted yet.
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {submissions.map((sub) => {
            const isSubmitted = sub.status === "submitted";
            return (
              <li
                key={sub.id}
                id={`submission-${sub.id}`}
                className="rounded-lg bg-surface-sunken p-3 border border-border/60"
              >
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <FileText
                        size={14}
                        className="shrink-0 text-ink-muted"
                        aria-hidden="true"
                      />
                      <p className="text-sm font-medium text-ink truncate">
                        {sub.formTitle}
                      </p>
                    </div>
                    <p className="mt-1 text-[11px] text-ink-muted">
                      {isSubmitted && sub.submittedAt ? (
                        <>
                          Submitted{" "}
                          {new Date(sub.submittedAt).toLocaleDateString(
                            "en-US",
                            {
                              day: "numeric",
                              month: "short",
                              year: "numeric",
                            },
                          )}
                        </>
                      ) : (
                        <>
                          Assigned{" "}
                          {new Date(sub.assignedAt).toLocaleDateString(
                            "en-US",
                            {
                              day: "numeric",
                              month: "short",
                              year: "numeric",
                            },
                          )}
                        </>
                      )}
                    </p>
                  </div>
                  <Badge variant={isSubmitted ? "success" : "warning"}>
                    {isSubmitted ? "Submitted" : "Assigned"}
                  </Badge>
                </div>

                {isSubmitted ? (
                  <button
                    type="button"
                    onClick={() => void openSubmission(sub)}
                    disabled={loadingViewId === sub.id}
                    className="mt-2 inline-flex items-center gap-1 text-[11px] font-semibold text-primary hover:text-primary-hover disabled:opacity-50"
                  >
                    {loadingViewId === sub.id ? "Loading…" : "View responses"}
                  </button>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}

      {activeView ? (
        <SubmissionModal
          view={activeView}
          onClose={() => setActiveView(null)}
        />
      ) : null}

      {assignOpen ? (
        <Modal
          open
          onClose={() => setAssignOpen(false)}
          title={`Assign Form to ${clientName}`}
          description="Select an active form definition to send to the client."
          footer={
            <div className="flex justify-end gap-2">
              <Button
                variant="secondary"
                disabled={assigning}
                onClick={() => setAssignOpen(false)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                loading={assigning}
                disabled={!selectedFormId}
                onClick={() => void handleAssignSubmit()}
              >
                Assign
              </Button>
            </div>
          }
        >
          <div className="flex flex-col gap-4 py-2">
            {assignError ? (
              <p role="alert" className="text-sm text-danger-ink">
                {assignError}
              </p>
            ) : null}
            <BrandedSelect
              label="Select Form"
              value={selectedFormId}
              placeholder="Choose a form template…"
              options={forms.map((f) => ({ value: f.id, label: f.title }))}
              onChange={setSelectedFormId}
            />
          </div>
        </Modal>
      ) : null}
    </div>
  );
}
