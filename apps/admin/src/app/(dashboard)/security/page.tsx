"use client";

import { CheckCircle2, KeyRound, ShieldCheck } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useState } from "react";
import { OtpInput } from "@/components/auth/OtpInput";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ApiError, authApi } from "@/lib/api";
import { useAuth } from "@/lib/auth";

type Enrollment = { secret: string; otpAuthUrl: string };

export default function SecurityPage() {
  const { user, accessToken, logout, setMfaEnabled } = useAuth();
  const [enabled, setEnabled] = useState(Boolean(user?.mfaEnabled));
  const [enrollment, setEnrollment] = useState<Enrollment | null>(null);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function startEnrollment() {
    if (!accessToken) return;
    setBusy(true); setError(null);
    try { setEnrollment(await authApi.beginMfa(accessToken)); setCode(""); }
    catch (cause) { setError(cause instanceof ApiError ? cause.message : "MFA setup could not start."); }
    finally { setBusy(false); }
  }
  async function confirm() {
    if (!accessToken || code.length !== 6) { setError("Enter all six digits shown in your authenticator app."); return; }
    setBusy(true); setError(null);
    try { await authApi.confirmMfa(accessToken, code); setEnabled(true); setMfaEnabled(true); setEnrollment(null); setCode(""); }
    catch (cause) { setError(cause instanceof ApiError ? cause.message : "That code could not be verified."); setCode(""); }
    finally { setBusy(false); }
  }
  async function disable() {
    if (!accessToken || code.length !== 6) { setError("Enter the current six-digit code before disabling MFA."); return; }
    setBusy(true); setError(null);
    try { await authApi.disableMfa(accessToken, code); await logout("MFA was disabled. Sign in again to continue."); }
    catch (cause) { setError(cause instanceof ApiError ? cause.message : "MFA could not be disabled."); setCode(""); setBusy(false); }
  }

  return <div className="mx-auto max-w-3xl"><header className="mb-8"><p className="text-xs font-semibold tracking-[0.12em] text-primary uppercase">Account security</p><h1 className="mt-2 font-display text-4xl font-semibold tracking-[-0.04em] text-ink">Multi-factor authentication</h1><p className="mt-3 max-w-2xl text-sm leading-relaxed text-ink-muted">MFA is optional. You will only be asked for an authenticator code after you enable and verify it here.</p></header>
    <Card className="p-6 sm:p-8">{error ? <div role="alert" className="mb-5 rounded-xl bg-danger-bg px-4 py-3 text-sm text-danger-ink">{error}</div> : null}
      {enabled ? <div><div className="flex items-start gap-4"><span className="flex size-11 items-center justify-center rounded-xl bg-primary/12 text-primary"><ShieldCheck size={22} /></span><div><h2 className="font-display text-xl font-semibold text-ink">MFA is enabled</h2><p className="mt-1 text-sm leading-relaxed text-ink-muted">Each future password sign-in requires the current six-digit code.</p></div></div><div className="mt-7 max-w-sm"><OtpInput value={code} onChange={(next) => { setCode(next); setError(null); }} disabled={busy} label="Current code to disable MFA" /><Button variant="secondary" className="mt-4" loading={busy} onClick={disable}>Disable MFA</Button></div></div> : enrollment ? <div><div className="flex items-start gap-4"><span className="flex size-11 items-center justify-center rounded-xl bg-clay-100 text-clay-700"><KeyRound size={22} /></span><div><h2 className="font-display text-xl font-semibold text-ink">Scan and verify</h2><p className="mt-1 text-sm leading-relaxed text-ink-muted">Scan this QR code with Google Authenticator, Microsoft Authenticator, 1Password, or another TOTP app.</p></div></div><div className="mt-6 grid gap-7 sm:grid-cols-[220px_1fr] sm:items-center"><div className="rounded-2xl border border-border bg-white p-4"><QRCodeSVG value={enrollment.otpAuthUrl} size={186} level="M" title="Terios MFA enrollment QR code" /></div><div><p className="text-xs font-semibold tracking-wide text-ink-muted uppercase">Can’t scan?</p><code className="mt-2 block break-all rounded-xl bg-surface-sunken p-3 text-xs text-ink">{enrollment.secret}</code><div className="mt-5"><OtpInput value={code} onChange={(next) => { setCode(next); setError(null); }} disabled={busy} /><Button className="mt-4" loading={busy} onClick={confirm}><CheckCircle2 size={16} />Verify and enable</Button></div></div></div></div> : <div className="flex flex-col items-start gap-5 sm:flex-row sm:items-center sm:justify-between"><div className="flex items-start gap-4"><span className="flex size-11 items-center justify-center rounded-xl bg-surface-sunken text-ink-muted"><ShieldCheck size={22} /></span><div><h2 className="font-display text-xl font-semibold text-ink">MFA is off</h2><p className="mt-1 max-w-lg text-sm leading-relaxed text-ink-muted">Your password is currently the only sign-in factor. Turn MFA on when you are ready.</p></div></div><Button loading={busy} onClick={startEnrollment}>Enable MFA</Button></div>}
    </Card></div>;
}
