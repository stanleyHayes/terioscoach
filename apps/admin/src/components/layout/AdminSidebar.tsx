"use client";

import {
  BarChart3,
  Calendar,
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
 * 264px fixed, surface-sunken with a right border. Fraunces wordmark,
 * micro section labels, 36px nav items (18px icon + label ink-muted),
 * active item: eucalyptus-100 fill, eucalyptus-800 text, weight 600,
 * 3px rounded primary bar on the left. Bottom: practitioner user card.
 * (Collapse-to-rail and the mobile Drawer variant are later tasks.)
 */

const NAV_ITEMS = [
  { label: "Overview", href: "/", icon: LayoutDashboard },
  { label: "Calendar", href: "#", icon: Calendar },
  { label: "Clients", href: "#", icon: Users },
  { label: "Services", href: "/services", icon: Sparkles },
  { label: "Payments", href: "#", icon: CreditCard },
  { label: "Content", href: "#", icon: FileText },
  { label: "Forms", href: "#", icon: ClipboardList },
  { label: "Enquiries", href: "#", icon: Mail },
  { label: "Reviews", href: "#", icon: Star },
  { label: "Reports", href: "#", icon: BarChart3 },
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
      className="sticky top-0 flex h-screen w-[264px] shrink-0 flex-col border-r border-border bg-surface-sunken"
    >
      <div className="px-5 py-5">
        <Link href="/" className="inline-block rounded-sm">
          <span className="font-display text-xl tracking-[-0.01em] text-ink">
            Terios
          </span>
        </Link>
        <p className="mt-1 text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
          Practice dashboard
        </p>
      </div>

      <p className="px-5 pt-4 pb-2 text-[11px] leading-[1.3] font-semibold tracking-[0.08em] uppercase text-ink-faint">
        Practice
      </p>

      <ul className="flex flex-1 flex-col gap-0.5 overflow-y-auto pb-4">
        {NAV_ITEMS.map(({ label, href, icon: Icon }) => {
          const active = href === "/" ? pathname === "/" : pathname === href;
          return (
            <li key={label}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "relative mx-3 flex h-9 items-center gap-3 rounded-md px-3 text-sm font-medium tracking-[0.005em] text-ink-muted transition-colors duration-fast ease-out hover:bg-surface-raised",
                  active && "bg-eucalyptus-100 font-semibold text-eucalyptus-800 hover:bg-eucalyptus-100",
                )}
              >
                {active ? (
                  <span
                    aria-hidden="true"
                    className="absolute top-1/2 left-0 h-5 w-[3px] -translate-y-1/2 rounded-full bg-primary"
                  />
                ) : null}
                <Icon size={18} aria-hidden="true" />
                {label}
              </Link>
            </li>
          );
        })}
      </ul>

      <div className="flex items-center gap-3 border-t border-border px-5 py-4">
        <span
          aria-hidden="true"
          className="flex size-8 shrink-0 items-center justify-center rounded-full bg-primary text-[13px] font-semibold text-on-primary"
        >
          {initials(userName)}
        </span>
        <div className="min-w-0">
          <p className="truncate text-sm leading-[1.55] text-ink">{userName}</p>
          <p className="text-[13px] leading-[1.45] font-medium tracking-[0.01em] text-ink-faint">
            Practitioner
          </p>
        </div>
      </div>
    </nav>
  );
}
