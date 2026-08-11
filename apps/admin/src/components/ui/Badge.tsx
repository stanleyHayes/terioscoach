import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Badge — design-system §3.20.
 * Status pill: micro type (11px, uppercase, +0.08em), 3px/8px padding,
 * radius-full, status-bg/status-ink token pairs. Optional 6px status dot.
 */

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  /** 6px status dot before the label, colored with the status token. */
  dot?: boolean;
  children: ReactNode;
}

const variants: Record<BadgeVariant, string> = {
  success: "bg-success-bg text-success-ink",
  warning: "bg-warning-bg text-warning-ink",
  danger: "bg-danger-bg text-danger-ink",
  info: "bg-info-bg text-info-ink",
  neutral: "bg-surface-sunken text-ink-muted",
};

const dotColors: Record<BadgeVariant, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
  neutral: "bg-border-strong",
};

export function Badge({
  variant = "neutral",
  dot = false,
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full px-2 py-[3px] text-[11px] leading-[1.3] font-semibold tracking-[0.08em] uppercase",
        variants[variant],
        className,
      )}
      {...rest}
    >
      {dot ? (
        <span
          aria-hidden="true"
          className={cn("size-1.5 rounded-full", dotColors[variant])}
        />
      ) : null}
      {children}
    </span>
  );
}
