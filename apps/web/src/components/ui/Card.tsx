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
        "terios-card relative overflow-hidden rounded-[1.5rem] border border-border/75 bg-surface-raised/92 p-6 shadow-[0_18px_60px_rgba(31,41,34,.045)] backdrop-blur-sm",
        hoverable &&
          "terios-card-interactive transition-[transform,border-color,box-shadow] duration-base ease-out hover:-translate-y-1 hover:border-eucalyptus-200 hover:shadow-md active:scale-[.99]",
        className,
      )}
      {...rest}
    >
      {children}
    </div>
  );
}
