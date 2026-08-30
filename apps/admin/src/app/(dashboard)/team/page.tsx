"use client";

import { Check, Copy, ShieldCheck, UserPlus, UsersRound } from "lucide-react";
import { useState } from "react";
import { AdminPageHeader } from "@/components/layout/AdminPageHeader";
import { KpiStrip } from "@/components/insights/KpiStrip";
import {
  EmptyState,
  ErrorBanner,
  Skeletons,
} from "@/components/content/states";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Modal } from "@/components/ui/Modal";
import { Switch } from "@/components/ui/Switch";
import { TextInput } from "@/components/ui/TextInput";
import { useAuth } from "@/lib/auth";
import {
  PERMISSIONS,
  ROLE_PRESETS,
  teamApi,
  type Permission,
  type StaffDraft,
  type StaffMember,
} from "@/lib/team";
import { useResource } from "@/lib/use-resource";

const EMPTY_DRAFT: StaffDraft = {
  email: "",
  name: "",
  roleName: "Care coordinator",
  permissions: ROLE_PRESETS["Care coordinator"],
};

export default function TeamPage() {
  const { session, refreshCallbacks, user } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<StaffMember | "new" | null>(null);
  const [draft, setDraft] = useState<StaffDraft>(EMPTY_DRAFT);
  const [busy, setBusy] = useState(false);
  const [temporaryPassword, setTemporaryPassword] = useState<string | null>(
    null,
  );
  const [copied, setCopied] = useState(false);

  const team = useResource<StaffMember[]>(
    (activeSession, callbacks) =>
      teamApi.list(activeSession, callbacks).then((result) => result.items),
    [],
  );
  const items = team.data;

  function openCreate() {
    setDraft(EMPTY_DRAFT);
    setTemporaryPassword(null);
    setEditing("new");
  }
  function openEdit(member: StaffMember) {
    setDraft({
      name: member.name,
      roleName: member.roleName,
      permissions: member.permissions,
      disabled: member.disabled,
    });
    setTemporaryPassword(null);
    setEditing(member);
  }
  function choosePreset(roleName: string) {
    setDraft((current) => ({
      ...current,
      roleName,
      permissions: ROLE_PRESETS[roleName]!,
    }));
  }
  function togglePermission(permission: Permission) {
    setDraft((current) => ({
      ...current,
      permissions: current.permissions.includes(permission)
        ? current.permissions.filter((item) => item !== permission)
        : [...current.permissions, permission],
    }));
  }

  async function save() {
    if (
      !session ||
      !draft.name.trim() ||
      !draft.roleName.trim() ||
      draft.permissions.length === 0 ||
      (editing === "new" && !draft.email?.trim())
    ) {
      setError("Add a name, email, role, and at least one permission.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      if (editing === "new") {
        const result = await teamApi.create(session, refreshCallbacks, draft);
        setTemporaryPassword(result.temporaryPassword);
        await team.refresh();
      } else if (editing) {
        await teamApi.update(session, refreshCallbacks, editing.id, draft);
        await team.refresh();
        setEditing(null);
      }
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Staff access could not be saved.",
      );
    } finally {
      setBusy(false);
    }
  }

  const canManage =
    user?.role === "practitioner" || user?.permissions?.includes("team.manage");
  return (
    <div data-admin-page="team" className="flex flex-col gap-6">
      <AdminPageHeader
        eyebrow="Access control"
        title="Team, roles & permissions"
        description="Give each staff member only the parts of the practice they need. Changes apply when their access token refreshes."
        actions={
          canManage ? (
            <Button onClick={openCreate}>
              <UserPlus size={16} />
              Add staff member
            </Button>
          ) : undefined
        }
      />
      {items ? (
        <KpiStrip
          label="Team access summary"
          items={[
            {
              label: "Staff accounts",
              value: String(
                items.filter((member) => member.role === "staff").length,
              ),
              detail: "plus the practice owner",
            },
            {
              label: "Active",
              value: String(items.filter((member) => !member.disabled).length),
              detail: "can currently sign in",
            },
            {
              label: "MFA enabled",
              value: String(items.filter((member) => member.mfaEnabled).length),
              detail: "protected accounts",
            },
            {
              label: "Roles in use",
              value: String(
                new Set(items.map((member) => member.roleName || member.role))
                  .size,
              ),
              detail: "access profiles",
            },
          ]}
        />
      ) : null}
      <ErrorBanner message={error ?? team.error} />
      {items === null && !error && !team.error ? (
        <Skeletons label="Loading team…" />
      ) : !team.error && items?.length === 0 ? (
        <EmptyState
          icon={<UsersRound size={26} />}
          title="No staff members yet"
          body="Add your first team member, choose a role, then fine-tune exactly what they can access."
          action={
            <Button onClick={openCreate}>
              <UserPlus size={16} />
              Add staff member
            </Button>
          }
        />
      ) : !team.error ? (
        <div className="grid gap-4">
          {items?.map((member) => (
            <Card
              key={member.id}
              className="flex flex-col gap-5 p-5 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="flex min-w-0 items-start gap-4">
                <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-eucalyptus-100 font-semibold text-eucalyptus-800">
                  {member.name.slice(0, 1).toUpperCase()}
                </span>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="truncate font-semibold text-ink">
                      {member.name}
                    </h2>
                    <Badge variant={member.disabled ? "danger" : "success"} dot>
                      {member.disabled ? "Disabled" : "Active"}
                    </Badge>
                    {member.role === "practitioner" ? (
                      <Badge variant="info">Owner</Badge>
                    ) : null}
                  </div>
                  <p className="mt-1 truncate text-sm text-ink-muted">
                    {member.email}
                  </p>
                  <p className="mt-2 text-xs font-semibold uppercase tracking-[.08em] text-primary">
                    {member.roleName}
                  </p>
                  <p className="mt-1 text-xs text-ink-faint">
                    {member.role === "practitioner"
                      ? "Full practice access"
                      : `${member.permissions.length} of ${PERMISSIONS.length} permissions`}
                    {member.mfaEnabled ? " · MFA on" : " · MFA off"}
                  </p>
                </div>
              </div>
              {member.role === "staff" ? (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => openEdit(member)}
                >
                  Manage access
                </Button>
              ) : null}
            </Card>
          ))}
        </div>
      ) : null}

      <Modal
        open={editing !== null && !temporaryPassword}
        onClose={() => !busy && setEditing(null)}
        title={editing === "new" ? "Add staff member" : "Manage staff access"}
        description="Roles provide a sensible starting point. Every permission can be adjusted."
        size="form"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => setEditing(null)}
              disabled={busy}
            >
              Cancel
            </Button>
            <Button onClick={save} loading={busy}>
              {editing === "new" ? "Create account" : "Save access"}
            </Button>
          </>
        }
      >
        <div className="grid gap-5 py-2">
          <TextInput
            label="Full name"
            value={draft.name}
            onChange={(event) =>
              setDraft((current) => ({ ...current, name: event.target.value }))
            }
            required
          />
          {editing === "new" ? (
            <TextInput
              label="Email address"
              type="email"
              value={draft.email}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  email: event.target.value,
                }))
              }
              required
            />
          ) : null}
          <fieldset>
            <legend className="text-sm font-medium text-ink">
              Role template
            </legend>
            <div className="mt-2 flex flex-wrap gap-2">
              {Object.keys(ROLE_PRESETS).map((roleName) => (
                <button
                  key={roleName}
                  type="button"
                  onClick={() => choosePreset(roleName)}
                  className={`rounded-full border px-3 py-2 text-xs font-semibold ${draft.roleName === roleName ? "border-primary bg-primary text-on-primary" : "border-border bg-surface-raised text-ink-muted"}`}
                >
                  {roleName}
                </button>
              ))}
            </div>
          </fieldset>
          <TextInput
            label="Role name"
            value={draft.roleName}
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                roleName: event.target.value,
              }))
            }
            required
          />
          <fieldset>
            <legend className="text-sm font-medium text-ink">
              Permissions
            </legend>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {PERMISSIONS.map(([permission, label]) => (
                <button
                  key={permission}
                  type="button"
                  aria-pressed={draft.permissions.includes(permission)}
                  onClick={() => togglePermission(permission)}
                  className={`flex min-h-11 items-center gap-3 rounded-xl border px-3 py-2 text-left text-sm ${draft.permissions.includes(permission) ? "border-primary bg-eucalyptus-50 text-ink" : "border-border bg-surface-raised text-ink-muted"}`}
                >
                  <span
                    className={`flex size-5 shrink-0 items-center justify-center rounded-md border ${draft.permissions.includes(permission) ? "border-primary bg-primary text-on-primary" : "border-border-strong"}`}
                  >
                    {draft.permissions.includes(permission) ? (
                      <Check size={13} />
                    ) : null}
                  </span>
                  {label}
                </button>
              ))}
            </div>
          </fieldset>
          {editing !== "new" ? (
            <div className="flex items-center justify-between rounded-xl border border-border p-4">
              <div>
                <p className="text-sm font-semibold text-ink">
                  Account enabled
                </p>
                <p className="mt-1 text-xs text-ink-muted">
                  Disabled staff cannot sign in.
                </p>
              </div>
              <Switch
                checked={!draft.disabled}
                onChange={(enabled) =>
                  setDraft((current) => ({ ...current, disabled: !enabled }))
                }
                label="Account enabled"
              />
            </div>
          ) : null}
        </div>
      </Modal>

      <Modal
        open={temporaryPassword !== null}
        onClose={() => {
          setTemporaryPassword(null);
          setEditing(null);
        }}
        title="Staff account created"
        description="Share this temporary password securely. It is shown only once."
        footer={
          <Button
            onClick={() => {
              setTemporaryPassword(null);
              setEditing(null);
            }}
          >
            Done
          </Button>
        }
      >
        <div className="rounded-2xl border border-success/30 bg-success-bg p-5">
          <div className="flex items-center gap-2 text-success-ink">
            <ShieldCheck size={18} />
            <p className="text-sm font-semibold">One-time credential</p>
          </div>
          <code className="mt-4 block break-all rounded-xl bg-surface-raised p-4 text-sm text-ink">
            {temporaryPassword}
          </code>
          <Button
            variant="secondary"
            size="sm"
            className="mt-4"
            onClick={async () => {
              if (temporaryPassword) {
                await navigator.clipboard.writeText(temporaryPassword);
                setCopied(true);
              }
            }}
          >
            {copied ? <Check size={15} /> : <Copy size={15} />}
            {copied ? "Copied" : "Copy password"}
          </Button>
        </div>
      </Modal>
    </div>
  );
}
