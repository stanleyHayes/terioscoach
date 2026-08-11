import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * Card — design-system §3.21.
 * Default: surface-raised, 1px border, radius-lg, padding space-6, no shadow
 * (structure is defined by borders, not shadows). `hoverable` adds the hover
 * treatment for clickable cards — wrap the card in a single <a>/<button>;
 * never nest interactive elements inside.
 */

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  hoverable?: boolean;
  children: ReactNode;
}

export function Card({ hoverable = false, className, children, ...rest }: CardProps) {
  return (
    <div
      className={cn(
        "rounded-lg border border-border bg-surface-raised p-6",
        hoverable &&
          "transition-[transform,border-color,box-shadow] duration-base ease-out hover:-translate-y-0.5 hover:border-border-strong hover:shadow-sm",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
