"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { CalendarPlus, ChevronDown, ExternalLink, LogOut, Menu, PanelLeftClose, PanelLeftOpen, Settings } from "lucide-react";
import { PORTAL_NAV_ITEMS } from "./PortalSidebar";
import { cn } from "@/lib/cn";

export function PortalTopbar({ userName, userEmail, onSignOut, signingOut = false, collapsed = false, onToggleCollapse, onOpenMobileNav }: { userName: string; userEmail?: string; onSignOut: () => void; signingOut?: boolean; collapsed?: boolean; onToggleCollapse?: () => void; onOpenMobileNav?: () => void }) {
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const current = PORTAL_NAV_ITEMS.filter((item) => item.href === "/portal" ? pathname === item.href : pathname === item.href || pathname.startsWith(`${item.href}/`)).sort((a, b) => b.href.length - a.href.length)[0];

  useEffect(() => {
    const close = (event: MouseEvent) => { if (!menuRef.current?.contains(event.target as Node)) setOpen(false); };
    const escape = (event: KeyboardEvent) => { if (event.key === "Escape") setOpen(false); };
    document.addEventListener("mousedown", close); document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, []);

  return <header className="relative z-[40] shrink-0 border-b border-border/80 bg-surface-raised/95 backdrop-blur-xl">
    <div className="flex h-[72px] items-center justify-between gap-3 px-4 sm:px-6">
      <div className="flex min-w-0 items-center gap-1">
        <button type="button" onClick={() => { setOpen(false); onOpenMobileNav?.(); }} aria-label="Open navigation" className="rounded-xl p-2.5 text-ink hover:bg-surface-sunken lg:hidden"><Menu size={20}/></button>
        <button type="button" onClick={onToggleCollapse} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"} className="hidden rounded-xl p-2.5 text-ink hover:bg-surface-sunken lg:inline-flex">{collapsed ? <PanelLeftOpen size={19}/> : <PanelLeftClose size={19}/>}</button>
        <div className="ml-2 min-w-0"><p className="text-[10px] font-semibold uppercase tracking-[.12em] text-ink-faint">Client portal</p><p className="truncate text-sm font-semibold text-ink">{current?.label ?? "Terios"}</p></div>
      </div>
      <div className="flex items-center gap-1 sm:gap-2">
        <Link href="/portal/book" className="hidden h-10 items-center gap-2 rounded-xl bg-primary px-4 text-sm font-semibold text-on-primary shadow-sm transition-colors hover:bg-primary-hover sm:inline-flex"><CalendarPlus size={16}/>Book a session</Link>
        <a href={process.env.NEXT_PUBLIC_WEBSITE_URL ?? "https://terioscoach.com"} target="_blank" rel="noreferrer" aria-label="Visit Terios website" className="rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink"><ExternalLink size={18}/></a>
        <div className="relative" ref={menuRef}>
          <button type="button" aria-expanded={open} aria-haspopup="menu" onClick={() => setOpen((value) => !value)} className="ml-1 flex items-center gap-2 rounded-xl border border-border bg-surface-raised px-2 py-1.5 text-ink hover:bg-surface-sunken">
            <span className="flex size-8 items-center justify-center rounded-lg bg-eucalyptus-100 text-xs font-bold text-eucalyptus-800">{userName.slice(0, 1).toUpperCase()}</span>
            <span className="hidden max-w-32 truncate text-sm font-medium text-ink sm:block">{userName}</span><ChevronDown size={14} className={cn("text-ink-muted transition-transform", open && "rotate-180")}/>
          </button>
          {open ? <div role="menu" aria-label="Account" className="absolute right-0 z-[50] mt-2 w-[min(16rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-border bg-surface-raised p-2 text-ink shadow-lg">
            <div className="border-b border-border px-3 py-3"><p className="text-sm font-semibold text-ink">{userName}</p>{userEmail ? <p className="truncate text-xs text-ink-muted">{userEmail}</p> : null}</div>
            <Link role="menuitem" href="/portal/settings" onClick={() => setOpen(false)} className="mt-2 flex items-center gap-3 rounded-lg px-3 py-2 text-sm text-ink-muted hover:bg-surface-sunken hover:text-ink"><Settings size={16}/>Profile & preferences</Link>
            <button type="button" role="menuitem" disabled={signingOut} onClick={() => { setOpen(false); onSignOut(); }} className="mt-1 flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm text-danger hover:bg-danger-bg disabled:pointer-events-none disabled:opacity-50"><LogOut size={16}/>{signingOut ? "Signing out…" : "Sign out"}</button>
          </div> : null}
        </div>
      </div>
    </div>
  </header>;
}
