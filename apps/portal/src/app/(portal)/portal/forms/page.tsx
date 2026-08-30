"use client";

import Link from "next/link";
import { ClipboardList } from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import { buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import {
  PortalEmpty,
  PortalError,
  PortalLoading,
  PortalPage,
} from "@/components/portal/PortalPage";
import { formsApi, type FormSubmission } from "@/lib/portal";
import { usePortalData } from "@/lib/use-portal-data";

/**
 * Forms waiting to be completed, and those already sent (CX-07).
 *
 * Outstanding forms lead, because they are the only ones that need
 * anything. A completed form stays visible — a client should be able to see
 * what they consented to, not have it disappear the moment they sign.
 */
export default function FormsPage() {
  const forms = usePortalData<FormSubmission[]>(
    (session, callbacks) => formsApi.listMine(session, callbacks),
    [],
  );

  const items = forms.data ?? [];
  const waiting = items.filter((item) => item.status === "assigned");
  const done = items.filter((item) => item.status === "submitted");

  return (
    <PortalPage
      title="Your forms"
      intro="Intake and consent forms from your practitioner. Anything outstanding is at the top."
    >
      {forms.error ? (
        <PortalError message={forms.error} onRetry={forms.refresh} />
      ) : forms.data === null ? (
        <PortalLoading label="Loading your forms…" rows={2} />
      ) : items.length === 0 ? (
        <PortalEmpty
          icon={<ClipboardList size={32} />}
          title="No forms right now"
          body="If your practitioner sends you a form to complete, it appears here and you are emailed about it."
        />
      ) : (
        <>
          {waiting.length > 0 ? (
            <section aria-labelledby="waiting-heading" className="flex flex-col gap-3">
              <h2
                id="waiting-heading"
                className="font-display text-[1.5rem] leading-[1.2] font-medium text-ink"
              >
                To complete
              </h2>
              {waiting.map((form) => (
                <Card key={form.id} className="terios-record-card">
                  <div className="flex flex-wrap items-center justify-between gap-4">
                    <div className="min-w-0">
                      <h3 className="text-base font-semibold leading-[1.4] text-ink">
                        {form.formTitle}
                      </h3>
                      <p className="mt-1 text-[13px] text-ink-muted">
                        Sent{" "}
                        <time dateTime={form.assignedAt}>
                          {new Date(form.assignedAt).toLocaleDateString("en-GB", {
                            day: "numeric",
                            month: "short",
                            year: "numeric",
                          })}
                        </time>
                      </p>
                    </div>
                    <Link
                      href={`/portal/forms/${form.id}`}
                      className={buttonClasses({ size: "sm" })}
                    >
                      Complete it
                    </Link>
                  </div>
                </Card>
              ))}
            </section>
          ) : null}

          {done.length > 0 ? (
            <section aria-labelledby="completed-heading" className="flex flex-col gap-3">
              <h2
                id="completed-heading"
                className="font-display text-[1.5rem] leading-[1.2] font-medium text-ink"
              >
                Completed
              </h2>
              {done.map((form) => (
                <Card key={form.id} className="terios-record-card terios-record-card-complete">
                  <div className="flex flex-wrap items-center justify-between gap-4">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-3">
                        <h3 className="text-base font-semibold leading-[1.4] text-ink">
                          {form.formTitle}
                        </h3>
                        <Badge tone="success">Sent</Badge>
                      </div>
                      <p className="mt-1 text-[13px] text-ink-muted">
                        {form.submittedAt ? (
                          <>
                            Completed{" "}
                            <time dateTime={form.submittedAt}>
                              {new Date(form.submittedAt).toLocaleDateString("en-GB", {
                                day: "numeric",
                                month: "short",
                                year: "numeric",
                              })}
                            </time>
                          </>
                        ) : null}
                        {form.signature ? " · signed" : null}
                      </p>
                    </div>
                    <Link
                      href={`/portal/forms/${form.id}`}
                      className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
                    >
                      View
                    </Link>
                  </div>
                </Card>
              ))}
            </section>
          ) : null}
        </>
      )}
    </PortalPage>
  );
}
