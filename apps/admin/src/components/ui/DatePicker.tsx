"use client";

import {
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
} from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { cn } from "@/lib/cn";

interface DatePickerProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  required?: boolean;
  min?: string;
}

const WEEKDAYS = ["Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"];
const iso = (date: Date) =>
  `${date.getUTCFullYear()}-${String(date.getUTCMonth() + 1).padStart(2, "0")}-${String(date.getUTCDate()).padStart(2, "0")}`;
const parse = (value: string) => {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const date = new Date(
    Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])),
  );
  return iso(date) === value ? date : null;
};
const pretty = (value: string) => {
  const date = parse(value);
  return date
    ? new Intl.DateTimeFormat("en-GB", {
        day: "numeric",
        month: "short",
        year: "numeric",
        timeZone: "UTC",
      }).format(date)
    : "Choose date";
};

export function DatePicker({
  label,
  value,
  onChange,
  error,
  required,
  min,
}: DatePickerProps) {
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const selected = parse(value);
  const initial = selected ?? parse(min ?? "") ?? new Date();
  const [open, setOpen] = useState(false);
  const [month, setMonth] = useState(
    () =>
      new Date(Date.UTC(initial.getUTCFullYear(), initial.getUTCMonth(), 1)),
  );
  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", escape);
    };
  }, []);
  const cells = useMemo(() => {
    const firstOffset = (month.getUTCDay() + 6) % 7;
    const start = new Date(
      Date.UTC(month.getUTCFullYear(), month.getUTCMonth(), 1 - firstOffset),
    );
    return Array.from(
      { length: 42 },
      (_, index) => new Date(start.getTime() + index * 86400000),
    );
  }, [month]);
  const monthLabel = new Intl.DateTimeFormat("en-GB", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  }).format(month);

  return (
    <div ref={root} className="relative flex flex-col gap-1.5">
      <label id={`${id}-label`} className="text-sm font-medium text-ink">
        {label}
        {required ? (
          <span className="text-accent" aria-hidden="true">
            {" "}
            *
          </span>
        ) : null}
      </label>
      <button
        type="button"
      aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        onClick={() => {
          setOpen((current) => !current);
          if (selected)
            setMonth(
              new Date(
                Date.UTC(selected.getUTCFullYear(), selected.getUTCMonth(), 1),
              ),
            );
        }}
        className={cn(
          "flex h-11 w-full items-center justify-between gap-3 rounded-xl border bg-surface-raised px-3.5 text-left text-sm transition-colors",
          error ? "border-danger" : "border-border-strong hover:border-primary",
          !value && "text-ink-faint",
        )}
      >
        <span id={`${id}-value`}>{pretty(value)}</span>
        <CalendarDays size={17} className="shrink-0 text-primary" />
      </button>
      {error ? (
        <p
          role="alert"
          className="flex items-center gap-1.5 text-[13px] font-medium text-danger-ink"
        >
          <CircleAlert size={14} />
          {error}
        </p>
      ) : null}
      {open ? (
        <div
          role="dialog"
          aria-label={`${label} calendar`}
          className="absolute top-full left-0 z-dropdown mt-2 w-[300px] rounded-2xl border border-border bg-surface-raised p-4 shadow-lg"
        >
          <div className="flex items-center justify-between">
            <button
              type="button"
              aria-label="Previous month"
              onClick={() =>
                setMonth(
                  new Date(
                    Date.UTC(
                      month.getUTCFullYear(),
                      month.getUTCMonth() - 1,
                      1,
                    ),
                  ),
                )
              }
              className="flex size-9 items-center justify-center rounded-xl text-ink-muted hover:bg-surface-sunken"
            >
              <ChevronLeft size={17} />
            </button>
            <p className="text-sm font-semibold text-ink">{monthLabel}</p>
            <button
              type="button"
              aria-label="Next month"
              onClick={() =>
                setMonth(
                  new Date(
                    Date.UTC(
                      month.getUTCFullYear(),
                      month.getUTCMonth() + 1,
                      1,
                    ),
                  ),
                )
              }
              className="flex size-9 items-center justify-center rounded-xl text-ink-muted hover:bg-surface-sunken"
            >
              <ChevronRight size={17} />
            </button>
          </div>
          <div className="mt-3 grid grid-cols-7 gap-1" aria-hidden="true">
            {WEEKDAYS.map((day) => (
              <span
                key={day}
                className="py-1 text-center text-[10px] font-semibold uppercase text-ink-faint"
              >
                {day}
              </span>
            ))}
          </div>
          <div className="grid grid-cols-7 gap-1">
            {cells.map((date) => {
              const valueForDay = iso(date);
              const outside = date.getUTCMonth() !== month.getUTCMonth();
              const disabled = Boolean(min && valueForDay < min);
              const active = valueForDay === value;
              return (
                <button
                  key={valueForDay}
                  type="button"
                  disabled={disabled}
                  aria-label={`Choose ${valueForDay}`}
                  aria-pressed={active}
                  onClick={() => {
                    onChange(valueForDay);
                    setOpen(false);
                  }}
                  className={cn(
                    "flex size-9 items-center justify-center rounded-xl text-xs tabular-nums transition-colors",
                    active
                      ? "bg-primary font-semibold text-on-primary"
                      : "text-ink hover:bg-surface-sunken",
                    outside && !active && "text-ink-faint",
                    disabled && "cursor-not-allowed opacity-30",
                  )}
                >
                  {date.getUTCDate()}
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}
