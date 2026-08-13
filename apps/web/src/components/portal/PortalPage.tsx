"use client";

import { CircleAlert } from "lucide-react";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/Card";
import { buttonClasses } from "@/components/ui/Button";

/**
 * The shell every portal screen shares: heading, then one of four states —
 * loading, failed, empty, or content.
 *
 * These four are the same on every screen, and getting them subtly
 * different per page is how a portal starts to feel unreliable. The wording
 * is the caller's; the structure is not.
 */

export interface PortalPageProps {
  title: string;
  intro?: string;
  children: ReactNode;
}

export function PortalPage({ title, intro, children }: PortalPageProps) {
  return (
    <div data-portal-page className="animate-fade-in flex flex-col gap-8">
      <header className="relative overflow-hidden rounded-[1.75rem] bg-eucalyptus-900 px-6 py-8 text-sand-0 shadow-[0_24px_70px_rgba(28,51,40,.16)] sm:px-8 sm:py-10">
        <div aria-hidden="true" className="absolute -right-20 -top-24 size-64 rounded-full bg-eucalyptus-300/15 blur-3xl" />
        <p className="relative text-[11px] font-semibold tracking-[0.12em] text-eucalyptus-300 uppercase">Your care record</p>
        <h1 className="relative mt-3 font-display text-[2.5rem] leading-[1.02] font-semibold tracking-[-0.035em] text-sand-0">
          {title}
        </h1>
        {intro ? (
          <p className="relative mt-4 max-w-[60ch] text-base leading-[1.65] text-eucalyptus-200">{intro}</p>
        ) : null}
      </header>
      {children}
    </div>
  );
}

/** First-load skeleton. Distinct from "nothing here" on purpose: a client
 * should never read an empty state that is really a slow network. */
export function PortalLoading({ label, rows = 3 }: { label: string; rows?: number }) {
  return (
    <div role="status" aria-busy="true" className="flex flex-col gap-4">
      <span className="sr-only">{label}</span>
      {Array.from({ length: rows }, (_, index) => (
        <span key={index} aria-hidden="true" className="skeleton-shimmer h-28 rounded-[1.25rem] bg-surface-sunken" />
      ))}
    </div>
  );
}

/** A failure the client can act on, phrased so it never reads as their
 * fault. */
export function PortalError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Card>
      <div role="alert" className="flex flex-col items-center gap-3 py-6 text-center">
        <CircleAlert size={20} aria-hidden="true" className="text-danger-ink" />
        <p className="max-w-[46ch] text-sm leading-[1.55] text-ink-muted">{message}</p>
        <button
          type="button"
          onClick={onRetry}
          className="text-sm font-medium text-primary transition-colors duration-instant ease-out hover:text-primary-hover"
        >
          Try again
        </button>
      </div>
    </Card>
  );
}

export interface PortalEmptyProps {
  icon: ReactNode;
  title: string;
  body: string;
  action?: { href: string; label: string };
}

/** The empty state (design-system §3.27) — always says what will fill it. */
export function PortalEmpty({ icon, title, body, action }: PortalEmptyProps) {
  return (
    <Card>
      <div className="mx-auto flex max-w-[380px] flex-col items-center px-6 py-12 text-center">
        <span
          aria-hidden="true"
          className="flex size-16 items-center justify-center rounded-full bg-surface-sunken text-ink-faint"
        >
          {icon}
        </span>
        <h2 className="mt-6 font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
          {title}
        </h2>
        <p className="mt-2 text-sm leading-[1.55] text-ink-muted">{body}</p>
        {action ? (
          <div className="mt-6">
            <a href={action.href} className={buttonClasses({ size: "sm" })}>
              {action.label}
            </a>
          </div>
        ) : null}
      </div>
    </Card>
  );
}
