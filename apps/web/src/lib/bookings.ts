/**
 * Typed client for the availability + bookings slices of the Terios API
 * (design/api-contract.md §Availability BE-04, §Bookings BE-05 — both final).
 *
 *   GET  /v1/availability/slots  public  ?serviceId=&from=&to=&tz= → {serviceId, durationMinutes, timezone, slots}
 *   POST /v1/bookings            client  {serviceId, startAt, tz?} → 201 {booking}   (409 slot_unavailable)
 *   GET  /v1/bookings/mine       client  → {items: [booking]} ascending by startAt
 *   POST /v1/bookings/{id}/reschedule    {startAt, tz?} → {booking} (409 slot_unavailable, 422 cutoff_passed)
 *   POST /v1/bookings/{id}/cancel        → {booking} (422 cutoff_passed)
 *
 * All timestamps are RFC 3339 UTC; `tz` (IANA) only tells the server which
 * wall clock to evaluate the schedule in. Authed calls go through
 * `authedRequest` with the caller's session + refresh callback (from useAuth).
 */

import {
  authedRequest,
  request,
  type RefreshCallbacks,
  type Session,
} from "./api";

/** One bookable opening. Always UTC instants, regardless of the requested tz. */
export interface Slot {
  startAt: string;
  endAt: string;
}

export interface AvailabilitySlots {
  serviceId: string;
  durationMinutes: number;
  /** The IANA timezone the schedule was evaluated in. */
  timezone: string;
  slots: Slot[];
}

export type BookingStatus = "confirmed" | "cancelled" | "completed" | "no_show";

export interface Booking {
  id: string;
  clientId: string;
  practitionerId: string;
  serviceId: string;
  /** RFC 3339 UTC. */
  startAt: string;
  endAt: string;
  status: BookingStatus;
  createdAt: string;
  updatedAt: string;
  cancelledAt?: string;
  completedAt?: string;
}

export interface GetSlotsParams {
  serviceId: string;
  /** Inclusive calendar dates, YYYY-MM-DD, interpreted in `tz`. */
  from: string;
  to: string;
  /** IANA name, e.g. "Africa/Accra". */
  tz: string;
}

/**
 * GET /v1/availability/slots — public, no auth. Fetched with cache "no-store":
 * slots go stale the moment anyone books, so nothing may serve a cached list.
 * ApiError propagates (404 service_not_found, 400 invalid_timezone, 503 …).
 */
export function getSlots(params: GetSlotsParams): Promise<AvailabilitySlots> {
  const query = new URLSearchParams({
    serviceId: params.serviceId,
    from: params.from,
    to: params.to,
    tz: params.tz,
  });
  return request<AvailabilitySlots>(`/v1/availability/slots?${query.toString()}`, {
    cache: "no-store",
  });
}

/** POST /v1/bookings → 201 {booking}. Throws ApiError 409 slot_unavailable
 * when the slot was taken in a race — the caller should refresh the picker. */
export async function createBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  input: { serviceId: string; startAt: string; tz: string },
): Promise<Booking> {
  const { booking } = await authedRequest<{ booking: Booking }>(
    "/v1/bookings",
    session,
    callbacks,
    { method: "POST", body: input },
  );
  return booking;
}

/** GET /v1/bookings/mine → the client's own bookings, ascending by startAt. */
export async function myBookings(
  session: Session,
  callbacks: RefreshCallbacks,
): Promise<Booking[]> {
  const { items } = await authedRequest<{ items: Booking[] }>(
    "/v1/bookings/mine",
    session,
    callbacks,
  );
  return items;
}

/** POST /v1/bookings/{id}/reschedule → {booking}. Throws ApiError 409
 * slot_unavailable (race) or 422 cutoff_passed (inside the 24h cutoff). */
export async function rescheduleBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  bookingId: string,
  input: { startAt: string; tz: string },
): Promise<Booking> {
  const { booking } = await authedRequest<{ booking: Booking }>(
    `/v1/bookings/${bookingId}/reschedule`,
    session,
    callbacks,
    { method: "POST", body: input },
  );
  return booking;
}

/** POST /v1/bookings/{id}/cancel → {booking}. Throws ApiError 422
 * cutoff_passed inside the 24h cutoff, 409 invalid_status on terminal states. */
export async function cancelBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  bookingId: string,
): Promise<Booking> {
  const { booking } = await authedRequest<{ booking: Booking }>(
    `/v1/bookings/${bookingId}/cancel`,
    session,
    callbacks,
    { method: "POST" },
  );
  return booking;
}

/** The platform-default client change window: reschedule/cancel is allowed
 * until 24 hours before the appointment (contract §Bookings cutoff rule). */
export const RESCHEDULE_CUTOFF_HOURS = 24;

/** Client-side mirror of the cutoff rule. Past the cutoff the server answers
 * 422 cutoff_passed — this lets the UI say so before the round trip. */
export function cutoffPassed(startAt: string, now: Date = new Date()): boolean {
  return (
    new Date(startAt).getTime() - now.getTime() <
    RESCHEDULE_CUTOFF_HOURS * 60 * 60 * 1000
  );
}

export interface SplitBookings {
  /** Confirmed sessions that have not ended yet, soonest first. */
  upcoming: Booking[];
  /** Terminal or already-ended sessions, most recent first. */
  past: Booking[];
}

/** Splits a client's bookings for the portal views. A confirmed booking moves
 * to `past` once its end time has passed (the practitioner marks it completed
 * afterwards); terminal statuses are always past. */
export function splitBookings(bookings: Booking[], now: Date = new Date()): SplitBookings {
  const nowMs = now.getTime();
  const upcoming = bookings
    .filter((b) => b.status === "confirmed" && new Date(b.endAt).getTime() > nowMs)
    .sort((a, b) => a.startAt.localeCompare(b.startAt));
  const past = bookings
    .filter((b) => !(b.status === "confirmed" && new Date(b.endAt).getTime() > nowMs))
    .sort((a, b) => b.startAt.localeCompare(a.startAt));
  return { upcoming, past };
}
