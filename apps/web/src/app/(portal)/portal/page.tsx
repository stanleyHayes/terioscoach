"use client";

import { Calendar, CircleAlert } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";
import { SessionRow } from "@/components/booking/SessionRow";
import { useMyBookings } from "@/components/booking/use-my-bookings";
import { buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
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
    () => (bookings ? splitBookings(bookings).upcoming.slice(0, UPCOMING_PREVIEW_COUNT) : []),
    [bookings],
  );

  return (
    <div data-portal-page="overview" className="animate-fade-in flex flex-col gap-8">
      <Card className="relative overflow-hidden border-eucalyptus-800 bg-eucalyptus-900 p-8 text-sand-0 shadow-[0_24px_70px_rgba(28,51,40,.18)] sm:p-10">
        <div aria-hidden="true" className="absolute -right-20 -top-24 size-64 rounded-full bg-eucalyptus-300/15 blur-3xl" />
        <p className="relative text-[11px] font-semibold uppercase tracking-[0.12em] text-eucalyptus-300">
          Overview
        </p>
        <h1 className="relative mt-3 font-display text-[2.75rem] leading-[1.02] font-semibold tracking-[-0.04em] text-sand-0">
          Welcome back, {user?.name}
        </h1>
        <p className="relative mt-4 max-w-[60ch] text-base leading-[1.7] text-eucalyptus-200">
          This is your private space for sessions, forms and documents — everything
          between you and your practitioner, in one calm place.
        </p>
      </Card>

      <section aria-labelledby="upcoming-sessions-heading">
        <div className="mb-4 flex items-baseline justify-between gap-4">
          <h2
            id="upcoming-sessions-heading"
            className="font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink"
          >
            Upcoming sessions
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
              <span key={index} aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
            ))}
          </div>
        ) : error ? (
          <Card>
            <div role="alert" className="flex flex-col items-center gap-3 py-6 text-center">
              <CircleAlert size={20} aria-hidden="true" className="text-danger-ink" />
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
            <div className="mx-auto flex max-w-[360px] flex-col items-center px-6 py-12 text-center">
              <span
                aria-hidden="true"
                className="flex size-16 items-center justify-center rounded-full bg-surface-sunken text-ink-faint"
              >
                <Calendar size={32} />
              </span>
              <h3 className="mt-6 font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
                No sessions yet
              </h3>
              <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
                Your upcoming sessions will appear here. When you book one, this page
                is where you will find it.
              </p>
              <div className="mt-6">
                <Link href="/portal/book" className={buttonClasses({ size: "sm" })}>
                  Book a session
                </Link>
              </div>
            </div>
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
