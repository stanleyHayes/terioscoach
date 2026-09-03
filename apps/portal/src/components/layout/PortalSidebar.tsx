"use client";

import {
  CalendarPlus,
  ChevronRight,
  ClipboardList,
  CreditCard,
  FileText,
  LayoutDashboard,
  Settings,
  Star,
  Stethoscope,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import Image from "next/image";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/cn";

export interface PortalNavItem {
  label: string;
  href: string;
  icon: LucideIcon;
}

export const PORTAL_NAV_GROUPS = [
  {
    id: "overview",
    label: null,
    defaultOpen: true,
    items: [{ label: "Overview", href: "/portal", icon: LayoutDashboard }],
  },
  {
    id: "care",
    label: "Your care",
    defaultOpen: true,
    items: [
      { label: "Book a session", href: "/portal/book", icon: CalendarPlus },
      { label: "Consultations", href: "/portal/sessions", icon: Stethoscope },
      { label: "Forms", href: "/portal/forms", icon: ClipboardList },
    ],
  },
  {
    id: "records",
    label: "Care records",
    defaultOpen: true,
    items: [
      { label: "Documents", href: "/portal/documents", icon: FileText },
      { label: "Payments", href: "/portal/payments", icon: CreditCard },
      { label: "Reviews", href: "/portal/reviews", icon: Star },
    ],
  },
  {
    id: "account",
    label: "Account",
    defaultOpen: false,
    items: [{ label: "Profile & preferences", href: "/portal/settings", icon: Settings }],
  },
] as const;

export const PORTAL_NAV_ITEMS: readonly PortalNavItem[] = PORTAL_NAV_GROUPS.flatMap(
  (group) => [...group.items],
) as PortalNavItem[];

const STORAGE_KEY = "terios-portal-nav-groups";

function initials(name: string) {
  return name.trim().split(/\s+/).filter(Boolean).slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "").join("") || "·";
}

export function PortalSidebar({
  userName,
  userEmail,
  collapsed = false,
  mobileOpen = false,
  onMobileClose,
  onRequestExpand,
}: {
  userName: string;
  userEmail?: string;
  collapsed?: boolean;
  mobileOpen?: boolean;
  onMobileClose?: () => void;
  onRequestExpand?: () => void;
}) {
  const pathname = usePathname();
  const defaults = useMemo(
    () => PORTAL_NAV_GROUPS.filter((group) => group.defaultOpen).map((group) => group.id),
    [],
  );
  const [openGroups, setOpenGroups] = useState<string[]>(() => {
    if (typeof window === "undefined") return defaults;
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      return saved ? JSON.parse(saved) : defaults;
    } catch {
      return defaults;
    }
  });

  useEffect(() => {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(openGroups)); } catch {}
  }, [openGroups]);

  const active = (href: string) => href === "/portal"
    ? pathname === href
    : pathname === href || pathname.startsWith(`${href}/`);
  const compact = collapsed && !mobileOpen;

  return (
    <>
      {mobileOpen ? (
        <button type="button" aria-label="Close navigation" onClick={onMobileClose}
          className="fixed inset-0 z-[60] bg-overlay lg:hidden" />
      ) : null}
      <aside
        aria-label="Client portal navigation"
        className={cn(
          "fixed inset-y-0 left-0 z-[70] flex h-dvh w-[min(264px,calc(100vw-2rem))] shrink-0 flex-col overflow-hidden border-r border-eucalyptus-800 bg-eucalyptus-900 text-sand-0 shadow-2xl transition-[width,transform] duration-base lg:relative lg:inset-auto lg:z-auto lg:w-[264px] lg:shadow-none",
          compact && "lg:w-[76px]",
          mobileOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0",
        )}
      >
        <div className="flex h-[72px] shrink-0 items-center justify-between border-b border-sand-0/10 px-5">
          <Link href="/portal" className="flex min-w-0 items-center gap-3" onClick={onMobileClose}>
            <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-sand-0 p-1.5">
              <Image src="/images/brand/identity/terios-mark.svg" alt="" width={24} height={36} className="h-full w-auto" priority />
            </span>
            {!compact ? <span><span className="block font-display text-xl font-semibold text-sand-0">Terios</span><span className="block text-[10px] uppercase tracking-[.12em] text-eucalyptus-300">Client portal</span></span> : null}
          </Link>
          {mobileOpen ? <button type="button" onClick={onMobileClose} aria-label="Close navigation panel" className="rounded-lg p-2 text-eucalyptus-200 hover:bg-sand-0/10 hover:text-sand-0"><X size={18}/></button> : null}
        </div>

        <nav className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-3 py-4">
          {PORTAL_NAV_GROUPS.map((group) => {
            const hasActive = group.items.some((item) => active(item.href));
            const expanded = openGroups.includes(group.id) || hasActive;
            const GroupIcon = group.items[0].icon;
            if (group.label === null) return <div key={group.id} className="mb-3">{group.items.map((item) => <PortalNavLink key={item.href} item={item} active={active(item.href)} compact={compact} onNavigate={onMobileClose}/>)}</div>;
            return (
              <section key={group.id} className="mb-2">
                <button type="button" aria-expanded={compact ? undefined : expanded}
                  onClick={() => compact ? onRequestExpand?.() : setOpenGroups((current) => current.includes(group.id) ? current.filter((id) => id !== group.id) : [...current, group.id])}
                  title={compact ? `${group.label} — expand sidebar` : undefined}
                  className={cn("flex h-10 w-full items-center rounded-xl px-3 text-eucalyptus-300 transition-colors hover:bg-sand-0/8 hover:text-sand-0", compact ? "justify-center" : "gap-3", hasActive && "text-sand-0")}
                >
                  <GroupIcon size={17} className="shrink-0"/>
                  {!compact ? <><span className="flex-1 text-left text-xs font-semibold uppercase tracking-[.08em]">{group.label}</span><ChevronRight size={14} className={cn("transition-transform", expanded && "rotate-90")}/></> : <span className="sr-only">Expand {group.label}</span>}
                </button>
                <div className={cn("grid transition-[grid-template-rows] duration-base", expanded && !compact ? "grid-rows-[1fr]" : "grid-rows-[0fr]")}>
                  <div className="overflow-hidden"><div className="relative ml-5 border-l border-eucalyptus-700 py-1 pl-3">
                    {group.items.map((item) => <PortalNavLink key={item.href} item={item} active={active(item.href)} compact={false} nested onNavigate={onMobileClose}/>) }
                  </div></div>
                </div>
              </section>
            );
          })}
        </nav>

        <div className={cn("m-3 flex items-center rounded-2xl border border-sand-0/10 bg-sand-0/6 p-3", compact ? "justify-center" : "gap-3")}>
          <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-clay-300 text-xs font-semibold text-eucalyptus-900">{initials(userName)}</span>
          {!compact ? <div className="min-w-0"><p className="truncate text-sm text-sand-0">{userName}</p><p className="truncate text-xs text-eucalyptus-300">{userEmail ?? "Client"}</p></div> : null}
        </div>
      </aside>
    </>
  );
}

function PortalNavLink({ item, active, compact, nested = false, onNavigate }: { item: PortalNavItem; active: boolean; compact: boolean; nested?: boolean; onNavigate?: () => void }) {
  return <Link href={item.href} onClick={onNavigate} aria-current={active ? "page" : undefined} title={compact ? item.label : undefined}
    className={cn("relative flex h-10 items-center rounded-xl text-sm font-medium transition-[background-color,color,transform] duration-fast active:scale-[.98]", compact ? "justify-center px-2" : "gap-3 px-3", active ? "bg-sand-0 font-semibold text-eucalyptus-900" : "text-eucalyptus-200 hover:bg-sand-0/8 hover:text-sand-0", nested && "before:absolute before:-left-[13px] before:top-1/2 before:h-px before:w-3 before:bg-eucalyptus-700")}>
    <item.icon size={17} className="shrink-0"/>{!compact ? <span>{item.label}</span> : <span className="sr-only">{item.label}</span>}
  </Link>;
}
