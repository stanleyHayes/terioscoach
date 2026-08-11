"use client";

import { Globe } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { getSlots, type Slot } from "@/lib/bookings";
import { browserTimeZone, formatTimeOfDay, gmtOffsetLabel } from "@/lib/format";
import { cn } from "@/lib/cn";

/**
 * SlotPicker — design-system §3.11 (TimeSlotPicker) + the booking day strip.
 *
 * A horizontal strip of the next 14 days (custom day cells — never a native
 * date input) drives a well of time chips for the selected day, fetched live
 * from GET /v1/availability/slots and shown in the visitor's own timezone
 * (stated explicitly on the pinned timezone chip). Chips are grouped AM/PM
 * with micro headers, render "3:30 PM" with tabular nums, and follow the
 * spec's selected/hover/unavailable states. Loading shows 6 skeleton chips;
 * an empty day says so in words.
 *
 * Race condition: when the parent's create/reschedule answers 409
 * slot_unavailable it passes the losing startAt via `conflictStartAt` — the
 * picker shows the inline "just taken" message, marks that chip unavailable
 * (with the spec's one-shot shake) until the refreshed slot list replaces it.
 */

/* Spec §3.11 held-by-another-client shake: 2px horizontal, 150ms, once. Kept
 * here rather than globals.css (the token arbiter, copied verbatim to admin). */
const shakeKeyframes = `
@keyframes terios-slot-shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-2px); }
  75% { transform: translateX(2px); }
}
`;

const DAY_COUNT = 14;

interface DayCell {
  /** YYYY-MM-DD in the display timezone — the API's from/to value. */
  key: string;
  weekday: string;
  dayNumber: string;
  isToday: boolean;
}

function buildDays(timeZone: string): DayCell[] {
  const now = new Date();
  const keyFormat = new Intl.DateTimeFormat("en-CA", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    timeZone,
  });
  const weekdayFormat = new Intl.DateTimeFormat("en-US", { weekday: "short", timeZone });
  const dayFormat = new Intl.DateTimeFormat("en-US", { day: "numeric", timeZone });
  const todayKey = keyFormat.format(now);

  return Array.from({ length: DAY_COUNT }, (_, index) => {
    const date = new Date(now.getTime() + index * 24 * 60 * 60 * 1000);
    const key = keyFormat.format(date);
    return {
      key,
      weekday: weekdayFormat.format(date),
      dayNumber: dayFormat.format(date),
      isToday: key === todayKey,
    };
  });
}

/** "9:30 AM" → "AM" — the chip's day period in the display timezone. */
function dayPeriod(isoUtc: string, timeZone: string): "AM" | "PM" {
  const part = new Intl.DateTimeFormat("en-US", {
    hour: "numeric",
    minute: "2-digit",
    timeZone,
  })
    .formatToParts(new Date(isoUtc))
    .find((p) => p.type === "dayPeriod");
  return part?.value?.toUpperCase() === "PM" ? "PM" : "AM";
}

type LoadState =
  | { status: "loading" }
  | { status: "ready"; slots: Slot[] }
  | { status: "error" };

export interface SlotPickerProps {
  serviceId: string;
  selectedSlot: Slot | null;
  onSelect: (slot: Slot) => void;
  /** startAt of a slot the server reported as just-taken (409
   * slot_unavailable). Triggers the race state + a slot refresh. */
  conflictStartAt?: string | null;
  /** Display timezone override; defaults to the visitor's browser tz. */
  timeZone?: string;
  /** Accessible label for the picker region. */
  "aria-label"?: string;
}

export function SlotPicker({
  serviceId,
  selectedSlot,
  onSelect,
  conflictStartAt = null,
  timeZone,
  "aria-label": ariaLabel = "Available times",
}: SlotPickerProps) {
  const tz = useMemo(() => timeZone ?? browserTimeZone(), [timeZone]);
  const days = useMemo(() => buildDays(tz), [tz]);
  const [selectedDay, setSelectedDay] = useState(days[0].key);
  const [loadState, setLoadState] = useState<LoadState>({ status: "loading" });
  const [refreshIndex, setRefreshIndex] = useState(0);
  const [raceStartAt, setRaceStartAt] = useState<string | null>(null);

  /* Load the selected day's slots (public endpoint, visitor tz). */
  useEffect(() => {
    let cancelled = false;
    setLoadState({ status: "loading" });
    getSlots({ serviceId, from: selectedDay, to: selectedDay, tz })
      .then((result) => {
        if (!cancelled) setLoadState({ status: "ready", slots: result.slots });
      })
      .catch(() => {
        if (!cancelled) setLoadState({ status: "error" });
      });
    return () => {
      cancelled = true;
    };
  }, [serviceId, selectedDay, tz, refreshIndex]);

  /* 409 slot_unavailable from the parent: show the race state and pull a
   * fresh slot list — the taken chip drops out of it. */
  useEffect(() => {
    if (conflictStartAt) {
      setRaceStartAt(conflictStartAt);
      setRefreshIndex((index) => index + 1);
    }
  }, [conflictStartAt]);

  function handleSelect(slot: Slot) {
    setRaceStartAt(null);
    onSelect(slot);
  }

  const groups = useMemo(() => {
    if (loadState.status !== "ready") return [];
    const am: Slot[] = [];
    const pm: Slot[] = [];
    for (const slot of loadState.slots) {
      (dayPeriod(slot.startAt, tz) === "AM" ? am : pm).push(slot);
    }
    return [
      { label: "AM", slots: am },
      { label: "PM", slots: pm },
    ].filter((group) => group.slots.length > 0);
  }, [loadState, tz]);

  return (
    <div aria-label={ariaLabel} className="flex flex-col gap-4">
      <style>{shakeKeyframes}</style>

      {/* Day strip — next 14 days, custom cells. */}
      <div
        role="group"
        aria-label="Choose a day"
        className="flex gap-2 overflow-x-auto pb-1"
      >
        {days.map((day) => {
          const selected = day.key === selectedDay;
          return (
            <button
              key={day.key}
              type="button"
              aria-pressed={selected}
              onClick={() => setSelectedDay(day.key)}
              className={cn(
                "flex min-h-[40px] w-14 shrink-0 flex-col items-center justify-center gap-0.5 rounded-md border py-2",
                "transition-colors duration-fast ease-out",
                selected
                  ? "border-primary bg-primary text-on-primary"
                  : "border-border-strong bg-surface-raised text-ink hover:border-primary hover:bg-eucalyptus-50",
                day.isToday && !selected && "ring-1 ring-primary ring-inset",
              )}
            >
              <span
                className={cn(
                  "text-[11px] font-semibold uppercase tracking-[0.08em]",
                  selected ? "text-on-primary/80" : "text-ink-faint",
                )}
              >
                {day.weekday}
              </span>
              <span className="text-sm font-semibold tabular-nums">{day.dayNumber}</span>
            </button>
          );
        })}
      </div>

      {/* Timezone chip — scheduling surfaces always state their tz. */}
      <p className="flex items-center gap-1.5 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
        <Globe size={14} aria-hidden="true" />
        Times shown in {gmtOffsetLabel(tz)} ({tz})
      </p>

      {raceStartAt ? (
        <p
          role="alert"
          className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-danger-ink"
        >
          That time was just taken — pick another
        </p>
      ) : null}

      {/* The well. */}
      <div className="rounded-lg bg-surface-sunken p-3" aria-busy={loadState.status === "loading"}>
        {loadState.status === "loading" ? (
          <div role="status" className="grid grid-cols-2 gap-2 sm:grid-cols-3">
            <span className="sr-only">Loading available times…</span>
            {Array.from({ length: 6 }, (_, index) => (
              <span
                key={index}
                aria-hidden="true"
                className="h-9 rounded-md bg-surface-raised"
              />
            ))}
          </div>
        ) : loadState.status === "error" ? (
          <div className="px-3 py-6 text-center">
            <p className="text-sm leading-[1.55] text-ink-muted">
              The times didn&rsquo;t load. Check your connection and try again.
            </p>
            <button
              type="button"
              onClick={() => setRefreshIndex((index) => index + 1)}
              className="mt-3 text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
            >
              Try again
            </button>
          </div>
        ) : groups.length === 0 ? (
          <p className="px-3 py-6 text-center text-sm leading-[1.55] text-ink-muted">
            No times available on this day
          </p>
        ) : (
          <div className="flex flex-col gap-3">
            {groups.map((group) => (
              <div key={group.label}>
                <p className="px-1 pb-2 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-faint">
                  {group.label}
                </p>
                <div role="listbox" aria-label={`${group.label} times`} className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                  {group.slots.map((slot) => {
                    const selected = selectedSlot?.startAt === slot.startAt;
                    const taken = raceStartAt === slot.startAt;
                    return (
                      <button
                        key={slot.startAt}
                        type="button"
                        role="option"
                        aria-selected={selected}
                        aria-disabled={taken || undefined}
                        tabIndex={taken ? -1 : undefined}
                        disabled={taken}
                        onClick={() => handleSelect(slot)}
                        style={
                          taken
                            ? { animation: "terios-slot-shake 150ms ease-out 1" }
                            : undefined
                        }
                        className={cn(
                          "flex h-9 min-w-[88px] items-center justify-center rounded-md border text-sm tabular-nums",
                          "transition-colors duration-fast ease-out",
                          selected
                            ? "border-primary bg-primary text-on-primary"
                            : taken
                              ? "cursor-not-allowed border-border bg-surface-sunken text-ink-faint"
                              : "border-border-strong bg-surface-raised text-ink hover:border-primary hover:bg-eucalyptus-50",
                        )}
                      >
                        {formatTimeOfDay(slot.startAt, tz)}
                      </button>
                    );
                  })}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Legend (§3.11). */}
      <div className="flex items-center gap-4 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
        <span className="flex items-center gap-1.5">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-success" />
          Available
        </span>
        <span className="flex items-center gap-1.5">
          <span aria-hidden="true" className="size-1.5 rounded-full bg-border-strong" />
          Unavailable
        </span>
      </div>
    </div>
  );
}
