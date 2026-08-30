import { describe, expect, it } from "vitest";
import { buildAdminNotifications, resolveAdminNotificationSources } from "./notifications";
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

  it("keeps permitted notification sources when RBAC rejects another", () => {
    const result = resolveAdminNotificationSources([
      { status: "rejected", reason: new Error("forbidden") },
      { status: "fulfilled", value: [{ id: "review" }] },
      { status: "fulfilled", value: [] },
    ]);
    expect(result).toEqual({ unreadEnquiries: 0, pendingReviews: 1, bookings: [] });
  });

  it("normalizes a fully available set of notification sources", () => {
    const upcoming = booking("next", "2026-08-30T12:00:00Z");
    expect(resolveAdminNotificationSources([
      { status: "fulfilled", value: 3 },
      { status: "fulfilled", value: [] },
      { status: "fulfilled", value: [upcoming] },
    ])).toEqual({ unreadEnquiries: 3, pendingReviews: 0, bookings: [upcoming] });
  });

  it("defaults inaccessible review and schedule sources independently", () => {
    const forbidden = { status: "rejected", reason: new Error("forbidden") } as const;
    expect(resolveAdminNotificationSources([
      { status: "fulfilled", value: 1 }, forbidden, forbidden,
    ])).toEqual({ unreadEnquiries: 1, pendingReviews: 0, bookings: [] });
  });

  it("surfaces an outage when every source fails", () => {
    const failure = new Error("offline");
    expect(() => resolveAdminNotificationSources([
      { status: "rejected", reason: failure },
      { status: "rejected", reason: failure },
      { status: "rejected", reason: failure },
    ])).toThrow("offline");
  });
});
