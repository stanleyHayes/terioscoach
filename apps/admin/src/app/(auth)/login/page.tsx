"use client";

import { CircleAlert } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { SIGN_OUT_MESSAGE_KEY, useAuth } from "@/lib/auth";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function LoginPage() {
  const router = useRouter();
  const { status, login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  // Already signed in — the dashboard is the right place to be.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace("/");
    }
  }, [status, router]);

  // Pick up a branded message handed over by a forced sign-out (e.g. a
  // non-practitioner session restored from storage). sessionStorage is an
  // external store, so the state update happens in a callback, not the
  // effect body.
  useEffect(() => {
    let message: string | null = null;
    try {
      message = sessionStorage.getItem(SIGN_OUT_MESSAGE_KEY);
      if (message) {
        sessionStorage.removeItem(SIGN_OUT_MESSAGE_KEY);
      }
    } catch {
      /* storage unavailable */
    }
    if (message) {
      queueMicrotask(() => setFormError(message));
    }
  }, []);

  function validate(): boolean {
    const errors: { email?: string; password?: string } = {};
    if (!email.trim()) {
      errors.email = "Enter your email";
    } else if (!EMAIL_PATTERN.test(email.trim())) {
      errors.email = "Enter a valid email address, e.g. you@example.com";
    }
    if (!password) {
      errors.password = "Enter your password";
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
      await login(email.trim(), password);
      router.replace("/");
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "invalid_credentials") {
          setFormError("That email and password don't match. Try again.");
        } else {
          setFormError(error.message);
        }
      } else {
        setFormError("Something went wrong. Try again.");
      }
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-surface px-6 py-12">
      <div className="w-full max-w-[400px]">
        <div className="mb-8 text-center">
          <p className="font-display text-[36px] leading-[1.15] tracking-[-0.01em] text-ink">
            Terios
          </p>
          <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
            Sign in to your practice dashboard
          </p>
        </div>

        <Card>
          {/* noValidate: native validation bubbles are forbidden — errors are custom per §30 */}
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
              autoComplete="current-password"
              placeholder="Your password"
              value={password}
              error={fieldErrors.password}
              onChange={(event) => {
                setPassword(event.target.value);
                setFieldErrors((errors) => ({ ...errors, password: undefined }));
              }}
            />

            <Button type="submit" fullWidth loading={submitting} className="mt-2">
              Sign in
            </Button>
          </form>
        </Card>
      </div>
    </main>
  );
}
