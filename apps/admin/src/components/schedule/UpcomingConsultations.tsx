"use client";

import { Calendar, ChevronRight } from "lucide-react";
import { useState } from "react";
import { Badge } from "@/components/ui/Badge";
import { formatCivilDate, formatTime, PRACTICE_TIMEZONE, timezoneShortName, zonedParts, type Booking } from "@/lib/schedule";
import { BookingDetailModal, type BookingActionHandler, type RescheduleHandler } from "./BookingDetailModal";

export function UpcomingConsultations({ bookings, clientNames, serviceNames, onAction, onReschedule }: {
  bookings: Booking[];
  clientNames: Record<string, string>;
  serviceNames: Record<string, string>;
  onAction: BookingActionHandler;
  onReschedule: RescheduleHandler;
}) {
  const [selected, setSelected] = useState<Booking | null>(null);
  const upcoming = bookings.filter((booking) => booking.status === "confirmed")
    .sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt));

  return (
    <section aria-labelledby="upcoming-consultations-heading" className="flex flex-col gap-4">
      <div>
        <h2 id="upcoming-consultations-heading" className="font-display text-2xl font-medium tracking-tight text-ink">Upcoming consultations</h2>
        <p className="mt-1 text-sm text-ink-muted">Confirmed sessions from now onward, with the nearest first. Open a session to manage it.</p>
      </div>
      {upcoming.length === 0 ? (
        <div className="rounded-[1.5rem] border border-border bg-surface-raised p-8 text-center">
          <Calendar size={28} aria-hidden="true" className="mx-auto mb-3 text-primary" />
          <h3 className="font-semibold text-ink">No upcoming consultations</h3>
          <p className="mt-2 text-sm text-ink-muted">New confirmed bookings will appear here. Use the weekly calendar to review past or cancelled sessions.</p>
        </div>
      ) : (
        <ul className="flex flex-col gap-4">
          {upcoming.map((booking) => (
            <li key={booking.id}>
              <button type="button" onClick={() => setSelected(booking)} className="relative flex w-full items-center gap-4 overflow-hidden rounded-[1.5rem] border border-border/70 bg-surface-raised p-5 pl-7 text-left shadow-sm transition-colors hover:border-primary focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary sm:p-6 sm:pl-8">
                <span aria-hidden="true" className="absolute inset-y-5 left-0 w-1 rounded-r-full bg-primary" />
                <span className="min-w-0 flex-1">
                  <span className="flex flex-wrap items-center gap-3">
                    <span className="break-words text-base font-semibold text-ink">{serviceNames[booking.serviceId] ?? "Consultation"}</span>
                    <Badge variant="success" dot>Confirmed</Badge>
                  </span>
                  <span className="mt-2 block break-words text-sm font-medium text-ink">{clientNames[booking.clientId] ?? booking.clientId}</span>
                  <span className="mt-1 block text-sm tabular-nums text-ink-muted">{formatCivilDate(zonedParts(booking.startAt, PRACTICE_TIMEZONE))} · {formatTime(booking.startAt, PRACTICE_TIMEZONE)} – {formatTime(booking.endAt, PRACTICE_TIMEZONE)}</span>
                  <span className="mt-1 block text-xs text-ink-faint">{timezoneShortName(PRACTICE_TIMEZONE, new Date(booking.startAt))} ({PRACTICE_TIMEZONE})</span>
                </span>
                <ChevronRight size={20} aria-hidden="true" className="shrink-0 text-primary" />
              </button>
            </li>
          ))}
        </ul>
      )}
      {selected ? <BookingDetailModal booking={selected} clientName={clientNames[selected.clientId]} serviceName={serviceNames[selected.serviceId]} timeZone={PRACTICE_TIMEZONE} onClose={() => setSelected(null)} onAction={onAction} onReschedule={onReschedule} /> : null}
    </section>
  );
}
