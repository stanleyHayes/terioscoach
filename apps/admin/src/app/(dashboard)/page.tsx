"use client";

import {
  ArrowUpRight,
  Calendar,
  CalendarClock,
  CircleAlert,
  Mail,
  TrendingUp,
  UserPlus,
  Users,
} from "lucide-react";
import Link from "next/link";
import { EmptyState } from "@/components/content/states";
import { KpiStrip } from "@/components/insights/KpiStrip";
import { buttonClasses } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { useAuth } from "@/lib/auth";
import { formatMoney } from "@/lib/format";
import {
  currentMonthRange,
  peakIncome,
  reportsApi,
  type PracticeReport,
} from "@/lib/insights";
import { useResource } from "@/lib/use-resource";

export default function OverviewPage() {
  const { user } = useAuth();
  const canViewReports =
    user?.role === "practitioner" ||
    user?.permissions?.includes("reports.view");

  return (
    <div data-admin-page="overview" className="flex flex-col gap-8">
      <div className="flex flex-col justify-between gap-5 md:flex-row md:items-end">
        <div>
          <p className="text-xs font-semibold tracking-[0.08em] text-primary uppercase">
            Today at Terios
          </p>
          <h1 className="mt-2 font-display text-[2.25rem] leading-tight font-semibold tracking-[-0.025em] text-ink">
            Welcome back{user?.name ? `, ${user.name.split(" ")[0]}` : ""}
          </h1>
          <p className="mt-2 max-w-[54ch] text-sm leading-relaxed text-ink-muted">
            Your practice, clients, and care schedule in one calm workspace.
          </p>
        </div>
        <Link
          href="/calendar"
          className={buttonClasses({ className: "self-start" })}
        >
          Open calendar <ArrowUpRight size={16} aria-hidden="true" />
        </Link>
      </div>

      {canViewReports ? <OverviewSnapshot /> : null}

      <section
        aria-label="Practice shortcuts"
        className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {[
          {
            href: "/calendar",
            label: "Schedule",
            note: "View sessions",
            icon: Calendar,
          },
          {
            href: "/clients",
            label: "Clients",
            note: "Open client files",
            icon: Users,
          },
          {
            href: "/availability",
            label: "Availability",
            note: "Set your hours",
            icon: CalendarClock,
          },
          {
            href: "/enquiries",
            label: "Enquiries",
            note: "Review new messages",
            icon: Mail,
          },
        ].map(({ href, label, note, icon: Icon }) => (
          <Link
            key={href}
            href={href}
            className="terios-shortcut group rounded-[1.25rem] border border-border/80 bg-surface-raised/80 p-5 transition-[transform,border-color,background-color] duration-base hover:-translate-y-1 hover:border-primary/30 hover:bg-surface-raised active:scale-[.985]"
          >
            <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <Icon size={19} aria-hidden="true" />
            </span>
            <span className="mt-6 flex items-end justify-between gap-3">
              <span>
                <span className="block text-sm font-semibold text-ink">
                  {label}
                </span>
                <span className="mt-1 block text-xs text-ink-faint">
                  {note}
                </span>
              </span>
              <ArrowUpRight
                size={16}
                aria-hidden="true"
                className="text-ink-faint transition-transform group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-primary"
              />
            </span>
          </Link>
        ))}
      </section>
    </div>
  );
}

function OverviewSnapshot() {
  const range = currentMonthRange();
  const report = useResource<PracticeReport>(
    (session, callbacks) =>
      reportsApi.practice(session, callbacks, { ...range, granularity: "day" }),
    [range.from, range.to],
  );
  if (report.error)
    return (
      <Card>
        <div
          role="alert"
          className="flex items-center gap-3 text-sm text-ink-muted"
        >
          <CircleAlert size={18} className="text-danger-ink" />
          The monthly snapshot could not be loaded.
        </div>
      </Card>
    );
  if (!report.data)
    return (
      <div
        role="status"
        aria-label="Loading practice snapshot"
        className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {[0, 1, 2, 3].map((item) => (
          <span key={item} className="skeleton-shimmer h-28 rounded-2xl" />
        ))}
      </div>
    );
  const data = report.data;
  const currency = data.summary.currency || "GHS";
  return (
    <section aria-labelledby="snapshot-heading" className="flex flex-col gap-4">
      <div className="flex items-end justify-between gap-4">
        <div>
          <p className="text-[10px] font-semibold tracking-[.12em] text-primary uppercase">
            Live snapshot
          </p>
          <h2
            id="snapshot-heading"
            className="mt-1 font-display text-xl font-semibold text-ink"
          >
            This month at a glance
          </h2>
        </div>
        <Link href="/reports" className="text-sm font-semibold text-primary">
          Open full reports <ArrowUpRight size={14} className="inline" />
        </Link>
      </div>
      <KpiStrip
        items={[
          {
            label: "Income",
            value: formatMoney(data.summary.incomeKobo, currency),
            detail: `${formatMoney(data.summary.netKobo, currency)} net`,
            icon: <TrendingUp size={17} />,
          },
          {
            label: "Completed",
            value: String(data.summary.sessionsCompleted),
            detail: "sessions this month",
            icon: <Calendar size={17} />,
          },
          {
            label: "Coming up",
            value: String(data.summary.sessionsUpcoming),
            detail: "confirmed ahead",
            icon: <CalendarClock size={17} />,
          },
          {
            label: "New clients",
            value: String(data.summary.newClients),
            detail: "joined this month",
            icon: <UserPlus size={17} />,
          },
        ]}
      />
      {data.series.length > 0 ? (
        <SnapshotChart data={data} />
      ) : (
        <EmptyState
          compact
          icon={<TrendingUp size={25} />}
          title="Your trend will start here"
          body="Income and completed-session activity appears as the month develops."
        />
      )}
    </section>
  );
}

function SnapshotChart({ data }: { data: PracticeReport }) {
  const peak = peakIncome(data.series);
  return (
    <Card className="grid gap-6 p-5 lg:grid-cols-[13rem_1fr] lg:items-end">
      <div>
        <p className="text-[10px] font-semibold tracking-[.1em] text-ink-faint uppercase">
          Daily movement
        </p>
        <p className="mt-2 font-display text-3xl font-semibold tracking-[-.04em] text-ink">
          {data.series.reduce((sum, point) => sum + point.sessions, 0)}
        </p>
        <p className="mt-1 text-xs leading-relaxed text-ink-muted">
          sessions represented in this month&rsquo;s activity curve
        </p>
      </div>
      <div>
        <div className="flex h-32 items-end gap-1" aria-hidden="true">
          {data.series.map((point) => (
            <span
              key={point.start}
              className="group relative min-h-1 flex-1 rounded-t bg-primary/75 transition-colors hover:bg-primary"
              style={{
                height: `${Math.max(4, (point.incomeKobo / peak) * 100)}%`,
              }}
            />
          ))}
        </div>
        <div className="mt-2 flex justify-between border-t border-border pt-2 text-[10px] font-medium text-ink-faint">
          <span>
            {new Date(data.from).toLocaleDateString("en-GB", {
              day: "numeric",
              month: "short",
            })}
          </span>
          <span>
            {new Date(
              new Date(data.to).getTime() - 86400000,
            ).toLocaleDateString("en-GB", { day: "numeric", month: "short" })}
          </span>
        </div>
        <table className="sr-only">
          <caption>Daily practice activity</caption>
          <thead>
            <tr>
              <th>Date</th>
              <th>Sessions</th>
              <th>Income</th>
            </tr>
          </thead>
          <tbody>
            {data.series.map((point) => (
              <tr key={point.start}>
                <td>{point.start}</td>
                <td>{point.sessions}</td>
                <td>{point.incomeKobo}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}
