/**
 * Typed client for payments and reporting
 * (design/api-contract.md §Payments BE-06, §Reporting BE-15).
 *
 *   GET  /v1/payments                    ?from=&to=  → {items}
 *   POST /v1/payments/{id}/refund                    → {payment}
 *   GET  /v1/admin/reports/practice      ?from=&to=&granularity=
 *   GET  /v1/admin/reports/upcoming-load ?days=
 */

import { authedRequest, type RefreshCallbacks, type Session } from "@/lib/api";

export type PaymentStatus = "pending" | "success" | "failed" | "refunded";

export interface Payment {
  id: string;
  bookingId: string;
  clientId: string;
  amountKobo: number;
  currency: string;
  status: PaymentStatus;
  paystackReference: string;
  channel?: string;
  paidAt?: string;
  createdAt: string;
}

export interface PaymentRange {
  /** RFC 3339, filtering on createdAt. Both optional. */
  from?: string;
  to?: string;
}

export const paymentsApi = {
  async list(
    session: Session,
    callbacks: RefreshCallbacks,
    range: PaymentRange = {},
  ): Promise<Payment[]> {
    const query = new URLSearchParams();
    if (range.from) query.set("from", range.from);
    if (range.to) query.set("to", range.to);
    const suffix = query.toString() ? `?${query.toString()}` : "";

    const { items } = await authedRequest<{ items: Payment[] }>(
      `/v1/payments${suffix}`,
      session,
      callbacks,
    );
    return items;
  },

  /** Refundable only from `success`; anything else answers 409
   * invalid_status. The booking's payment stamp follows automatically. */
  async refund(
    session: Session,
    callbacks: RefreshCallbacks,
    paymentId: string,
  ): Promise<Payment> {
    const { payment } = await authedRequest<{ payment: Payment }>(
      `/v1/payments/${paymentId}/refund`,
      session,
      callbacks,
      { method: "POST" },
    );
    return payment;
  },
};

export type Granularity = "day" | "week" | "month";

export interface ReportSummary {
  sessionsCompleted: number;
  sessionsUpcoming: number;
  cancellations: number;
  noShows: number;
  newClients: number;
  incomeKobo: number;
  refundedKobo: number;
  netKobo: number;
  currency: string;
}

export interface ServiceIncome {
  serviceId: string;
  name: string;
  sessions: number;
  incomeKobo: number;
}

export interface PeriodIncome {
  start: string;
  sessions: number;
  incomeKobo: number;
}

export interface PracticeReport {
  from: string;
  to: string;
  granularity: Granularity;
  summary: ReportSummary;
  byService: ServiceIncome[];
  series: PeriodIncome[];
  reviews: { count: number; average: number; distribution: Record<string, number> };
}

export interface DayLoad {
  /** Calendar date, YYYY-MM-DD. */
  date: string;
  sessions: number;
}

export interface ReportRange {
  from?: string;
  to?: string;
  granularity?: Granularity;
}

export const reportsApi = {
  practice(
    session: Session,
    callbacks: RefreshCallbacks,
    range: ReportRange = {},
  ): Promise<PracticeReport> {
    const query = new URLSearchParams();
    if (range.from) query.set("from", range.from);
    if (range.to) query.set("to", range.to);
    if (range.granularity) query.set("granularity", range.granularity);
    const suffix = query.toString() ? `?${query.toString()}` : "";

    return authedRequest<PracticeReport>(
      `/v1/admin/reports/practice${suffix}`,
      session,
      callbacks,
    );
  },

  async upcomingLoad(
    session: Session,
    callbacks: RefreshCallbacks,
    days = 14,
  ): Promise<DayLoad[]> {
    const { items } = await authedRequest<{ items: DayLoad[] }>(
      `/v1/admin/reports/upcoming-load?days=${days}`,
      session,
      callbacks,
    );
    return items;
  },
};

/** The current calendar month as a half-open reporting window, which is the
 * dashboard's default view. Half-open so adjacent months add up. */
export function currentMonthRange(now = new Date()): { from: string; to: string } {
  const from = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));
  const to = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() + 1, 1));
  return { from: from.toISOString(), to: to.toISOString() };
}

/** Shifts a month window by `months`, for the report's back/forward controls. */
export function shiftMonthRange(
  range: { from: string; to: string },
  months: number,
): { from: string; to: string } {
  const from = new Date(range.from);
  const shifted = new Date(Date.UTC(from.getUTCFullYear(), from.getUTCMonth() + months, 1));
  const next = new Date(Date.UTC(shifted.getUTCFullYear(), shifted.getUTCMonth() + 1, 1));
  return { from: shifted.toISOString(), to: next.toISOString() };
}

/** The tallest bar in a series, used to scale a chart. Never zero, so a
 * flat-zero month renders as empty bars rather than dividing by zero. */
export function peakIncome(series: PeriodIncome[]): number {
  return series.reduce((max, bucket) => Math.max(max, bucket.incomeKobo), 0) || 1;
}
