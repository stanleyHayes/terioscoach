"use client";

import { CircleAlert, CreditCard } from "lucide-react";
import { useMemo, useState } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/content/states";
import { formatMoney } from "@/lib/format";
import { paymentsApi, type Payment, type PaymentStatus } from "@/lib/insights";
import { currentMonthRange, shiftMonthRange } from "@/lib/insights";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * Payments and earnings (ADM-06).
 *
 * Money is read one month at a time, matching how a practice actually
 * reconciles. Refunds are shown beside takings rather than subtracted from
 * them: a month with heavy refunds and a quiet month net out the same, and
 * a practitioner needs to tell those apart.
 *
 * A refund is real money leaving, so it asks for confirmation and names the
 * amount before it goes.
 */

const statusVariant: Record<PaymentStatus, BadgeVariant> = {
  success: "success",
  pending: "warning",
  failed: "danger",
  refunded: "neutral",
};

const statusLabel: Record<PaymentStatus, string> = {
  success: "Paid",
  pending: "Awaiting payment",
  failed: "Failed",
  refunded: "Refunded",
};

export default function PaymentsPage() {
  const [range, setRange] = useState(() => currentMonthRange());
  const [confirming, setConfirming] = useState<Payment | null>(null);

  const payments = useResource<Payment[]>(
    (session, callbacks) => paymentsApi.list(session, callbacks, range),
    [range.from, range.to],
  );
  const action = useAction();

  // Memoized, not `?? []` inline: a fresh empty array every render would
  // make every useMemo below it recompute, which is the opposite of why
  // they are there.
  const items = useMemo(() => payments.data ?? [], [payments.data]);
  const totals = useMemo(() => summarize(items), [items]);
  const monthLabel = new Date(range.from).toLocaleDateString("en-GB", {
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });

  async function refund(payment: Payment) {
    const updated = await action.run(payment.id, (session, callbacks) =>
      paymentsApi.refund(session, callbacks, payment.id),
    );
    if (updated) {
      payments.set((current) =>
        (current ?? []).map((p) => (p.id === updated.id ? updated : p)),
      );
      setConfirming(null);
    }
  }

  return (
    <div data-admin-page="payments" className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
            Payments
          </h1>
          <p className="mt-1.5 text-sm text-ink-muted">{monthLabel}</p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setRange(shiftMonthRange(range, -1))}
          >
            Previous month
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
            Next month
          </Button>
        </div>
      </header>

      <dl className="grid gap-4 sm:grid-cols-3">
        <Stat
          label="Taken"
          value={formatMoney(totals.paidKobo, totals.currency)}
        />
        <Stat
          label="Refunded"
          value={formatMoney(totals.refundedKobo, totals.currency)}
        />
        <Stat
          label="Net"
          value={formatMoney(
            totals.paidKobo - totals.refundedKobo,
            totals.currency,
          )}
        />
      </dl>

      {action.error ? (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0 text-danger-ink"
          />
          <p className="text-sm text-danger-ink">{action.error}</p>
        </div>
      ) : null}

      {payments.error ? (
        <div
          role="alert"
          className="rounded-lg border border-border bg-surface-raised p-8 text-center"
        >
          <p className="text-sm text-ink-muted">{payments.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={payments.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : payments.data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-2">
          <span className="sr-only">Loading payments…</span>
          {[0, 1, 2, 3].map((i) => (
            <span
              key={i}
              aria-hidden="true"
              className="h-14 rounded-lg bg-surface-sunken"
            />
          ))}
        </div>
      ) : items.length === 0 ? (
        <EmptyState
          icon={<CreditCard size={26} />}
          title={`Nothing in ${monthLabel}`}
          body="Payments appear here as clients pay for their sessions."
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-border bg-surface-raised">
          <table className="w-full border-collapse text-left">
            <caption className="sr-only">
              Payments in {monthLabel}, newest first
            </caption>
            <thead>
              <tr className="border-b border-border">
                {["Date", "Amount", "Status", "Method", ""].map(
                  (heading, i) => (
                    <th
                      key={heading || i}
                      scope="col"
                      className="px-5 py-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted"
                    >
                      {heading || <span className="sr-only">Actions</span>}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {items.map((payment) => (
                <tr key={payment.id}>
                  <td className="px-5 py-4 text-sm tabular-nums text-ink-muted">
                    <time dateTime={payment.createdAt}>
                      {new Date(payment.createdAt).toLocaleDateString("en-GB", {
                        day: "numeric",
                        month: "short",
                        year: "numeric",
                      })}
                    </time>
                  </td>
                  <td className="px-5 py-4 text-sm font-medium tabular-nums text-ink">
                    {formatMoney(payment.amountKobo, payment.currency)}
                  </td>
                  <td className="px-5 py-4">
                    <Badge variant={statusVariant[payment.status]}>
                      {statusLabel[payment.status]}
                    </Badge>
                  </td>
                  <td className="px-5 py-4 text-sm text-ink-muted">
                    {payment.channel ? payment.channel.replace(/_/g, " ") : "—"}
                  </td>
                  <td className="px-5 py-4 text-right">
                    {payment.status === "success" ? (
                      <Button
                        variant="secondary"
                        size="sm"
                        disabled={action.pending === payment.id}
                        onClick={() => setConfirming(payment)}
                      >
                        Refund
                      </Button>
                    ) : null}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Refunds move real money, so the amount is named before it goes. */}
      {confirming ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby="refund-heading"
          className="fixed inset-0 z-50 flex items-center justify-center bg-ink/40 p-6"
        >
          <div className="w-full max-w-[420px] rounded-xl border border-border bg-surface-raised p-6">
            <h2
              id="refund-heading"
              className="font-display text-xl font-medium text-ink"
            >
              Refund {formatMoney(confirming.amountKobo, confirming.currency)}?
            </h2>
            <p className="mt-3 text-sm leading-[1.55] text-ink-muted">
              This sends the money back through Stripe. It cannot be undone
              from here.
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <Button variant="ghost" onClick={() => setConfirming(null)}>
                Cancel
              </Button>
              <Button
                disabled={action.pending === confirming.id}
                onClick={() => void refund(confirming)}
              >
                Refund it
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-surface-raised p-4">
      <dt className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-muted">
        {label}
      </dt>
      <dd className="mt-1.5 font-display text-2xl font-medium tabular-nums text-ink">
        {value}
      </dd>
    </div>
  );
}

/** Totals for the month. Refunds are counted apart from takings — netting
 * them would hide a refund-heavy month behind an ordinary-looking figure. */
export function summarize(payments: Payment[]): {
  paidKobo: number;
  refundedKobo: number;
  currency: string;
} {
  let paidKobo = 0;
  let refundedKobo = 0;
  let currency = "USD";

  for (const payment of payments) {
    if (payment.status === "success") paidKobo += payment.amountKobo;
    if (payment.status === "refunded") refundedKobo += payment.amountKobo;
    if (payment.currency) currency = payment.currency;
  }
  return { paidKobo, refundedKobo, currency };
}
