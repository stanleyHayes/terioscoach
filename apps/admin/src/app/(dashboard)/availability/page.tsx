"use client";

import { CalendarOff, CheckCircle2, CircleAlert, Plus, X } from "lucide-react";
import { EmptyState } from "@/components/content/states";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { IconButton } from "@/components/ui/IconButton";
import { Switch } from "@/components/ui/Switch";
import { TextInput } from "@/components/ui/TextInput";
import { DatePicker } from "@/components/ui/DatePicker";
import { TimePicker } from "@/components/ui/TimePicker";
import { ApiError, SessionExpiredError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import {
  addDaysCivil,
  formatCivilDate,
  minutesToTimeString,
  parseDateInput,
  parseTimeInput,
  scheduleApi,
  wallClockToUtcIso,
  weekdayLongName,
  zonedParts,
  PRACTICE_TIMEZONE,
  timezoneShortName,
  type AvailabilityRule,
  type TimeOff,
} from "@/lib/schedule";

/**
 * Availability editor (ADM-02, contract BE-04).
 *
 * Weekly rules: one row per weekday — an open/closed Switch, custom time
 * pickers (native time inputs are forbidden), and a
 * per-day bufferMinutes field. PUT /v1/availability/rules replaces the whole
 * week, so "Save changes" sends every open day and omits closed ones. Dirty
 * tracking disables the button while the form matches the last saved state.
 * Client-side validation mirrors the API's 400s before submit: windows must
 * be valid HH:MM, start before end (no overnight), sorted and non-overlapping;
 * buffer 0–120.
 *
 * Time off: POST /v1/availability/time-off with a validated YYYY-MM-DD range.
 * The end date is inclusive for the practitioner, so endAt is the day after
 * at 00:00 (the contract's range is [startAt, endAt)).
 *
 * API gaps (BE-04 v1): there is no GET to list existing time-off and no
 * DELETE to remove it — the list below therefore shows only entries added in
 * this session and is intentionally list-only (no delete action). Revisit
 * when the contract grows those endpoints.
 */

interface WindowDraft {
  start: string;
  end: string;
}

interface DayDraft {
  enabled: boolean;
  windows: WindowDraft[];
  buffer: string;
}

interface WindowErrors {
  start?: string;
  end?: string;
}

interface DayErrors {
  windows: Record<number, WindowErrors>;
  noWindows?: string;
  buffer?: string;
}

/** Editor order: Monday first, contract weekday numbering kept as index. */
const DISPLAY_ORDER = [1, 2, 3, 4, 5, 6, 0] as const;

const DEFAULT_WINDOW: WindowDraft = { start: "09:00", end: "17:00" };

function emptyWeek(): DayDraft[] {
  return Array.from({ length: 7 }, () => ({
    enabled: false,
    windows: [{ ...DEFAULT_WINDOW }],
    buffer: "0",
  }));
}

function draftsFromRules(rules: AvailabilityRule[]): DayDraft[] {
  const week = emptyWeek();
  for (const rule of rules) {
    if (rule.weekday < 0 || rule.weekday > 6) continue;
    week[rule.weekday] = {
      enabled: true,
      windows:
        rule.windows.length > 0
          ? rule.windows.map((w) => ({
              start: minutesToTimeString(w.startMin),
              end: minutesToTimeString(w.endMin),
            }))
          : [{ ...DEFAULT_WINDOW }],
      buffer: String(rule.bufferMinutes),
    };
  }
  return week;
}

function errorMessage(error: unknown): string {
  return error instanceof ApiError
    ? error.message
    : "Something went wrong. Try again.";
}

export default function AvailabilityPage() {
  const { session, refreshCallbacks, logout } = useAuth();
  const [days, setDays] = useState<DayDraft[] | null>(null);
  const [savedDays, setSavedDays] = useState<DayDraft[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<number, DayErrors>>({});
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState<string | null>(null);

  const [timeOffs, setTimeOffs] = useState<TimeOff[]>([]);
  const [timeOffStart, setTimeOffStart] = useState("");
  const [timeOffEnd, setTimeOffEnd] = useState("");
  const [timeOffReason, setTimeOffReason] = useState("");
  const [timeOffErrors, setTimeOffErrors] = useState<{
    start?: string;
    end?: string;
  }>({});
  const [timeOffError, setTimeOffError] = useState<string | null>(null);
  const [addingTimeOff, setAddingTimeOff] = useState(false);

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
    scheduleApi
      .getRules(session, refreshCallbacks)
      .then((rules) => {
        if (cancelled) return;
        const drafts = draftsFromRules(rules);
        setLoadError(null);
        setDays(drafts);
        setSavedDays(drafts);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setLoadError(errorMessage(error));
        handleSessionExpiry(error);
      });
    return () => {
      cancelled = true;
    };
  }, [session, refreshCallbacks, handleSessionExpiry]);

  useEffect(() => load(), [load]);

  const dirty =
    days !== null &&
    savedDays !== null &&
    JSON.stringify(days) !== JSON.stringify(savedDays);

  function updateDay(weekday: number, patch: Partial<DayDraft>) {
    setDays((prev) =>
      prev
        ? prev.map((d, i) => (i === weekday ? { ...d, ...patch } : d))
        : prev,
    );
    // Editing a day clears its validation errors.
    setErrors((prev) => {
      if (!prev[weekday]) return prev;
      const next = { ...prev };
      delete next[weekday];
      return next;
    });
  }

  function updateWindow(
    weekday: number,
    index: number,
    patch: Partial<WindowDraft>,
  ) {
    const day = days?.[weekday];
    if (!day) return;
    updateDay(weekday, {
      windows: day.windows.map((w, i) =>
        i === index ? { ...w, ...patch } : w,
      ),
    });
  }

  function addWindow(weekday: number) {
    const day = days?.[weekday];
    if (!day) return;
    updateDay(weekday, { windows: [...day.windows, { start: "", end: "" }] });
  }

  function removeWindow(weekday: number, index: number) {
    const day = days?.[weekday];
    if (!day) return;
    updateDay(weekday, { windows: day.windows.filter((_, i) => i !== index) });
  }

  /** Mirrors the API's 400 rules; returns normalized rules or null. */
  function validate(): AvailabilityRule[] | null {
    if (!days) return null;
    const nextErrors: Record<number, DayErrors> = {};
    const rules: AvailabilityRule[] = [];

    for (let weekday = 0; weekday < 7; weekday++) {
      const day = days[weekday]!;
      if (!day.enabled) continue;

      const dayErrors: DayErrors = { windows: {} };
      if (day.windows.length === 0) {
        dayErrors.noWindows =
          "Add at least one window, or mark the day closed.";
      }

      const parsed: { startMin: number; endMin: number }[] = [];
      day.windows.forEach((window, index) => {
        const startMin = parseTimeInput(window.start);
        const endMin = parseTimeInput(window.end);
        const windowErrors: WindowErrors = {};
        if (startMin === null) {
          windowErrors.start = "Enter a start time as HH:MM, e.g. 09:00";
        }
        if (endMin === null) {
          windowErrors.end = "Enter an end time as HH:MM, e.g. 17:00";
        }
        if (startMin !== null && endMin !== null) {
          const previous = parsed[parsed.length - 1];
          if (startMin >= endMin) {
            // The API rejects overnight windows with a 400 — catch it here.
            windowErrors.end =
              "End must be after start — overnight windows aren't allowed.";
          } else if (previous && startMin < previous.endMin) {
            windowErrors.start = "This window overlaps the one above.";
          } else {
            parsed.push({ startMin, endMin });
          }
        }
        if (windowErrors.start || windowErrors.end) {
          dayErrors.windows[index] = windowErrors;
        }
      });

      const buffer = day.buffer.trim();
      if (!/^\d+$/.test(buffer) || Number(buffer) > 120) {
        dayErrors.buffer = "Enter a buffer between 0 and 120 minutes.";
      }

      if (
        dayErrors.noWindows ||
        dayErrors.buffer ||
        Object.keys(dayErrors.windows).length > 0
      ) {
        nextErrors[weekday] = dayErrors;
      } else {
        rules.push({
          weekday,
          windows: parsed.sort((a, b) => a.startMin - b.startMin),
          bufferMinutes: Number(buffer),
        });
      }
    }

    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0 ? rules : null;
  }

  async function handleSave() {
    if (!session || !days) return;
    setSaveError(null);
    const rules = validate();
    if (!rules) return;

    setSaving(true);
    try {
      const saved = await scheduleApi.putRules(
        session,
        refreshCallbacks,
        rules,
      );
      const drafts = draftsFromRules(saved);
      setDays(drafts);
      setSavedDays(drafts);
      setToast("Availability saved.");
    } catch (error) {
      setSaveError(errorMessage(error));
      handleSessionExpiry(error);
    } finally {
      setSaving(false);
    }
  }

  async function handleAddTimeOff(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!session) return;
    setTimeOffError(null);

    const startCivil = parseDateInput(timeOffStart);
    const endCivil = parseDateInput(timeOffEnd);
    const fieldErrors: { start?: string; end?: string } = {};
    if (!startCivil)
      fieldErrors.start = "Enter a start date as YYYY-MM-DD, e.g. 2026-09-01";
    if (!endCivil)
      fieldErrors.end = "Enter an end date as YYYY-MM-DD, e.g. 2026-09-03";
    if (startCivil && endCivil) {
      const startNum =
        startCivil.year * 10000 + startCivil.month * 100 + startCivil.day;
      const endNum =
        endCivil.year * 10000 + endCivil.month * 100 + endCivil.day;
      if (endNum < startNum) {
        fieldErrors.end = "The end date must be on or after the start date.";
      }
    }
    setTimeOffErrors(fieldErrors);
    if (fieldErrors.start || fieldErrors.end) return;

    // The end date is inclusive for the practitioner; the contract's range is
    // [startAt, endAt), so endAt lands on the following midnight.
    const draft = {
      startAt: wallClockToUtcIso(
        `${startCivil!.year}-${String(startCivil!.month).padStart(2, "0")}-${String(startCivil!.day).padStart(2, "0")}`,
        "00:00",
        PRACTICE_TIMEZONE,
      )!,
      endAt: wallClockToUtcIso(
        (() => {
          const next = addDaysCivil(endCivil!, 1);
          return `${next.year}-${String(next.month).padStart(2, "0")}-${String(next.day).padStart(2, "0")}`;
        })(),
        "00:00",
        PRACTICE_TIMEZONE,
      )!,
      ...(timeOffReason.trim() ? { reason: timeOffReason.trim() } : {}),
    };

    setAddingTimeOff(true);
    try {
      const created = await scheduleApi.addTimeOff(
        session,
        refreshCallbacks,
        draft,
      );
      setTimeOffs((prev) => [created, ...prev]);
      setTimeOffStart("");
      setTimeOffEnd("");
      setTimeOffReason("");
      setToast("Time off saved.");
    } catch (error) {
      setTimeOffError(errorMessage(error));
      handleSessionExpiry(error);
    } finally {
      setAddingTimeOff(false);
    }
  }

  return (
    <div data-admin-page="availability" className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-[22px] leading-[1.3] font-semibold tracking-[-0.005em] text-ink">
            Availability
          </h1>
          <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
            The hours clients can book, week after week. Times are{" "}
            {timezoneShortName(PRACTICE_TIMEZONE)}.
          </p>
        </div>
        <Button
          onClick={() => void handleSave()}
          loading={saving}
          disabled={!dirty}
        >
          Save changes
        </Button>
      </div>

      {saveError ? (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0"
          />
          {saveError}
        </div>
      ) : null}

      {days === null && !loadError ? (
        <RulesSkeleton />
      ) : loadError ? (
        <Card className="flex flex-col items-center gap-3 py-12 text-center">
          <p
            role="alert"
            className="flex items-center gap-2 text-sm text-danger-ink"
          >
            <CircleAlert size={16} aria-hidden="true" className="shrink-0" />
            {loadError}
          </p>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setLoadError(null);
              load();
            }}
          >
            Try again
          </Button>
        </Card>
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
          {DISPLAY_ORDER.map((weekday) => {
            const day = days![weekday]!;
            const dayErrors = errors[weekday];
            return (
              <section
                key={weekday}
                aria-label={weekdayLongName(weekday)}
                className="flex flex-col gap-4 border-b border-border px-6 py-4 last:border-0 sm:flex-row sm:gap-6"
              >
                <div className="flex w-44 shrink-0 items-center gap-3">
                  <Switch
                    checked={day.enabled}
                    label={`${weekdayLongName(weekday)} open`}
                    onChange={(enabled) => updateDay(weekday, { enabled })}
                  />
                  <p
                    className={
                      day.enabled
                        ? "text-sm font-medium tracking-[0.005em] text-ink"
                        : "text-sm font-medium tracking-[0.005em] text-ink-muted"
                    }
                  >
                    {weekdayLongName(weekday)}
                  </p>
                </div>

                <div className="flex flex-1 flex-col gap-3">
                  {day.enabled ? (
                    <>
                      {day.windows.map((window, index) => (
                        <div key={index} className="flex items-start gap-3">
                          <div className="w-36">
                            <TimePicker
                              label={
                                index === 0
                                  ? "Start"
                                  : `Window ${index + 1} start`
                              }
                              value={window.start}
                              error={dayErrors?.windows[index]?.start}
                              onChange={(value) =>
                                updateWindow(weekday, index, { start: value })
                              }
                            />
                          </div>
                          <div className="w-36">
                            <TimePicker
                              label={
                                index === 0 ? "End" : `Window ${index + 1} end`
                              }
                              value={window.end}
                              error={dayErrors?.windows[index]?.end}
                              onChange={(value) =>
                                updateWindow(weekday, index, { end: value })
                              }
                            />
                          </div>
                          <div className="pt-7">
                            <IconButton
                              variant="ghost"
                              size="sm"
                              aria-label={`Remove window ${index + 1} on ${weekdayLongName(weekday)}`}
                              onClick={() => removeWindow(weekday, index)}
                            >
                              <X aria-hidden="true" />
                            </IconButton>
                          </div>
                        </div>
                      ))}
                      {dayErrors?.noWindows ? (
                        <p
                          role="alert"
                          className="flex items-center gap-1.5 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-danger-ink"
                        >
                          <CircleAlert
                            size={14}
                            aria-hidden="true"
                            className="shrink-0"
                          />
                          {dayErrors.noWindows}
                        </p>
                      ) : null}
                      <div className="flex flex-wrap items-start gap-3">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => addWindow(weekday)}
                        >
                          <Plus size={16} aria-hidden="true" />
                          Add window
                        </Button>
                        <div className="w-40">
                          <TextInput
                            label="Buffer (minutes)"
                            size="sm"
                            type="number"
                            inputMode="numeric"
                            min={0}
                            max={120}
                            placeholder="0"
                            hint="Kept free around each session, 0–120."
                            value={day.buffer}
                            error={dayErrors?.buffer}
                            onChange={(event) =>
                              updateDay(weekday, { buffer: event.target.value })
                            }
                          />
                        </div>
                      </div>
                    </>
                  ) : (
                    <p className="py-2 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
                      Closed
                    </p>
                  )}
                </div>
              </section>
            );
          })}
        </div>
      )}

      <Card className="z-[1] flex flex-col gap-5 overflow-visible">
        <div>
          <h2 className="text-base leading-[1.4] font-semibold tracking-[-0.005em] text-ink">
            Time off
          </h2>
          <p className="mt-1 text-sm leading-[1.55] text-ink-muted">
            Block whole days — vacations, holidays, errands. The end date is
            included.
          </p>
        </div>

        {/* noValidate: native validation bubbles are forbidden — errors are custom */}
        <form
          noValidate
          onSubmit={handleAddTimeOff}
          className="flex flex-col gap-4"
        >
          {timeOffError ? (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-md bg-danger-bg px-4 py-3 text-sm leading-[1.55] text-danger-ink"
            >
              <CircleAlert
                size={16}
                aria-hidden="true"
                className="mt-0.5 shrink-0"
              />
              {timeOffError}
            </div>
          ) : null}
          <div className="flex flex-wrap items-start gap-4">
            <div className="w-56">
              <DatePicker
                label="Start date"
                required
                value={timeOffStart}
                error={timeOffErrors.start}
                onChange={(value) => {
                  setTimeOffStart(value);
                  setTimeOffErrors((errors) => ({
                    ...errors,
                    start: undefined,
                  }));
                }}
              />
            </div>
            <div className="w-56">
              <DatePicker
                label="End date"
                required
                min={timeOffStart || undefined}
                value={timeOffEnd}
                error={timeOffErrors.end}
                onChange={(value) => {
                  setTimeOffEnd(value);
                  setTimeOffErrors((errors) => ({ ...errors, end: undefined }));
                }}
              />
            </div>
            <div className="min-w-48 flex-1">
              <TextInput
                label="Reason (optional)"
                placeholder="Family trip"
                value={timeOffReason}
                onChange={(event) => setTimeOffReason(event.target.value)}
              />
            </div>
            <div className="pt-7">
              <Button type="submit" variant="secondary" loading={addingTimeOff}>
                Add time off
              </Button>
            </div>
          </div>
        </form>

        {/* List-only: BE-04 has no GET/DELETE for time-off yet (see header note). */}
        {timeOffs.length > 0 ? (
          <ul className="flex flex-col divide-y divide-border border-t border-border">
            {timeOffs.map((entry) => {
              const start = zonedParts(entry.startAt, PRACTICE_TIMEZONE);
              // endAt is exclusive; the last blocked day is the one before it.
              const lastDay = addDaysCivil(
                zonedParts(entry.endAt, PRACTICE_TIMEZONE),
                -1,
              );
              return (
                <li
                  key={entry.id}
                  className="flex flex-wrap items-center gap-3 py-3"
                >
                  <p className="text-sm tabular-nums text-ink">
                    {formatCivilDate(start)}
                    {formatCivilDate(start) !== formatCivilDate(lastDay)
                      ? ` – ${formatCivilDate(lastDay)}`
                      : ""}
                  </p>
                  {entry.reason ? (
                    <p className="text-sm text-ink-muted">{entry.reason}</p>
                  ) : null}
                  <Badge variant="warning" dot>
                    Blocked
                  </Badge>
                </li>
              );
            })}
          </ul>
        ) : (
          <EmptyState
            compact
            icon={<CalendarOff size={24} />}
            title="No time off added"
            body="Entries you add here block booking across their whole date range."
          />
        )}
      </Card>

      {toast ? <Toast message={toast} onClose={() => setToast(null)} /> : null}
    </div>
  );
}

/** Success toast — design-system §3.16: bottom-right, 3px accent bar, 5s auto-dismiss. */
function Toast({ message, onClose }: { message: string; onClose: () => void }) {
  useEffect(() => {
    const timer = setTimeout(onClose, 5000);
    return () => clearTimeout(timer);
  }, [onClose]);

  return (
    <div className="fixed right-6 bottom-6 z-toast">
      <div
        role="status"
        className="animate-modal-enter flex min-w-80 max-w-[420px] items-center gap-3 rounded-lg border border-border border-l-[3px] border-l-success bg-surface-raised px-4 py-3 shadow-lg"
      >
        <CheckCircle2
          size={18}
          aria-hidden="true"
          className="shrink-0 text-success-ink"
        />
        <p className="flex-1 text-sm leading-[1.55] text-ink">{message}</p>
        <IconButton aria-label="Dismiss" size="sm" onClick={onClose}>
          <X aria-hidden="true" />
        </IconButton>
      </div>
    </div>
  );
}

/** Skeleton mirroring the 7 day rows (§3.28). */
function RulesSkeleton() {
  return (
    <div
      role="status"
      aria-busy="true"
      className="overflow-hidden rounded-lg border border-border bg-surface-raised"
    >
      <span className="sr-only">Loading your availability…</span>
      {Array.from({ length: 7 }, (_, i) => (
        <div
          key={i}
          className="flex items-center gap-6 border-b border-border px-6 py-4 last:border-0"
        >
          <div
            className="skeleton-shimmer h-6 w-40 rounded-full"
            aria-hidden="true"
          />
          <div
            className="skeleton-shimmer h-8 w-28 rounded-md"
            aria-hidden="true"
          />
          <div
            className="skeleton-shimmer h-8 w-28 rounded-md"
            aria-hidden="true"
          />
        </div>
      ))}
    </div>
  );
}
