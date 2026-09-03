"use client";

import { CircleAlert } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/Button";
import { WeekCalendar } from "@/components/schedule/WeekCalendar";
import { KpiStrip } from "@/components/insights/KpiStrip";
import type { BookingAction } from "@/components/schedule/BookingDetailModal";
import { ApiError, SessionExpiredError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { clientsApi } from "@/lib/clients";
import { cn } from "@/lib/cn";
import { servicesApi } from "@/lib/services";
import {
  addDaysCivil,
  dateKey,
  mondayOfWeek,
  scheduleApi,
  todayCivil,
  wallClockToUtcIso,
  PRACTICE_TIMEZONE,
  type Booking,
  type BookingStatus,
  type CivilDate,
} from "@/lib/schedule";

/**
 * Practice calendar (ADM-02). A WeekCalendar fed by GET /v1/bookings for the
 * visible week (from/to are the week's bounds converted from practice wall
 * clock to UTC). Status filter chips (§3.20) map onto the API's `status`
 * param — "all" omits it. Loading renders a skeleton grid mirroring the
 * calendar (§3.28); failures get a banner with retry.
 */

type StatusFilter = "all" | BookingStatus;

const FILTERS: { value: StatusFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "confirmed", label: "Confirmed" },
  { value: "completed", label: "Completed" },
  { value: "cancelled", label: "Cancelled" },
  { value: "no_show", label: "No-show" },
];

function errorMessage(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Something went wrong. Try again.";
}

export default function CalendarPage() {
  const { session, refreshCallbacks, logout } = useAuth();
  const [weekStart, setWeekStart] = useState<CivilDate>(() =>
    mondayOfWeek(todayCivil(PRACTICE_TIMEZONE)),
  );
  const [filter, setFilter] = useState<StatusFilter>("all");
  const [bookings, setBookings] = useState<Booking[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  // ID → name lookups so the calendar and its detail modal can show who and
  // what a booking is for, rather than the raw IDs the API returns.
  const [clientNames, setClientNames] = useState<Record<string, string>>({});
  const [serviceNames, setServiceNames] = useState<Record<string, string>>({});

  const handleSessionExpiry = useCallback(
    (error: unknown) => {
      if (error instanceof SessionExpiredError) {
        void logout(error.message);
      }
    },
    [logout],
  );

  const load = useCallback(() => {
    if (!session) return;
    let cancelled = false;
    const weekEnd = addDaysCivil(weekStart, 7);
    scheduleApi
      .listBookings(session, refreshCallbacks, {
        from: wallClockToUtcIso(
          dateKey(weekStart),
          "00:00",
          PRACTICE_TIMEZONE,
        )!,
        to: wallClockToUtcIso(dateKey(weekEnd), "00:00", PRACTICE_TIMEZONE)!,
        ...(filter === "all" ? {} : { status: filter }),
      })
      .then((items) => {
        if (!cancelled) {
          setError(null);
          setBookings(items);
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setBookings(null);
        setError(errorMessage(err));
        handleSessionExpiry(err);
      });
    return () => {
      cancelled = true;
    };
  }, [session, refreshCallbacks, weekStart, filter, handleSessionExpiry]);

  useEffect(() => load(), [load]);

  // Load the client and service directories once per session so calendar
  // blocks and the detail modal can resolve booking.clientId/serviceId to
  // display names instead of raw IDs.
  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    Promise.all([
      clientsApi.list(session, refreshCallbacks),
      servicesApi.listAll(session, refreshCallbacks),
    ])
      .then(([clients, services]) => {
        if (cancelled) return;
        setClientNames(Object.fromEntries(clients.map((c) => [c.id, c.name])));
        setServiceNames(Object.fromEntries(services.map((s) => [s.id, s.name])));
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        handleSessionExpiry(err);
      });
    return () => {
      cancelled = true;
    };
  }, [session, refreshCallbacks, handleSessionExpiry]);

  function replaceBooking(updated: Booking) {
    setBookings((prev) =>
      prev ? prev.map((b) => (b.id === updated.id ? updated : b)) : prev,
    );
    return updated;
  }

  function requireSession(): { session: NonNullable<typeof session> } {
    if (!session)
      throw new ApiError(
        0,
        "no_session",
        "Your session isn't ready. Try again.",
      );
    return { session };
  }

  async function handleAction(
    booking: Booking,
    action: BookingAction,
  ): Promise<Booking> {
    const { session: live } = requireSession();
    const call =
      action === "complete"
        ? scheduleApi.completeBooking
        : action === "cancel"
          ? scheduleApi.cancelBooking
          : scheduleApi.noShowBooking;
    try {
      return replaceBooking(await call(live, refreshCallbacks, booking.id));
    } catch (err) {
      handleSessionExpiry(err);
      throw err;
    }
  }

  async function handleReschedule(
    booking: Booking,
    startAt: string,
  ): Promise<Booking> {
    const { session: live } = requireSession();
    try {
      return replaceBooking(
        await scheduleApi.rescheduleBooking(
          live,
          refreshCallbacks,
          booking.id,
          startAt,
        ),
      );
    } catch (err) {
      handleSessionExpiry(err);
      throw err;
    }
  }

  return (
    <div data-admin-page="calendar" className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-[22px] leading-[1.3] font-semibold tracking-[-0.005em] text-ink">
            Calendar & consultations
          </h1>
          <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
            Open a confirmed booking to start its secure video consultation, or manage its status and timing.
          </p>
        </div>
        {/* status filter chips (§3.20): selected = eucalyptus-100 + primary border */}
        <div
          role="group"
          aria-label="Filter by status"
          className="flex flex-wrap gap-2"
        >
          {FILTERS.map(({ value, label }) => (
            <button
              key={value}
              type="button"
              aria-pressed={filter === value}
              onClick={() => {
                setBookings(null);
                setError(null);
                setFilter(value);
              }}
              className={cn(
                "h-7 rounded-full border px-3 text-[13px] leading-[1.45] font-medium tracking-[0.01em] transition-colors duration-fast ease-out",
                filter === value
                  ? "border-primary bg-primary text-on-primary shadow-sm"
                  : "border-border-strong bg-surface-raised text-ink hover:border-ink-faint",
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {bookings ? (
        <KpiStrip
          label="Weekly schedule summary"
          items={[
            {
              label: "This view",
              value: String(bookings.length),
              detail:
                filter === "all"
                  ? "sessions this week"
                  : `${filter.replace("_", "-")} sessions`,
            },
            {
              label: "Confirmed",
              value: String(
                bookings.filter((booking) => booking.status === "confirmed")
                  .length,
              ),
              detail: "still ahead",
            },
            {
              label: "Completed",
              value: String(
                bookings.filter((booking) => booking.status === "completed")
                  .length,
              ),
              detail: "care delivered",
            },
            {
              label: "Changed",
              value: String(
                bookings.filter(
                  (booking) =>
                    booking.status === "cancelled" ||
                    booking.status === "no_show",
                ).length,
              ),
              detail: "cancelled or no-show",
            },
          ]}
        />
      ) : null}

      {error ? (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0"
          />
          <span className="flex-1">{error}</span>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setError(null);
              load();
            }}
          >
            Try again
          </Button>
        </div>
      ) : null}

      {bookings === null && !error ? (
        <CalendarSkeleton />
      ) : error ? null : (
        <WeekCalendar
          weekStart={weekStart}
          bookings={bookings ?? []}
          clientNames={clientNames}
          serviceNames={serviceNames}
          onPrevWeek={() => {
            setBookings(null);
            setWeekStart((current) => addDaysCivil(current, -7));
          }}
          onNextWeek={() => {
            setBookings(null);
            setWeekStart((current) => addDaysCivil(current, 7));
          }}
          onToday={() => {
            setBookings(null);
            setWeekStart(mondayOfWeek(todayCivil(PRACTICE_TIMEZONE)));
          }}
          onAction={handleAction}
          onReschedule={handleReschedule}
        />
      )}
    </div>
  );
}

/** Skeleton mirroring the calendar chrome 1:1 (§3.28): header + 7-day grid. */
function CalendarSkeleton() {
  return (
    <div role="status" aria-busy="true" className="flex flex-col gap-4">
      <span className="sr-only">Loading your calendar…</span>
      <div className="flex items-end justify-between gap-3">
        <div className="flex flex-col gap-2">
          <div
            className="skeleton-shimmer h-7 w-48 rounded-sm"
            aria-hidden="true"
          />
          <div
            className="skeleton-shimmer h-4 w-28 rounded-sm"
            aria-hidden="true"
          />
        </div>
        <div
          className="skeleton-shimmer h-10 w-36 rounded-md"
          aria-hidden="true"
        />
      </div>
      <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
        <div className="grid grid-cols-[56px_repeat(7,1fr)] border-b border-border">
          <div />
          {Array.from({ length: 7 }, (_, i) => (
            <div
              key={i}
              className="flex flex-col items-center gap-1.5 border-l border-border px-2 py-2"
            >
              <div
                className="skeleton-shimmer h-3 w-8 rounded-sm"
                aria-hidden="true"
              />
              <div
                className="skeleton-shimmer h-4 w-4 rounded-sm"
                aria-hidden="true"
              />
            </div>
          ))}
        </div>
        <div
          className="grid grid-cols-[56px_repeat(7,1fr)]"
          style={{ height: 384 }}
        >
          <div />
          {Array.from({ length: 7 }, (_, i) => (
            <div key={i} className="border-l border-border p-1">
              <div
                className="skeleton-shimmer h-12 rounded-sm"
                aria-hidden="true"
                style={{ marginTop: (i % 3) * 56 }}
              />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
