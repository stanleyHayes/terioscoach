"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Bell, CheckCheck, RefreshCw, X } from "lucide-react";
import { notificationFeedApi, notificationHref, type FeedNotification } from "@/lib/notification-feed";
import { usePortalData, usePortalAction } from "@/lib/use-portal-data";

export function PortalNotificationCenter() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const resource = usePortalData(notificationFeedApi.list, []);
  const action = usePortalAction();
  const items = resource.data?.items ?? [];
  const count = resource.data?.unreadCount ?? 0;

  useEffect(() => {
    function close(event: MouseEvent) { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); }
    function escape(event: KeyboardEvent) { if (event.key === "Escape" && open) { setOpen(false); triggerRef.current?.focus(); } }
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", escape);
    return () => { document.removeEventListener("mousedown", close); document.removeEventListener("keydown", escape); };
  }, [open]);
  useEffect(() => {
    const timer = window.setInterval(resource.refresh, 30_000);
    window.addEventListener("focus", resource.refresh);
    return () => { window.clearInterval(timer); window.removeEventListener("focus", resource.refresh); };
  }, [resource.refresh]);

  async function read(item?: FeedNotification) {
    if (action.pending || (item && item.read)) return;
    const saved = await action.run(item?.id ?? "all", async (session, callbacks) => {
      if (item) await notificationFeedApi.markRead(session, callbacks, item.id);
      else await notificationFeedApi.markAllRead(session, callbacks);
      return true;
    });
    if (saved) resource.refresh();
  }

  return <div className="sm:relative" ref={rootRef}>
    <button ref={triggerRef} type="button" aria-label={count ? `Notifications, ${count} unread` : "Notifications"} aria-expanded={open} aria-haspopup="dialog" onClick={() => { setOpen(!open); if (!open) resource.refresh(); }} className="terios-icon-button relative rounded-xl p-2.5 text-ink-muted hover:bg-surface-sunken hover:text-ink">
      <Bell size={18} aria-hidden="true" />
      {count > 0 ? <span className="absolute right-1 top-1 flex min-w-4 -translate-y-1/2 translate-x-1/2 items-center justify-center rounded-full border-2 border-surface-raised bg-clay-300 px-1 text-[9px] font-bold leading-3 text-eucalyptus-950">{count > 9 ? "9+" : count}</span> : null}
    </button>
    {open ? <section role="dialog" aria-label="Client notifications" className="terios-popover absolute inset-x-3 top-full z-[55] mt-2 w-auto sm:inset-x-auto sm:right-0 sm:top-auto sm:w-96 overflow-hidden rounded-3xl border border-border bg-surface-raised text-ink shadow-2xl">
      <header className="flex items-start justify-between gap-2 border-b border-border px-5 py-4">
        <div><p className="text-sm font-semibold">Client notifications</p><p className="mt-0.5 text-xs text-ink-muted">{count ? `${count} unread updates` : "Your recent updates"}</p></div>
        <div className="flex">
          <button type="button" onClick={resource.refresh} aria-label="Refresh notifications" className="terios-icon-button rounded-lg p-2"><RefreshCw size={15} /></button>
          <button type="button" onClick={() => { setOpen(false); triggerRef.current?.focus(); }} aria-label="Close notifications" className="terios-icon-button rounded-lg p-2"><X size={15} /></button>
        </div>
      </header>
      {count > 0 ? <button type="button" disabled={Boolean(action.pending)} onClick={() => void read()} className="flex w-full items-center justify-end gap-2 border-b border-border px-5 py-3 text-xs font-semibold text-primary disabled:opacity-50"><CheckCheck size={14} />Mark all as read</button> : null}
      {action.error ? <p role="alert" className="px-5 py-3 text-sm text-danger-ink">{action.error}</p> : null}
      <div className="max-h-[min(31rem,60vh)] overflow-y-auto p-2">
        {resource.error ? <div role="alert" className="px-4 py-6 text-center"><p className="text-sm">Notifications could not be refreshed</p><button type="button" onClick={resource.refresh} className="mt-2 text-xs font-semibold text-primary">Try again</button></div> : null}
        {resource.data === null && !resource.error ? <div className="space-y-2 p-2" aria-label="Loading notifications">{[0,1,2].map((item) => <div key={item} className="h-20 animate-pulse rounded-2xl bg-surface-sunken" />)}</div> : null}
        {items.length ? <ul>{items.map((item) => {
          const href = notificationHref(item.link);
          const contents = <><span className="block text-sm font-semibold text-ink">{item.title}</span>{item.body ? <span className="mt-1 block text-xs leading-relaxed text-ink-muted">{item.body}</span> : null}<time dateTime={item.createdAt} className="mt-2 block text-[11px] text-ink-faint">{new Date(item.createdAt).toLocaleString()}</time></>;
          return <li key={item.id} className={`mb-1 rounded-2xl p-3 ${item.read ? "" : "bg-primary/5"}`}>
            <div className="flex items-start gap-2">
              {!item.read ? <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary" aria-label="Unread" /> : null}
              {href ? <Link href={href} onClick={() => { void read(item); setOpen(false); }} className="min-w-0 flex-1 break-words hover:underline">{contents}</Link> : <div className="min-w-0 flex-1 break-words">{contents}</div>}
            </div>
            {!item.read ? <button type="button" disabled={Boolean(action.pending)} onClick={() => void read(item)} className="mt-2 text-xs font-semibold text-primary disabled:opacity-50" aria-label={`Mark as read: ${item.title}`}>Mark as read</button> : null}
          </li>;
        })}</ul> : resource.data && !resource.error ? <div className="px-5 py-10 text-center"><Bell size={24} className="mx-auto text-primary" /><p className="mt-3 text-sm font-semibold">You’re all caught up</p><p className="mt-1 text-xs text-ink-muted">New activity will appear here.</p></div> : null}
      </div>
      <footer className="border-t border-border px-4 py-3 text-center text-[11px] text-ink-faint">Your latest 50 updates · Read status is saved</footer>
    </section> : null}
  </div>;
}
