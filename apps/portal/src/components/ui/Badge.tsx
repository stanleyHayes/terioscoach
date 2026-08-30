import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Badge (status) — design-system §3.20.
 * micro (11px, uppercase, +0.08em), padding 3px 8px, radius-full, with the
 * status-bg/status-ink token pairs and an optional 6px dot in the solid
 * status color.
 */

export type BadgeTone = "success" | "warning" | "danger" | "info" | "neutral";

const toneClasses: Record<BadgeTone, string> = {
  success: "bg-success-bg text-success-ink",
  warning: "bg-warning-bg text-warning-ink",
  danger: "bg-danger-bg text-danger-ink",
  info: "bg-info-bg text-info-ink",
  neutral: "bg-surface-sunken text-ink-muted",
};

const dotClasses: Record<BadgeTone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
  neutral: "bg-border-strong",
};

export interface BadgeProps {
  tone: BadgeTone;
  /** 6px status dot (default on). */
  dot?: boolean;
  children: ReactNode;
  className?: string;
}

export function Badge({ tone, dot = true, children, className }: BadgeProps) {
  return (
    <span
      className={cn(
        "terios-badge inline-flex items-center gap-1.5 rounded-full border border-current/10 px-2.5 py-1",
        "text-[11px] font-semibold uppercase tracking-[0.08em]",
        toneClasses[tone],
        className,
      )}
    >
      {dot ? (
        <span aria-hidden="true" className={cn("size-1.5 rounded-full", dotClasses[tone])} />
      ) : null}
      {children}
    </span>
  );
}
