"use client";

import { CircleAlert } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "@/components/ui/Button";

/**
 * The four states every content list is in at some point — writing error,
 * load failure, first load, and nothing there. Four sections repeating them
 * is four chances to word an empty state differently or forget a
 * `role="status"`, so they live here once.
 */

export function ErrorBanner({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div
      role="alert"
      className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
    >
      <CircleAlert size={16} aria-hidden="true" className="mt-0.5 shrink-0 text-danger-ink" />
      <p className="text-sm text-danger-ink">{message}</p>
    </div>
  );
}

export function LoadFailure({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div
      role="alert"
      className="rounded-lg border border-border bg-surface-raised p-8 text-center"
    >
      <p className="text-sm text-ink-muted">{message}</p>
      <div className="mt-4">
        <Button variant="secondary" size="sm" onClick={onRetry}>
          Try again
        </Button>
      </div>
    </div>
  );
}

export function Skeletons({ label, count = 3 }: { label: string; count?: number }) {
  return (
    <div role="status" aria-busy="true" className="flex flex-col gap-3">
      <span className="sr-only">{label}</span>
      {Array.from({ length: count }, (_, i) => (
        <span key={i} aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken" />
      ))}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon: ReactNode;
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-center rounded-lg border border-border bg-surface-raised px-6 py-16 text-center">
      <span aria-hidden="true" className="terios-empty-icon flex size-14 items-center justify-center rounded-full bg-surface-sunken text-ink-faint">
        {icon}
      </span>
      <h3 className="mt-5 font-display text-xl leading-[1.3] font-medium text-ink">{title}</h3>
      <p className="mt-2 max-w-[46ch] text-sm leading-[1.55] text-ink-muted">{body}</p>
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}
