/**
 * Typed client for the availability + bookings slices of the Terios API
 * (design/api-contract.md §Availability BE-04, §Bookings BE-05 — both final).
 *
 *   GET  /v1/availability/slots  public  ?serviceId=&from=&to=&tz= → {serviceId, durationMinutes, timezone, slots}
 *   POST /v1/bookings            client  {serviceId, startAt, tz?} → 201 {booking}   (409 slot_unavailable)
 *   GET  /v1/bookings/mine       client  → {items: [booking]} ascending by startAt
 *   POST /v1/bookings/{id}/reschedule-request {startAt, tz?} → 202 {booking}
 *   POST /v1/bookings/{id}/cancel-request     {reason, tz?} → 202 {booking}
 *
 * All timestamps are RFC 3339 UTC; `tz` (IANA) tells the server which client
 * calendar day to return and which timezone to include in notifications.
 * Authed calls go through `authedRequest` with the caller's session + refresh
 * callback (from useAuth).
 */

import { synchronizeClock, serverNow } from "./server-clock";
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

export type BookingStatus =
  | "pending_payment"
  | "confirmed"
  | "cancelled"
  | "completed"
  | "no_show";

export interface Participant {
  name: string;
  under18: boolean;
  accurate: boolean;
  guardianName?: string;
  guardianRelationship?: string;
  guardianEmail?: string;
  revision?: number;
  appointmentAt?: string;
}
export interface Booking {
  paymentExpired?: boolean;
  paymentExpiresAt?: string;
  participant?: Participant;
  bookingTimezone?: string;
  readinessVersion?: number;
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
  /** Inclusive client-visible calendar dates, YYYY-MM-DD, interpreted in `tz`. */
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
  return request<AvailabilitySlots>(
    `/v1/availability/slots?${query.toString()}`,
    {
      cache: "no-store",
    },
  );
}

/** POST /v1/bookings → 201 {booking}. Throws ApiError 409 slot_unavailable
 * when the slot was taken in a race — the caller should refresh the picker. */
export async function createBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  input: {
    serviceId: string;
    startAt: string;
    tz: string;
    participant: Participant;
  },
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
  const { items, serverTime } = await authedRequest<{
    items: Booking[];
    serverTime?: string;
  }>("/v1/bookings/mine", session, callbacks);
  if (serverTime) synchronizeClock(serverTime);
  return items;
}

/** POST /v1/bookings/{id}/reschedule-request → 202 {booking}. This only
 * notifies the practitioner; it does not move the meeting automatically. */
export async function requestRescheduleBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  bookingId: string,
  input: { startAt: string; tz: string },
): Promise<Booking> {
  const { booking } = await authedRequest<{ booking: Booking }>(
    `/v1/bookings/${bookingId}/reschedule-request`,
    session,
    callbacks,
    { method: "POST", body: input },
  );
  return booking;
}

/** POST /v1/bookings/{id}/cancel-request → 202 {booking}. A reason is
 * required and the booking remains confirmed until the practitioner acts. */
export async function requestCancelBooking(
  session: Session,
  callbacks: RefreshCallbacks,
  bookingId: string,
  input: { reason: string; tz: string },
): Promise<Booking> {
  const { booking } = await authedRequest<{ booking: Booking }>(
    `/v1/bookings/${bookingId}/cancel-request`,
    session,
    callbacks,
    { method: "POST", body: input },
  );
  return booking;
}

/** The platform-default client change window: reschedule/cancel is allowed
 * until 48 hours before the appointment (contract §Bookings cutoff rule). */
export const RESCHEDULE_CUTOFF_HOURS = 48;

/** Client-side mirror of the cutoff rule. Past the cutoff the server answers
 * 422 cutoff_passed — this lets the UI say so before the round trip.
 *
 * The boundary matches the server exactly: the domain allows a change while
 * `now.Before(startAt - cutoff)`, so landing *on* the 48-hour mark is already
 * closed (booking.ReschedulePolicy.CanChange, api/internal/domain/booking). */
export function cutoffPassed(
  startAt: string,
  now: Date = serverNow(),
): boolean {
  return (
    new Date(startAt).getTime() - now.getTime() <=
    RESCHEDULE_CUTOFF_HOURS * 60 * 60 * 1000
  );
}

export interface SplitBookings {
  /** Confirmed sessions that have not ended yet, soonest first. */
  upcoming: Booking[];
  pending: Booking[];
  inProgress: Booking[];
  /** Terminal or already-ended sessions, most recent first. */
  past: Booking[];
}

/** Splits a client's bookings for the portal views. A confirmed booking moves
 * to `past` once its end time has passed (the practitioner marks it completed
 * afterwards); terminal statuses are always past. */
export function splitBookings(
  bookings: Booking[],
  now: Date = serverNow(),
): SplitBookings {
  const groups: SplitBookings = {
    upcoming: [],
    pending: [],
    inProgress: [],
    past: [],
  };
  for (const booking of bookings)
    groups[bookingGroup(booking, now)].push(booking);
  for (const group of [groups.upcoming, groups.pending, groups.inProgress])
    group.sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt));
  groups.past.sort((a, b) => Date.parse(b.startAt) - Date.parse(a.startAt));
  return groups;
}

export function bookingGroup(
  booking: Booking,
  now = serverNow(),
): keyof SplitBookings {
  if (booking.status === "pending_payment") return "pending";
  if (
    booking.status !== "confirmed" ||
    Date.parse(booking.endAt) <= now.getTime()
  )
    return "past";
  return Date.parse(booking.startAt) <= now.getTime()
    ? "inProgress"
    : "upcoming";
}
