"use client";

import { Calendar } from "lucide-react";
import { Card } from "@/components/ui/Card";
import { useAuth } from "@/lib/auth";

/**
 * Portal Overview — welcome card + EmptyState (design-system §3.27).
 * Session data arrives with the booking tasks; until then the empty state
 * says what will appear here, per brand voice.
 */
export default function PortalOverviewPage() {
  const { user } = useAuth();

  return (
    <div className="animate-fade-in flex flex-col gap-6">
      <Card>
        <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
          Overview
        </p>
        <h1 className="mt-3 font-display text-[2rem] leading-[1.15] font-medium tracking-[-0.01em] text-ink">
          Welcome back, {user?.name}
        </h1>
        <p className="mt-3 max-w-[60ch] text-base leading-[1.6] text-ink-muted">
          This is your private space for sessions, forms and documents — everything
          between you and your practitioner, in one calm place.
        </p>
      </Card>

      <Card>
        <div className="mx-auto flex max-w-[360px] flex-col items-center px-6 py-12 text-center">
          <span
            aria-hidden="true"
            className="flex size-16 items-center justify-center rounded-full bg-surface-sunken text-ink-faint"
          >
            <Calendar size={32} />
          </span>
          <h2 className="mt-6 font-display text-[1.5rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
            No sessions yet
          </h2>
          <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
            Your upcoming sessions will appear here. When you book one, this page
            is where you will find it.
          </p>
        </div>
      </Card>
    </div>
  );
}
