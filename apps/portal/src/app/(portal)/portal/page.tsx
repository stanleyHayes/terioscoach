"use client";

import { Calendar, CircleAlert, Video } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";
import { SessionRow } from "@/components/booking/SessionRow";
import { useMyBookings } from "@/components/booking/use-my-bookings";
import { buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { splitBookings } from "@/lib/bookings";
import { useAuth } from "@/lib/auth";
import { browserTimeZone } from "@/lib/format";

const UPCOMING_PREVIEW_COUNT = 3;

/**
 * Portal Overview (CX-04) — welcome card + the next three upcoming sessions
 * (service, date/time with timezone, status Badge), a link through to the
 * full sessions page, and the EmptyState (design-system §3.27) when there is
 * nothing booked yet.
 */
export default function PortalOverviewPage() {
  const { user } = useAuth();
  const timeZone = useMemo(() => browserTimeZone(), []);
  const { bookings, servicesById, error, refresh } = useMyBookings();

  const upcoming = useMemo(
    () =>
      bookings
        ? splitBookings(bookings).upcoming.slice(0, UPCOMING_PREVIEW_COUNT)
        : [],
    [bookings],
  );

  return (
    <div
      data-portal-page="overview"
      className="animate-fade-in flex flex-col gap-8"
    >
      <Card className="relative overflow-hidden border-border/70 bg-surface-raised/85 p-8 shadow-[0_28px_90px_rgba(31,41,34,.08)] backdrop-blur-xl sm:p-10">
        <div aria-hidden="true" className="absolute inset-y-0 left-0 w-1 bg-primary" />
        <div
          aria-hidden="true"
          className="absolute -right-20 -top-24 size-64 rounded-full bg-primary/10 blur-3xl"
        />
        <p className="relative text-[11px] font-semibold uppercase tracking-[0.12em] text-primary">
          Overview
        </p>
        <h1 className="relative mt-3 font-display text-[2.5rem] leading-[1.02] font-semibold tracking-[-0.04em] text-ink sm:text-[2.75rem]">
          Welcome back, {user?.name}
        </h1>
        <p className="relative mt-4 max-w-[60ch] text-base leading-[1.7] text-ink-muted">
          This is your private space for sessions, forms and documents —
          everything between you and your practitioner, in one calm place.
        </p>
        {upcoming[0] ? <Link href={`/portal/sessions/${upcoming[0].id}/room`} className={buttonClasses({ size: "sm", className: "relative mt-6" })}><Video size={16}/>Join next consultation</Link> : null}
      </Card>

      <section aria-labelledby="upcoming-sessions-heading">
        <div className="mb-4 flex items-baseline justify-between gap-4">
          <h2
            id="upcoming-sessions-heading"
            className="font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink"
          >
            Upcoming consultations
          </h2>
          <Link
            href="/portal/sessions"
            className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
          >
            View all sessions
          </Link>
        </div>

        {bookings === null && !error ? (
          <div role="status" aria-busy="true" className="flex flex-col gap-4">
            <span className="sr-only">Loading your sessions…</span>
            {[0, 1].map((index) => (
              <span
                key={index}
                aria-hidden="true"
                className="h-24 rounded-lg bg-surface-sunken"
              />
            ))}
          </div>
        ) : error ? (
          <Card>
            <div
              role="alert"
              className="flex flex-col items-center gap-3 py-6 text-center"
            >
              <CircleAlert
                size={20}
                aria-hidden="true"
                className="text-danger-ink"
              />
              <p className="text-sm leading-[1.55] text-ink-muted">{error}</p>
              <button
                type="button"
                onClick={refresh}
                className={buttonClasses({ variant: "ghost", size: "sm" })}
              >
                Try again
              </button>
            </div>
          </Card>
        ) : upcoming.length === 0 ? (
          <Card>
            <EmptyState
              icon={<Calendar size={32} />}
              title="No sessions yet"
              description="Your upcoming sessions will appear here. When you book one, this page is where you will find it."
              action={
                <Link
                  href="/portal/book"
                  className={buttonClasses({ size: "sm" })}
                >
                  Book a session
                </Link>
              }
            />
          </Card>
        ) : (
          <ul className="flex flex-col gap-4">
            {upcoming.map((booking) => (
              <li key={booking.id}>
                <SessionRow
                  booking={booking}
                  serviceName={servicesById.get(booking.serviceId)?.name}
                  timeZone={timeZone}
                />
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
