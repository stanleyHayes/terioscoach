import { authedRequest, type AuthTokens, type RefreshCallbacks } from "@/lib/api";

export const PERMISSIONS = [
  ["dashboard.view", "Dashboard"], ["schedule.manage", "Calendar & availability"],
  ["clients.manage", "Clients & care records"], ["services.manage", "Services & pricing"],
  ["content.manage", "Website content"], ["forms.manage", "Forms"],
  ["enquiries.manage", "Enquiries"], ["reviews.manage", "Reviews"],
  ["payments.manage", "Payments"], ["reports.view", "Reports"],
  ["documents.manage", "Documents"], ["team.manage", "Team & access"],
] as const;

export type Permission = typeof PERMISSIONS[number][0];
export interface StaffMember { id: string; email: string; name: string; role: string; roleName: string; permissions: Permission[]; disabled: boolean; mfaEnabled: boolean }
export interface StaffDraft { email?: string; name: string; roleName: string; permissions: Permission[]; disabled?: boolean }

export const ROLE_PRESETS: Record<string, Permission[]> = {
  "Practice manager": PERMISSIONS.map(([value]) => value),
  "Care coordinator": ["dashboard.view", "schedule.manage", "clients.manage", "forms.manage", "enquiries.manage", "documents.manage"],
  "Content manager": ["dashboard.view", "content.manage", "reviews.manage", "enquiries.manage"],
  "Finance officer": ["dashboard.view", "payments.manage", "reports.view"],
};

export const teamApi = {
  list: (session: AuthTokens, callbacks: RefreshCallbacks) => authedRequest<{ items: StaffMember[] }>("/v1/admin/team/", session, callbacks),
  create: (session: AuthTokens, callbacks: RefreshCallbacks, body: StaffDraft) => authedRequest<{ member: StaffMember; temporaryPassword: string }>("/v1/admin/team/", session, callbacks, { method: "POST", body }),
  update: (session: AuthTokens, callbacks: RefreshCallbacks, id: string, body: StaffDraft) => authedRequest<{ member: StaffMember }>(`/v1/admin/team/${id}`, session, callbacks, { method: "PATCH", body }),
};
