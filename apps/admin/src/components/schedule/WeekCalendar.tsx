"use client";

import { ChevronLeft, ChevronRight, Globe } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/Button";
import { IconButton } from "@/components/ui/IconButton";
import { cn } from "@/lib/cn";
import {
  addDaysCivil,
  dateKey,
  formatTime,
  formatWeekRange,
  minutesToTimeString,
  todayCivil,
  wallClockToUtcIso,
  weekdayLongName,
  weekdayShortName,
  zonedParts,
  PRACTICE_TIMEZONE,
  timezoneShortName,
  type Booking,
  type CivilDate,
} from "@/lib/schedule";
import { BookingDetailModal, type BookingActionHandler, type RescheduleHandler } from "./BookingDetailModal";

/**
 * WeekCalendar — design-system §3.12 week view (admin).
 * Seven day columns over hour lanes (06:00–20:00 default, 48px rows); bookings
 * are absolute blocks positioned from their UTC startAt/endAt converted into
 * the practice wall clock, status-colored per the spec (confirmed primary,
 * completed muted, cancelled struck danger, no-show warning). Header chrome:
 * heading-lg week label, prev/today/next nav, and an always-visible timezone
 * caption (brand voice rule). All times use tabular figures.
 *
 * Keyboard: day columns form a roving-tabindex row — ←/→ (Home/End for the
 * edges) move between columns, Enter drops focus onto the column's first
 * booking; bookings themselves are buttons that open the detail modal.
 */

/** Default lane range, minutes since local midnight (06:00–20:00 per spec). */
export const DAY_START_MIN = 6 * 60;
export const DAY_END_MIN = 20 * 60;
/** Hour row height (design-system §3.12: 48px). */
export const HOUR_PX = 48;
/** 15-minute minimum visual height is 24px per spec. */
const MIN_BLOCK_PX = 24;

const GRID_HEIGHT_PX = ((DAY_END_MIN - DAY_START_MIN) / 60) * HOUR_PX;
const HOURS: number[] = [];
for (let min = DAY_START_MIN; min < DAY_END_MIN; min += 60) {
  HOURS.push(min);
}

const STATUS_LABEL: Record<Booking["status"], string> = {
  confirmed: "Confirmed",
  completed: "Completed",
  cancelled: "Cancelled",
  no_show: "No-show",
};

const STATUS_BLOCK_CLASSES: Record<Booking["status"], string> = {
  confirmed: "border-l-primary bg-eucalyptus-100 text-eucalyptus-800",
  completed: "border-l-ink-faint bg-surface-sunken text-ink-muted",
  cancelled: "border-l-danger bg-danger-bg text-danger-ink line-through",
  no_show: "border-l-warning bg-warning-bg text-warning-ink",
};

interface PositionedBooking {
  booking: Booking;
  top: number;
  height: number;
}

function positionBooking(booking: Booking, timeZone: string): PositionedBooking | null {
  const start = zonedParts(booking.startAt, timeZone);
  const end = zonedParts(booking.endAt, timeZone);
  const startMin = Math.max(start.minutesSinceMidnight, DAY_START_MIN);
  const endMin = Math.min(
    dateKey(end) === dateKey(start) ? end.minutesSinceMidnight : DAY_END_MIN,
    DAY_END_MIN,
  );
  if (endMin <= startMin) return null; // entirely outside the lane range
  const top = ((startMin - DAY_START_MIN) / 60) * HOUR_PX;
  const height = Math.max(((endMin - startMin) / 60) * HOUR_PX, MIN_BLOCK_PX);
  return { booking, top, height };
}

export interface WeekCalendarProps {
  /** Monday of the visible week, as a civil date in `timeZone`. */
  weekStart: CivilDate;
  bookings: Booking[];
  timeZone?: string;
  onPrevWeek: () => void;
  onNextWeek: () => void;
  onToday: () => void;
  /** Parent performs the API call and resolves with the updated booking. */
  onAction: BookingActionHandler;
  onReschedule: RescheduleHandler;
}

export function WeekCalendar({
  weekStart,
  bookings,
  timeZone = PRACTICE_TIMEZONE,
  onPrevWeek,
  onNextWeek,
  onToday,
  onAction,
  onReschedule,
}: WeekCalendarProps) {
  const [selected, setSelected] = useState<Booking | null>(null);
  const [focusColumn, setFocusColumn] = useState(0);
  const [now, setNow] = useState(() => new Date());
  const columnRefs = useRef<(HTMLDivElement | null)[]>([]);

  // The now-line re-evaluates once a minute.
  useEffect(() => {
    const timer = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(timer);
  }, []);

  const days = Array.from({ length: 7 }, (_, i) => addDaysCivil(weekStart, i));
  const today = todayCivil(timeZone, now);
  const todayKey = dateKey(today);
  const zoneLabel = timezoneShortName(timeZone, now);

  const byDay = new Map<string, PositionedBooking[]>();
  for (const booking of bookings) {
    const key = dateKey(zonedParts(booking.startAt, timeZone));
    const positioned = positionBooking(booking, timeZone);
    if (!positioned) continue;
    const list = byDay.get(key) ?? [];
    list.push(positioned);
    byDay.set(key, list);
  }

  const nowParts = zonedParts(now, timeZone);
  const nowInLanes =
    nowParts.minutesSinceMidnight >= DAY_START_MIN &&
    nowParts.minutesSinceMidnight <= DAY_END_MIN;
  const nowTop = ((nowParts.minutesSinceMidnight - DAY_START_MIN) / 60) * HOUR_PX;

  function focusColumnAt(index: number) {
    const clamped = Math.min(Math.max(index, 0), 6);
    setFocusColumn(clamped);
    columnRefs.current[clamped]?.focus();
  }

  function handleColumnKeyDown(event: React.KeyboardEvent, index: number) {
    if (event.key === "ArrowLeft") {
      event.preventDefault();
      focusColumnAt(index - 1);
    } else if (event.key === "ArrowRight") {
      event.preventDefault();
      focusColumnAt(index + 1);
    } else if (event.key === "Home") {
      event.preventDefault();
      focusColumnAt(0);
    } else if (event.key === "End") {
      event.preventDefault();
      focusColumnAt(6);
    } else if (event.key === "Enter") {
      // Enter on a cell moves focus to its first event (design-system §3.12).
      const first = columnRefs.current[index]?.querySelector("button");
      if (first) {
        event.preventDefault();
        first.focus();
      }
    }
  }

  return (
    <section aria-label="Practice calendar" className="flex flex-col gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-[22px] leading-[1.3] font-semibold tracking-[-0.005em] text-ink">
            {formatWeekRange(weekStart)}
          </h2>
          {/* timezone is always shown on scheduling surfaces (brand voice rule) */}
          <p className="mt-1 flex items-center gap-1.5 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
            <Globe size={14} aria-hidden="true" />
            Times in {zoneLabel}
          </p>
        </div>
        <div className="flex items-center gap-1">
          <IconButton aria-label="Previous week" onClick={onPrevWeek}>
            <ChevronLeft aria-hidden="true" />
          </IconButton>
          <Button variant="ghost" size="sm" onClick={onToday}>
            Today
          </Button>
          <IconButton aria-label="Next week" onClick={onNextWeek}>
            <ChevronRight aria-hidden="true" />
          </IconButton>
        </div>
      </div>

      <div className="overflow-x-auto rounded-lg border border-border bg-surface-raised">
        <div className="min-w-[720px]">
          {/* weekday header row: micro ink-faint, hairline below (§3.12) */}
          <div className="grid grid-cols-[56px_repeat(7,1fr)] border-b border-border">
            <div aria-hidden="true" />
            {days.map((day) => {
              const key = dateKey(day);
              const weekday = new Date(
                Date.UTC(day.year, day.month - 1, day.day),
              ).getUTCDay();
              const isToday = key === todayKey;
              return (
                <div
                  key={key}
                  className="border-l border-border px-2 py-2 text-center"
                >
                  <p className="text-[11px] leading-[1.3] font-semibold tracking-[0.08em] uppercase text-ink-faint">
                    {weekdayShortName(weekday)}
                  </p>
                  <p
                    className={cn(
                      "mt-0.5 text-[13px] leading-[1.45] font-medium tabular-nums text-ink-muted",
                      isToday && "font-semibold text-primary",
                    )}
                  >
                    {day.day}
                    {isToday ? (
                      <span
                        aria-hidden="true"
                        className="ml-1 inline-block size-1.5 rounded-full bg-primary align-middle"
                      />
                    ) : null}
                    {isToday ? <span className="sr-only"> (today)</span> : null}
                  </p>
                </div>
              );
            })}
          </div>

          <div className="flex">
            {/* hour gutter: tabular figures, label sits on the hairline */}
            <div aria-hidden="true" className="w-14 shrink-0">
              {HOURS.map((min) => (
                <div key={min} className="h-12 pr-2 text-right">
                  <span className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] tabular-nums text-ink-faint">
                    {formatTime(
                      wallClockAnchor(weekStart, min, timeZone),
                      timeZone,
                    )}
                  </span>
                </div>
              ))}
            </div>

            <div
              role="grid"
              aria-label={`Week of ${formatWeekRange(weekStart)} (${zoneLabel})`}
              aria-readonly="true"
              className="grid flex-1 grid-cols-7"
            >
              {days.map((day, index) => {
                const key = dateKey(day);
                const weekday = new Date(
                  Date.UTC(day.year, day.month - 1, day.day),
                ).getUTCDay();
                const dayBookings = byDay.get(key) ?? [];
                const isToday = key === todayKey;
                return (
                  <div
                    key={key}
                    ref={(el) => {
                      columnRefs.current[index] = el;
                    }}
                    role="gridcell"
                    tabIndex={focusColumn === index ? 0 : -1}
                    aria-label={`${weekdayLongName(weekday)}, ${key}, ${dayBookings.length} booking${dayBookings.length === 1 ? "" : "s"}`}
                    onKeyDown={(event) => handleColumnKeyDown(event, index)}
                    onFocus={() => setFocusColumn(index)}
                    className="relative border-l border-border"
                    style={{ height: GRID_HEIGHT_PX }}
                  >
                    {/* hour hairlines */}
                    {HOURS.map((min) => (
                      <div
                        key={min}
                        aria-hidden="true"
                        className="h-12 border-t border-border"
                      />
                    ))}

                    {dayBookings.map(({ booking, top, height }) => (
                      <button
                        key={booking.id}
                        type="button"
                        onClick={() => setSelected(booking)}
                        aria-label={`${formatTime(booking.startAt, timeZone)} to ${formatTime(booking.endAt, timeZone)}, client ${booking.clientId}, ${STATUS_LABEL[booking.status]}`}
                        className={cn(
                          "absolute right-1 left-1 overflow-hidden rounded-sm border-l-[3px] px-1.5 py-0.5 text-left text-[11px] leading-[1.3] tabular-nums transition-opacity duration-fast ease-out hover:opacity-80",
                          STATUS_BLOCK_CLASSES[booking.status],
                        )}
                        style={{ top, height }}
                      >
                        <span className="block truncate font-semibold">
                          {formatTime(booking.startAt, timeZone)}–
                          {formatTime(booking.endAt, timeZone)}
                        </span>
                        {height >= 40 ? (
                          <span className="block truncate">
                            {STATUS_LABEL[booking.status]}
                          </span>
                        ) : null}
                      </button>
                    ))}

                    {/* now-line: 1px accent + 8px dot at the left edge (§3.12) */}
                    {isToday && nowInLanes ? (
                      <div
                        aria-hidden="true"
                        className="pointer-events-none absolute right-0 left-0 z-[1] border-t border-accent"
                        style={{ top: nowTop }}
                      >
                        <span className="absolute -top-1 left-0 size-2 rounded-full bg-accent" />
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>

      {selected ? (
        <BookingDetailModal
          booking={selected}
          timeZone={timeZone}
          onClose={() => setSelected(null)}
          onAction={onAction}
          onReschedule={onReschedule}
        />
      ) : null}
    </section>
  );
}

/** Anchor instant for a lane label: `minutes` wall clock on the week's Monday. */
function wallClockAnchor(day: CivilDate, minutes: number, timeZone: string): string {
  const anchor = wallClockToUtcIso(dateKey(day), minutesToTimeString(minutes), timeZone);
  // The lane hours are always valid wall-clock times; fall back to UTC math.
  if (anchor) return anchor;
  return new Date(
    Date.UTC(day.year, day.month - 1, day.day, Math.floor(minutes / 60), minutes % 60),
  ).toISOString();
}
