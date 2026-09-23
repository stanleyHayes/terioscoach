"use client";

import { useMyBookings } from "@/components/booking/use-my-bookings";
import { SessionRow } from "@/components/booking/SessionRow";
import { Button } from "@/components/ui/Button";
import { useAuth } from "@/lib/auth";
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
import {
  paymentsApi,
  type ClientPayment,
  type PaymentStatus,
} from "@/lib/portal";
import { usePortalAction, usePortalData } from "@/lib/use-portal-data";

/**
 * Payment history, and paying for a session that is still owed (CX-10).
 *
 * Paying leaves this app entirely: the API returns Stripe's hosted
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
  const { user } = useAuth();
  const {
    bookings,
    servicesById,
    error: bookingError,
    refresh: refreshBookings,
  } = useMyBookings();
  const pendingBookings =
    bookings?.filter((b) => b.status === "pending_payment") ?? [];

  async function pay(bookingId: string) {
    const url = await action.run(bookingId, (session, callbacks) =>
      paymentsApi.initialize(session, callbacks, bookingId),
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

      {bookingError && (
        <PortalError message={bookingError} onRetry={refreshBookings} />
      )}
      {pendingBookings.length > 0 && (
        <section className="mb-6 space-y-3">
          <h2 className="font-display text-xl">
            Appointments awaiting payment
          </h2>
          <p className="text-sm text-ink-muted">
            An unpaid request expires at the appointment start. Payment confirms
            the slot only while it is still available.
          </p>
          {pendingBookings.map((booking) => (
            <div id={`booking-${booking.id}`} key={booking.id}>
              <SessionRow
                booking={booking}
                serviceName={servicesById.get(booking.serviceId)?.name}
                timeZone={user?.timezone ?? "UTC"}
                actions={
                  <Button
                    loading={action.pending === booking.id}
                    onClick={() => void pay(booking.id)}
                  >
                    Continue to payment
                  </Button>
                }
              />
            </div>
          ))}
        </section>
      )}
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
            <li key={payment.id} id={`payment-${payment.id}`}>
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
                        {new Date(
                          payment.paidAt ?? payment.createdAt,
                        ).toLocaleDateString("en-GB", {
                          day: "numeric",
                          month: "short",
                          year: "numeric",
                          timeZone: user?.timezone ?? "UTC",
                        })}
                      </time>
                      {payment.channel ? (
                        <>
                          <span
                            aria-hidden="true"
                            className="mx-2 text-ink-faint"
                          >
                            ·
                          </span>
                          {payment.channel.replace(/_/g, " ")}
                        </>
                      ) : null}
                    </p>
                  </div>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </PortalPage>
  );
}
