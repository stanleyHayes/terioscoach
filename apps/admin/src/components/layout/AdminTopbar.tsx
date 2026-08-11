"use client";

import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/Button";

/**
 * Admin top bar — practitioner identity on the right with a sign-out action.
 * 64px, surface-raised, hairline bottom border. Copy is terse per the admin voice.
 */

export function AdminTopbar({
  userName,
  onSignOut,
  signingOut = false,
}: {
  userName: string;
  onSignOut: () => void;
  signingOut?: boolean;
}) {
  return (
    <header className="flex h-16 shrink-0 items-center justify-between border-b border-border bg-surface-raised px-6">
      <div />
      <div className="flex items-center gap-4">
        <p className="text-sm leading-[1.55] text-ink-muted">
          Signed in as <span className="font-medium text-ink">{userName}</span>
        </p>
        <Button
          variant="ghost"
          size="sm"
          loading={signingOut}
          onClick={onSignOut}
        >
          <LogOut size={16} aria-hidden="true" />
          Sign out
        </Button>
      </div>
    </header>
  );
}
