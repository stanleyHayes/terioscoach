"use client";

import { LogOut } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { NAV_ITEMS } from "@/components/layout/AdminSidebar";
import { Button } from "@/components/ui/Button";
import { cn } from "@/lib/cn";

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
  const pathname = usePathname();
  return (
    <header className="sticky top-0 z-sticky border-b border-border/80 bg-surface-raised/88 backdrop-blur-xl">
      <div className="flex h-16 items-center justify-between px-4 sm:px-6">
        <Link href="/" className="font-display text-xl font-semibold tracking-[-0.025em] text-ink lg:hidden">Terios</Link>
        <p className="hidden text-xs font-semibold tracking-[0.08em] text-ink-faint uppercase lg:block">Practice workspace</p>
        <div className="flex items-center gap-3">
        <p className="hidden text-sm leading-[1.55] text-ink-muted sm:block">
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
      </div>
      <nav aria-label="Practice sections" className="overflow-x-auto border-t border-border/60 px-3 lg:hidden">
        <ul className="flex min-w-max gap-1 py-2">
          {NAV_ITEMS.map(({ label, href, icon: Icon }) => {
            const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
            return <li key={href}><Link href={href} aria-current={active ? "page" : undefined} className={cn("flex h-9 items-center gap-2 rounded-full px-3 text-xs font-semibold text-ink-muted transition-colors hover:bg-surface-sunken hover:text-ink", active && "bg-primary/12 text-primary")}><Icon size={15} aria-hidden="true" />{label}</Link></li>;
          })}
        </ul>
      </nav>
    </header>
  );
}
