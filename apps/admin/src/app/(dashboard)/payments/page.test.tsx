import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Payment } from "@/lib/insights";
import PaymentsPage, { summarize } from "./page";

const list = vi.hoisted(() => vi.fn());
const refund = vi.hoisted(() => vi.fn());

vi.mock("@/lib/insights", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/insights")>();
  return { ...original, paymentsApi: { list, refund } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "prac-1", email: "t@example.com", role: "practitioner", name: "Terios" },
      session: { accessToken: "a1", refreshToken: "r1" },
      refreshCallbacks: { onTokensRefreshed: vi.fn() },
      logout: vi.fn(),
    }),
  };
});

function payment(overrides: Partial<Payment> = {}): Payment {
  return {
    id: "payment-1",
    bookingId: "booking-1",
    clientId: "client-1",
    amountKobo: 25000,
    currency: "GHS",
    status: "success",
    providerReference: "ref-1",
    channel: "mobile_money",
    paidAt: "2026-08-03T10:00:00Z",
    createdAt: "2026-08-03T09:00:00Z",
    ...overrides,
  };
}

describe("PaymentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue([payment()]);
    refund.mockImplementation((_s, _c, id) =>
      Promise.resolve(payment({ id, status: "refunded" })),
    );
  });

  it("shows takings and refunds apart, plus the net", async () => {
    list.mockResolvedValue([
      payment({ id: "p1", amountKobo: 25000, status: "success" }),
      payment({ id: "p2", amountKobo: 10000, status: "refunded" }),
    ]);

    const { container } = render(<PaymentsPage />);
    await screen.findAllByText(/GH₵250.00/);

    // Scoped to the stat tiles: both the amounts and the word "Refunded"
    // also appear in the table rows below, so bare text queries would be
    // ambiguous.
    const stats = within(container.querySelector("dl")!);
    const stat = (label: string) =>
      stats.getByText(label).nextElementSibling?.textContent ?? "";

    // Netting them would hide a refund-heavy month behind an ordinary total.
    expect(stat("Taken")).toContain("250.00");
    expect(stat("Refunded")).toContain("100.00");
    expect(stat("Net")).toContain("150.00");
  });

  it("names the amount before refunding, because money is leaving", async () => {
    render(<PaymentsPage />);

    fireEvent.click(await screen.findByRole("button", { name: /refund/i }));

    const dialog = screen.getByRole("dialog");
    expect(dialog.textContent).toContain("GH₵250.00");
    expect(dialog.textContent).toMatch(/cannot be undone/i);
    expect(refund).not.toHaveBeenCalled();
  });

  it("refunds once confirmed and updates the row", async () => {
    render(<PaymentsPage />);
    fireEvent.click(await screen.findByRole("button", { name: /^refund$/i }));

    fireEvent.click(screen.getByRole("button", { name: /refund it/i }));

    await waitFor(() => {
      expect(refund).toHaveBeenCalledWith(expect.anything(), expect.anything(), "payment-1");
    });
    expect(await screen.findByText("Refunded")).toBeTruthy();
  });

  it("can be dismissed without refunding", async () => {
    render(<PaymentsPage />);
    fireEvent.click(await screen.findByRole("button", { name: /^refund$/i }));

    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));

    expect(screen.queryByRole("dialog")).toBeNull();
    expect(refund).not.toHaveBeenCalled();
  });

  it("offers no refund on a payment that never succeeded", async () => {
    list.mockResolvedValue([payment({ status: "pending" })]);

    render(<PaymentsPage />);

    expect(await screen.findByText(/awaiting payment/i)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /^refund$/i })).toBeNull();
  });

  it("moves between months", async () => {
    render(<PaymentsPage />);
    await screen.findByRole("button", { name: /previous month/i });
    const firstCallRange = list.mock.calls[0][2];

    fireEvent.click(screen.getByRole("button", { name: /previous month/i }));

    await waitFor(() => {
      expect(list.mock.calls.length).toBeGreaterThan(1);
    });
    const secondCallRange = list.mock.calls[list.mock.calls.length - 1][2];
    expect(new Date(secondCallRange.from).getTime()).toBeLessThan(
      new Date(firstCallRange.from).getTime(),
    );
  });

  it("says which month is empty rather than showing a blank table", async () => {
    list.mockResolvedValue([]);

    render(<PaymentsPage />);

    expect(await screen.findByRole("heading", { name: /nothing in/i })).toBeTruthy();
  });

  it("surfaces a failed refund", async () => {
    const { ApiError } = await import("@/lib/api");
    refund.mockRejectedValueOnce(new ApiError(502, "payment_gateway_error", "Stripe refused."));
    render(<PaymentsPage />);

    fireEvent.click(await screen.findByRole("button", { name: /^refund$/i }));
    fireEvent.click(screen.getByRole("button", { name: /refund it/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("Stripe refused.");
  });
});

describe("summarize", () => {
  it("counts only successful payments as taken", () => {
    const totals = summarize([
      payment({ amountKobo: 25000, status: "success" }),
      payment({ amountKobo: 10000, status: "refunded" }),
      payment({ amountKobo: 5000, status: "pending" }),
      payment({ amountKobo: 7000, status: "failed" }),
    ]);

    expect(totals.paidKobo).toBe(25000);
    expect(totals.refundedKobo).toBe(10000);
    expect(totals.currency).toBe("GHS");
  });

  it("is all zeros for an empty month", () => {
    expect(summarize([])).toEqual({ paidKobo: 0, refundedKobo: 0, currency: "USD" });
  });
});
