import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Booking } from "@/lib/bookings";
import PortalOverviewPage from "./page";

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
      accessToken: "a1",
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

// The overview reads its data through useMyBookings; each state below is
// driven by swapping what the hook returns, so the page is tested without a
// network layer. `bookings === null` is the first-load skeleton.
const useMyBookings = vi.hoisted(() => vi.fn());
vi.mock("@/components/booking/use-my-bookings", () => ({ useMyBookings }));

const refresh = vi.fn();

function hookState(overrides: {
  bookings?: Booking[] | null;
  servicesById?: Map<string, { id: string; name: string }>;
  error?: string | null;
}) {
  return {
    bookings: null,
    servicesById: new Map(),
    error: null,
    refresh,
    ...overrides,
  };
}

function booking(overrides: Partial<Booking> = {}): Booking {
  return {
    id: "b1",
    clientId: "u1",
    practitionerId: "p1",
    serviceId: "s1",
    startAt: "2099-08-20T09:00:00Z",
    endAt: "2099-08-20T10:00:00Z",
    status: "confirmed",
    createdAt: "2026-08-01T09:00:00Z",
    updatedAt: "2026-08-01T09:00:00Z",
    ...overrides,
  };
}

describe("PortalOverviewPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useMyBookings.mockReturnValue(hookState({ bookings: [] }));
  });

  it("welcomes the client by name", () => {
    render(<PortalOverviewPage />);

    expect(
      screen.getByRole("heading", { level: 1, name: /welcome back, ama serwaa/i }),
    ).toBeTruthy();
  });

  it("renders the sessions empty state per the design-system voice", () => {
    render(<PortalOverviewPage />);

    expect(screen.getByRole("heading", { level: 3, name: "No sessions yet" })).toBeTruthy();
    expect(screen.getByText(/Your upcoming sessions will appear here/)).toBeTruthy();
    expect(screen.getByRole("link", { name: "Book a session" })).toBeTruthy();
  });

  it("shows a busy status while the first load is in flight", () => {
    useMyBookings.mockReturnValue(hookState({ bookings: null }));

    render(<PortalOverviewPage />);

    expect(screen.getByRole("status")).toBeTruthy();
    expect(screen.getByText(/Loading your sessions/)).toBeTruthy();
    expect(screen.queryByRole("heading", { level: 3, name: "No sessions yet" })).toBeNull();
  });

  it("lists only the next three upcoming sessions with their service names", () => {
    useMyBookings.mockReturnValue(
      hookState({
        bookings: [
          booking({ id: "b1", startAt: "2099-08-20T09:00:00Z", endAt: "2099-08-20T10:00:00Z" }),
          booking({ id: "b2", startAt: "2099-08-21T09:00:00Z", endAt: "2099-08-21T10:00:00Z" }),
          booking({ id: "b3", startAt: "2099-08-22T09:00:00Z", endAt: "2099-08-22T10:00:00Z" }),
          booking({ id: "b4", startAt: "2099-08-23T09:00:00Z", endAt: "2099-08-23T10:00:00Z" }),
          booking({ id: "b5", status: "cancelled" }),
        ],
        servicesById: new Map([["s1", { id: "s1", name: "Deep Tissue Massage" }]]),
      }),
    );

    render(<PortalOverviewPage />);

    expect(screen.getAllByRole("heading", { level: 3, name: "Deep Tissue Massage" })).toHaveLength(
      3,
    );
    expect(screen.getByRole("link", { name: "View all sessions" })).toBeTruthy();
  });

  it("offers a retry when the load failed", () => {
    useMyBookings.mockReturnValue(
      hookState({ bookings: null, error: "Your sessions didn't load." }),
    );

    render(<PortalOverviewPage />);

    expect(screen.getByRole("alert").textContent).toContain("Your sessions didn't load.");
    screen.getByRole("button", { name: "Try again" }).click();
    expect(refresh).toHaveBeenCalledTimes(1);
  });
});
