"use client";

import { useEffect, useRef, useState } from "react";
import { Check, ScrollText } from "lucide-react";
import { ApiError } from "@/lib/api";
import { parseAgreement, type Agreement } from "@/lib/agreements";
import { BrandedCheckbox } from "@/components/ui/ChoiceControls";
import { cn } from "@/lib/cn";

/**
 * AgreementStep — reading and signing a service agreement, in the booking
 * flow, before any money moves.
 *
 * Three things this screen has to get right, because it produces a binding
 * record:
 *
 *   - The text is scrollable and complete. No "click here to read the
 *     terms" link that most people will not follow, and no truncation.
 *   - Signing is deliberate: the client types their name and ticks a box.
 *     A single button that means "I agree" to text nobody scrolled is worth
 *     very little.
 *   - What is signed is what was shown. The body rendered here is the one
 *     the server recorded the signature against.
 */
export function AgreementStep({
  agreement,
  clientName,
  onSigned,
  onBack,
}: {
  agreement: Agreement;
  /** The name on the account, offered as the starting value. */
  clientName: string;
  onSigned: (signedName: string) => Promise<void>;
  onBack: () => void;
}) {
  const [typedName, setTypedName] = useState(clientName);
  const [confirmed, setConfirmed] = useState(false);
  const [scrolledToEnd, setScrolledToEnd] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const blocks = parseAgreement(agreement.body);

  // A short agreement that does not overflow has already been "scrolled to
  // the end"; requiring a scroll that cannot happen would trap the client.
  useEffect(() => {
    const node = scrollRef.current;
    if (!node) return;
    if (node.scrollHeight <= node.clientHeight + 4) setScrolledToEnd(true);
  }, [agreement.id]);

  function handleScroll() {
    const node = scrollRef.current;
    if (!node) return;
    if (node.scrollTop + node.clientHeight >= node.scrollHeight - 24) {
      setScrolledToEnd(true);
    }
  }

  const canSign = confirmed && typedName.trim().length >= 2 && !submitting;

  async function submit() {
    if (!canSign) return;
    setSubmitting(true);
    setError(null);
    try {
      await onSigned(typedName.trim());
    } catch (failure) {
      setError(
        failure instanceof ApiError
          ? failure.message
          : "That could not be saved. Try again.",
      );
      setSubmitting(false);
    }
  }

  return (
    <section className="flex flex-col gap-5">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex size-10 shrink-0 items-center justify-center rounded-xl bg-eucalyptus-100 text-eucalyptus-800">
          <ScrollText size={18} aria-hidden="true" />
        </span>
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-ink">{agreement.title}</h2>
          <p className="mt-1 text-sm leading-relaxed text-ink-muted">
            Please read this agreement and sign it to continue. You only sign
            it once — it covers this session and every other session under it.
          </p>
        </div>
      </div>

      <div
        ref={scrollRef}
        onScroll={handleScroll}
        tabIndex={0}
        role="region"
        aria-label={`${agreement.title}, scrollable`}
        className="max-h-[26rem] overflow-y-auto rounded-2xl border border-border bg-surface-raised p-5 text-sm leading-relaxed text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        {blocks.map((block, index) => {
          if (block.kind === "heading") {
            return (
              <h3
                key={index}
                className="mt-5 mb-2 text-sm font-semibold tracking-[0.04em] text-ink uppercase first:mt-0"
              >
                {block.text}
              </h3>
            );
          }
          return (
            <p
              key={index}
              className={cn(
                "mb-3 leading-[1.65] text-ink-muted last:mb-0",
                block.kind === "clause" && "pl-4",
              )}
            >
              {block.text}
            </p>
          );
        })}
      </div>

      {!scrolledToEnd ? (
        <p className="text-xs text-ink-faint">Scroll to the end to continue.</p>
      ) : null}

      <div
        className={cn(
          "flex flex-col gap-4 rounded-2xl border border-border p-5 transition-opacity",
          scrolledToEnd ? "bg-surface-raised" : "pointer-events-none opacity-45",
        )}
      >
        <label className="flex flex-col gap-1.5">
          <span className="text-sm font-medium text-ink">
            Type your full name to sign
          </span>
          <input
            type="text"
            value={typedName}
            onChange={(event) => setTypedName(event.target.value)}
            maxLength={120}
            autoComplete="name"
            disabled={!scrolledToEnd}
            className="rounded-xl border border-border bg-surface px-4 py-3 font-display text-lg text-ink outline-none transition-[border-color,box-shadow] placeholder:text-ink-faint focus:border-primary focus:ring-2 focus:ring-primary/20"
            placeholder="Your full name"
          />
          <span className="text-xs text-ink-muted">
            Typing your name here is your signature on this agreement.
          </span>
        </label>

        <BrandedCheckbox
          checked={confirmed}
          disabled={!scrolledToEnd}
          onChange={setConfirmed}
          label={`I have read and agree to the ${agreement.title}`}
          description="I am signing this of my own free will."
        />

        {error ? (
          <p role="alert" className="text-sm text-danger-ink">
            {error}
          </p>
        ) : null}

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => void submit()}
            disabled={!canSign}
            className="inline-flex h-12 items-center gap-2 rounded-full bg-primary px-6 text-sm font-semibold text-on-primary transition-colors hover:bg-primary-hover disabled:pointer-events-none disabled:opacity-45"
          >
            <Check size={16} aria-hidden="true" />
            {submitting ? "Signing…" : "Sign and continue"}
          </button>
          <button
            type="button"
            onClick={onBack}
            className="h-12 rounded-full px-4 text-sm font-medium text-ink-muted transition-colors hover:text-ink"
          >
            Back
          </button>
        </div>
      </div>
    </section>
  );
}
