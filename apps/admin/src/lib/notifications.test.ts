import { describe, expect, it } from "vitest";
import { buildAdminNotifications } from "./notifications";
import type { Booking } from "./schedule";

const booking = (id: string, startAt: string, status: Booking["status"] = "confirmed"): Booking => ({
  id, clientId: "client", practitionerId: "practitioner", serviceId: "service",
  startAt, endAt: new Date(new Date(startAt).getTime() + 30 * 60_000).toISOString(),
  status, createdAt: startAt, updatedAt: startAt,
});

describe("buildAdminNotifications", () => {
  it("turns live practice work into concise actionable items", () => {
    const now = new Date("2026-08-30T09:00:00Z");
    const items = buildAdminNotifications({
      unreadEnquiries: 2,
      pendingReviews: 1,
      bookings: [booking("soon", "2026-08-30T12:00:00Z"), booking("late", "2026-09-02T12:00:00Z")],
      now,
    });
    expect(items.map((item) => item.id)).toEqual(["enquiries", "reviews", "booking-soon"]);
    expect(items[0]?.href).toBe("/enquiries");
  });

  it("is empty when nothing needs attention", () => {
    expect(buildAdminNotifications({ unreadEnquiries: 0, pendingReviews: 0, bookings: [] })).toEqual([]);
  });
});
