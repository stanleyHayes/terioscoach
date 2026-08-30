import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SessionFeedback } from "./SessionFeedback";

const getShared = vi.hoisted(() => vi.fn());

vi.mock("@/lib/portal", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/portal")>();
  return { ...original, notesApi: { getShared } };
});

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

const sharedNote = {
  bookingId: "booking-1",
  sharedFeedback: "Great progress with the shoulder.",
  sharedResources: ["https://example.com/stretch"],
  sharedAt: "2026-08-11T09:00:00Z",
};

describe("SessionFeedback", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getShared.mockResolvedValue(sharedNote);
  });

  it("fetches nothing until the row is opened", () => {
    render(<SessionFeedback bookingId="booking-1" />);

    // Most past sessions are never expanded; a client's notes are not
    // something to prefetch in bulk.
    expect(getShared).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: /feedback from this session/i }).getAttribute("aria-expanded")).toBe("false");
  });

  it("shows the shared feedback and resources when opened", async () => {
    render(<SessionFeedback bookingId="booking-1" />);

    fireEvent.click(screen.getByRole("button", { name: /feedback from this session/i }));

    expect(await screen.findByText(/great progress with the shoulder/i)).toBeTruthy();
    const link = screen.getByRole("link", { name: "https://example.com/stretch" });
    expect(link.getAttribute("href")).toBe("https://example.com/stretch");
    expect(link.getAttribute("rel")).toContain("noopener");
  });

  it("reads a 404 as 'nothing shared yet', which is all it can honestly say", async () => {
    const { ApiError } = await import("@/lib/api");
    getShared.mockRejectedValue(new ApiError(404, "note_not_found", "session note not found"));

    render(<SessionFeedback bookingId="booking-1" />);
    fireEvent.click(screen.getByRole("button", { name: /feedback from this session/i }));

    // The API makes "nothing shared" and "no note exists" identical on
    // purpose, so the client can never infer that something was withheld.
    expect(await screen.findByText(/nothing has been shared for this session yet/i)).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("shows a real failure as a failure", async () => {
    const { ApiError } = await import("@/lib/api");
    getShared.mockRejectedValue(new ApiError(500, "internal_error", "Something went wrong."));

    render(<SessionFeedback bookingId="booking-1" />);
    fireEvent.click(screen.getByRole("button", { name: /feedback from this session/i }));

    expect((await screen.findByRole("alert")).textContent).toContain("Something went wrong.");
  });

  it("closes again on a second press", async () => {
    render(<SessionFeedback bookingId="booking-1" />);
    const toggle = screen.getByRole("button", { name: /feedback from this session/i });

    fireEvent.click(toggle);
    await screen.findByText(/great progress/i);

    fireEvent.click(toggle);

    await waitFor(() => {
      expect(toggle.getAttribute("aria-expanded")).toBe("false");
    });
  });

  it("handles feedback with no resources", async () => {
    getShared.mockResolvedValue({ ...sharedNote, sharedResources: [] });

    render(<SessionFeedback bookingId="booking-1" />);
    fireEvent.click(screen.getByRole("button", { name: /feedback from this session/i }));

    await screen.findByText(/great progress/i);
    expect(screen.queryByText("Resources")).toBeNull();
  });
});
