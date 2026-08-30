import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ClientReview } from "@/lib/portal";
import ReviewsPage from "./page";

const listMine = vi.hoisted(() => vi.fn());
const submit = vi.hoisted(() => vi.fn());
const useMyBookings = vi.hoisted(() => vi.fn());

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, reviewsApi: { listMine, submit, update: vi.fn() } };
});

vi.mock("@/components/booking/use-my-bookings", () => ({ useMyBookings }));

vi.mock("@/lib/auth", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/auth")>();
  return {
    ...original,
    useAuth: () => ({
      status: "authenticated",
      user: { id: "u1", email: "ama@example.com", role: "client", name: "Ama Serwaa" },
      accessToken: "a1",
      session: { accessToken: "a1", accessTokenExpiresAt: "2099-01-01T00:00:00Z", refreshToken: "r1" },
      onTokensRefreshed: vi.fn(),
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
    }),
  };
});

function review(overrides: Partial<ClientReview> = {}): ClientReview {
  return {
    id: "review-1",
    bookingId: "booking-1",
    rating: 5,
    comment: "Wonderful session.",
    status: "pending",
    createdAt: "2026-08-10T09:00:00Z",
    ...overrides,
  };
}

const completedBooking = {
  id: "booking-1",
  clientId: "u1",
  practitionerId: "p1",
  serviceId: "svc-1",
  startAt: "2026-08-01T09:00:00Z",
  endAt: "2026-08-01T10:00:00Z",
  status: "completed" as const,
  createdAt: "2026-07-01T09:00:00Z",
  updatedAt: "2026-08-01T10:00:00Z",
};

describe("Portal ReviewsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listMine.mockResolvedValue([]);
    submit.mockImplementation((_s, _c, input) =>
      Promise.resolve(review({ ...input, id: "review-new", status: "pending" })),
    );
    useMyBookings.mockReturnValue({
      bookings: [completedBooking],
      servicesById: new Map([["svc-1", { id: "svc-1", name: "Deep Tissue Massage" }]]),
      error: null,
      refresh: vi.fn(),
    });
  });

  it("offers only completed sessions that have not been reviewed", async () => {
    useMyBookings.mockReturnValue({
      bookings: [
        completedBooking,
        { ...completedBooking, id: "booking-2", status: "confirmed" as const },
        { ...completedBooking, id: "booking-3", status: "cancelled" as const },
      ],
      servicesById: new Map([["svc-1", { id: "svc-1", name: "Deep Tissue Massage" }]]),
      error: null,
      refresh: vi.fn(),
    });

    render(<ReviewsPage />);

    const section = await screen.findByRole("region", { name: /sessions you can review/i });
    // Only the completed one: a session that has not happened cannot be
    // reviewed, and the API would refuse it anyway.
    expect(within(section).getAllByRole("button", { name: /leave a review/i })).toHaveLength(1);
  });

  it("does not offer a session that has already been reviewed", async () => {
    listMine.mockResolvedValue([review({ bookingId: "booking-1" })]);

    render(<ReviewsPage />);
    await screen.findByText("Wonderful session.");

    expect(screen.queryByRole("region", { name: /sessions you can review/i })).toBeNull();
  });

  it("submits a rating and comment", async () => {
    render(<ReviewsPage />);
    fireEvent.click(await screen.findByRole("button", { name: /leave a review/i }));

    fireEvent.click(screen.getByRole("button", { name: "4 stars" }));
    fireEvent.change(screen.getByLabelText(/anything you would like to add/i), {
      target: { value: "Very restful." },
    });
    fireEvent.click(screen.getByRole("button", { name: /send review/i }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith(expect.anything(), expect.anything(), {
        bookingId: "booking-1",
        rating: 4,
        comment: "Very restful.",
      });
    });
  });

  it("sends a rating on its own when no comment is written", async () => {
    render(<ReviewsPage />);
    fireEvent.click(await screen.findByRole("button", { name: /leave a review/i }));

    fireEvent.click(screen.getByRole("button", { name: /send review/i }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith(expect.anything(), expect.anything(), {
        bookingId: "booking-1",
        rating: 5,
        comment: undefined,
      });
    });
  });

  it("says a moderated review can no longer be edited", async () => {
    listMine.mockResolvedValue([review({ status: "approved" })]);

    render(<ReviewsPage />);

    expect(await screen.findByText(/on the website/i)).toBeTruthy();
    expect(screen.getByText(/can no longer be edited/i)).toBeTruthy();
  });

  it("does not say that about a review still waiting", async () => {
    listMine.mockResolvedValue([review({ status: "pending" })]);

    render(<ReviewsPage />);
    await screen.findByText("Wonderful session.");

    expect(screen.getByText(/waiting to be published/i)).toBeTruthy();
    expect(screen.queryByText(/can no longer be edited/i)).toBeNull();
  });

  it("explains a rejected submission", async () => {
    const { ApiError } = await import("@/lib/api");
    submit.mockRejectedValueOnce(
      new ApiError(422, "session_not_complete", "only a completed session can be reviewed"),
    );

    render(<ReviewsPage />);
    fireEvent.click(await screen.findByRole("button", { name: /leave a review/i }));
    fireEvent.click(screen.getByRole("button", { name: /send review/i }));

    expect((await screen.findByRole("alert")).textContent).toContain(
      "only a completed session can be reviewed",
    );
  });

  it("invites a first review when there are none", async () => {
    useMyBookings.mockReturnValue({
      bookings: [],
      servicesById: new Map(),
      error: null,
      refresh: vi.fn(),
    });

    render(<ReviewsPage />);

    expect(await screen.findByRole("heading", { name: /no reviews yet/i })).toBeTruthy();
  });
});
