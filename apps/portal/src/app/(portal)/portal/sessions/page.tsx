"use client";

import { Calendar, CircleAlert } from "lucide-react";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { SessionRow } from "@/components/booking/SessionRow";
import { RecordingPlayer } from "@/components/portal/RecordingPlayer";
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
  formatBytes,
  recordingsApi,
  type SessionRecording,
} from "@/lib/portal";
import {
  cutoffPassed,
  requestCancelBooking,
  requestRescheduleBooking,
  splitBookings,
  type Booking,
  type Slot,
} from "@/lib/bookings";
import {
  formatSessionDate,
  formatTimeOfDay,
  gmtOffsetLabel,
} from "@/lib/format";

/**
 * Sessions (CX-04) — every booking the client has made.
 * Upcoming (changeable): Reschedule and Cancel run as practitioner-reviewed
 * requests from custom Modals (§3.14); rescheduling reuses the SlotPicker,
 * cancellation requires a reason, and both actions honor the 48-hour cutoff.
 * Past sessions are listed with their terminal-status badges.
 */

/** Brand-voice copy for the 422 cutoff_passed race (say what happened, what
 * to do next, no blame). */
const CUTOFF_MESSAGE =
  "This session is less than 48 hours away, so online changes have closed. Contact the practice and we'll help.";

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
  const { session, user, onTokensRefreshed } = useAuth();
  const timeZone = user?.timezone ?? "UTC";
  const { now, bookings, servicesById, error, refresh } = useMyBookings();

  const { upcoming, pending, inProgress, past } = useMemo(
    () => splitBookings(bookings ?? [], now),
    [bookings, now],
  );

  // Reschedule modal state.
  const [rescheduling, setRescheduling] = useState<Booking | null>(null);
  const [newSlot, setNewSlot] = useState<Slot | null>(null);
  const [rescheduleError, setRescheduleError] = useState<string | null>(null);
  const [rescheduleConflict, setRescheduleConflict] = useState<string | null>(
    null,
  );
  const [reschedulingBusy, setReschedulingBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  // Cancel modal state.
  const [cancelling, setCancelling] = useState<Booking | null>(null);
  const [cancelReason, setCancelReason] = useState("");
  const [cancelError, setCancelError] = useState<string | null>(null);
  const [cancelBusy, setCancelBusy] = useState(false);

  function openReschedule(booking: Booking) {
    setRescheduling(booking);
    setNewSlot(null);
    setRescheduleError(null);
    setRescheduleConflict(null);
    setNotice(null);
  }

  async function handleRescheduleConfirm() {
    if (!session || !rescheduling || !newSlot) return;
    setRescheduleError(null);
    setReschedulingBusy(true);
    try {
      await requestRescheduleBooking(
        session,
        { onTokensRefreshed },
        rescheduling.id,
        {
          startAt: newSlot.startAt,
          tz: timeZone,
        },
      );
      setRescheduling(null);
      setNotice(
        "Your reschedule request has been sent. Your session stays at its current time until the practitioner confirms a change.",
      );
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
    if (!cancelReason.trim()) {
      setCancelError("Add a reason before sending a cancellation request.");
      return;
    }
    setCancelError(null);
    setCancelBusy(true);
    try {
      await requestCancelBooking(
        session,
        { onTokensRefreshed },
        cancelling.id,
        {
          reason: cancelReason.trim(),
          tz: timeZone,
        },
      );
      setCancelling(null);
      setCancelReason("");
      setNotice(
        "Your cancellation request has been sent. Your session remains booked until the practitioner reviews it.",
      );
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
            Video care
          </p>
          <h1 className="mt-3 font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
            Your consultations
          </h1>
        </div>
        <Link href="/portal/book" className={buttonClasses({ size: "sm" })}>
          Book a session
        </Link>
      </div>

      {notice ? (
        <div
          role="status"
          className="rounded-xl border border-eucalyptus-200 bg-eucalyptus-50 px-4 py-3 text-sm leading-[1.55] text-eucalyptus-950"
        >
          {notice}
        </div>
      ) : null}

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
          {pending.length > 0 && (
            <section className="space-y-4" aria-label="Awaiting payment">
              <h2 className="font-display text-2xl">Awaiting payment</h2>
              <p className="text-sm text-ink-muted">
                These requests are not confirmed. Complete checkout from your
                payments page.
              </p>
              {pending.map((booking) => (
                <SessionRow
                  key={booking.id}
                  booking={booking}
                  serviceName={servicesById.get(booking.serviceId)?.name}
                  timeZone={timeZone}
                  actions={
                    <Link
                      href={`/portal/payments#booking-${booking.id}`}
                      className={buttonClasses({ size: "sm" })}
                    >
                      Review payment
                    </Link>
                  }
                />
              ))}
            </section>
          )}
          {inProgress.length > 0 && (
            <section className="space-y-4" aria-label="In progress">
              <h2 className="font-display text-2xl">In progress</h2>
              {inProgress.map((booking) => (
                <SessionRow
                  key={booking.id}
                  booking={booking}
                  serviceName={servicesById.get(booking.serviceId)?.name}
                  timeZone={timeZone}
                  actions={
                    <>
                      <Link
                        href={`/portal/sessions/${booking.id}/documents`}
                        className={buttonClasses({
                          size: "sm",
                          variant: "secondary",
                        })}
                      >
                        Required documents
                      </Link>
                      <Link
                        href={`/portal/sessions/${booking.id}/room`}
                        className={buttonClasses({ size: "sm" })}
                      >
                        Join video room
                      </Link>
                    </>
                  }
                />
              ))}
            </section>
          )}
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
                  const locked = cutoffPassed(booking.startAt, now);
                  return (
                    <li key={booking.id}>
                      <SessionRow
                        booking={booking}
                        serviceName={servicesById.get(booking.serviceId)?.name}
                        timeZone={timeZone}
                        actions={
                          <>
                            <Link
                              href={`/portal/sessions/${booking.id}/documents`}
                              className={buttonClasses({
                                size: "sm",
                                variant: "secondary",
                              })}
                            >
                              Required documents
                            </Link>
                            {/* The room enforces its own opening hours;
                                this link is always offered so a client is
                                told *why* rather than finding no way in. */}
                            <Link
                              href={`/portal/sessions/${booking.id}/room`}
                              className={buttonClasses({ size: "sm" })}
                            >
                              Join video room
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
                                setCancelReason("");
                                setNotice(null);
                                setCancelling(booking);
                              }}
                              className="text-danger hover:bg-danger-bg hover:text-danger"
                            >
                              Cancel
                            </Button>
                          </>
                        }
                      />
                      <RecordingList bookingId={booking.id} />
                      <p className="mt-2 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
                        {locked
                          ? "Change requests close 48 hours before a session."
                          : "Request rescheduling or cancellation up to 48 hours before."}
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
                    <RecordingList bookingId={booking.id} />
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
            ? `Currently ${formatSessionDate(rescheduling.startAt, timeZone)} at ${formatTimeOfDay(rescheduling.startAt, timeZone)} (${gmtOffsetLabel(timeZone, new Date(rescheduling.startAt))}). Choose a proposed time; the practitioner will review it before anything changes.`
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
              Send request
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
              Send request
            </Button>
          </>
        }
      >
        {cancelling ? (
          <div className="flex flex-col gap-4">
            <p className="text-sm leading-[1.55] text-ink-muted">
              Your {formatSessionDate(cancelling.startAt, timeZone)} session at{" "}
              {formatTimeOfDay(cancelling.startAt, timeZone)} (
              {gmtOffsetLabel(timeZone, new Date(cancelling.startAt))}) will
              stay booked while the practitioner reviews your cancellation
              request.
            </p>
            <label className="flex flex-col gap-1.5">
              <span className="text-sm font-medium tracking-[0.005em] text-ink">
                Reason{" "}
                <span aria-hidden="true" className="text-accent">
                  *
                </span>
                <span className="sr-only"> (required)</span>
              </span>
              <textarea
                required
                value={cancelReason}
                onChange={(event) => {
                  setCancelReason(event.target.value);
                  setCancelError(null);
                }}
                rows={4}
                placeholder="Tell the practitioner why you need to cancel."
                className="min-h-28 w-full resize-y rounded-xl border border-border-strong bg-surface-raised px-3.5 py-3 text-sm leading-[1.55] text-ink caret-primary transition-[border-color,box-shadow] placeholder:text-ink-faint hover:border-ink-faint focus:border-primary focus:outline-none"
              />
            </label>
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

function RecordingList({ bookingId }: { bookingId: string }) {
  const { session, onTokensRefreshed } = useAuth();
  const [recordings, setRecordings] = useState<SessionRecording[] | null>(null);

  useEffect(() => {
    if (!session) return;
    let cancelled = false;
    recordingsApi
      .list(session, { onTokensRefreshed }, bookingId)
      .then((items) => {
        if (!cancelled) setRecordings(items);
      })
      .catch(() => {
        if (!cancelled) setRecordings([]);
      });
    return () => {
      cancelled = true;
    };
  }, [bookingId, onTokensRefreshed, session]);

  if (!recordings || recordings.length === 0) return null;

  return (
    <section className="mt-3 rounded-[1.25rem] border border-border bg-surface-raised p-4">
      <h3 className="text-sm font-semibold text-ink">Session recording</h3>
      <p className="mt-1 text-xs leading-relaxed text-ink-muted">
        Available only in your portal and your practitioner&rsquo;s dashboard.
      </p>
      <ul className="mt-3 flex flex-col gap-3">
        {recordings.map((recording) => (
          <li key={recording.id}>
            <RecordingPlayer
              url={recording.url}
              contentType={recording.contentType}
              fileName={`terios-session-${recording.bookingId}.${
                recording.contentType.includes("mp4") ? "mp4" : "webm"
              }`}
              className="aspect-video w-full rounded-lg bg-ink"
            />
            <p className="mt-2 text-xs text-ink-muted">
              {formatBytes(recording.bytes)} · retained until{" "}
              {new Date(recording.retainUntil).toLocaleDateString("en-GB")}
            </p>
          </li>
        ))}
      </ul>
    </section>
  );
}
