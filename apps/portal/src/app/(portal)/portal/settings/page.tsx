"use client";

import { FormEvent, useState } from "react";
import { Bell, LockKeyhole, Save, UserRound } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { BrandedCheckbox } from "@/components/ui/ChoiceControls";
import { TextInput } from "@/components/ui/TextInput";
import { PortalPage } from "@/components/portal/PortalPage";
import { accountApi, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";

const PREFS_KEY = "terios.portal.preferences";

export default function PortalSettingsPage() {
  const { user, session, onTokensRefreshed, setUserProfile, logout } = useAuth();
  const [name, setName] = useState(user?.name ?? "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [emailReminders, setEmailReminders] = useState(() => {
    if (typeof window === "undefined") return true;
    try { const saved = JSON.parse(localStorage.getItem(PREFS_KEY) ?? "null") as { emailReminders?: boolean } | null; return saved?.emailReminders ?? true; } catch { return true; }
  });
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);

  async function saveProfile(event: FormEvent) {
    event.preventDefault(); if (!session) return; setBusy(true); setMessage("");
    try { const result = await accountApi.updateProfile(session, { onTokensRefreshed }, name); setUserProfile(result.user); setMessage("Profile updated."); }
    catch (cause) { setMessage(cause instanceof ApiError ? cause.message : "Profile could not be updated."); }
    finally { setBusy(false); }
  }
  async function changePassword(event: FormEvent) {
    event.preventDefault(); if (!session) return; setBusy(true); setMessage("");
    try { await accountApi.changePassword(session, { onTokensRefreshed }, currentPassword, newPassword); await logout(); }
    catch (cause) { setMessage(cause instanceof ApiError ? cause.message : "Password could not be updated."); setBusy(false); }
  }

  return <PortalPage title="Profile & preferences" intro="Keep your personal details and care reminders up to date.">
    <div className="space-y-5">{message ? <p role="status" className="rounded-xl border border-border bg-surface-raised px-4 py-3 text-sm">{message}</p> : null}
      <Card><div className="mb-5 flex items-center gap-3"><UserRound className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Profile</h2><p className="text-sm text-ink-muted">Used on bookings and care records.</p></div></div><form onSubmit={saveProfile} className="grid gap-4 sm:grid-cols-2"><TextInput label="Full name" value={name} onChange={(e)=>setName(e.target.value)} required/><TextInput label="Email" value={user?.email ?? ""} readOnly/><Button type="submit" className="sm:col-span-2 sm:justify-self-start" loading={busy}><Save size={16}/>Save profile</Button></form></Card>
      <Card><div className="mb-5 flex items-center gap-3"><LockKeyhole className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Password</h2><p className="text-sm text-ink-muted">All devices sign out after a change.</p></div></div><form onSubmit={changePassword} className="grid gap-4 sm:grid-cols-2"><TextInput label="Current password" type="password" value={currentPassword} onChange={(e)=>setCurrentPassword(e.target.value)} required/><TextInput label="New password" type="password" hint="At least 12 characters" value={newPassword} onChange={(e)=>setNewPassword(e.target.value)} required minLength={12}/><Button type="submit" className="sm:col-span-2 sm:justify-self-start" loading={busy}>Update password</Button></form></Card>
      <Card><div className="mb-5 flex items-center gap-3"><Bell className="text-primary"/><div><h2 className="font-display text-xl font-semibold">Care preferences</h2><p className="text-sm text-ink-muted">Choose how Terios supports you between sessions.</p></div></div><BrandedCheckbox label="Email me before upcoming consultations" description="We will send a gentle reminder so you have time to prepare." checked={emailReminders} onChange={setEmailReminders}/><Button type="button" className="mt-4" onClick={()=>{localStorage.setItem(PREFS_KEY, JSON.stringify({emailReminders})); setMessage("Preferences saved.");}}>Save preferences</Button></Card>
    </div>
  </PortalPage>;
}
