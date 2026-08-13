"use client";

import { ArrowUpRight, Calendar, CalendarClock, Mail, Users } from "lucide-react";
import Link from "next/link";
import { buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { useAuth } from "@/lib/auth";

export default function OverviewPage() {
  const { user } = useAuth();

  return (
    <div data-admin-page="overview" className="flex flex-col gap-8">
      <div className="flex flex-col justify-between gap-5 md:flex-row md:items-end">
        <div>
          <p className="text-xs font-semibold tracking-[0.08em] text-primary uppercase">Today at Terios</p>
          <h1 className="mt-2 font-display text-[2.25rem] leading-tight font-semibold tracking-[-0.025em] text-ink">
            Welcome back{user?.name ? `, ${user.name.split(" ")[0]}` : ""}
          </h1>
          <p className="mt-2 max-w-[54ch] text-sm leading-relaxed text-ink-muted">Your practice, clients, and care schedule in one calm workspace.</p>
        </div>
        <Link href="/calendar" className={buttonClasses({ className: "self-start" })}>
          Open calendar <ArrowUpRight size={16} aria-hidden="true" />
        </Link>
      </div>

      <section aria-label="Practice shortcuts" className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {[
          { href: "/calendar", label: "Schedule", note: "View sessions", icon: Calendar },
          { href: "/clients", label: "Clients", note: "Open client files", icon: Users },
          { href: "/availability", label: "Availability", note: "Set your hours", icon: CalendarClock },
          { href: "/enquiries", label: "Enquiries", note: "Review new messages", icon: Mail },
        ].map(({ href, label, note, icon: Icon }) => (
          <Link key={href} href={href} className="terios-shortcut group rounded-[1.25rem] border border-border/80 bg-surface-raised/80 p-5 transition-[transform,border-color,background-color] duration-base hover:-translate-y-1 hover:border-primary/30 hover:bg-surface-raised active:scale-[.985]">
            <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary"><Icon size={19} aria-hidden="true" /></span>
            <span className="mt-6 flex items-end justify-between gap-3">
              <span><span className="block text-sm font-semibold text-ink">{label}</span><span className="mt-1 block text-xs text-ink-faint">{note}</span></span>
              <ArrowUpRight size={16} aria-hidden="true" className="text-ink-faint transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-primary" />
            </span>
          </Link>
        ))}
      </section>

      {/* EmptyState per design-system §3.27 (admin variant: heading-md title) */}
      <Card className="relative flex min-h-80 justify-center overflow-hidden py-16">
        <div aria-hidden="true" className="absolute -right-20 -top-24 size-72 rounded-full bg-primary/8 blur-3xl" />
        <div className="flex max-w-[360px] flex-col items-center px-6 text-center">
          <span className="flex size-16 items-center justify-center rounded-full bg-surface-sunken">
            <Calendar size={32} aria-hidden="true" className="text-ink-faint" />
          </span>
          <h2 className="mt-6 text-lg leading-[1.35] font-semibold text-ink">
            A quiet schedule
          </h2>
          <p className="mt-2 text-sm leading-[1.55] text-ink-muted">
            Upcoming sessions will appear here. Until then, you can refine your availability or review client notes.
          </p>
        </div>
      </Card>
    </div>
  );
}
