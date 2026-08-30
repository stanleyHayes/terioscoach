"use client";

import {
  BarChart3,
  Calendar,
  CalendarClock,
  ChevronRight,
  ClipboardList,
  CreditCard,
  FileText,
  LayoutDashboard,
  Mail,
  ShieldCheck,
  Sparkles,
  Star,
  Users,
  UserRoundCog,
  X,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { cn } from "@/lib/cn";

export const NAV_GROUPS = [
  {
    id: "overview",
    label: null,
    icon: LayoutDashboard,
    defaultOpen: true,
    items: [{ label: "Overview", href: "/", icon: LayoutDashboard }],
  },
  {
    id: "practice",
    label: "Practice",
    icon: Calendar,
    defaultOpen: true,
    items: [
      { label: "Calendar", href: "/calendar", icon: Calendar },
      { label: "Availability", href: "/availability", icon: CalendarClock },
      { label: "Clients", href: "/clients", icon: Users },
      { label: "Services", href: "/services", icon: Sparkles },
    ],
  },
  {
    id: "publishing",
    label: "Publishing",
    icon: FileText,
    defaultOpen: true,
    items: [
      { label: "Content", href: "/content", icon: FileText },
      { label: "Forms", href: "/forms", icon: ClipboardList },
      { label: "Enquiries", href: "/enquiries", icon: Mail },
      { label: "Reviews", href: "/reviews", icon: Star },
    ],
  },
  {
    id: "operations",
    label: "Operations",
    icon: BarChart3,
    defaultOpen: false,
    items: [
      { label: "Payments", href: "/payments", icon: CreditCard },
      { label: "Reports", href: "/reports", icon: BarChart3 },
      { label: "Team & access", href: "/team", icon: UserRoundCog },
      { label: "Security", href: "/security", icon: ShieldCheck },
    ],
  },
] as const;
export interface NavItem {
  label: string;
  href: string;
  icon: LucideIcon;
}
export const NAV_ITEMS: readonly NavItem[] = NAV_GROUPS.flatMap((group) => [
  ...group.items,
]) as NavItem[];
const STORAGE_KEY = "terios-admin-nav-groups";

function initials(name: string) {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]!.toUpperCase())
    .join("");
}

interface AdminSidebarProps {
  userName: string;
  userRole?: string;
  collapsed?: boolean;
  mobileOpen?: boolean;
  onMobileClose?: () => void;
  onRequestExpand?: () => void;
  permissions?: string[];
  owner?: boolean;
}

const NAV_PERMISSION: Record<string, string> = {
  "/": "dashboard.view",
  "/calendar": "schedule.manage",
  "/availability": "schedule.manage",
  "/clients": "clients.manage",
  "/services": "services.manage",
  "/content": "content.manage",
  "/forms": "forms.manage",
  "/enquiries": "enquiries.manage",
  "/reviews": "reviews.manage",
  "/payments": "payments.manage",
  "/reports": "reports.view",
  "/team": "team.manage",
};

export function AdminSidebar({
  userName,
  userRole = "Practitioner",
  collapsed = false,
  mobileOpen = false,
  onMobileClose,
  onRequestExpand,
  permissions = [],
  owner = true,
}: AdminSidebarProps) {
  const pathname = usePathname();
  const defaults = useMemo(
    () =>
      NAV_GROUPS.filter((group) => group.defaultOpen).map((group) => group.id),
    [],
  );
  const [openGroups, setOpenGroups] = useState<string[]>(() => {
    if (typeof window === "undefined") return defaults;
    try {
      const value = localStorage.getItem(STORAGE_KEY);
      return value ? JSON.parse(value) : defaults;
    } catch {
      return defaults;
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(openGroups));
    } catch {}
  }, [openGroups]);
  const active = (href: string) =>
    href === "/"
      ? pathname === "/"
      : pathname === href || pathname.startsWith(`${href}/`);
  const compact = collapsed && !mobileOpen;

  return (
    <>
      {mobileOpen ? (
        <button
          type="button"
          aria-label="Close navigation"
          onClick={onMobileClose}
          className="fixed inset-0 z-[60] bg-overlay lg:hidden"
        />
      ) : null}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-[70] flex h-dvh w-[min(264px,calc(100vw-2rem))] shrink-0 flex-col overflow-hidden border-r border-eucalyptus-800 bg-eucalyptus-900 text-sand-0 shadow-2xl transition-[width,transform] duration-base lg:relative lg:inset-auto lg:z-auto lg:w-[264px] lg:shadow-none",
          compact && "lg:w-[76px]",
          mobileOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0",
        )}
        aria-label="Practice navigation"
      >
        <div className="flex h-[72px] shrink-0 items-center justify-between border-b border-sand-0/10 px-5">
          <Link href="/" className="flex min-w-0 items-center gap-3">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-sand-0 font-display font-semibold text-eucalyptus-900">
              T
            </span>
            {!compact ? (
              <span>
                <span className="block font-display text-xl font-semibold">
                  Terios
                </span>
                <span className="block text-[10px] uppercase tracking-[.12em] text-eucalyptus-300">
                  Care workspace
                </span>
              </span>
            ) : null}
          </Link>
          {mobileOpen ? (
            <button
              type="button"
              onClick={onMobileClose}
              aria-label="Close navigation panel"
              className="rounded-lg p-2 text-eucalyptus-200 hover:bg-white/10"
            >
              <X size={18} />
            </button>
          ) : null}
        </div>
        <nav className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden px-3 py-4">
          {NAV_GROUPS.map((group) => {
            const visibleItems = group.items.filter(
              (item) =>
                owner ||
                item.href === "/security" ||
                permissions.includes(
                  NAV_PERMISSION[item.href] ?? "dashboard.view",
                ),
            );
            if (visibleItems.length === 0) return null;
            const hasActive = visibleItems.some((item) => active(item.href));
            const expanded = openGroups.includes(group.id) || hasActive;
            if (group.label === null)
              return (
                <div key={group.id} className="mb-3">
                  {visibleItems.map((item) => (
                    <NavLink
                      key={item.href}
                      item={item}
                      active={active(item.href)}
                      compact={compact}
                      onNavigate={onMobileClose}
                    />
                  ))}
                </div>
              );
            return (
              <section key={group.id} className="mb-2">
                <button
                  type="button"
                  aria-expanded={compact ? undefined : expanded}
                  onClick={() => {
                    if (compact) {
                      onRequestExpand?.();
                      return;
                    }
                    setOpenGroups((current) =>
                      current.includes(group.id)
                        ? current.filter((id) => id !== group.id)
                        : [...current, group.id],
                    );
                  }}
                  title={
                    compact ? `${group.label} — expand sidebar` : undefined
                  }
                  className={cn(
                    "flex h-10 w-full items-center rounded-xl px-3 text-eucalyptus-300 transition-colors hover:bg-white/8 hover:text-sand-0",
                    compact ? "justify-center" : "gap-3",
                    hasActive && "text-sand-0",
                  )}
                >
                  <group.icon size={17} className="shrink-0" />
                  {!compact ? (
                    <>
                      <span className="flex-1 text-left text-xs font-semibold uppercase tracking-[.08em]">
                        {group.label}
                      </span>
                      <ChevronRight
                        size={14}
                        className={cn(
                          "transition-transform",
                          expanded && "rotate-90",
                        )}
                      />
                    </>
                  ) : (
                    <span className="sr-only">Expand {group.label}</span>
                  )}
                </button>
                <div
                  className={cn(
                    "grid transition-[grid-template-rows] duration-base",
                    expanded && !compact
                      ? "grid-rows-[1fr]"
                      : "grid-rows-[0fr]",
                  )}
                >
                  <div className="overflow-hidden">
                    <div className="relative ml-5 border-l border-eucalyptus-700 py-1 pl-3">
                      {visibleItems.map((item) => (
                        <NavLink
                          key={item.href}
                          item={item}
                          active={active(item.href)}
                          compact={false}
                          onNavigate={onMobileClose}
                          nested
                        />
                      ))}
                    </div>
                  </div>
                </div>
              </section>
            );
          })}
        </nav>
        <div
          className={cn(
            "m-3 flex items-center rounded-2xl border border-sand-0/10 bg-sand-0/6 p-3",
            compact ? "justify-center" : "gap-3",
          )}
        >
          <span className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-clay-300 text-xs font-semibold text-eucalyptus-900">
            {initials(userName)}
          </span>
          {!compact ? (
            <div className="min-w-0">
              <p className="truncate text-sm">{userName}</p>
              <p className="truncate text-xs text-eucalyptus-300">{userRole}</p>
            </div>
          ) : null}
        </div>
      </aside>
    </>
  );
}

function NavLink({
  item,
  active,
  compact,
  nested = false,
  onNavigate,
}: {
  item: NavItem;
  active: boolean;
  compact: boolean;
  nested?: boolean;
  onNavigate?: () => void;
}) {
  return (
    <Link
      href={item.href}
      onClick={onNavigate}
      aria-current={active ? "page" : undefined}
      title={compact ? item.label : undefined}
      className={cn(
        "relative flex h-10 items-center rounded-xl text-sm font-medium transition-colors",
        compact ? "justify-center px-2" : "gap-3 px-3",
        active
          ? "bg-sand-0 font-semibold text-eucalyptus-900"
          : "text-eucalyptus-200 hover:bg-sand-0/8 hover:text-sand-0",
        nested &&
          "before:absolute before:-left-[13px] before:top-1/2 before:h-px before:w-3 before:bg-eucalyptus-700",
      )}
    >
      <item.icon size={17} className="shrink-0" />
      {!compact ? (
        <span>{item.label}</span>
      ) : (
        <span className="sr-only">{item.label}</span>
      )}
    </Link>
  );
}
