/**
 * Typed client for the Availability (BE-04) and Bookings (BE-05) slices of the
 * API contract:
 *
 *   GET  /v1/availability/rules            → 200 {rules}   (full weekly set)
 *   PUT  /v1/availability/rules            → 200 {rules}   (replaces the set)
 *   POST /v1/availability/time-off         → 201 {timeOff}
 *   GET  /v1/bookings?from=&to=&status=    → 200 {items}   (practitioner)
 *   POST /v1/bookings/{id}/reschedule      → 200 {booking}
 *   POST /v1/bookings/{id}/cancel          → 200 {booking}
 *   POST /v1/bookings/{id}/complete        → 200 {booking}
 *   POST /v1/bookings/{id}/no-show         → 200 {booking}
 *
 * All calls go through authedRequest (single 401 → refresh → retry).
 *
 * This module also owns the calendar date/time helpers. The practice schedule
 * is evaluated in one IANA timezone (the contract default, Africa/Accra), so
 * all civil-date math below is done via Intl parts in that zone rather than
 * browser-local Date math — the rendered week is identical wherever the
 * practitioner's browser happens to be.
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

/** Practice wall clock; mirrors the contract's `tz` default. */
export const PRACTICE_TIMEZONE = "Africa/Accra";

/* ---------- contract shapes ---------- */

export interface AvailabilityWindow {
  /** Minutes since local midnight; 0 <= startMin < endMin <= 1440. */
  startMin: number;
  endMin: number;
}

export interface AvailabilityRule {
  /** 0 = Sunday … 6 = Saturday. A weekday with no rule is closed. */
  weekday: number;
  /** Sorted, non-overlapping; overnight windows are rejected by the API. */
  windows: AvailabilityWindow[];
  /** Recovery gap kept free around busy intervals; 0–120. */
  bufferMinutes: number;
}

export interface TimeOff {
  id: string;
  practitionerId: string;
  startAt: string;
  endAt: string;
  reason: string;
  createdAt: string;
}

export interface TimeOffDraft {
  startAt: string;
  endAt: string;
  reason?: string;
}

export type BookingStatus = "confirmed" | "cancelled" | "completed" | "no_show";

export interface Booking {
  id: string;
  clientId: string;
  practitionerId: string;
  serviceId: string;
  startAt: string;
  endAt: string;
  status: BookingStatus;
  createdAt: string;
  updatedAt: string;
  cancelledAt?: string;
  completedAt?: string;
}

export interface ListBookingsParams {
  /** RFC 3339 range bounds; both optional per contract. */
  from?: string;
  to?: string;
  status?: BookingStatus;
}

/* ---------- API ---------- */

export const scheduleApi = {
  async getRules(
    session: Session,
    callbacks: RefreshCallbacks,
  ): Promise<AvailabilityRule[]> {
    const data = await authedRequest<{ rules: AvailabilityRule[] }>(
      "/v1/availability/rules",
      session,
      callbacks,
    );
    return data.rules;
  },

  /** Full weekly replacement: send every open day; closed days are omitted. */
  async putRules(
    session: Session,
    callbacks: RefreshCallbacks,
    rules: AvailabilityRule[],
  ): Promise<AvailabilityRule[]> {
    const data = await authedRequest<{ rules: AvailabilityRule[] }>(
      "/v1/availability/rules",
      session,
      callbacks,
      { method: "PUT", body: { rules } },
    );
    return data.rules;
  },

  async addTimeOff(
    session: Session,
    callbacks: RefreshCallbacks,
    draft: TimeOffDraft,
  ): Promise<TimeOff> {
    const data = await authedRequest<{ timeOff: TimeOff }>(
      "/v1/availability/time-off",
      session,
      callbacks,
      { method: "POST", body: draft },
    );
    return data.timeOff;
  },

  async listBookings(
    session: Session,
    callbacks: RefreshCallbacks,
    params: ListBookingsParams = {},
  ): Promise<Booking[]> {
    const query = new URLSearchParams();
    if (params.from) query.set("from", params.from);
    if (params.to) query.set("to", params.to);
    if (params.status) query.set("status", params.status);
    const suffix = query.size > 0 ? `?${query.toString()}` : "";
    const data = await authedRequest<{ items: Booking[] }>(
      `/v1/bookings${suffix}`,
      session,
      callbacks,
    );
    return data.items;
  },

  async rescheduleBooking(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
    startAt: string,
    tz: string = PRACTICE_TIMEZONE,
  ): Promise<Booking> {
    const data = await authedRequest<{ booking: Booking }>(
      `/v1/bookings/${id}/reschedule`,
      session,
      callbacks,
      { method: "POST", body: { startAt, tz } },
    );
    return data.booking;
  },

  async cancelBooking(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
  ): Promise<Booking> {
    const data = await authedRequest<{ booking: Booking }>(
      `/v1/bookings/${id}/cancel`,
      session,
      callbacks,
      { method: "POST" },
    );
    return data.booking;
  },

  async completeBooking(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
  ): Promise<Booking> {
    const data = await authedRequest<{ booking: Booking }>(
      `/v1/bookings/${id}/complete`,
      session,
      callbacks,
      { method: "POST" },
    );
    return data.booking;
  },

  async noShowBooking(
    session: Session,
    callbacks: RefreshCallbacks,
    id: string,
  ): Promise<Booking> {
    const data = await authedRequest<{ booking: Booking }>(
      `/v1/bookings/${id}/no-show`,
      session,
      callbacks,
      { method: "POST" },
    );
    return data.booking;
  },
};

/* ---------- civil-date + timezone helpers ---------- */

export interface CivilDate {
  year: number;
  month: number; // 1–12
  day: number; // 1–31
}

export interface ZonedParts extends CivilDate {
  hour: number;
  minute: number;
  /** Minutes since local midnight in the zone. */
  minutesSinceMidnight: number;
}

const dtfCache = new Map<string, Intl.DateTimeFormat>();

function zonedDtf(timeZone: string): Intl.DateTimeFormat {
  let dtf = dtfCache.get(timeZone);
  if (!dtf) {
    dtf = new Intl.DateTimeFormat("en-US", {
      timeZone,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    });
    dtfCache.set(timeZone, dtf);
  }
  return dtf;
}

/** Civil + clock parts of an instant in `timeZone`. */
export function zonedParts(iso: string | Date, timeZone: string): ZonedParts {
  const date = typeof iso === "string" ? new Date(iso) : iso;
  const parts: Record<string, number> = {};
  for (const part of zonedDtf(timeZone).formatToParts(date)) {
    if (part.type !== "literal") parts[part.type] = Number(part.value);
  }
  const hour = parts["hour"]! % 24; // h23 can yield "24" on some runtimes
  const minute = parts["minute"]!;
  return {
    year: parts["year"]!,
    month: parts["month"]!,
    day: parts["day"]!,
    hour,
    minute,
    minutesSinceMidnight: hour * 60 + minute,
  };
}

/** "2026-08-10" — the key day columns and bookings are matched on. */
export function dateKey(date: CivilDate): string {
  const mm = String(date.month).padStart(2, "0");
  const dd = String(date.day).padStart(2, "0");
  return `${date.year}-${mm}-${dd}`;
}

export function addDaysCivil(date: CivilDate, days: number): CivilDate {
  const utc = new Date(Date.UTC(date.year, date.month - 1, date.day + days));
  return {
    year: utc.getUTCFullYear(),
    month: utc.getUTCMonth() + 1,
    day: utc.getUTCDate(),
  };
}

/** Monday of the week containing `date` (weeks start Monday in the admin). */
export function mondayOfWeek(date: CivilDate): CivilDate {
  const utc = new Date(Date.UTC(date.year, date.month - 1, date.day));
  const weekday = utc.getUTCDay(); // 0 = Sunday
  return addDaysCivil(date, -((weekday + 6) % 7));
}

/** Today as a civil date in `timeZone`. */
export function todayCivil(timeZone: string, now: Date = new Date()): CivilDate {
  const parts = zonedParts(now, timeZone);
  return { year: parts.year, month: parts.month, day: parts.day };
}

/**
 * Convert a wall-clock date+time in `timeZone` to an RFC 3339 UTC instant.
 * Returns null for malformed input. Offset is resolved via Intl, so DST
 * transitions in other zones are handled; `time` is strict "HH:MM" (24h).
 */
export function wallClockToUtcIso(
  date: string,
  time: string,
  timeZone: string,
): string | null {
  const civil = parseDateInput(date);
  const minutes = parseTimeInput(time);
  if (!civil || minutes === null) return null;
  const guess = Date.UTC(
    civil.year,
    civil.month - 1,
    civil.day,
    Math.floor(minutes / 60),
    minutes % 60,
  );
  const wallAtGuess = zonedParts(new Date(guess), timeZone);
  const wallGuessUtc = Date.UTC(
    wallAtGuess.year,
    wallAtGuess.month - 1,
    wallAtGuess.day,
    wallAtGuess.hour,
    wallAtGuess.minute,
  );
  const offsetMs = wallGuessUtc - guess;
  return new Date(guess - offsetMs).toISOString();
}

/* ---------- input parsing / formatting ---------- */

const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/;
const TIME_PATTERN = /^(\d{1,2}):([0-5]\d)$/;

/** Strict "YYYY-MM-DD" → CivilDate, null on impossible dates (e.g. Feb 30). */
export function parseDateInput(input: string): CivilDate | null {
  const match = DATE_PATTERN.exec(input.trim());
  if (!match) return null;
  const [, y, m, d] = match;
  const date = { year: Number(y), month: Number(m), day: Number(d) };
  const utc = new Date(Date.UTC(date.year, date.month - 1, date.day));
  if (
    utc.getUTCFullYear() !== date.year ||
    utc.getUTCMonth() !== date.month - 1 ||
    utc.getUTCDate() !== date.day
  ) {
    return null;
  }
  return date;
}

/** "9:30" / "09:30" → 570; null on anything else (hour 0–23). */
export function parseTimeInput(input: string): number | null {
  const match = TIME_PATTERN.exec(input.trim());
  if (!match) return null;
  const hour = Number(match[1]);
  if (hour > 23) return null;
  return hour * 60 + Number(match[2]);
}

/** 570 → "09:30" (24h, zero-padded — edit format for HH:MM fields). */
export function minutesToTimeString(minutes: number): string {
  const h = String(Math.floor(minutes / 60)).padStart(2, "0");
  const m = String(minutes % 60).padStart(2, "0");
  return `${h}:${m}`;
}

const timeFormatCache = new Map<string, Intl.DateTimeFormat>();

function timeFormat(timeZone: string): Intl.DateTimeFormat {
  let format = timeFormatCache.get(timeZone);
  if (!format) {
    format = new Intl.DateTimeFormat("en-US", {
      timeZone,
      hour: "numeric",
      minute: "2-digit",
    });
    timeFormatCache.set(timeZone, format);
  }
  return format;
}

/** "2026-08-11T15:30:00Z" → "3:30 PM" in the practice zone. */
export function formatTime(iso: string, timeZone: string): string {
  return timeFormat(timeZone).format(new Date(iso));
}

/** Short zone label for the always-visible timezone caption, e.g. "GMT". */
export function timezoneShortName(timeZone: string, at: Date = new Date()): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    timeZoneName: "short",
  }).formatToParts(at);
  return parts.find((part) => part.type === "timeZoneName")?.value ?? timeZone;
}

const WEEKDAY_SHORT = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
const WEEKDAY_LONG = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
] as const;

/** 0 = Sunday … 6 = Saturday, matching the contract's rule.weekday. */
export function weekdayShortName(weekday: number): string {
  return WEEKDAY_SHORT[weekday] ?? "";
}

export function weekdayLongName(weekday: number): string {
  return WEEKDAY_LONG[weekday] ?? "";
}

const MONTH_SHORT = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
] as const;

/** "Tue, Aug 11, 2026" — the brand scheduling date format. */
export function formatCivilDate(date: CivilDate): string {
  const weekday = new Date(
    Date.UTC(date.year, date.month - 1, date.day),
  ).getUTCDay();
  return `${WEEKDAY_SHORT[weekday]}, ${MONTH_SHORT[date.month - 1]} ${date.day}, ${date.year}`;
}

/** "Aug 10–16, 2026" / "Dec 28, 2026 – Jan 3, 2027" for the week header. */
export function formatWeekRange(weekStart: CivilDate): string {
  const weekEnd = addDaysCivil(weekStart, 6);
  const startMonth = MONTH_SHORT[weekStart.month - 1];
  const endMonth = MONTH_SHORT[weekEnd.month - 1];
  if (weekStart.year !== weekEnd.year) {
    return `${startMonth} ${weekStart.day}, ${weekStart.year} – ${endMonth} ${weekEnd.day}, ${weekEnd.year}`;
  }
  if (weekStart.month !== weekEnd.month) {
    return `${startMonth} ${weekStart.day} – ${endMonth} ${weekEnd.day}, ${weekStart.year}`;
  }
  return `${startMonth} ${weekStart.day}–${weekEnd.day}, ${weekStart.year}`;
}
