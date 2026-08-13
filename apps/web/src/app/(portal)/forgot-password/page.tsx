"use client";

import { ArrowLeft, Check, CircleAlert, Mail } from "lucide-react";
import Link from "next/link";
import { useState, type FormEvent } from "react";
import { RecoveryShell } from "@/components/auth/RecoveryShell";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/TextInput";
import { authApi } from "@/lib/api";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState(""); const [error, setError] = useState<string | null>(null); const [sent, setSent] = useState(false); const [loading, setLoading] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setError(null); const value = email.trim(); if (!EMAIL_PATTERN.test(value)) { setError("Enter a valid email address, e.g. you@example.com"); return; } setLoading(true); try { await authApi.forgotPassword(value); setSent(true); } catch { setError("We couldn't send the recovery email just now. Try again in a moment."); } finally { setLoading(false); } }
  return <RecoveryShell eyebrow="Account recovery" title={sent ? "Check your inbox." : "Reset your password."} description={sent ? "If an account uses that address, a private reset link is on its way." : "Enter the email you use for Terios and we’ll send you a one-time link."}>
    {sent ? <div className="rounded-[1.5rem_2.5rem_1.5rem_1.5rem] border border-eucalyptus-200 bg-eucalyptus-50 p-6"><span className="flex size-11 items-center justify-center rounded-full bg-eucalyptus-900 text-sand-0"><Check size={20} /></span><p className="mt-5 text-sm leading-[1.65] text-ink-muted">For privacy, we show the same message whether or not the address is registered. The link expires in one hour.</p><Link href="/login" className="mt-6 inline-flex items-center gap-2 text-sm font-semibold text-primary"><ArrowLeft size={15} />Back to sign in</Link></div> : <form onSubmit={submit} noValidate className="flex flex-col gap-4"><TextInput label="Email" type="email" autoComplete="email" placeholder="you@example.com" value={email} error={error ?? undefined} onChange={(event) => { setEmail(event.target.value); setError(null); }} />{error ? <p role="alert" className="flex items-start gap-2 text-sm text-danger-ink"><CircleAlert size={16} className="mt-0.5 shrink-0" />{error}</p> : null}<Button fullWidth loading={loading} type="submit"><Mail size={16} />Send reset link</Button><Link href="/login" className="mt-2 inline-flex items-center justify-center gap-2 text-sm font-semibold text-primary"><ArrowLeft size={15} />Back to sign in</Link></form>}
  </RecoveryShell>;
}
