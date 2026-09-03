"use client";

import { CircleAlert, ShieldCheck } from "lucide-react";
import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { safeNextPath } from "@/lib/redirect";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PASSWORD_MIN_LENGTH = 12; // per api-contract.md Auth section

/** Maps API error codes to brand-voice copy (brand.md §2: say what happened,
 * no blame). */
function bannerMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "email_taken") {
      return "An account already exists for this email. Try signing in instead.";
    }
    return error.message;
  }
  return "Something went wrong on our side. Try again in a moment.";
}

export default function RegisterPage() {
  // useSearchParams needs a Suspense boundary at prerender time (Next.js).
  return (
    <Suspense fallback={null}>
      <RegisterForm />
    </Suspense>
  );
}

function RegisterForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, register } = useAuth();
  // ?next= carries through from the booking flow (via /login) so a new
  // account lands back on its in-progress booking. Same-site paths only.
  const next = safeNextPath(searchParams.get("next")) ?? "/portal";
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{
    name?: string;
    email?: string;
    password?: string;
  }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Already signed in — head straight to the intended destination.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace(next);
    }
  }, [status, router, next]);

  function validate(): boolean {
    const errors: { name?: string; email?: string; password?: string } = {};
    if (!name.trim()) {
      errors.name = "Enter your name";
    }
    if (!email.trim()) {
      errors.email = "Enter your email";
    } else if (!EMAIL_PATTERN.test(email.trim())) {
      errors.email = "Enter a valid email address, e.g. you@example.com";
    }
    if (!password) {
      errors.password = "Choose a password";
    } else if (password.length < PASSWORD_MIN_LENGTH) {
      errors.password = "Use at least 12 characters for your password";
    }
    setFieldErrors(errors);
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError(null);
    if (!validate()) return;

    setSubmitting(true);
    try {
      // Registration returns tokens (201) — the client is signed in immediately.
      await register(name.trim(), email.trim(), password);
      router.replace(next);
    } catch (error) {
      setFormError(bannerMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="grid min-h-[100dvh] flex-1 bg-eucalyptus-900 lg:grid-cols-[minmax(22rem,.9fr)_1.1fr]">
      <aside className="relative hidden overflow-hidden border-r border-sand-0/10 p-12 text-sand-0 lg:flex lg:flex-col lg:justify-between">
        <div aria-hidden="true" className="absolute inset-0 [background-image:radial-gradient(circle_at_15%_20%,rgba(157,195,174,.22),transparent_25rem),radial-gradient(circle_at_90%_85%,rgba(222,166,132,.16),transparent_28rem)]" />
        <Link href="/" className="relative inline-flex items-center gap-3 font-display text-2xl font-semibold tracking-[-0.03em]"><span className="flex size-10 items-center justify-center rounded-xl bg-sand-0 p-1.5"><Image src="/images/brand/identity/terios-mark.svg" alt="" width={24} height={36} className="h-full w-auto" priority /></span>Terios Wellness</Link>
        <div className="relative max-w-md"><p className="font-display text-5xl leading-[.98] font-semibold tracking-[-0.045em]">A private place for your whole care story.</p><p className="mt-6 max-w-[40ch] text-base leading-relaxed text-eucalyptus-200">Book, meet, sign, review and return—without losing the thread.</p></div>
        <p className="relative flex items-center gap-2 text-xs text-eucalyptus-300"><ShieldCheck size={15} />Built for confidential care</p>
      </aside>
      <section className="flex items-start justify-center rounded-t-[2.5rem] bg-surface px-6 pb-12 pt-16 lg:items-center lg:rounded-l-[3rem] lg:rounded-tr-none lg:py-12">
      <div className="w-full max-w-[440px]">
        <div className="mb-9"><p className="text-xs font-semibold tracking-[0.12em] text-primary uppercase">New client</p><h1 className="mt-3 font-display text-[2.75rem] leading-none font-semibold tracking-[-0.04em] text-ink">Make this space yours.</h1><p className="mt-3 text-sm leading-relaxed text-ink-muted">Create the private account that travels with your care.</p></div>

        <Card className="border-0 bg-surface-raised/70 p-0 shadow-none backdrop-blur-none">
          {/* noValidate: native validation bubbles are forbidden — errors are custom per §3.29 */}
          <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-4">
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
              label="Name"
              type="text"
              autoComplete="name"
              placeholder="Your full name"
              value={name}
              error={fieldErrors.name}
              onChange={(event) => {
                setName(event.target.value);
                setFieldErrors((errors) => ({ ...errors, name: undefined }));
              }}
            />
            <TextInput
              label="Email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              error={fieldErrors.email}
              onChange={(event) => {
                setEmail(event.target.value);
                setFieldErrors((errors) => ({ ...errors, email: undefined }));
              }}
            />
            <TextInput
              label="Password"
              type="password"
              autoComplete="new-password"
              placeholder="At least 12 characters"
              hint="Use at least 12 characters."
              value={password}
              error={fieldErrors.password}
              onChange={(event) => {
                setPassword(event.target.value);
                setFieldErrors((errors) => ({ ...errors, password: undefined }));
              }}
            />

            <Button type="submit" fullWidth loading={submitting} className="mt-2">
              Create account
            </Button>
          </form>
        </Card>

        <p className="mt-6 text-center text-sm leading-[1.55] text-ink-muted">
          Already have an account?{" "}
          <Link
            href={next === "/portal" ? "/login" : `/login?next=${encodeURIComponent(next)}`}
            className="font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
          >
            Sign in
          </Link>
        </p>
      </div>
      </section>
    </main>
  );
}
