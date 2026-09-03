"use client";

import Link from "next/link";


import { CircleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button, buttonClasses } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";
import { TextInput } from "@/components/ui/TextInput";
import { ApiError } from "@/lib/api";
import {
  dateKey,
  formatCivilDate,
  formatTime,
  minutesToTimeString,
  parseDateInput,
  parseTimeInput,
  timezoneShortName,
  wallClockToUtcIso,
  zonedParts,
  type Booking,
} from "@/lib/schedule";

/**
 * Booking detail modal (design-system §3.14 + §3.12 event keyboard target).
 * Shows the booking's client name, service name, practice-zone time range (with
 * the timezone always spelled out, per the brand voice rule), and status.
 * Confirmed bookings get the four transitions: Complete / No-show / Cancel /
 * Reschedule — reschedule is a small date+time form (validated HH:MM text
 * inputs, never native pickers) that POSTs the new startAt; validation and
 * API errors (e.g. 409 slot_unavailable) render inline.
 */

export type BookingAction = "complete" | "cancel" | "no_show";

export type BookingActionHandler = (
  booking: Booking,
  action: BookingAction,
) => Promise<Booking>;

export type RescheduleHandler = (
  booking: Booking,
  startAt: string,
) => Promise<Booking>;

const STATUS_LABEL: Record<Booking["status"], string> = {
  confirmed: "Confirmed",
  completed: "Completed",
  cancelled: "Cancelled",
  no_show: "No-show",
};

const STATUS_BADGE: Record<Booking["status"], BadgeVariant> = {
  confirmed: "success",
  completed: "neutral",
  cancelled: "danger",
  no_show: "warning",
};

interface RescheduleErrors {
  date?: string;
  time?: string;
}

export function BookingDetailModal({
  booking,
  clientName,
  serviceName,
  timeZone,
  onClose,
  onAction,
  onReschedule,
}: {
  booking: Booking;
  /** Resolved display name for booking.clientId; falls back to the id if unresolved. */
  clientName?: string;
  /** Resolved display name for booking.serviceId; falls back to the id if unresolved. */
  serviceName?: string;
  timeZone: string;
  onClose: () => void;
  onAction: BookingActionHandler;
  onReschedule: RescheduleHandler;
}) {
  // Local copy so a successful transition updates the modal in place.
  const [current, setCurrent] = useState(booking);
  const [rescheduling, setRescheduling] = useState(false);
  const [pending, setPending] = useState<BookingAction | "reschedule" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<RescheduleErrors>({});

  const start = zonedParts(current.startAt, timeZone);
  const [date, setDate] = useState(dateKey(start));
  const [time, setTime] = useState(minutesToTimeString(start.minutesSinceMidnight));

  const zoneLabel = timezoneShortName(timeZone);
  const isConfirmed = current.status === "confirmed";

  function errorMessage(error: unknown): string {
    return error instanceof ApiError ? error.message : "Couldn't save that. Try again.";
  }

  async function runAction(action: BookingAction) {
    setActionError(null);
    setPending(action);
    try {
      setCurrent(await onAction(current, action));
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setPending(null);
    }
  }

  async function submitReschedule(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setActionError(null);

    const errors: RescheduleErrors = {};
    if (!parseDateInput(date)) {
      errors.date = "Enter a date as YYYY-MM-DD, e.g. 2026-08-12";
    }
    if (parseTimeInput(time) === null) {
      errors.time = "Enter a time as HH:MM, e.g. 09:30";
    }
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;

    const startAt = wallClockToUtcIso(date, time, timeZone);
    if (!startAt) {
      setActionError("That date and time don't work. Check both and try again.");
      return;
    }

    setPending("reschedule");
    try {
      setCurrent(await onReschedule(current, startAt));
      setRescheduling(false);
    } catch (error) {
      setActionError(errorMessage(error));
    } finally {
      setPending(null);
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Booking"
      description={`${formatCivilDate(start)} · ${formatTime(current.startAt, timeZone)}–${formatTime(current.endAt, timeZone)} (${zoneLabel})`}
      size="form"
      footer={
        isConfirmed && !rescheduling ? (
          <>
            {/* Starting the session from the calendar is the one-click
                path the practitioner actually uses; the room enforces its
                own opening hours, so it is always offered. */}
            <Link
              href={`/sessions/${current.id}/room?client=${current.clientId}`}
              className={buttonClasses({ size: "sm" })}
            >
              Start session
            </Link>
            <Button
              variant="danger"
              size="sm"
              loading={pending === "cancel"}
              disabled={pending !== null}
              onClick={() => void runAction("cancel")}
            >
              Cancel booking
            </Button>
            <Button
              variant="secondary"
              size="sm"
              loading={pending === "no_show"}
              disabled={pending !== null}
              onClick={() => void runAction("no_show")}
            >
              No-show
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={pending !== null}
              onClick={() => {
                setActionError(null);
                setRescheduling(true);
              }}
            >
              Reschedule
            </Button>
            <Button
              size="sm"
              loading={pending === "complete"}
              disabled={pending !== null}
              onClick={() => void runAction("complete")}
            >
              Complete
            </Button>
          </>
        ) : undefined
      }
    >
      <div className="flex flex-col gap-4">
        {actionError ? (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0" />
            {actionError}
          </div>
        ) : null}

        <dl className="flex flex-col gap-3 text-sm leading-[1.55]">
          <div className="flex items-center justify-between gap-4">
            <dt className="text-ink-muted">Status</dt>
            <dd>
              <Badge variant={STATUS_BADGE[current.status]} dot>
                {STATUS_LABEL[current.status]}
              </Badge>
            </dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-ink-muted">Client</dt>
            <dd className="text-ink">{clientName ?? current.clientId}</dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-ink-muted">Service</dt>
            <dd className="text-ink">{serviceName ?? current.serviceId}</dd>
          </div>
          <div className="flex items-center justify-between gap-4">
            <dt className="text-ink-muted">Time</dt>
            <dd className="tabular-nums text-ink">
              {formatTime(current.startAt, timeZone)}–{formatTime(current.endAt, timeZone)}{" "}
              ({zoneLabel})
            </dd>
          </div>
          {current.cancelledAt ? (
            <div className="flex items-center justify-between gap-4">
              <dt className="text-ink-muted">Cancelled</dt>
              <dd className="tabular-nums text-ink">
                {formatCivilDate(zonedParts(current.cancelledAt, timeZone))} ·{" "}
                {formatTime(current.cancelledAt, timeZone)} ({zoneLabel})
              </dd>
            </div>
          ) : null}
          {current.completedAt ? (
            <div className="flex items-center justify-between gap-4">
              <dt className="text-ink-muted">
                {current.status === "no_show" ? "Marked no-show" : "Completed"}
              </dt>
              <dd className="tabular-nums text-ink">
                {formatCivilDate(zonedParts(current.completedAt, timeZone))} ·{" "}
                {formatTime(current.completedAt, timeZone)} ({zoneLabel})
              </dd>
            </div>
          ) : null}
        </dl>

        {!isConfirmed ? (
          <p className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
            This booking is {STATUS_LABEL[current.status].toLowerCase()} — no further
            actions available.
          </p>
        ) : null}

        {rescheduling && isConfirmed ? (
          // noValidate: native validation bubbles are forbidden — errors are custom
          <form
            noValidate
            onSubmit={submitReschedule}
            className="flex flex-col gap-4 rounded-md border border-border bg-surface-sunken p-4"
          >
            <p className="text-sm font-medium tracking-[0.005em] text-ink">
              Move to a new slot
            </p>
            <div className="grid grid-cols-2 gap-4">
              <TextInput
                label="Date"
                required
                inputMode="numeric"
                placeholder="2026-08-12"
                hint="YYYY-MM-DD"
                value={date}
                error={fieldErrors.date}
                data-autofocus
                onChange={(event) => {
                  setDate(event.target.value);
                  setFieldErrors((errors) => ({ ...errors, date: undefined }));
                }}
              />
              <TextInput
                label="Time"
                required
                inputMode="numeric"
                placeholder="09:30"
                hint={`HH:MM, ${zoneLabel}`}
                value={time}
                error={fieldErrors.time}
                onChange={(event) => {
                  setTime(event.target.value);
                  setFieldErrors((errors) => ({ ...errors, time: undefined }));
                }}
              />
            </div>
            <p className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
              The new start must match an open slot, or the booking server rejects it.
            </p>
            <div className="flex justify-end gap-3">
              <Button
                variant="ghost"
                size="sm"
                disabled={pending !== null}
                onClick={() => {
                  setActionError(null);
                  setFieldErrors({});
                  setRescheduling(false);
                }}
              >
                Back
              </Button>
              <Button type="submit" size="sm" loading={pending === "reschedule"}>
                Save new time
              </Button>
            </div>
          </form>
        ) : null}
      </div>
    </Modal>
  );
}
