import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Review } from "@/lib/inbox";
import ReviewsPage from "./page";

const list = vi.hoisted(() => vi.fn());
const moderate = vi.hoisted(() => vi.fn());

vi.mock("@/lib/inbox", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/inbox")>();
  return { ...original, reviewsApi: { list, moderate } };
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

function review(overrides: Partial<Review> = {}): Review {
  return {
    id: "review-1",
    bookingId: "booking-1",
    clientId: "client-1",
    rating: 5,
    comment: "Wonderful session.",
    status: "pending",
    createdAt: "2026-08-10T09:00:00Z",
    updatedAt: "2026-08-10T09:00:00Z",
    ...overrides,
  };
}

describe("ReviewsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    list.mockResolvedValue([review()]);
    moderate.mockImplementation((_s, _c, id, approve) =>
      Promise.resolve(review({ id, status: approve ? "approved" : "rejected" })),
    );
  });

  it("opens on the pending queue, which is the only state needing action", async () => {
    list.mockResolvedValue([
      review({ id: "r1", comment: "Waiting", status: "pending" }),
      review({ id: "r2", comment: "Already live", status: "approved" }),
    ]);

    render(<ReviewsPage />);

    expect(await screen.findByText("Waiting")).toBeTruthy();
    expect(screen.queryByText("Already live")).toBeNull();
    expect(screen.getByText(/1 waiting for your decision/i)).toBeTruthy();
  });

  it("publishes a pending review", async () => {
    render(<ReviewsPage />);

    fireEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

    await waitFor(() => {
      expect(moderate).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "review-1",
        true,
      );
    });
    expect(await screen.findByText(/the queue is clear/i)).toBeTruthy();
  });

  it("keeps moderation reversible in both directions", async () => {
    list.mockResolvedValue([review({ status: "approved" })]);
    render(<ReviewsPage />);
    fireEvent.click(screen.getByRole("button", { name: "All" }));

    // An approved review can be taken back off the site…
    const takeDown = await screen.findByRole("button", { name: /take off the site/i });
    fireEvent.click(takeDown);

    await waitFor(() => {
      expect(moderate).toHaveBeenCalledWith(
        expect.anything(),
        expect.anything(),
        "review-1",
        false,
      );
    });
    // …and a rejected one can be published after all.
    expect(await screen.findByRole("button", { name: /publish after all/i })).toBeTruthy();
  });

  it("states the rating in words for anyone not seeing the stars", async () => {
    render(<ReviewsPage />);

    expect(await screen.findByText("5 out of 5")).toBeTruthy();
  });

  it("says so plainly when a review is a rating with no comment", async () => {
    list.mockResolvedValue([review({ comment: undefined })]);

    render(<ReviewsPage />);

    expect(await screen.findByText(/a rating with no comment/i)).toBeTruthy();
  });

  it("shows an empty queue as good news", async () => {
    list.mockResolvedValue([]);

    render(<ReviewsPage />);

    expect(await screen.findByRole("heading", { name: /nothing waiting/i })).toBeTruthy();
  });

  it("explains a failed decision without losing the queue", async () => {
    const { ApiError } = await import("@/lib/api");
    moderate.mockRejectedValueOnce(new ApiError(409, "invalid_status", "Already moderated."));
    render(<ReviewsPage />);

    fireEvent.click(await screen.findByRole("button", { name: /^publish$/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("Already moderated.");
    expect(screen.getByText("Wonderful session.")).toBeTruthy();
  });
});
