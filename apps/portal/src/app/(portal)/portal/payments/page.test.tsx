import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClientPayment } from "@/lib/portal";
import PaymentsPage from "./page";

const listMine = vi.hoisted(() => vi.fn());
const useMyBookings = vi.hoisted(() => vi.fn());
vi.mock("@/components/booking/use-my-bookings", () => ({ useMyBookings }));
const pendingBooking = {
  id: "booking-1",
  serviceId: "s1",
  startAt: "2099-08-20T09:00:00Z",
  endAt: "2099-08-20T10:00:00Z",
  status: "pending_payment",
};
const initialize = vi.hoisted(() => vi.fn());

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, paymentsApi: { listMine, initialize } };
});

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: {
        id: "u1",
        email: "ama@example.com",
        role: "client",
        name: "Ama Serwaa",
      },
      accessToken: "a1",
      session: {
        accessToken: "a1",
        accessTokenExpiresAt: "2099-01-01T00:00:00Z",
        refreshToken: "r1",
      },
      onTokensRefreshed: vi.fn(),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

function payment(overrides: Partial<ClientPayment> = {}): ClientPayment {
  return {
    id: "payment-1",
    bookingId: "booking-1",
    amountKobo: 25000,
    currency: "GHS",
    status: "success",
    channel: "mobile_money",
    paidAt: "2026-08-03T10:00:00Z",
    createdAt: "2026-08-03T09:00:00Z",
    ...overrides,
  };
}

describe("Portal PaymentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useMyBookings.mockReturnValue({
      bookings: [],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });
    listMine.mockResolvedValue([payment()]);
    initialize.mockResolvedValue(
      "https://checkout.stripe.com/c/pay/cs_test_abc",
    );
  });

  it("shows the payment history with amounts and status", async () => {
    render(<PaymentsPage />);

    expect(await screen.findByText(/GH₵250.00/)).toBeTruthy();
    expect(screen.getByText("Paid")).toBeTruthy();
    expect(screen.getByText(/mobile money/i)).toBeTruthy();
  });

  it("offers no payment action on something already settled", async () => {
    render(<PaymentsPage />);
    await screen.findByText(/GH₵250.00/);

    expect(
      screen.queryByRole("button", { name: /continue to payment/i }),
    ).toBeNull();
  });

  it("sends an unpaid session to the hosted checkout", async () => {
    useMyBookings.mockReturnValue({
      bookings: [pendingBooking],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });
    listMine.mockResolvedValue([
      payment({ status: "pending", paidAt: undefined }),
    ]);
    const assign = vi.fn();
    Object.defineProperty(window, "location", {
      // Both routes to a navigation, so the test does not quietly pass
      // because the page happened to pick the one that was stubbed.
      value: {
        assign,
        set href(value: string) {
          assign(value);
        },
      },
      writable: true,
    });

    render(<PaymentsPage />);
    fireEvent.click(
      await screen.findByRole("button", { name: /continue to payment/i }),
    );

    await waitFor(() => {
      expect(initialize).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "booking-1",
      );
    });
    // Card details never touch this app: the browser leaves for Stripe.
    expect(assign).toHaveBeenCalledWith(
      "https://checkout.stripe.com/c/pay/cs_test_abc",
    );
  });

  it("lets a failed payment be tried again", async () => {
    useMyBookings.mockReturnValue({
      bookings: [pendingBooking],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });
    listMine.mockResolvedValue([
      payment({ status: "failed", paidAt: undefined }),
    ]);

    render(<PaymentsPage />);

    expect(await screen.findByText(/payment failed/i)).toBeTruthy();
    expect(
      screen.getByRole("button", { name: /continue to payment/i }),
    ).toBeTruthy();
  });

  it("explains a checkout that could not be opened", async () => {
    const { ApiError } = await import("@/lib/api");
    initialize.mockRejectedValueOnce(
      new ApiError(
        502,
        "payment_gateway_error",
        "The payment gateway could not respond.",
      ),
    );
    useMyBookings.mockReturnValue({
      bookings: [pendingBooking],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });
    listMine.mockResolvedValue([
      payment({ status: "pending", paidAt: undefined }),
    ]);

    render(<PaymentsPage />);
    fireEvent.click(
      await screen.findByRole("button", { name: /continue to payment/i }),
    );

    expect((await screen.findByRole("alert")).textContent).toContain(
      "The payment gateway could not respond.",
    );
  });

  it("recovers checkout when no payment record was created", async () => {
    listMine.mockResolvedValue([]);
    useMyBookings.mockReturnValue({
      bookings: [pendingBooking],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });
    render(<PaymentsPage />);
    expect(
      await screen.findByRole("button", { name: /continue to payment/i }),
    ).toBeTruthy();
  });

  it("points an empty history at booking a session", async () => {
    listMine.mockResolvedValue([]);

    render(<PaymentsPage />);

    expect(
      await screen.findByRole("heading", { name: /no payments yet/i }),
    ).toBeTruthy();
    expect(screen.getByRole("link", { name: /book a session/i })).toBeTruthy();
  });
});
