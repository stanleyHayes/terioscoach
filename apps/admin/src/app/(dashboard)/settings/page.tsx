"use client";

import { BookOpen, LockKeyhole, Save, UserRound } from "lucide-react";
import { FormEvent, useState } from "react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { BrandedCheckbox, BrandedSelect } from "@/components/ui/ChoiceControls";
import { TextInput } from "@/components/ui/TextInput";
import { accountApi, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

const PREFS_KEY = "terios.admin.preferences";

export default function SettingsPage() {
  const { user, session, refreshCallbacks, setUserProfile, logout } = useAuth();
  const [name, setName] = useState(user?.name ?? "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [timezone, setTimezone] = useState(() => {
    if (typeof window === "undefined") return "Africa/Accra";
    try { return (JSON.parse(localStorage.getItem(PREFS_KEY) ?? "null") as { timezone?: string } | null)?.timezone ?? "Africa/Accra"; } catch { return "Africa/Accra"; }
  });
  const [reminders, setReminders] = useState(() => {
    if (typeof window === "undefined") return true;
    try { return (JSON.parse(localStorage.getItem(PREFS_KEY) ?? "null") as { reminders?: boolean } | null)?.reminders ?? true; } catch { return true; }
  });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function saveProfile(event: FormEvent) {
    event.preventDefault(); if (!session) return;
    setBusy(true); setMessage("");
    try { const result = await accountApi.updateProfile(session, refreshCallbacks, name); setUserProfile(result.user); setMessage("Profile updated."); }
    catch (cause) { setMessage(cause instanceof ApiError ? cause.message : "Profile could not be updated."); }
    finally { setBusy(false); }
  }

  async function changePassword(event: FormEvent) {
    event.preventDefault(); if (!session) return;
    setBusy(true); setMessage("");
    try { await accountApi.changePassword(session, refreshCallbacks, currentPassword, newPassword); await logout("Password updated. Sign in again on your devices."); }
    catch (cause) { setMessage(cause instanceof ApiError ? cause.message : "Password could not be updated."); setBusy(false); }
  }

  function savePreferences() {
    localStorage.setItem(PREFS_KEY, JSON.stringify({ timezone, reminders }));
    setMessage("Preferences saved.");
  }

  return <div className="mx-auto max-w-4xl space-y-6">
    <header><p className="text-xs font-semibold tracking-[.12em] text-primary uppercase">Your account</p><h1 className="mt-2 font-display text-4xl font-semibold tracking-[-.04em] text-ink">Profile & preferences</h1><p className="mt-3 text-sm text-ink-muted">Keep your details, password and working preferences current.</p></header>
    {message ? <p role="status" className="rounded-xl border border-border bg-surface-raised px-4 py-3 text-sm text-ink">{message}</p> : null}
    <Card><div className="mb-5 flex items-center gap-3"><UserRound className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Profile</h2><p className="text-sm text-ink-muted">Your name appears across the practice workspace.</p></div></div><form onSubmit={saveProfile} className="grid gap-4 sm:grid-cols-2"><TextInput label="Full name" value={name} onChange={(e)=>setName(e.target.value)} required/><TextInput label="Email" value={user?.email ?? ""} readOnly/><Button type="submit" className="sm:col-span-2 sm:justify-self-start" loading={busy}><Save size={16}/>Save profile</Button></form></Card>
    <Card><div className="mb-5 flex items-center gap-3"><LockKeyhole className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Password</h2><p className="text-sm text-ink-muted">Changing it signs out every active device.</p></div></div><form onSubmit={changePassword} className="grid gap-4 sm:grid-cols-2"><TextInput label="Current password" type="password" value={currentPassword} onChange={(e)=>setCurrentPassword(e.target.value)} required/><TextInput label="New password" type="password" hint="At least 12 characters" value={newPassword} onChange={(e)=>setNewPassword(e.target.value)} required minLength={12}/><Button type="submit" className="sm:col-span-2 sm:justify-self-start" loading={busy}>Update password</Button></form></Card>
    <Card><div className="mb-5 flex items-center gap-3"><BookOpen className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Preferences & tutorial</h2><p className="text-sm text-ink-muted">Control reminders and restart the guided workspace tour.</p></div></div><div className="grid items-end gap-5 sm:grid-cols-2"><BrandedSelect label="Timezone" value={timezone} onChange={setTimezone} options={[{value:"Africa/Accra",label:"Accra (GMT)",description:"Greenwich Mean Time"},{value:"Europe/London",label:"London",description:"GMT or British Summer Time"},{value:"America/New_York",label:"New York",description:"Eastern Time"}]}/><BrandedCheckbox label="Email session reminders" description="Receive a helpful email before upcoming appointments." checked={reminders} onChange={setReminders}/></div><div className="mt-5 flex flex-wrap gap-3"><Button type="button" onClick={savePreferences}>Save preferences</Button><Button type="button" variant="secondary" onClick={()=>{ localStorage.removeItem("terios.admin.onboarding.complete"); window.dispatchEvent(new Event("terios:onboarding")); }}>Restart tutorial</Button></div></Card>
  </div>;
}
