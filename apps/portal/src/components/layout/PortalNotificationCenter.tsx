"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Bell, CalendarClock, ClipboardList, CreditCard, RefreshCw, Star } from "lucide-react";
import { myBookings } from "@/lib/bookings";
import { buildPortalNotifications, type NotificationTone } from "@/lib/notifications";
import { formsApi, paymentsApi, reviewsApi } from "@/lib/portal";
import { usePortalData } from "@/lib/use-portal-data";
import { cn } from "@/lib/cn";

const toneClass: Record<NotificationTone, string> = {
  attention: "bg-clay-100 text-clay-700",
  care: "bg-eucalyptus-100 text-eucalyptus-800",
  neutral: "bg-sand-100 text-ink-muted",
};

export function PortalNotificationCenter() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const resource = usePortalData(async (session, callbacks) => {
    const [bookings, forms, payments, reviews] = await Promise.all([
      myBookings(session, callbacks),
      formsApi.listMine(session, callbacks),
      paymentsApi.listMine(session, callbacks),
      reviewsApi.listMine(session, callbacks),
    ]);
    return buildPortalNotifications({ bookings, forms, payments, reviews });
  }, []);

  useEffect(() => {
    function close(event: MouseEvent) { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); }
    function escape(event: KeyboardEvent) { if (event.key === "Escape") setOpen(false); }
    document.addEventListener("mousedown", close); document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, []);
  useEffect(() => {
    const timer = window.setInterval(resource.refresh, 60_000);
    window.addEventListener("focus", resource.refresh);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", resource.refresh); };
  }, [resource.refresh]);

  const items = resource.data ?? [];
  const count = items.length;
  return <div className="relative" ref={rootRef}>
    <button type="button" aria-label={count ? `Notifications, ${count} ${count === 1 ? "item needs" : "items need"} attention` : "Notifications"} aria-expanded={open} aria-haspopup="dialog" onClick={() => setOpen((value) => !value)} className="terios-icon-button relative rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink">
      <Bell size={18}/>{count ? <span className="absolute right-1 top-1 flex min-w-4 -translate-y-1/2 translate-x-1/2 items-center justify-center rounded-full border-2 border-surface-raised bg-clay-300 px-1 text-[9px] font-bold leading-3 text-eucalyptus-950">{count > 9 ? "9+" : count}</span> : null}
    </button>
    {open ? <section role="dialog" aria-label="Client notifications" className="terios-popover absolute right-0 z-[55] mt-2 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-3xl border border-border bg-surface-raised text-ink shadow-2xl">
      <header className="flex items-start justify-between border-b border-border px-5 py-4"><div><p className="text-sm font-semibold">Your care updates</p><p className="mt-0.5 text-xs text-ink-muted">The next useful steps in your care journey</p></div><button type="button" onClick={resource.refresh} aria-label="Refresh notifications" className="terios-icon-button rounded-lg p-2 text-ink-muted hover:bg-surface-sunken hover:text-ink"><RefreshCw size={15}/></button></header>
      <div className="max-h-[min(31rem,70vh)] overflow-y-auto p-2">
        {resource.error ? <div className="px-4 py-8 text-center"><p className="text-sm font-medium">Updates could not be refreshed</p><button type="button" onClick={resource.refresh} className="mt-2 text-xs font-semibold text-primary">Try again</button></div>
        : resource.data === null ? <div className="space-y-2 p-2" aria-label="Loading notifications">{[0,1,2].map((item) => <div key={item} className="h-20 animate-pulse rounded-2xl bg-surface-sunken"/>)}</div>
        : items.length ? items.map((item) => {
          const Icon = item.id.startsWith("form-") ? ClipboardList : item.id.startsWith("payment-") ? CreditCard : item.id.startsWith("booking-") ? CalendarClock : Star;
          return <Link key={item.id} href={item.href} onClick={() => setOpen(false)} className="group flex gap-3 rounded-2xl p-3 transition-colors hover:bg-surface-sunken"><span className={cn("flex size-10 shrink-0 items-center justify-center rounded-xl", toneClass[item.tone])}><Icon size={17}/></span><span className="min-w-0 flex-1"><span className="block text-sm font-semibold text-ink">{item.title}</span><span className="mt-0.5 block text-xs leading-relaxed text-ink-muted">{item.description}</span></span></Link>;
        }) : <div className="px-5 py-10 text-center"><span className="mx-auto flex size-12 items-center justify-center rounded-2xl bg-eucalyptus-100 text-eucalyptus-800"><Bell size={20}/></span><p className="mt-3 text-sm font-semibold">You’re all caught up</p><p className="mt-1 text-xs leading-relaxed text-ink-muted">Forms, payments, session reminders, and review prompts will appear here.</p></div>}
      </div><footer className="border-t border-border px-4 py-3 text-center text-[11px] text-ink-faint">Updates reflect your live care records</footer>
    </section> : null}
  </div>;
}
