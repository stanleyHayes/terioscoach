"use client";

import { ArrowLeft, Check, Mail } from "lucide-react";
import Link from "next/link";
import { useState, type FormEvent } from "react";
import { RecoveryShell } from "@/components/auth/RecoveryShell";
import { Button } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/TextInput";
import { authApi } from "@/lib/api";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState(""); const [error, setError] = useState<string | null>(null); const [sent, setSent] = useState(false); const [loading, setLoading] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const value = email.trim(); setError(null); if (!EMAIL_PATTERN.test(value)) { setError("Enter a valid email address, e.g. you@example.com"); return; } setLoading(true); try { await authApi.forgotPassword(value); setSent(true); } catch { setError("We couldn't send the recovery email just now. Try again in a moment."); } finally { setLoading(false); } }
  return <RecoveryShell eyebrow="Practice recovery" title={sent ? "Check your inbox." : "Recover your workspace."} description={sent ? "If that address belongs to the practice, a private reset link is on its way." : "Enter the practitioner email and we’ll send a one-time recovery link."}>{sent ? <div className="rounded-[1.5rem_2.5rem_1.5rem_1.5rem] border border-eucalyptus-700 bg-eucalyptus-900/45 p-6"><span className="flex size-11 items-center justify-center rounded-full bg-primary text-on-primary"><Check size={20} /></span><p className="mt-5 text-sm leading-[1.65] text-ink-muted">For security, we show the same message for every address. The link expires in one hour.</p><Link href="/login" className="mt-6 inline-flex items-center gap-2 text-sm font-semibold text-primary"><ArrowLeft size={15} />Back to sign in</Link></div> : <form onSubmit={submit} noValidate className="flex flex-col gap-4"><TextInput label="Practitioner email" type="email" autoComplete="email" value={email} error={error ?? undefined} onChange={(event) => { setEmail(event.target.value); setError(null); }} /><Button fullWidth loading={loading} type="submit"><Mail size={16} />Send recovery link</Button><Link href="/login" className="text-center text-sm font-semibold text-primary">Back to sign in</Link></form>}</RecoveryShell>;
}
