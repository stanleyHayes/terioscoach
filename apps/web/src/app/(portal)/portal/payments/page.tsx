"use client";

import { Receipt } from "lucide-react";
import { Badge, type BadgeTone } from "@/components/ui/Badge";
import { Card } from "@/components/ui/Card";
import {
  PortalEmpty,
  PortalError,
  PortalLoading,
  PortalPage,
} from "@/components/portal/PortalPage";
import { formatMoney } from "@/lib/format";
import { paymentsApi, type ClientPayment, type PaymentStatus } from "@/lib/portal";
import { usePortalAction, usePortalData } from "@/lib/use-portal-data";

/**
 * Payment history, and paying for a session that is still owed (CX-10).
 *
 * Paying leaves this app entirely: the API returns Paystack's hosted
 * checkout URL and the browser goes there. Card and mobile-money details
 * never pass through the portal or the API, which is the whole reason the
 * flow is shaped this way.
 */

const statusTone: Record<PaymentStatus, BadgeTone> = {
  success: "success",
  pending: "warning",
  failed: "danger",
  refunded: "neutral",
};

const statusLabel: Record<PaymentStatus, string> = {
  success: "Paid",
  pending: "Not paid yet",
  failed: "Payment failed",
  refunded: "Refunded",
};

export default function PaymentsPage() {
  const payments = usePortalData<ClientPayment[]>(
    (session, callbacks) => paymentsApi.listMine(session, callbacks),
    [],
  );
  const action = usePortalAction();

  async function pay(payment: ClientPayment) {
    const url = await action.run(payment.id, (session, callbacks) =>
      paymentsApi.initialize(session, callbacks, payment.bookingId),
    );
    if (url) {
      // assign() rather than setting location.href: same navigation, but
      // it reads as a call rather than a mutation of a global.
      window.location.assign(url);
    }
  }

  return (
    <PortalPage
      title="Payments"
      intro="Every payment for your sessions, and anything still outstanding."
    >
      {action.error ? (
        <Card>
          <p role="alert" className="text-sm text-danger-ink">
            {action.error}
          </p>
        </Card>
      ) : null}

      {payments.error ? (
        <PortalError message={payments.error} onRetry={payments.refresh} />
      ) : payments.data === null ? (
        <PortalLoading label="Loading your payments…" />
      ) : payments.data.length === 0 ? (
        <PortalEmpty
          icon={<Receipt size={32} />}
          title="No payments yet"
          body="When you pay for a session, the record appears here."
          action={{ href: "/portal/book", label: "Book a session" }}
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {payments.data.map((payment) => (
            <li key={payment.id}>
              <Card className="terios-record-card">
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-3">
                      <span className="font-display text-xl font-medium tabular-nums text-ink">
                        {formatMoney(payment.amountKobo, payment.currency)}
                      </span>
                      <Badge tone={statusTone[payment.status]}>
                        {statusLabel[payment.status]}
                      </Badge>
                    </div>
                    <p className="mt-1 text-[13px] tabular-nums text-ink-muted">
                      <time dateTime={payment.paidAt ?? payment.createdAt}>
                        {new Date(payment.paidAt ?? payment.createdAt).toLocaleDateString(
                          "en-GB",
                          { day: "numeric", month: "short", year: "numeric" },
                        )}
                      </time>
                      {payment.channel ? (
                        <>
                          <span aria-hidden="true" className="mx-2 text-ink-faint">
                            ·
                          </span>
                          {payment.channel.replace(/_/g, " ")}
                        </>
                      ) : null}
                    </p>
                  </div>

                  {/* Only an unfinished payment can be picked up again. A
                      failed attempt re-opens the same checkout. */}
                  {payment.status === "pending" || payment.status === "failed" ? (
                    <button
                      type="button"
                      disabled={action.pending === payment.id}
                      onClick={() => void pay(payment)}
                      className="inline-flex shrink-0 items-center rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary transition-colors duration-instant ease-out hover:bg-primary-hover disabled:opacity-50"
                    >
                      {action.pending === payment.id ? "Opening…" : "Pay now"}
                    </button>
                  ) : null}
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </PortalPage>
  );
}
