"use client";

import { CircleAlert } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

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
  const router = useRouter();
  const { status, register } = useAuth();
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

  // Already signed in — the portal is the right place to be.
  useEffect(() => {
    if (status === "authenticated") {
      router.replace("/portal");
    }
  }, [status, router]);

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
      router.replace("/portal");
    } catch (error) {
      setFormError(bannerMessage(error));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="flex flex-1 items-center justify-center bg-surface px-6 py-12">
      <div className="w-full max-w-[400px]">
        <div className="mb-8 text-center">
          <p className="font-display text-[36px] leading-[1.15] tracking-[-0.01em] text-ink">
            Terios Wellness
          </p>
          <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
            Create your client portal account
          </p>
        </div>

        <Card>
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
            href="/login"
            className="font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
          >
            Sign in
          </Link>
        </p>
      </div>
    </main>
  );
}
