import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SessionsPage from "./page";

vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ session: null, user: { timezone: "UTC" } }),
}));
vi.mock("@/components/portal/SessionFeedback", () => ({
  SessionFeedback: () => null,
}));
vi.mock("@/components/booking/use-my-bookings", () => ({
  useMyBookings: () => ({
    now: new Date("2026-09-24T10:30:00Z"),
    bookings: [
      {
        id: "past",
        serviceId: "s1",
        status: "confirmed",
        startAt: "2026-09-23T10:00:00Z",
        endAt: "2026-09-23T11:00:00Z",
      },
      {
        id: "active",
        serviceId: "s1",
        status: "confirmed",
        startAt: "2026-09-24T10:00:00Z",
        endAt: "2026-09-24T11:00:00Z",
      },
      {
        id: "future",
        serviceId: "s1",
        status: "confirmed",
        startAt: "2026-09-25T10:00:00Z",
        endAt: "2026-09-25T11:00:00Z",
      },
    ],
    servicesById: new Map([["s1", { name: "Nurse consultation" }]]),
    error: null,
    refresh: vi.fn(),
  }),
}));

describe("consultation preparation actions", () => {
  it("keeps document actions with active appointments and excludes past appointments", () => {
    render(<SessionsPage />);
    expect(screen.queryByText("Prepare for your sessions")).toBeNull();
    const links = screen.getAllByRole("link", { name: "Required documents" });
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      "/portal/sessions/active/documents",
      "/portal/sessions/future/documents",
    ]);
    for (const link of links) {
      const card = link.closest(".terios-record-card");
      expect(card?.textContent).toContain("Nurse consultation");
      expect(card?.textContent).toContain("2026");
      expect(card?.textContent).toContain("UTC");
    }
  });
});
