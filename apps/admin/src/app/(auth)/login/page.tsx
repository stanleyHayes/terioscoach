"use client";

import { CircleAlert, HeartPulse, Leaf } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { TextInput } from "@/components/ui/TextInput";
import { OtpInput } from "@/components/auth/OtpInput";
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
  const [mfaRequired, setMfaRequired] = useState(false);
  const [code, setCode] = useState("");

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
      if (mfaRequired && code.length !== 6) { setFormError("Enter all six digits from your authenticator app."); return; }
      if (mfaRequired) await login(email.trim(), password, code);
      else await login(email.trim(), password);
      router.replace("/");
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "mfa_required") {
          setMfaRequired(true);
          setCode("");
          setFormError(null);
        } else if (error.code === "mfa_invalid") {
          setFormError("That code is invalid or has expired. Enter the current six-digit code.");
          setCode("");
        } else if (error.code === "invalid_credentials") {
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
    <main className="grid min-h-[100dvh] bg-eucalyptus-900 lg:grid-cols-[1.08fr_.92fr]">
      <aside className="relative hidden overflow-hidden p-12 text-sand-0 lg:flex lg:flex-col lg:justify-between">
        <div aria-hidden="true" className="absolute inset-0 [background-image:radial-gradient(circle_at_20%_15%,rgba(157,195,174,.18),transparent_26rem),radial-gradient(circle_at_85%_90%,rgba(222,166,132,.14),transparent_28rem)]" />
        <Link href="/" className="relative inline-flex items-center gap-3 font-display text-2xl font-semibold tracking-[-0.035em]"><span className="flex size-9 items-center justify-center rounded-full bg-sand-0 text-eucalyptus-900"><Leaf size={16} /></span><span>Terios</span><span className="-ml-2 font-medium text-eucalyptus-300">Practice</span></Link>
        <div className="relative max-w-xl"><HeartPulse className="mb-8 size-8 text-eucalyptus-300" /><p className="font-display text-[4rem] leading-[.92] font-semibold tracking-[-0.055em]">The whole practice, without the noise.</p><p className="mt-7 max-w-[42ch] text-base leading-relaxed text-eucalyptus-200">Schedule care, keep records, publish guidance and follow every client thread from one focused workspace.</p></div>
        <p className="relative text-xs text-eucalyptus-300">Practitioner access only</p>
      </aside>
      <section className="flex items-start justify-center rounded-t-[2.5rem] bg-surface px-6 pb-12 pt-20 lg:items-center lg:rounded-l-[3rem] lg:rounded-tr-none lg:py-12">
      <div className="w-full max-w-[440px]">
        <div className="mb-9"><p className="text-xs font-semibold tracking-[0.12em] text-primary uppercase">Practice workspace</p><h1 className="mt-3 font-display text-[2.75rem] leading-none font-semibold tracking-[-0.045em] text-ink">Good to see you.</h1><p className="mt-3 text-sm leading-relaxed text-ink-muted">Sign in to open today’s care desk.</p></div>

        <Card className="border-0 bg-transparent p-0 shadow-none backdrop-blur-none">
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

            {!mfaRequired ? <TextInput
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
            /> : null}
            {!mfaRequired ? <TextInput
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
            /> : null}
            {mfaRequired ? <><p className="text-sm leading-relaxed text-ink-muted">MFA is enabled for this account. Open your authenticator app and enter the current code.</p><OtpInput value={code} onChange={(next) => { setCode(next); setFormError(null); }} disabled={submitting} /></> : null}
            {!mfaRequired ? <div className="-mt-1 flex justify-end">
              <Link href="/forgot-password" className="text-sm font-semibold text-primary transition-colors hover:text-primary-hover">
                Forgot password?
              </Link>
            </div> : <button type="button" className="text-left text-sm font-semibold text-primary" onClick={() => { setMfaRequired(false); setCode(""); setPassword(""); setFormError(null); }}>Use a different account</button>}

            <Button type="submit" fullWidth loading={submitting} className="mt-2">
              {mfaRequired ? "Verify and sign in" : "Sign in"}
            </Button>
          </form>
        </Card>
      </div>
      </section>
    </main>
  );
}
