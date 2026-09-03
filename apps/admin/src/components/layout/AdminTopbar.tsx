"use client";
import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ChevronDown,
  CircleHelp,
  BookOpenCheck,
  ExternalLink,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import { NAV_ITEMS } from "./AdminSidebar";
import { Button } from "@/components/ui/Button";
import { PageHelpDialog } from "@/components/help/PageHelpDialog";
import { adminHelpForPath } from "@/lib/help";
import { AdminNotificationCenter } from "./AdminNotificationCenter";
import { PracticeSearch } from "./PracticeSearch";

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
  const [helpOpen, setHelpOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const current = NAV_ITEMS.filter((item) =>
    item.href === "/"
      ? pathname === "/"
      : pathname === item.href || pathname.startsWith(`${item.href}/`),
  ).sort((a, b) => b.href.length - a.href.length)[0];
  const helpTopic = adminHelpForPath(pathname);
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
              {current?.label ?? helpTopic.title}
            </p>
          </div>
        </div>
        <div className="hidden max-w-md flex-1 px-8 md:block">
          <PracticeSearch />
        </div>
        <div className="flex items-center gap-1">
          <AdminNotificationCenter />
          <button
            type="button"
            aria-label={`Help with ${helpTopic.title}`}
            onClick={() => setHelpOpen(true)}
            className="inline-flex rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink"
          >
            <CircleHelp size={18} />
          </button>
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
                  href="/guide"
                  className="flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink"
                >
                  <BookOpenCheck size={16} />
                  User guide
                </Link>
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
      <PageHelpDialog open={helpOpen} onClose={() => setHelpOpen(false)} topic={helpTopic} />
    </header>
  );
}
