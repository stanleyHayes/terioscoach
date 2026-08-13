"use client";

import { useEffect, useRef, type ReactNode } from "react";
import { cn } from "@/lib/cn";

export type SectionBackground = "surface" | "raised" | "sunken" | "night";

const backgroundClasses: Record<SectionBackground, string> = {
  surface: "bg-surface",
  raised: "bg-surface-raised",
  sunken: "bg-surface-sunken",
  night: "bg-eucalyptus-900",
};

export interface SectionProps {
  id?: string;
  /** Point at the SectionHeading id to give the region an accessible name. */
  ariaLabelledby?: string;
  /** Select a surface here rather than passing a conflicting bg-* utility in
   * className. `cn` concatenates classes and does not resolve conflicts. */
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
  const sectionRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const node = sectionRef.current;
    if (!node) return;
    if (!("IntersectionObserver" in window)) return;
    node.dataset.revealReady = "true";
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          node.dataset.revealed = "true";
          observer.disconnect();
        }
      },
      { rootMargin: "0px 0px -10%", threshold: 0.08 },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return (
    <section
      ref={sectionRef}
      id={id}
      aria-labelledby={ariaLabelledby}
      className={cn("terios-section-reveal", backgroundClasses[background], className)}
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
