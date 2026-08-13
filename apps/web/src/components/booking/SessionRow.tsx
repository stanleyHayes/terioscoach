import type { ReactNode } from "react";
import { bookingStatusMeta } from "@/components/booking/booking-status";
import { Badge } from "@/components/ui/Badge";
import type { Booking } from "@/lib/bookings";
import { formatSessionDate, formatTimeRange, gmtOffsetLabel } from "@/lib/format";

/**
 * One booking row for the portal lists: service name, date/time range with
 * the timezone stated (scheduling-surface rule), status Badge, and an
 * optional actions slot (Reschedule/Cancel on the sessions page).
 */
export interface SessionRowProps {
  booking: Booking;
  /** Resolved service name; falls back gracefully if the catalog is gone. */
  serviceName?: string;
  timeZone: string;
  actions?: ReactNode;
}

export function SessionRow({ booking, serviceName, timeZone, actions }: SessionRowProps) {
  const meta = bookingStatusMeta[booking.status];
  return (
    <div className="terios-record-card relative flex flex-col gap-4 overflow-hidden rounded-[1.5rem] border border-border/70 bg-surface-raised p-5 pl-7 shadow-[0_14px_50px_rgba(31,41,34,.04)] sm:flex-row sm:items-center sm:justify-between">
      <span aria-hidden="true" className="absolute inset-y-4 left-0 w-1 rounded-r-full bg-primary" />
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-3">
          <h3 className="text-base font-semibold leading-[1.4] text-ink">
            {serviceName ?? "Session"}
          </h3>
          <Badge tone={meta.tone}>{meta.label}</Badge>
        </div>
        <p className="mt-1 text-sm tabular-nums text-ink-muted">
          {formatSessionDate(booking.startAt, timeZone)}
          <span aria-hidden="true" className="mx-2 text-ink-faint">
            ·
          </span>
          {formatTimeRange(booking.startAt, booking.endAt, timeZone)}
        </p>
        <p className="mt-0.5 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
          {gmtOffsetLabel(timeZone, new Date(booking.startAt))} ({timeZone})
        </p>
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  );
}
