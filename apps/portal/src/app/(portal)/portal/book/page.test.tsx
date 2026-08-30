import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import BookPage from "./page";

/**
 * The booking flow's hand-off to checkout (WEB-09 / CX-10).
 *
 * This file exists because its absence was the bug. The flow ended at a
 * confirmation screen with a `TODO(payments)` where the Paystack hand-off
 * should have been, and nothing anywhere noticed: the API's own tests cover
 * `/v1/payments/initialize` thoroughly, and this page — the only thing that
 * would ever call it after a booking — had no tests at all.
 *
 * The rule the tests below pin down is that **the booking and the payment
 * are separate**. Confirming books the session. Checkout is what happens
 * next, and if it fails the client keeps the slot.
 */

const listServices = vi.hoisted(() => vi.fn());
const createBooking = vi.hoisted(() => vi.fn());
const initialize = vi.hoisted(() => vi.fn());
const replace = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return { ...original, listServices };
});

vi.mock("@/lib/bookings", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/bookings")>();
  return { ...original, createBooking };
});

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, paymentsApi: { ...original.paymentsApi, initialize } };
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => "/portal/book",
}));

// The slot picker's own behaviour is covered by its own tests; here it
// only has to offer a slot so the flow can reach the review step.
vi.mock("@/components/booking/SlotPicker", () => ({
  SlotPicker: ({ onSelect }: { onSelect: (slot: unknown) => void }) => (
    <button
      type="button"
      onClick={() =>
        onSelect({
          startAt: "2026-09-01T09:00:00.000Z",
          endAt: "2026-09-01T10:00:00.000Z",
        })
      }
    >
      Pick 09:00
    </button>
  ),
}));

const authValue = {
  status: "authenticated" as const,
  user: { id: "client-1", email: "ama@example.com", role: "client" as const, name: "Ama" },
  session: { accessToken: "a1", refreshToken: "r1" },
  onTokensRefreshed: vi.fn(),
  logout: vi.fn(),
};

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return { ...original, useAuth: () => authValue };
});

const service = {
  id: "service-1",
  name: "Aromatherapy massage",
  description: "A calming full-body treatment.",
  durationMinutes: 60,
  priceKobo: 25000,
  currency: "GHS",
  sortOrder: 1,
};

const booking = {
  id: "booking-1",
  clientId: "client-1",
  practitionerId: "prac-1",
  serviceId: "service-1",
  startAt: "2026-09-01T09:00:00.000Z",
  endAt: "2026-09-01T10:00:00.000Z",
  status: "confirmed" as const,
};

/** Drives the flow to the point of confirming. */
async function confirmABooking() {
  render(<BookPage />);
  // A RadioCard (design-system §3.8): the whole card is the control, and
  // it is a custom radio rather than a native one.
  fireEvent.click(await screen.findByRole("radio", { name: /aromatherapy massage/i }));
  fireEvent.click(await screen.findByRole("button", { name: /pick 09:00/i }));
  // Picking a slot arms Continue; it does not skip the review step.
  fireEvent.click(await screen.findByRole("button", { name: /^continue$/i }));
  fireEvent.click(await screen.findByRole("button", { name: /confirm booking/i }));
}

describe("Booking flow → checkout", () => {
  let assign: ReturnType<typeof vi.fn<(url: string) => void>>;

  beforeEach(() => {
    vi.clearAllMocks();
    listServices.mockResolvedValue([service]);
    createBooking.mockResolvedValue(booking);
    initialize.mockResolvedValue("https://checkout.paystack.com/abc123");

    assign = vi.fn<(url: string) => void>();
    Object.defineProperty(window, "location", {
      value: { assign, set href(value: string) { assign(value); } },
      writable: true,
    });
  });

  afterEach(() => vi.restoreAllMocks());

  it("books the session and then sends the client to checkout", async () => {
    await confirmABooking();

    await waitFor(() => expect(createBooking).toHaveBeenCalled());
    await waitFor(() =>
      expect(initialize).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "booking-1",
      ),
    );
    // The booking's own id, not the slot or the service: initializing
    // against the wrong id would charge for someone else's session.
    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith("https://checkout.paystack.com/abc123"),
    );
  });

  it("books first and pays second, never the other way round", async () => {
    await confirmABooking();

    await waitFor(() => expect(initialize).toHaveBeenCalled());
    // A payment initialized before the booking exists would have no
    // booking to attach to, and a client could be charged for a slot
    // somebody else took in the meantime.
    expect(createBooking.mock.invocationCallOrder[0]).toBeLessThan(
      initialize.mock.invocationCallOrder[0],
    );
  });

  it("keeps the session when checkout cannot be opened", async () => {
    initialize.mockRejectedValue(new ApiError(503, "service_unavailable", "payments are down"));

    await confirmABooking();

    // The booking stands. Losing a confirmed slot because a payment
    // provider was briefly unreachable would be the worse failure.
    expect(await screen.findByText(/you.re booked/i)).toBeTruthy();
    expect(await screen.findByRole("status")).toHaveProperty(
      "textContent",
      expect.stringContaining("Your session is held"),
    );
    expect(assign).not.toHaveBeenCalled();
  });

  it("offers a way to pay after a failed hand-off", async () => {
    initialize.mockRejectedValue(new ApiError(503, "service_unavailable", "down"));

    await confirmABooking();

    const payLink = await screen.findByRole("link", { name: /pay for this session/i });
    expect(payLink.getAttribute("href")).toBe("/portal/payments");
  });

  it("does not try to pay for a booking that was never created", async () => {
    createBooking.mockRejectedValue(new ApiError(409, "slot_unavailable", "taken"));

    await confirmABooking();

    await waitFor(() => expect(createBooking).toHaveBeenCalled());
    expect(initialize).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
  });
});
