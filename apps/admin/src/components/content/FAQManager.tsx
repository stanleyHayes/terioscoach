"use client";

import { CircleAlert, GripVertical, HelpCircle, Pencil, Trash2 } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { Switch } from "@/components/ui/Switch";
import { TextArea } from "@/components/ui/TextArea";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { faqsApi, type FAQ, type FAQDraft } from "@/lib/content";
import { useAction, useResource } from "@/lib/use-resource";
import { EmptyState, ErrorBanner, LoadFailure, Skeletons } from "@/components/content/states";

/**
 * FAQ manager (ADM-07).
 *
 * FAQs have no draft/published pair — they are either on the site or not,
 * which is the `active` switch. Ordering is the practitioner's, because the
 * order questions are asked in is editorial, not alphabetical.
 */
export function FAQManager() {
  const faqs = useResource<FAQ[]>((session, callbacks) => faqsApi.list(session, callbacks), []);
  const action = useAction();
  const [editing, setEditing] = useState<FAQ | null | undefined>(undefined);
  const [confirming, setConfirming] = useState<FAQ | null>(null);

  const items = [...(faqs.data ?? [])].sort((a, b) => a.sortOrder - b.sortOrder);

  async function save(draft: FAQDraft, existing: FAQ | null) {
    const saved = await action.run("form", (session, callbacks) =>
      existing
        ? faqsApi.update(session, callbacks, existing.id, draft)
        : faqsApi.create(session, callbacks, draft),
    );
    // useAction swallows the failure into its own banner and returns
    // undefined; the modal needs something to catch so it stays open.
    if (!saved) throw new ApiError(500, "write_failed", "The change didn't save. Try again.");
    faqs.set((current) =>
      existing
        ? (current ?? []).map((f) => (f.id === saved.id ? saved : f))
        : [...(current ?? []), saved],
    );
  }

  async function toggleActive(faq: FAQ, active: boolean) {
    const updated = await action.run(faq.id, (session, callbacks) =>
      faqsApi.update(session, callbacks, faq.id, { active }),
    );
    if (updated) {
      faqs.set((current) => (current ?? []).map((f) => (f.id === updated.id ? updated : f)));
    }
  }

  async function move(faq: FAQ, direction: -1 | 1) {
    const ordered = items;
    const index = ordered.findIndex((f) => f.id === faq.id);
    const swap = ordered[index + direction];
    if (!swap) return;

    // Both rows change, so both are written. Swapping the stored values
    // rather than renumbering the list keeps this to two calls whatever the
    // list length.
    const first = await action.run(faq.id, (session, callbacks) =>
      faqsApi.update(session, callbacks, faq.id, { sortOrder: swap.sortOrder }),
    );
    if (!first) return;
    const second = await action.run(faq.id, (session, callbacks) =>
      faqsApi.update(session, callbacks, swap.id, { sortOrder: faq.sortOrder }),
    );
    if (!second) return;

    faqs.set((current) =>
      (current ?? []).map((f) => {
        if (f.id === first.id) return first;
        if (f.id === second.id) return second;
        return f;
      }),
    );
  }

  async function remove(faq: FAQ) {
    const done = await action.run(faq.id, (session, callbacks) =>
      faqsApi.remove(session, callbacks, faq.id).then(() => true),
    );
    if (done) {
      faqs.set((current) => (current ?? []).filter((f) => f.id !== faq.id));
      setConfirming(null);
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-ink-muted">
          {items.length === 0
            ? "Answer the questions you're asked most."
            : `${items.filter((f) => f.active).length} of ${items.length} showing on the site`}
        </p>
        <Button size="sm" onClick={() => setEditing(null)}>
          Add a question
        </Button>
      </div>

      <ErrorBanner message={action.error} />

      {faqs.error ? (
        <LoadFailure message={faqs.error} onRetry={faqs.refresh} />
      ) : faqs.data === null ? (
        <Skeletons label="Loading questions…" />
      ) : items.length === 0 ? (
        <EmptyState
          icon={<HelpCircle size={26} aria-hidden="true" className="text-ink-faint" />}
          title="No questions yet"
          body="Anything a client emails you twice belongs here. Questions you add show on the FAQ page in the order you set."
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((faq, index) => {
            const busy = action.pending === faq.id;
            return (
              <li key={faq.id} className="rounded-lg border border-border bg-surface-raised p-5">
                <div className="flex items-start gap-4">
                  <div className="flex shrink-0 flex-col items-center gap-1 pt-0.5">
                    <GripVertical size={16} aria-hidden="true" className="text-ink-faint" />
                    <span className="text-[11px] tabular-nums text-ink-faint">{index + 1}</span>
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-sm font-medium text-ink">{faq.question}</h3>
                      {faq.category ? <Badge variant="neutral">{faq.category}</Badge> : null}
                      {!faq.active ? <Badge variant="warning">Hidden</Badge> : null}
                    </div>
                    <p className="mt-2 max-w-[68ch] text-sm leading-[1.6] whitespace-pre-line text-ink-muted">
                      {faq.answer}
                    </p>

                    <div className="mt-4 flex flex-wrap items-center gap-3">
                      <Switch
                        checked={faq.active}
                        disabled={busy}
                        label={`Show "${faq.question}" on the site`}
                        onChange={(next) => void toggleActive(faq, next)}
                      />
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={busy || index === 0}
                        onClick={() => void move(faq, -1)}
                      >
                        Move up
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={busy || index === items.length - 1}
                        onClick={() => void move(faq, 1)}
                      >
                        Move down
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={busy}
                        onClick={() => setEditing(faq)}
                      >
                        <Pencil size={14} aria-hidden="true" className="mr-1.5" />
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        disabled={busy}
                        onClick={() => setConfirming(faq)}
                      >
                        <Trash2 size={14} aria-hidden="true" className="mr-1.5" />
                        Delete
                      </Button>
                    </div>
                  </div>
                </div>
              </li>
            );
          })}
        </ul>
      )}

      {editing !== undefined ? (
        <FAQFormModal
          faq={editing}
          nextSortOrder={items.length}
          onClose={() => setEditing(undefined)}
          onSubmit={save}
        />
      ) : null}

      {confirming ? (
        <Modal
          open
          onClose={() => setConfirming(null)}
          title="Delete this question?"
          description="It comes off the FAQ page straight away. This can't be undone."
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
          <p className="text-sm leading-[1.55] text-ink-muted">{confirming.question}</p>
        </Modal>
      ) : null}
    </div>
  );
}

function FAQFormModal({
  faq,
  nextSortOrder,
  onClose,
  onSubmit,
}: {
  faq: FAQ | null;
  nextSortOrder: number;
  onClose: () => void;
  onSubmit: (draft: FAQDraft, existing: FAQ | null) => Promise<void>;
}) {
  const initial = {
    question: faq?.question ?? "",
    answer: faq?.answer ?? "",
    category: faq?.category ?? "",
  };
  const [question, setQuestion] = useState(initial.question);
  const [answer, setAnswer] = useState(initial.answer);
  const [category, setCategory] = useState(initial.category);
  const [errors, setErrors] = useState<{ question?: string; answer?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const dirty =
    question !== initial.question || answer !== initial.answer || category !== initial.category;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const next: { question?: string; answer?: string } = {};
    if (!question.trim()) next.question = "What's the question?";
    if (!answer.trim()) next.answer = "An answer is what makes it useful";
    setErrors(next);
    if (Object.keys(next).length > 0) return;

    setFormError(null);
    setSubmitting(true);
    try {
      await onSubmit(
        {
          question: question.trim(),
          answer: answer.trim(),
          category: category.trim() || undefined,
          ...(faq ? {} : { sortOrder: nextSortOrder }),
        },
        faq,
      );
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
      title={faq ? "Edit question" : "New question"}
      size="form"
      dirty={dirty}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" form="faq-form" loading={submitting}>
            {faq ? "Save changes" : "Add question"}
          </Button>
        </>
      }
    >
      <form id="faq-form" noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
        {formError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
            {formError}
          </div>
        ) : null}

        <TextInput
          label="Question"
          required
          data-autofocus
          value={question}
          error={errors.question}
          placeholder="Do I need to bring anything?"
          onChange={(event) => {
            setQuestion(event.target.value);
            setErrors((current) => ({ ...current, question: undefined }));
          }}
        />
        <TextArea
          label="Answer"
          required
          rows={5}
          value={answer}
          error={errors.answer}
          onChange={(event) => {
            setAnswer(event.target.value);
            setErrors((current) => ({ ...current, answer: undefined }));
          }}
        />
        <TextInput
          label="Category"
          value={category}
          hint="Optional. Groups related questions together on the page."
          placeholder="Booking"
          onChange={(event) => setCategory(event.target.value)}
        />
      </form>
    </Modal>
  );
}
