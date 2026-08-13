"use client";

import { CircleAlert, Quote, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  testimonialsApi,
  type Moderation,
  type Testimonial,
  type TestimonialDraft,
} from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";
import { EmptyState, ErrorBanner, LoadFailure, Skeletons } from "@/components/content/states";

/**
 * Testimonial moderation (ADM-07).
 *
 * Nothing reaches the public site until it is approved here, and that holds
 * for testimonials the practitioner types in herself — a quote taken from an
 * email still goes through the same gate, so there is exactly one rule about
 * what is live rather than one rule per origin.
 */

const statusTone: Record<Moderation, BadgeVariant> = {
  pending: "warning",
  approved: "success",
  rejected: "neutral",
};

const statusLabel: Record<Moderation, string> = {
  pending: "Awaiting approval",
  approved: "On the site",
  rejected: "Not published",
};

const FILTERS = ["pending", "approved", "rejected", "all"] as const;

export function TestimonialModeration() {
  const [filter, setFilter] = useState<Moderation | "all">("pending");
  const [composing, setComposing] = useState(false);
  const [confirming, setConfirming] = useState<Testimonial | null>(null);

  const testimonials = useResource<Testimonial[]>(
    (session, callbacks) => testimonialsApi.list(session, callbacks),
    [],
  );
  const action = useAction();

  const items = testimonials.data ?? [];
  const visible = filter === "all" ? items : items.filter((t) => t.status === filter);
  const pendingCount = items.filter((t) => t.status === "pending").length;

  async function moderate(testimonial: Testimonial, approve: boolean) {
    const updated = await action.run(testimonial.id, (session, callbacks) =>
      testimonialsApi.moderate(session, callbacks, testimonial.id, approve),
    );
    if (updated) {
      testimonials.set((current) =>
        (current ?? []).map((t) => (t.id === updated.id ? updated : t)),
      );
    }
  }

  async function create(draft: TestimonialDraft) {
    const created = await action.run("form", (session, callbacks) =>
      testimonialsApi.create(session, callbacks, draft),
    );
    if (!created) throw new ApiError(500, "write_failed", "It didn't save. Try again.");
    testimonials.set((current) => [created, ...(current ?? [])]);
  }

  async function remove(testimonial: Testimonial) {
    const done = await action.run(testimonial.id, (session, callbacks) =>
      testimonialsApi.remove(session, callbacks, testimonial.id).then(() => true),
    );
    if (done) {
      testimonials.set((current) => (current ?? []).filter((t) => t.id !== testimonial.id));
      setConfirming(null);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-ink-muted">
          {pendingCount > 0
            ? `${pendingCount} waiting for your approval`
            : "Nothing waiting — the queue is clear"}
        </p>
        <Button size="sm" onClick={() => setComposing(true)}>
          Add a testimonial
        </Button>
      </div>

      <div className="flex flex-wrap gap-2" role="group" aria-label="Filter testimonials">
        {FILTERS.map((value) => (
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
            {value === "all" ? "All" : statusLabel[value]}
          </button>
        ))}
      </div>

      <ErrorBanner message={action.error} />

      {testimonials.error ? (
        <LoadFailure message={testimonials.error} onRetry={testimonials.refresh} />
      ) : testimonials.data === null ? (
        <Skeletons label="Loading testimonials…" />
      ) : visible.length === 0 ? (
        <EmptyState
          icon={<Quote size={26} aria-hidden="true" className="text-ink-faint" />}
          title={filter === "pending" ? "Nothing waiting" : "None here"}
          body="Testimonials appear on the public site only once you approve them — including the ones you add yourself."
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {visible.map((testimonial) => {
            const busy = action.pending === testimonial.id;
            return (
              <li
                key={testimonial.id}
                className="rounded-lg border border-border bg-surface-raised p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0">
                    <Badge variant={statusTone[testimonial.status]}>
                      {statusLabel[testimonial.status]}
                    </Badge>
                    <blockquote className="mt-3 max-w-[68ch] text-sm leading-[1.6] whitespace-pre-line text-ink">
                      {testimonial.quote}
                    </blockquote>
                    <p className="mt-3 text-[13px] text-ink-muted">
                      {testimonial.authorName}
                      {testimonial.authorRole ? (
                        <span className="text-ink-faint"> · {testimonial.authorRole}</span>
                      ) : null}
                    </p>
                  </div>
                  <time
                    dateTime={testimonial.submittedAt}
                    className="shrink-0 text-[13px] tabular-nums text-ink-faint"
                  >
                    {new Date(testimonial.submittedAt).toLocaleDateString("en-GB", {
                      day: "numeric",
                      month: "short",
                      year: "numeric",
                    })}
                  </time>
                </div>

                <div className="mt-5 flex flex-wrap gap-2">
                  {testimonial.status !== "approved" ? (
                    <Button
                      size="sm"
                      disabled={busy}
                      onClick={() => void moderate(testimonial, true)}
                    >
                      {testimonial.status === "rejected" ? "Publish after all" : "Publish"}
                    </Button>
                  ) : null}
                  {testimonial.status !== "rejected" ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy}
                      onClick={() => void moderate(testimonial, false)}
                    >
                      {testimonial.status === "approved" ? "Take off the site" : "Don't publish"}
                    </Button>
                  ) : null}
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setConfirming(testimonial)}
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

      {composing ? (
        <TestimonialFormModal onClose={() => setComposing(false)} onSubmit={create} />
      ) : null}

      {confirming ? (
        <Modal
          open
          onClose={() => setConfirming(null)}
          title="Delete this testimonial?"
          description="It's removed for good. If you only want it off the site, take it down instead."
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
          <p className="text-sm leading-[1.55] text-ink-muted">
            &ldquo;{confirming.quote}&rdquo; — {confirming.authorName}
          </p>
        </Modal>
      ) : null}
    </div>
  );
}

function TestimonialFormModal({
  onClose,
  onSubmit,
}: {
  onClose: () => void;
  onSubmit: (draft: TestimonialDraft) => Promise<void>;
}) {
  const [authorName, setAuthorName] = useState("");
  const [authorRole, setAuthorRole] = useState("");
  const [quote, setQuote] = useState("");
  const [errors, setErrors] = useState<{ authorName?: string; quote?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty = authorName !== "" || authorRole !== "" || quote !== "";

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next: { authorName?: string; quote?: string } = {};
    if (!authorName.trim()) next.authorName = "Who said it?";
    if (!quote.trim()) next.quote = "The quote is the testimonial";
    setErrors(next);
    if (Object.keys(next).length > 0) return;

    setFormError(null);
    setSubmitting(true);
    try {
      await onSubmit({
        authorName: authorName.trim(),
        authorRole: authorRole.trim() || undefined,
        quote: quote.trim(),
      });
      onClose();
    } catch (error) {
      setFormError(
        error instanceof ApiError ? error.message : "Something went wrong. Try again.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Add a testimonial"
      description="It's saved as pending. Publish it when you've checked you have permission to use it."
      size="form"
      dirty={dirty}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="testimonial-form" loading={submitting}>
            Save
          </Button>
        </>
      }
    >
      <form
        id="testimonial-form"
        noValidate
        onSubmit={handleSubmit}
        className="flex flex-col gap-4"
      >
        {formError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
            {formError}
          </div>
        ) : null}

        <TextArea
          label="Quote"
          required
          rows={5}
          data-autofocus
          value={quote}
          error={errors.quote}
          onChange={(event) => {
            setQuote(event.target.value);
            setErrors((current) => ({ ...current, quote: undefined }));
          }}
        />
        <TextInput
          label="Name"
          required
          value={authorName}
          error={errors.authorName}
          placeholder="Ama K."
          onChange={(event) => {
            setAuthorName(event.target.value);
            setErrors((current) => ({ ...current, authorName: undefined }));
          }}
        />
        <TextInput
          label="Role"
          value={authorRole}
          hint="Optional — how they'd like to be described."
          placeholder="Client since 2024"
          onChange={(event) => setAuthorRole(event.target.value)}
        />
      </form>
    </Modal>
  );
}
