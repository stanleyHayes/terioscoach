"use client";

import { CircleAlert, ShieldCheck } from "lucide-react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import type { Answer, FormField, SubmissionView as View } from "@/lib/forms";

/**
 * A signed submission, read back (ADM-08).
 *
 * Read-only, with no route behind it that could change an answer: a consent
 * record that can be edited after signing is not a consent record. The
 * integrity check at the top is the server recomputing the digest over the
 * answers, the typed name and the timestamp — if it says otherwise, the row
 * was altered in the database and the signature no longer stands for what
 * it shows.
 */
export function SubmissionModal({ view, onClose }: { view: View; onClose: () => void }) {
  const { submission, form, integrityOk, signatureImage } = view;
  const signed = submission.status === "submitted";

  return (
    <Modal
      open
      onClose={onClose}
      title={submission.formTitle}
      description={
        signed && submission.submittedAt
          ? `Signed ${formatDateTime(submission.submittedAt)}`
          : `Assigned ${formatDateTime(submission.assignedAt)} — not filled in yet`
      }
      size="form"
      footer={
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className="flex flex-col gap-5">
        {signed ? (
          integrityOk ? (
            <p className="flex items-start gap-2 rounded-md bg-success-bg px-4 py-3 text-[13px] leading-[1.55] text-success-ink">
              <ShieldCheck size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
              The record matches its signature. Nothing has been altered since it
              was signed.
            </p>
          ) : (
            <p
              role="alert"
              className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-[13px] leading-[1.55] text-danger-ink"
            >
              <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
              This record does not match its signature. It has been changed since
              it was signed and should not be relied on.
            </p>
          )
        ) : (
          <p className="rounded-md bg-surface-sunken px-4 py-3 text-[13px] leading-[1.55] text-ink-muted">
            Waiting on the client. They see this in their portal.
          </p>
        )}

        <dl className="flex flex-col gap-4">
          {form.fields
            .filter((field) => field.type !== "signature")
            .map((field) => (
              <div key={field.key}>
                <dt className="text-[13px] font-medium text-ink-muted">{field.label}</dt>
                <dd className="mt-1 text-sm leading-[1.55] whitespace-pre-line text-ink">
                  {renderAnswer(field, submission.answers[field.key])}
                </dd>
              </div>
            ))}
        </dl>

        {submission.signature ? (
          <div className="rounded-lg border border-border p-4">
            <h3 className="text-[13px] font-medium text-ink-muted">Signature</h3>
            {signatureImage ? (
              // The stored image is a data: URL written by the signature pad;
              // next/image cannot take one and there is nothing to optimise.
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={signatureImage}
                alt={`Signature of ${submission.signature.typedName}`}
                className="mt-2 h-24 w-full rounded-md bg-surface object-contain"
              />
            ) : null}
            <p className="mt-2 text-sm text-ink">{submission.signature.typedName}</p>
            <p className="text-[13px] tabular-nums text-ink-faint">
              {formatDateTime(submission.signature.signedAt)}
            </p>
          </div>
        ) : null}

        {submission.bookingId ? (
          <p className="text-[13px] text-ink-faint">
            <Badge variant="neutral">Tied to a session</Badge>
          </p>
        ) : null}
      </div>
    </Modal>
  );
}

/** An unanswered optional question reads as "Not answered", not as blank —
 * blank is indistinguishable from a rendering failure. */
function renderAnswer(field: FormField, answer: Answer | undefined): string {
  if (!answer) return "Not answered";
  if (answer.values?.length) return answer.values.join(", ");
  const value = answer.value?.trim();
  if (!value) return "Not answered";
  if (field.type === "date") return formatDate(value);
  return value;
}

function formatDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleDateString("en-GB", { day: "numeric", month: "long", year: "numeric" });
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
