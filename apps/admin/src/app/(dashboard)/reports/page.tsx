"use client";

import { BarChart3, ChartNoAxesCombined } from "lucide-react";
import { EmptyState } from "@/components/content/states";
import { useState } from "react";
import { Button } from "@/components/ui/Button";
import { cn } from "@/lib/cn";
import { formatMoney } from "@/lib/format";
import {
  currentMonthRange,
  peakIncome,
  reportsApi,
  shiftMonthRange,
  type Granularity,
  type PracticeReport,
} from "@/lib/insights";
import { useResource } from "@/lib/use-resource";

/**
 * Reporting dashboard (ADM-11).
 *
 * Every number on this screen comes from one API call, so the headline
 * figures, the per-service table and the chart can never disagree with each
 * other — which is exactly what happens when a dashboard assembles itself
 * from several independent queries.
 *
 * The chart is drawn with plain elements rather than a charting library:
 * it is a bar per period, it must match the brand tokens, and a dependency
 * that ships its own canvas and colours would be harder to keep in line
 * than the twenty lines it replaces.
 */
export default function ReportsPage() {
  const [range, setRange] = useState(() => currentMonthRange());
  const [granularity, setGranularity] = useState<Granularity>("day");

  const report = useResource<PracticeReport>(
    (session, callbacks) =>
      reportsApi.practice(session, callbacks, { ...range, granularity }),
    [range.from, range.to, granularity],
  );

  const data = report.data;
  const hasSeriesActivity = data?.series.some(
    (bucket) => bucket.sessions > 0 || bucket.incomeKobo > 0,
  );
  const monthLabel = new Date(range.from).toLocaleDateString("en-GB", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });

  return (
    <div data-admin-page="reports" className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
            Reports
          </h1>
          <p className="mt-1.5 text-sm text-ink-muted">{monthLabel}</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setRange(shiftMonthRange(range, -1))}
          >
            Previous
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setRange(currentMonthRange())}
          >
            This month
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setRange(shiftMonthRange(range, 1))}
          >
            Next
          </Button>
          <div
            className="ml-2 flex gap-1"
            role="group"
            aria-label="Chart grouping"
          >
            {(["day", "week", "month"] as const).map((value) => (
              <button
                key={value}
                type="button"
                aria-pressed={granularity === value}
                onClick={() => setGranularity(value)}
                className={cn(
                  "rounded-full px-3 py-1.5 text-[13px] font-medium capitalize transition-colors duration-instant ease-out",
                  granularity === value
                    ? "bg-primary text-on-primary"
                    : "bg-surface-sunken text-ink-muted hover:text-ink",
                )}
              >
                {value}
              </button>
            ))}
          </div>
        </div>
      </header>

      {report.error ? (
        <div
          role="alert"
          className="rounded-lg border border-border bg-surface-raised p-8 text-center"
        >
          <p className="text-sm text-ink-muted">{report.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={report.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-4">
          <span className="sr-only">Loading the report…</span>
          <span
            aria-hidden="true"
            className="h-24 rounded-lg bg-surface-sunken"
          />
          <span
            aria-hidden="true"
            className="h-64 rounded-lg bg-surface-sunken"
          />
        </div>
      ) : (
        <>
          <dl className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Stat
              label="Sessions"
              value={String(data.summary.sessionsCompleted)}
            />
            <Stat
              label="Coming up"
              value={String(data.summary.sessionsUpcoming)}
            />
            <Stat
              label="Taken"
              value={formatMoney(
                data.summary.incomeKobo,
                data.summary.currency || "GHS",
              )}
            />
            <Stat label="New clients" value={String(data.summary.newClients)} />
          </dl>

          <dl className="grid gap-4 sm:grid-cols-3">
            <Stat
              label="Refunded"
              value={formatMoney(
                data.summary.refundedKobo,
                data.summary.currency || "GHS",
              )}
              muted
            />
            <Stat
              label="Cancellations"
              value={String(data.summary.cancellations)}
              muted
            />
            <Stat label="No shows" value={String(data.summary.noShows)} muted />
          </dl>

          <section
            aria-labelledby="income-chart-heading"
            className="rounded-lg border border-border bg-surface-raised p-5"
          >
            <h2
              id="income-chart-heading"
              className="text-base font-semibold text-ink"
            >
              Income by {granularity}
            </h2>

            {!hasSeriesActivity ? (
              <EmptyState
                compact
                className="mt-4 border-0 bg-transparent"
                icon={<ChartNoAxesCombined size={24} />}
                title="Nothing in this period"
                body="Choose another reporting period to compare income and sessions."
              />
            ) : (
              <>
                {/* The table is the accessible truth; the bars are a visual
                    reading of the same numbers. */}
                <ul
                  className="mt-6 flex h-48 items-end gap-1"
                  aria-hidden="true"
                >
                  {data.series.map((bucket) => (
                    <li
                      key={bucket.start}
                      className="flex-1 rounded-t-sm bg-primary/80"
                      style={{
                        height: `${Math.max(2, (bucket.incomeKobo / peakIncome(data.series)) * 100)}%`,
                      }}
                    />
                  ))}
                </ul>
                <table className="sr-only">
                  <caption>Income and sessions by {granularity}</caption>
                  <thead>
                    <tr>
                      <th scope="col">Period</th>
                      <th scope="col">Sessions</th>
                      <th scope="col">Income</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.series.map((bucket) => (
                      <tr key={bucket.start}>
                        <td>
                          {new Date(bucket.start).toLocaleDateString("en-GB")}
                        </td>
                        <td>{bucket.sessions}</td>
                        <td>
                          {formatMoney(
                            bucket.incomeKobo,
                            data.summary.currency || "GHS",
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </section>

          <section
            aria-labelledby="by-service-heading"
            className="overflow-hidden rounded-lg border border-border bg-surface-raised"
          >
            <h2
              id="by-service-heading"
              className="px-5 py-4 text-base font-semibold text-ink"
            >
              By service
            </h2>
            {data.byService.length === 0 ? (
              <EmptyState
                compact
                className="m-4 border-0 bg-surface-sunken"
                icon={<BarChart3 size={24} />}
                title="No sessions in this period"
                body="Service totals appear after a completed or paid session."
              />
            ) : (
              <table className="w-full border-collapse text-left">
                <thead>
                  <tr className="border-y border-border">
                    {["Service", "Sessions", "Income"].map((heading) => (
                      <th
                        key={heading}
                        scope="col"
                        className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                      >
                        {heading}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {data.byService.map((row) => (
                    <tr key={row.serviceId}>
                      <td className="px-5 py-4 text-sm text-ink">
                        {row.name || "Retired service"}
                      </td>
                      <td className="px-5 py-4 text-sm tabular-nums text-ink-muted">
                        {row.sessions}
                      </td>
                      <td className="px-5 py-4 text-sm font-medium tabular-nums text-ink">
                        {formatMoney(
                          row.incomeKobo,
                          data.summary.currency || "GHS",
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </section>

          {data.reviews.count > 0 ? (
            <section className="flex items-center gap-4 rounded-lg border border-border bg-surface-raised p-5">
              <BarChart3
                size={20}
                aria-hidden="true"
                className="text-ink-faint"
              />
              <p className="text-sm text-ink-muted">
                <span className="font-display text-xl font-medium tabular-nums text-ink">
                  {data.reviews.average.toFixed(1)}
                </span>{" "}
                average from {data.reviews.count}{" "}
                {data.reviews.count === 1
                  ? "published review"
                  : "published reviews"}
              </p>
            </section>
          ) : null}
        </>
      )}
    </div>
  );
}

function Stat({
  label,
  value,
  muted = false,
}: {
  label: string;
  value: string;
  muted?: boolean;
}) {
  return (
    <div className="rounded-lg border border-border bg-surface-raised p-4">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
        {label}
      </dt>
      <dd
        className={cn(
          "mt-1.5 font-display font-medium tabular-nums",
          muted ? "text-xl text-ink-muted" : "text-2xl text-ink",
        )}
      >
        {value}
      </dd>
    </div>
  );
}
