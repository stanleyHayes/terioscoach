"use client";

import { Check, ChevronDown, CircleAlert, Clock3 } from "lucide-react";
import { useLayoutEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
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
  const trigger = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  useLayoutEffect(() => {
    if (!open) return;
    const panel = menu.current;
    const button = trigger.current;
    if (!panel || !button) return;
    function place() {
      if (!panel || !button) return;
      const rect = button.getBoundingClientRect();
      const margin = 8;
      const below = window.innerHeight - rect.bottom - margin * 2;
      const above = rect.top - margin * 2;
      const upward = below < 256 && above > below;
      const height = Math.max(0, Math.min(256, upward ? above : below));
      const width = Math.min(Math.max(192, rect.width), window.innerWidth - margin * 2);
      Object.assign(panel.style, {
        width: `${width}px`, maxHeight: `${height}px`,
        left: `${Math.max(margin, Math.min(rect.left, window.innerWidth - width - margin))}px`,
        top: `${upward ? rect.top - height - margin : rect.bottom + margin}px`,
        visibility: "visible",
      });
    }
    const close = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!root.current?.contains(target) && !panel.contains(target)) setOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setOpen(false);
        button.focus();
      }
    };
    place();
    const selected = panel.querySelector<HTMLButtonElement>('[aria-selected="true"]') ?? panel.querySelector<HTMLButtonElement>('[role="option"]');
    selected?.focus({ preventScroll: true });
    if (selected) panel.scrollTop = selected.offsetTop - panel.clientHeight / 2 + selected.offsetHeight / 2;
    window.addEventListener("resize", place);
    window.addEventListener("scroll", place, true);
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", escape);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", place, true);
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);
  return (
    <div ref={root} className="relative flex flex-col gap-1.5">
      <label id={`${id}-label`} className="text-sm font-medium text-ink">
        {label}
      </label>
      <button
        ref={trigger}
        type="button"
        aria-label={label}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? `${id}-options` : undefined}
        onKeyDown={(event) => { if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); setOpen(true); } }}
        onClick={() => setOpen((current) => !current)}
        className={cn(
          "flex h-10 w-full items-center justify-between gap-2 rounded-xl border bg-surface-raised px-3 text-left text-sm shadow-sm outline-none transition-[border-color,box-shadow,background-color] focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20",
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
      {open ? createPortal(
        <div
          ref={menu}
          id={`${id}-options`}
          role="listbox"
          aria-label={`${label} options`}
          style={{ visibility: "hidden" }}
          onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node) && !root.current?.contains(event.relatedTarget as Node)) setOpen(false); }}
          onKeyDown={(event) => {
            const options = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="option"]'));
            const current = options.indexOf(document.activeElement as HTMLButtonElement);
            const next = event.key === "Home" ? 0 : event.key === "End" ? options.length - 1 : event.key === "ArrowDown" ? Math.min(current + 1, options.length - 1) : event.key === "ArrowUp" ? Math.max(current - 1, 0) : -1;
            if (next >= 0) { event.preventDefault(); options[next]?.focus(); }
          }}
          className="fixed z-[100] max-h-64 w-48 overflow-y-auto overscroll-contain rounded-2xl border border-border bg-surface-raised p-2 shadow-xl"
        >
          {TIMES.map((time) => (
            <button
              key={time}
              type="button"
              role="option"
              aria-selected={time === value}
              tabIndex={time === value ? 0 : -1}
              onClick={() => {
                onChange(time);
                setOpen(false);
                trigger.current?.focus();
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
        </div>,
        document.body,
      ) : null}
    </div>
  );
}
