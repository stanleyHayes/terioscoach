"use client";

import { CircleAlert } from "lucide-react";
import type { ReactNode } from "react";
import { Card } from "@/components/ui/Card";
import { buttonClasses } from "@/components/ui/Button";
import { EmptyState } from "@/components/ui/EmptyState";

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
      <header className="relative overflow-hidden rounded-[1.75rem] border border-border/70 bg-surface-raised/85 px-6 py-7 shadow-[0_28px_90px_rgba(31,41,34,.08)] backdrop-blur-xl sm:px-8 sm:py-9">
        <div aria-hidden="true" className="absolute inset-y-0 left-0 w-1 bg-primary" />
        <div
          aria-hidden="true"
          className="absolute -right-20 -top-24 size-64 rounded-full bg-primary/10 blur-3xl"
        />
        <p className="relative text-[11px] font-semibold tracking-[0.12em] text-primary uppercase">
          Your care record
        </p>
        <h1 className="relative mt-2 font-display text-[2rem] leading-tight font-semibold tracking-[-0.035em] text-ink sm:text-[2.5rem]">
          {title}
        </h1>
        {intro ? (
          <p className="relative mt-3 max-w-[60ch] text-sm leading-relaxed text-ink-muted">
            {intro}
          </p>
        ) : null}
      </header>
      {children}
    </div>
  );
}

/** First-load skeleton. Distinct from "nothing here" on purpose: a client
 * should never read an empty state that is really a slow network. */
export function PortalLoading({
  label,
  rows = 3,
}: {
  label: string;
  rows?: number;
}) {
  return (
    <div role="status" aria-busy="true" className="flex flex-col gap-4">
      <span className="sr-only">{label}</span>
      {Array.from({ length: rows }, (_, index) => (
        <span
          key={index}
          aria-hidden="true"
          className="skeleton-shimmer h-28 rounded-[1.25rem] bg-surface-sunken"
        />
      ))}
    </div>
  );
}

/** A failure the client can act on, phrased so it never reads as their
 * fault. */
export function PortalError({
  message,
  onRetry,
}: {
  message: string;
  onRetry: () => void;
}) {
  return (
    <Card>
      <div
        role="alert"
        className="flex flex-col items-center gap-3 py-6 text-center"
      >
        <CircleAlert size={20} aria-hidden="true" className="text-danger-ink" />
        <p className="max-w-[46ch] text-sm leading-[1.55] text-ink-muted">
          {message}
        </p>
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
      <EmptyState
        icon={icon}
        title={title}
        description={body}
        action={
          action ? (
            <a href={action.href} className={buttonClasses({ size: "sm" })}>
              {action.label}
            </a>
          ) : undefined
        }
      />
    </Card>
  );
}
