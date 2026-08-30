import { describe, expect, it } from "vitest";
import { buildPortalNotifications } from "./notifications";
import type { Booking } from "./bookings";

const booking = (id: string, status: Booking["status"], startAt = "2026-08-31T10:00:00Z"): Booking => ({
  id, clientId: "client", practitionerId: "practitioner", serviceId: "service", startAt,
  endAt: new Date(new Date(startAt).getTime() + 30 * 60_000).toISOString(), status, createdAt: startAt, updatedAt: startAt,
});

describe("buildPortalNotifications", () => {
  it("collects forms, payments, the next session, and review prompts", () => {
    const items = buildPortalNotifications({
      bookings: [booking("next", "confirmed"), booking("done", "completed", "2026-08-20T10:00:00Z")],
      forms: [{ id: "form", formId: "definition", formTitle: "Wellness intake", status: "assigned", answers: {}, assignedAt: "2026-08-30T08:00:00Z" }],
      payments: [{ id: "pay", bookingId: "next", amountKobo: 100, currency: "GHS", status: "pending", createdAt: "2026-08-30T08:00:00Z" }],
      reviews: [],
      now: new Date("2026-08-30T09:00:00Z"),
    });
    expect(items.map((item) => item.id)).toEqual(["form-form", "payment-pay", "booking-next", "review-done"]);
  });

  it("does not repeat a review prompt after feedback exists", () => {
    const items = buildPortalNotifications({
      bookings: [booking("done", "completed", "2026-08-20T10:00:00Z")], forms: [], payments: [],
      reviews: [{ id: "review", bookingId: "done", rating: 5, status: "approved", createdAt: "2026-08-21T10:00:00Z" }],
    });
    expect(items).toEqual([]);
  });
});
