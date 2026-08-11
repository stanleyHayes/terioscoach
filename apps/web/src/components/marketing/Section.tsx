import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type SectionBackground = "surface" | "raised" | "sunken";

const backgroundClasses: Record<SectionBackground, string> = {
  surface: "bg-surface",
  raised: "bg-surface-raised",
  sunken: "bg-surface-sunken",
};

export interface SectionProps {
  id?: string;
  /** Point at the SectionHeading id to give the region an accessible name. */
  ariaLabelledby?: string;
  background?: SectionBackground;
  className?: string;
  containerClassName?: string;
  children: ReactNode;
}

/** Marketing section shell (design-system §2): vertical padding space-12
 * mobile / space-20 desktop, 1200px container, 24px/48px gutters. Heroes
 * override with `containerClassName` (e.g. min-h + centering). */
export function Section({
  id,
  ariaLabelledby,
  background = "surface",
  className,
  containerClassName,
  children,
}: SectionProps) {
  return (
    <section
      id={id}
      aria-labelledby={ariaLabelledby}
      className={cn(backgroundClasses[background], className)}
    >
      <div
        className={cn(
          "mx-auto max-w-[1200px] px-6 py-12 lg:px-12 lg:py-20",
          containerClassName,
        )}
      >
        {children}
      </div>
    </section>
  );
}
