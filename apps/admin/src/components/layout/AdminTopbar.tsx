"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  Bell,
  ChevronDown,
  CircleHelp,
  ExternalLink,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import { NAV_ITEMS } from "./AdminSidebar";
import { Button } from "@/components/ui/Button";

export function AdminTopbar({
  userName,
  userRole = "Owner",
  onSignOut,
  signingOut = false,
  collapsed = false,
  onToggleCollapse,
  onOpenMobileNav,
}: {
  userName: string;
  userRole?: string;
  onSignOut: () => void;
  signingOut?: boolean;
  collapsed?: boolean;
  onToggleCollapse?: () => void;
  onOpenMobileNav?: () => void;
}) {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const current = NAV_ITEMS.filter((item) =>
    item.href === "/"
      ? pathname === "/"
      : pathname === item.href || pathname.startsWith(`${item.href}/`),
  ).sort((a, b) => b.href.length - a.href.length)[0];
  useEffect(() => {
    const close = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, []);
  return (
    <header className="relative z-[40] shrink-0 border-b border-border/80 bg-surface-raised/95 backdrop-blur-xl">
      <div className="flex h-[72px] items-center justify-between gap-3 px-4 sm:px-6">
        <div className="flex min-w-0 items-center gap-1">
          <button
            type="button"
            onClick={() => {
              setOpen(false);
              onOpenMobileNav?.();
            }}
            aria-label="Open navigation"
            className="rounded-xl p-2.5 text-ink hover:bg-surface-sunken lg:hidden"
          >
            <Menu size={20} />
          </button>
          <button
            type="button"
            onClick={onToggleCollapse}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className="hidden rounded-xl p-2.5 text-ink hover:bg-surface-sunken lg:inline-flex"
          >
            {collapsed ? (
              <PanelLeftOpen size={19} />
            ) : (
              <PanelLeftClose size={19} />
            )}
          </button>
          <div className="ml-2 min-w-0">
            <p className="text-[10px] font-semibold uppercase tracking-[.12em] text-ink-faint">
              Practice workspace
            </p>
            <p className="truncate text-sm font-semibold text-ink">
              {current?.label ?? "Terios"}
            </p>
          </div>
        </div>
        <div className="hidden max-w-md flex-1 px-8 md:block">
          <Link
            href="/clients"
            className="flex h-10 items-center gap-3 rounded-xl border border-border bg-surface-sunken px-3 text-sm text-ink-muted hover:border-border-strong"
          >
            <Search size={16} />
            <span className="flex-1">Find clients and records</span>
            <kbd className="rounded border border-border bg-surface-raised px-1.5 py-0.5 text-[10px]">
              ⌘K
            </kbd>
          </Link>
        </div>
        <div className="flex items-center gap-1">
          <Link
            href="/enquiries"
            aria-label="Enquiries and notifications"
            className="rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink"
          >
            <Bell size={18} />
          </Link>
          <Link
            href="/content"
            aria-label="Content help"
            className="hidden rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink sm:inline-flex"
          >
            <CircleHelp size={18} />
          </Link>
          <div className="relative" ref={menuRef}>
            <button
              type="button"
              aria-expanded={open}
              aria-haspopup="menu"
              onClick={() => setOpen((value) => !value)}
              className="ml-1 flex items-center gap-2 rounded-xl border border-border bg-surface-raised px-2 py-1.5 hover:bg-surface-sunken"
            >
              <span className="flex size-8 items-center justify-center rounded-lg bg-eucalyptus-100 text-xs font-bold text-eucalyptus-800">
                {userName.slice(0, 1).toUpperCase()}
              </span>
              <span className="hidden max-w-32 truncate text-sm font-medium text-ink sm:block">
                {userName}
              </span>
              <ChevronDown size={14} className="text-ink-muted" />
            </button>
            {open ? (
              <div
                role="menu"
                className="absolute right-0 z-[50] mt-2 w-[min(16rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-border bg-surface-raised p-2 shadow-lg"
              >
                <div className="border-b border-border px-3 py-3">
                  <p className="text-sm font-semibold text-ink">{userName}</p>
                  <p className="text-xs text-ink-muted">{userRole}</p>
                </div>
                <Link
                  role="menuitem"
                  href="/settings"
                  className="mt-2 flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink"
                >
                  <UserRound size={16} />
                  Profile & preferences
                </Link>
                <Link
                  role="menuitem"
                  href="/security"
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink"
                >
                  <ShieldCheck size={16} />
                  Security and MFA
                </Link>
                <Link
                  role="menuitem"
                  href="https://terioscoach.com"
                  target="_blank"
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink"
                >
                  <ExternalLink size={16} />
                  View website
                </Link>
                <div className="mt-1 border-t border-border pt-1">
                  <Button
                    variant="ghost"
                    size="sm"
                    className="w-full justify-start"
                    loading={signingOut}
                    onClick={onSignOut}
                  >
                    <UserRound size={16} />
                    Sign out
                  </Button>
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>
    </header>
  );
}
