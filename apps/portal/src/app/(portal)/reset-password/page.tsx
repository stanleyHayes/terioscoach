"use client";

import { Check, CircleAlert } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useState, type FormEvent } from "react";
import { RecoveryShell } from "@/components/auth/RecoveryShell";
import { Button, buttonClasses } from "@/components/ui/Button";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError, authApi } from "@/lib/api";

export default function ResetPasswordPage() { return <Suspense fallback={null}><ResetPasswordForm /></Suspense>; }

function ResetPasswordForm() {
  const token = useSearchParams().get("token") ?? ""; const [password, setPassword] = useState(""); const [confirm, setConfirm] = useState(""); const [error, setError] = useState<string | null>(null); const [done, setDone] = useState(false); const [loading, setLoading] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setError(null); if (password.length < 12) { setError("Use at least 12 characters for your new password."); return; } if (password !== confirm) { setError("The two passwords do not match."); return; } if (!token) { setError("This reset link is incomplete. Request a new one."); return; } setLoading(true); try { await authApi.resetPassword(token, password); setDone(true); } catch (caught) { setError(caught instanceof ApiError && caught.code === "password_reset_invalid" ? "This reset link is invalid or has expired. Request a new one." : "We couldn't reset your password just now. Try again."); } finally { setLoading(false); } }
  return <RecoveryShell eyebrow="Account recovery" title={done ? "Password updated." : "Choose a new password."} description={done ? "Your previous sessions have been signed out to keep your account secure." : "Use at least 12 characters. A phrase that is easy for you to remember works well."}>
    {done ? <div className="rounded-[1.5rem_2.5rem_1.5rem_1.5rem] border border-eucalyptus-200 bg-eucalyptus-50 p-6"><span className="flex size-11 items-center justify-center rounded-full bg-eucalyptus-900 text-sand-0"><Check size={20} /></span><Link href="/login" className={buttonClasses({ className: "mt-6 w-full" })}>Sign in with new password</Link></div> : <form onSubmit={submit} noValidate className="flex flex-col gap-4"><TextInput label="New password" type="password" autoComplete="new-password" value={password} onChange={(event) => { setPassword(event.target.value); setError(null); }} hint="At least 12 characters" /><TextInput label="Confirm new password" type="password" autoComplete="new-password" value={confirm} onChange={(event) => { setConfirm(event.target.value); setError(null); }} />{error ? <p role="alert" className="flex items-start gap-2 rounded-xl bg-danger-bg px-4 py-3 text-sm text-danger-ink"><CircleAlert size={16} className="mt-0.5 shrink-0" />{error}</p> : null}<Button fullWidth loading={loading} type="submit">Update password</Button><Link href="/forgot-password" className="text-center text-sm font-semibold text-primary">Request a new link</Link></form>}
  </RecoveryShell>;
}
