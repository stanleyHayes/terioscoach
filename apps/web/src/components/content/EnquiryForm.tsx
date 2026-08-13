"use client";

import { CircleAlert, CircleCheck } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { submitEnquiry } from "@/lib/content";
import { cn } from "@/lib/cn";

/**
 * Public contact form (design-system §3.29 form patterns).
 *
 * `noValidate` is deliberate: the browser's own validation bubbles are
 * native UI we do not use, so every message here is ours. The same rules
 * the API enforces are checked before sending, so the common mistakes are
 * caught without a round trip — but the server remains the authority, and
 * its errors are surfaced verbatim when it disagrees.
 */

interface FieldErrors {
  name?: string;
  email?: string;
  message?: string;
}

export function EnquiryForm() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [subject, setSubject] = useState("");
  const [message, setMessage] = useState("");

  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);

    const found = validate({ name, email, message });
    setErrors(found);
    if (Object.keys(found).length > 0) {
      return;
    }

    setSending(true);
    try {
      await submitEnquiry({
        name: name.trim(),
        email: email.trim(),
        phone: phone.trim() || undefined,
        subject: subject.trim() || undefined,
        message: message.trim(),
      });
      setSent(true);
    } catch (error) {
      setFormError(explain(error));
    } finally {
      setSending(false);
    }
  }

  if (sent) {
    return (
      <div
        role="status"
        className="rounded-xl border border-border bg-surface-raised p-8 text-center"
      >
        <span className="mx-auto flex size-14 items-center justify-center rounded-full bg-success-bg">
          <CircleCheck aria-hidden="true" className="size-7 text-success-ink" />
        </span>
        <h2 className="mt-6 font-display text-2xl leading-[1.2] font-medium text-ink">
          Thank you — it has arrived
        </h2>
        <p className="mx-auto mt-3 max-w-[46ch] text-base leading-[1.6] text-ink-muted [text-wrap:pretty]">
          Your message is with the practice. You will hear back at{" "}
          <span className="font-medium text-ink">{email.trim()}</span>, usually
          within a working day.
        </p>
      </div>
    );
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-6 rounded-[2rem] border border-border/70 bg-surface-raised p-6 shadow-[0_24px_80px_rgba(31,41,34,.06)] sm:p-8">
      {formError ? (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
        >
          <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0 text-danger-ink" />
          <p className="text-sm leading-[1.55] text-danger-ink">{formError}</p>
        </div>
      ) : null}

      <div className="grid gap-6 sm:grid-cols-2">
        <TextInput
          label="Your name"
          name="name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          error={errors.name}
          autoComplete="name"
          required
        />
        <TextInput
          label="Email"
          name="email"
          type="email"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          error={errors.email}
          autoComplete="email"
          required
        />
      </div>

      <div className="grid gap-6 sm:grid-cols-2">
        <TextInput
          label="Phone"
          name="phone"
          type="tel"
          value={phone}
          onChange={(event) => setPhone(event.target.value)}
          hint="Optional"
          autoComplete="tel"
        />
        <TextInput
          label="Subject"
          name="subject"
          value={subject}
          onChange={(event) => setSubject(event.target.value)}
          hint="Optional"
        />
      </div>

      <div>
        <label htmlFor="enquiry-message" className="block text-sm font-medium text-ink">
          Your message
        </label>
        <textarea
          id="enquiry-message"
          name="message"
          rows={6}
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          aria-invalid={errors.message ? true : undefined}
          aria-describedby={errors.message ? "enquiry-message-error" : undefined}
          className={cn(
            "mt-1.5 w-full rounded-lg border bg-surface-raised px-3 py-2.5 text-base leading-[1.6] text-ink",
            "placeholder:text-ink-faint transition-colors duration-instant ease-out",
            "focus:outline-none focus:ring-2",
            errors.message
              ? "border-danger focus:border-danger focus:ring-danger/20"
              : "border-border focus:border-primary focus:ring-primary/20",
          )}
        />
        {errors.message ? (
          <p
            id="enquiry-message-error"
            role="alert"
            className="mt-1.5 flex items-center gap-1.5 text-[13px] text-danger-ink"
          >
            <CircleAlert size={14} aria-hidden="true" />
            {errors.message}
          </p>
        ) : null}
      </div>

      <div className="flex items-center gap-4">
        {/* `loading` swaps the label for a spinner and locks the width, so
            the label stays constant. */}
        <Button type="submit" loading={sending}>
          Send message
        </Button>
        <p className="text-[13px] leading-[1.5] text-ink-faint">
          Your details go to the practice only.
        </p>
      </div>
    </form>
  );
}

/** Mirrors the API's own rules (contract §Enquiries) so the common mistakes
 * are caught without a round trip. The server stays the authority. */
function validate(values: { name: string; email: string; message: string }): FieldErrors {
  const errors: FieldErrors = {};

  const name = values.name.trim();
  if (!name) {
    errors.name = "Please tell us your name.";
  } else if (name.length > 120) {
    errors.name = "That name is too long for our form.";
  }

  const email = values.email.trim();
  if (!email) {
    errors.email = "We need an address to reply to.";
  } else if (!looksLikeEmail(email)) {
    errors.email = "That doesn't look like an email address.";
  }

  const message = values.message.trim();
  if (!message) {
    errors.message = "Tell us a little about what you need.";
  } else if (message.length > 5000) {
    errors.message = "That message is longer than our form accepts.";
  }

  return errors;
}

/** The same shape check the server applies: a local part, an @, and a
 * dotted domain. Anything stricter rejects addresses that are perfectly
 * valid. */
function looksLikeEmail(value: string): boolean {
  const at = value.lastIndexOf("@");
  if (at <= 0 || at === value.length - 1) return false;
  const domain = value.slice(at + 1);
  if (!domain.includes(".") || domain.startsWith(".") || domain.endsWith(".")) return false;
  return !/\s/.test(value);
}

/** Turns an API failure into something worth reading. */
function explain(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 429) {
      return "That's a few messages in a short time. Please wait a little while before sending another.";
    }
    if (error.status === 0) {
      return "We can't reach the practice right now. Check your connection and try again.";
    }
    return error.message;
  }
  return "Something went wrong sending that. Please try again in a moment.";
}
