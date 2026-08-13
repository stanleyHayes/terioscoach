import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Card — design-system §3.21.
 * Default: surface-raised, 1px border, radius-lg, padding space-6, no shadow
 * (structure is defined by borders, not shadows). `hoverable` adds the
 * hover treatment for clickable cards — wrap the card in a single
 * <a>/<button>; never nest interactive elements inside.
 */

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hoverable?: boolean;
  children: ReactNode;
}

export function Card({ hoverable = false, className, children, ...rest }: CardProps) {
  return (
    <div
      className={cn(
        "terios-card relative overflow-hidden rounded-[1.25rem] border border-border/75 bg-surface-raised/88 p-6 shadow-[0_20px_70px_rgba(0,0,0,.08)] backdrop-blur-sm",
        hoverable &&
          "terios-card-interactive transition-[transform,border-color,box-shadow] duration-base ease-out hover:-translate-y-1 hover:border-primary/50 hover:shadow-sm active:scale-[.99]",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
