import type { Booking } from "@/lib/schedule";

export type NotificationTone = "attention" | "care" | "neutral";

export interface AdminNotificationItem {
  id: string;
  title: string;
  description: string;
  href: string;
  tone: NotificationTone;
}

export function resolveAdminNotificationSources(
  results: readonly [
    PromiseSettledResult<number>,
    PromiseSettledResult<unknown[]>,
    PromiseSettledResult<Booking[]>,
  ],
) {
  if (results.every((result) => result.status === "rejected")) {
    const failure = results.find(
      (result): result is PromiseRejectedResult => result.status === "rejected",
    );
    throw failure?.reason ?? new Error("Notification sources are unavailable");
  }
  return {
    unreadEnquiries: results[0].status === "fulfilled" ? results[0].value : 0,
    pendingReviews: results[1].status === "fulfilled" ? results[1].value.length : 0,
    bookings: results[2].status === "fulfilled" ? results[2].value : [],
  };
}

export function buildAdminNotifications({
  unreadEnquiries,
  pendingReviews,
  bookings,
  now = new Date(),
}: {
  unreadEnquiries: number;
  pendingReviews: number;
  bookings: Booking[];
  now?: Date;
}): AdminNotificationItem[] {
  const items: AdminNotificationItem[] = [];
  if (unreadEnquiries > 0) {
    items.push({
      id: "enquiries",
      title: `${unreadEnquiries} new ${unreadEnquiries === 1 ? "enquiry" : "enquiries"}`,
      description: "A prospective client is waiting for a response.",
      href: "/enquiries",
      tone: "attention",
    });
  }
  if (pendingReviews > 0) {
    items.push({
      id: "reviews",
      title: `${pendingReviews} ${pendingReviews === 1 ? "review needs" : "reviews need"} moderation`,
      description: "Approve what is ready before it appears publicly.",
      href: "/reviews",
      tone: "neutral",
    });
  }

  const cutoff = now.getTime() + 24 * 60 * 60 * 1000;
  bookings
    .filter((booking) => {
      const start = new Date(booking.startAt).getTime();
      return booking.status === "confirmed" && start >= now.getTime() && start <= cutoff;
    })
    .sort((a, b) => a.startAt.localeCompare(b.startAt))
    .slice(0, 3)
    .forEach((booking) => {
      const time = new Intl.DateTimeFormat("en-GH", {
        timeZone: "Africa/Accra",
        weekday: "short",
        hour: "numeric",
        minute: "2-digit",
      }).format(new Date(booking.startAt));
      items.push({
        id: `booking-${booking.id}`,
        title: `Consultation ${time}`,
        description: "Open the calendar to review the session and join when ready.",
        href: "/calendar",
        tone: "care",
      });
    });
  return items;
}
