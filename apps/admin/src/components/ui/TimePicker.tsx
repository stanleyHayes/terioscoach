"use client";

import { Check, ChevronDown, CircleAlert, Clock3 } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { cn } from "@/lib/cn";

const TIMES = Array.from(
  { length: 96 },
  (_, index) =>
    `${String(Math.floor(index / 4)).padStart(2, "0")}:${String((index % 4) * 15).padStart(2, "0")}`,
);
const display = (value: string) => {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return "Choose time";
  const hour = Number(match[1]);
  return `${hour % 12 || 12}:${match[2]} ${hour < 12 ? "AM" : "PM"}`;
};

export function TimePicker({
  label,
  value,
  onChange,
  error,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
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
  return (
    <div ref={root} className="relative flex flex-col gap-1.5">
      <label id={`${id}-label`} className="text-sm font-medium text-ink">
        {label}
      </label>
      <button
        type="button"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        className={cn(
          "flex h-9 w-full items-center justify-between gap-2 rounded-xl border bg-surface-raised px-3 text-left text-sm",
          error ? "border-danger" : "border-border-strong hover:border-primary",
          !value && "text-ink-faint",
        )}
      >
        <span id={`${id}-value`} className="flex items-center gap-2">
          <Clock3 size={14} className="text-primary" />
          {display(value)}
        </span>
        <ChevronDown size={14} />
      </button>
      {error ? (
        <p
          role="alert"
          className="flex items-center gap-1 text-xs text-danger-ink"
        >
          <CircleAlert size={13} />
          {error}
        </p>
      ) : null}
      {open ? (
        <div
          role="listbox"
          aria-label={`${label} options`}
          className="absolute top-full left-0 z-dropdown mt-2 max-h-64 w-44 overflow-y-auto rounded-2xl border border-border bg-surface-raised p-2 shadow-lg"
        >
          {TIMES.map((time) => (
            <button
              key={time}
              type="button"
              role="option"
              aria-selected={time === value}
              onClick={() => {
                onChange(time);
                setOpen(false);
              }}
              className={cn(
                "flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm hover:bg-surface-sunken",
                time === value && "bg-eucalyptus-50 font-semibold text-primary",
              )}
            >
              {display(time)}
              {time === value ? <Check size={14} /> : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
