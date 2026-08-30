"use client";

import { Calendar, CircleAlert } from "lucide-react";
import Link from "next/link";
import { useMemo, useState } from "react";
import { SessionRow } from "@/components/booking/SessionRow";
import { SessionFeedback } from "@/components/portal/SessionFeedback";
import { SlotPicker } from "@/components/booking/SlotPicker";
import { useMyBookings } from "@/components/booking/use-my-bookings";
import { Button, buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Modal } from "@/components/ui/Modal";
import { EmptyState } from "@/components/ui/EmptyState";
import { ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import {
  cancelBooking,
  cutoffPassed,
  rescheduleBooking,
  splitBookings,
  type Booking,
  type Slot,
} from "@/lib/bookings";
import {
  browserTimeZone,
  formatSessionDate,
  formatTimeOfDay,
  gmtOffsetLabel,
} from "@/lib/format";

/**
 * Sessions (CX-04) — every booking the client has made.
 * Upcoming (changeable): Reschedule and Cancel run in custom Modals (§3.14);
 * rescheduling reuses the SlotPicker, and both actions honor the 24-hour
 * cutoff — past it, changes close client-side, and a racing 422
 * cutoff_passed from the server gets the same branded message.
 * Past sessions are listed with their terminal-status badges.
 */

/** Brand-voice copy for the 422 cutoff_passed race (say what happened, what
 * to do next, no blame). */
const CUTOFF_MESSAGE =
  "This session is less than 24 hours away, so online changes have closed. Contact the practice and we'll help.";

function actionErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof ApiError && error.code === "cutoff_passed") {
    return CUTOFF_MESSAGE;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}

export default function SessionsPage() {
  const { session, onTokensRefreshed } = useAuth();
  const timeZone = useMemo(() => browserTimeZone(), []);
  const { bookings, servicesById, error, refresh } = useMyBookings();

  const { upcoming, past } = useMemo(
    () => splitBookings(bookings ?? []),
    [bookings],
  );

  // Reschedule modal state.
  const [rescheduling, setRescheduling] = useState<Booking | null>(null);
  const [newSlot, setNewSlot] = useState<Slot | null>(null);
  const [rescheduleError, setRescheduleError] = useState<string | null>(null);
  const [rescheduleConflict, setRescheduleConflict] = useState<string | null>(
    null,
  );
  const [reschedulingBusy, setReschedulingBusy] = useState(false);

  // Cancel modal state.
  const [cancelling, setCancelling] = useState<Booking | null>(null);
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [cancelBusy, setCancelBusy] = useState(false);

  function openReschedule(booking: Booking) {
    setRescheduling(booking);
    setNewSlot(null);
    setRescheduleError(null);
    setRescheduleConflict(null);
  }

  async function handleRescheduleConfirm() {
    if (!session || !rescheduling || !newSlot) return;
    setRescheduleError(null);
    setReschedulingBusy(true);
    try {
      await rescheduleBooking(session, { onTokensRefreshed }, rescheduling.id, {
        startAt: newSlot.startAt,
        tz: timeZone,
      });
      setRescheduling(null);
      refresh();
    } catch (error) {
      if (error instanceof ApiError && error.code === "slot_unavailable") {
        // Lost the race — the picker flags the taken chip and refreshes.
        setRescheduleConflict(newSlot.startAt);
        setNewSlot(null);
      } else {
        setRescheduleError(
          actionErrorMessage(
            error,
            "The reschedule didn't go through. Try again in a moment.",
          ),
        );
      }
    } finally {
      setReschedulingBusy(false);
    }
  }

  async function handleCancelConfirm() {
    if (!session || !cancelling) return;
    setCancelError(null);
    setCancelBusy(true);
    try {
      await cancelBooking(session, { onTokensRefreshed }, cancelling.id);
      setCancelling(null);
      refresh();
    } catch (error) {
      setCancelError(
        actionErrorMessage(
          error,
          "The cancellation didn't go through. Try again in a moment.",
        ),
      );
    } finally {
      setCancelBusy(false);
    }
  }

  return (
    <div className="animate-fade-in flex flex-col gap-10">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
            Sessions
          </p>
          <h1 className="mt-3 font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
            Your sessions
          </h1>
        </div>
        <Link href="/portal/book" className={buttonClasses({ size: "sm" })}>
          Book a session
        </Link>
      </div>

      {bookings === null && !error ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-4">
          <span className="sr-only">Loading your sessions…</span>
          {[0, 1, 2].map((index) => (
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
              className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
            >
              Try again
            </button>
          </div>
        </Card>
      ) : (
        <>
          <section
            aria-labelledby="upcoming-heading"
            className="flex flex-col gap-4"
          >
            <h2
              id="upcoming-heading"
              className="font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink"
            >
              Upcoming
            </h2>
            {upcoming.length === 0 ? (
              <Card>
                <EmptyState
                  icon={<Calendar size={28} />}
                  title="Nothing on the calendar yet"
                  description="When you book a session, it will appear here."
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
                {upcoming.map((booking) => {
                  const locked = cutoffPassed(booking.startAt);
                  return (
                    <li key={booking.id}>
                      <SessionRow
                        booking={booking}
                        serviceName={servicesById.get(booking.serviceId)?.name}
                        timeZone={timeZone}
                        actions={
                          <>
                            {/* The room enforces its own opening hours;
                                this link is always offered so a client is
                                told *why* rather than finding no way in. */}
                            <Link
                              href={`/portal/sessions/${booking.id}/room`}
                              className={buttonClasses({ size: "sm" })}
                            >
                              Join
                            </Link>
                            <Button
                              variant="secondary"
                              size="sm"
                              disabled={locked}
                              onClick={() => openReschedule(booking)}
                            >
                              Reschedule
                            </Button>
                            <Button
                              variant="ghost"
                              size="sm"
                              disabled={locked}
                              onClick={() => {
                                setCancelError(null);
                                setCancelling(booking);
                              }}
                              className="text-danger hover:bg-danger-bg hover:text-danger"
                            >
                              Cancel
                            </Button>
                          </>
                        }
                      />
                      <p className="mt-2 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
                        {locked
                          ? "Online changes close 24 hours before a session."
                          : "Free rescheduling up to 24 hours before"}
                      </p>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {past.length > 0 ? (
            <section
              aria-labelledby="past-heading"
              className="flex flex-col gap-4"
            >
              <h2
                id="past-heading"
                className="font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink"
              >
                Past
              </h2>
              <ul className="flex flex-col gap-4">
                {past.map((booking) => (
                  <li
                    key={booking.id}
                    className="overflow-hidden rounded-lg border border-border bg-surface-raised"
                  >
                    {/* The row draws its own border; inside this wrapper it
                        would double up, so the border is suppressed and the
                        wrapper carries it for the row plus its feedback. */}
                    <div className="[&>div]:rounded-none [&>div]:border-0">
                      <SessionRow
                        booking={booking}
                        serviceName={servicesById.get(booking.serviceId)?.name}
                        timeZone={timeZone}
                      />
                    </div>
                    <SessionFeedback bookingId={booking.id} />
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
        </>
      )}

      {/* Reschedule — SlotPicker inside a custom Modal. */}
      <Modal
        open={rescheduling !== null}
        onClose={() => setRescheduling(null)}
        title="Reschedule session"
        description={
          rescheduling
            ? `Currently ${formatSessionDate(rescheduling.startAt, timeZone)} at ${formatTimeOfDay(rescheduling.startAt, timeZone)} (${gmtOffsetLabel(timeZone, new Date(rescheduling.startAt))}). Free rescheduling up to 24 hours before.`
            : undefined
        }
        size="lg"
        footer={
          <>
            <Button variant="secondary" onClick={() => setRescheduling(null)}>
              Back
            </Button>
            <Button
              disabled={!newSlot}
              loading={reschedulingBusy}
              onClick={handleRescheduleConfirm}
            >
              Confirm new time
            </Button>
          </>
        }
      >
        {rescheduling ? (
          <div className="flex flex-col gap-4">
            <SlotPicker
              serviceId={rescheduling.serviceId}
              selectedSlot={newSlot}
              onSelect={(slot) => {
                setRescheduleError(null);
                setNewSlot(slot);
              }}
              conflictStartAt={rescheduleConflict}
              timeZone={timeZone}
              aria-label="Choose a new time"
            />
            {rescheduleError ? (
              <p
                role="alert"
                className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
              >
                <CircleAlert
                  size={16}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0"
                />
                {rescheduleError}
              </p>
            ) : null}
          </div>
        ) : null}
      </Modal>

      {/* Cancel — destructive confirm (§3.14: danger Button + explicit
          consequence copy). */}
      <Modal
        open={cancelling !== null}
        onClose={() => setCancelling(null)}
        title="Cancel this session?"
        size="sm"
        footer={
          <>
            <Button variant="secondary" onClick={() => setCancelling(null)}>
              Keep session
            </Button>
            <Button
              variant="danger"
              loading={cancelBusy}
              onClick={handleCancelConfirm}
            >
              Cancel session
            </Button>
          </>
        }
      >
        {cancelling ? (
          <div className="flex flex-col gap-4">
            <p className="text-sm leading-[1.55] text-ink-muted">
              Your {formatSessionDate(cancelling.startAt, timeZone)} session at{" "}
              {formatTimeOfDay(cancelling.startAt, timeZone)} (
              {gmtOffsetLabel(timeZone, new Date(cancelling.startAt))}) will be
              cancelled and the time released for someone else. This can&rsquo;t
              be undone — you&rsquo;d need to book again.
            </p>
            {cancelError ? (
              <p
                role="alert"
                className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
              >
                <CircleAlert
                  size={16}
                  aria-hidden="true"
                  className="mt-0.5 shrink-0"
                />
                {cancelError}
              </p>
            ) : null}
          </div>
        ) : null}
      </Modal>
    </div>
  );
}
