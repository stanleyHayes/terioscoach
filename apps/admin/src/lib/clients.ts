/**
 * Typed client for the client-records and session-notes slices
 * (design/api-contract.md §Clients BE-07, §Session Notes BE-08).
 *
 *   GET   /v1/clients                    → {items: [clientSummary]}
 *   GET   /v1/clients/{id}               → {record: clientRecord}
 *   PATCH /v1/clients/{id}               → {profile}
 *   GET   /v1/bookings/{id}/notes        → {note}   (full practitioner shape)
 *   PUT   /v1/bookings/{id}/notes        → {note}
 *   POST  /v1/bookings/{id}/notes/share  → {note}   (one-way, idempotent)
 *
 * All calls go through authedRequest (single 401 → refresh → retry).
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export interface ClientSummary {
  id: string;
  name: string;
  email: string;
  phone?: string;
  tags: string[];
  totalSessions: number;
  /** Latest non-cancelled session; absent when there is none yet. */
  lastSessionAt?: string;
}

export type BookingStatus = "confirmed" | "cancelled" | "completed" | "no_show";

export interface ClientBooking {
  id: string;
  serviceId: string;
  startAt: string;
  endAt: string;
  status: BookingStatus;
  paymentStatus?: "paid" | "refunded";
  paidAt?: string;
}

export interface ClientRecord {
  id: string;
  name: string;
  email: string;
  phone?: string;
  tags: string[];
  practiceNotes: string;
  profileCreatedAt?: string;
  profileUpdatedAt?: string;
  recentBookings: ClientBooking[];
  payments: {
    totalPaidKobo: number;
    totalRefundedKobo: number;
    paymentCount: number;
    currency?: string;
  };
  documentCount: number;
  formSubmissionCount: number;
}

/** Practice-side fields only — a client can never write any of these. */
export interface ClientProfilePatch {
  phone?: string;
  practiceNotes?: string;
  tags?: string[];
}

export interface ClientProfile {
  id: string;
  phone?: string;
  tags: string[];
  practiceNotes: string;
  createdAt: string;
  updatedAt: string;
}

export const clientsApi = {
  async list(session: Session, callbacks: RefreshCallbacks): Promise<ClientSummary[]> {
    const { items } = await authedRequest<{ items: ClientSummary[] }>(
      "/v1/clients",
      session,
      callbacks,
    );
    return items;
  },

  async get(
    session: Session,
    callbacks: RefreshCallbacks,
    clientId: string,
  ): Promise<ClientRecord> {
    const { record } = await authedRequest<{ record: ClientRecord }>(
      `/v1/clients/${clientId}`,
      session,
      callbacks,
    );
    return record;
  },

  async updateProfile(
    session: Session,
    callbacks: RefreshCallbacks,
    clientId: string,
    patch: ClientProfilePatch,
  ): Promise<ClientProfile> {
    const { profile } = await authedRequest<{ profile: ClientProfile }>(
      `/v1/clients/${clientId}`,
      session,
      callbacks,
      { method: "PATCH", body: patch },
    );
    return profile;
  },
};

/** The practitioner's full note: private notes plus the shareable half. */
export interface SessionNote {
  id: string;
  bookingId: string;
  clientId: string;
  practitionerId: string;
  privateNotes: string;
  sharedFeedback: string;
  sharedResources: string[];
  /** Set once shared — the client sees the shared half only from then on. */
  sharedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface NoteContent {
  privateNotes: string;
  sharedFeedback: string;
  sharedResources: string[];
}

export const notesApi = {
  /** Throws ApiError 404 note_not_found when nothing has been written yet —
   * which is a normal state, not an error, for a session with no note. */
  async get(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
  ): Promise<SessionNote> {
    const { note } = await authedRequest<{ note: SessionNote }>(
      `/v1/bookings/${bookingId}/notes`,
      session,
      callbacks,
    );
    return note;
  },

  /** PUT replaces the content wholesale; sharedAt is never touched, so
   * editing after sharing neither unshares nor re-stamps. */
  async save(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
    content: NoteContent,
  ): Promise<SessionNote> {
    const { note } = await authedRequest<{ note: SessionNote }>(
      `/v1/bookings/${bookingId}/notes`,
      session,
      callbacks,
      { method: "PUT", body: content },
    );
    return note;
  },

  /** One-way and idempotent: the first share stamps sharedAt and emails the
   * client; repeats answer 200 with the note unchanged. There is no unshare. */
  async share(
    session: Session,
    callbacks: RefreshCallbacks,
    bookingId: string,
  ): Promise<SessionNote> {
    const { note } = await authedRequest<{ note: SessionNote }>(
      `/v1/bookings/${bookingId}/notes/share`,
      session,
      callbacks,
      { method: "POST" },
    );
    return note;
  },
};

/** Splits a client's bookings for the record view: what is still ahead, and
 * what has already happened (most recent first). */
export function splitClientBookings(bookings: ClientBooking[], now = new Date()) {
  const nowMs = now.getTime();
  const upcoming = bookings
    .filter((b) => b.status === "confirmed" && new Date(b.endAt).getTime() > nowMs)
    .sort((a, b) => a.startAt.localeCompare(b.startAt));
  const past = bookings
    .filter((b) => !(b.status === "confirmed" && new Date(b.endAt).getTime() > nowMs))
    .sort((a, b) => b.startAt.localeCompare(a.startAt));
  return { upcoming, past };
}
