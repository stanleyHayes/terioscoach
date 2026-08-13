"use client";

import {
  BarChart3,
  Calendar,
  CalendarClock,
  ClipboardList,
  CreditCard,
  FileText,
  LayoutDashboard,
  Mail,
  Sparkles,
  Star,
  Users,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/lib/cn";

/**
 * Admin sidebar — design-system §3.30.
 * 276px fixed, surface-sunken with a right border. Figtree wordmark,
 * micro section labels, 36px nav items (18px icon + label ink-muted),
 * active item: eucalyptus-100 fill, eucalyptus-800 text, weight 600,
 * 3px rounded primary bar on the left. Bottom: practitioner user card.
 * (Collapse-to-rail and the mobile Drawer variant are later tasks.)
 */

export const NAV_ITEMS = [
  { label: "Overview", href: "/", icon: LayoutDashboard },
  { label: "Calendar", href: "/calendar", icon: Calendar },
  { label: "Availability", href: "/availability", icon: CalendarClock },
  { label: "Clients", href: "/clients", icon: Users },
  { label: "Services", href: "/services", icon: Sparkles },
  { label: "Payments", href: "/payments", icon: CreditCard },
  { label: "Content", href: "/content", icon: FileText },
  { label: "Forms", href: "/forms", icon: ClipboardList },
  { label: "Enquiries", href: "/enquiries", icon: Mail },
  { label: "Reviews", href: "/reviews", icon: Star },
  { label: "Reports", href: "/reports", icon: BarChart3 },
] as const;

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]!.toUpperCase())
    .join("");
}

export function AdminSidebar({ userName }: { userName: string }) {
  const pathname = usePathname();

  return (
    <nav
      aria-label="Practice"
      className="sticky top-0 hidden h-screen w-[248px] shrink-0 flex-col overflow-hidden border-r border-eucalyptus-800 bg-eucalyptus-900 text-sand-0 lg:flex"
    >
      <div aria-hidden="true" className="absolute -left-24 top-1/3 size-64 rounded-full bg-eucalyptus-300/10 blur-3xl" />
      <div className="relative px-6 pb-6 pt-8">
        <Link href="/" className="inline-block rounded-sm">
          <span className="font-display text-[1.65rem] font-semibold tracking-[-0.035em] text-sand-0">
            Terios
          </span>
        </Link>
        <p className="mt-1 text-[12px] leading-[1.45] font-medium tracking-[0.04em] text-eucalyptus-300">
          Care workspace
        </p>
      </div>

      <p className="relative px-6 pb-3 pt-5 text-[10px] leading-[1.3] font-semibold tracking-[0.13em] uppercase text-eucalyptus-400">
        Working set
      </p>

      <ul className="relative flex flex-1 flex-col gap-1 overflow-y-auto px-3 pb-4">
        {NAV_ITEMS.map(({ label, href, icon: Icon }) => {
          const active = href === "/" ? pathname === "/" : pathname === href;
          return (
            <li key={label}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative flex h-10 items-center gap-3 rounded-xl px-3 text-sm font-medium tracking-[0.005em] text-eucalyptus-200 transition-[color,background-color,transform] duration-fast ease-out hover:translate-x-0.5 hover:bg-sand-0/8 hover:text-sand-0",
                  active && "bg-sand-0 font-semibold text-eucalyptus-900 hover:bg-sand-0 hover:text-eucalyptus-900",
                )}
              >
                {active ? (
                  <span
                    aria-hidden="true"
                    className="absolute top-1/2 left-0 h-5 w-[3px] -translate-y-1/2 rounded-full bg-clay-500"
                  />
                ) : null}
                <Icon size={18} aria-hidden="true" />
                {label}
              </Link>
            </li>
          );
        })}
      </ul>

      <div className="relative m-3 flex items-center gap-3 rounded-2xl border border-sand-0/10 bg-sand-0/6 px-4 py-4">
        <span
          aria-hidden="true"
          className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-clay-300 text-[13px] font-semibold text-eucalyptus-900"
        >
          {initials(userName)}
        </span>
        <div className="min-w-0">
          <p className="truncate text-sm leading-[1.55] text-sand-0">{userName}</p>
          <p className="text-[12px] leading-[1.45] font-medium tracking-[0.01em] text-eucalyptus-300">
            Practitioner
          </p>
        </div>
      </div>
    </nav>
  );
}
