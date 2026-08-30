import type { BadgeTone } from "@/components/ui/Badge";
import type { BookingStatus } from "@/lib/bookings";

/** Booking status → Badge tone + display label (design-system §3.20 status
 * pairs; contract §Bookings statuses). Shared by the portal overview,
 * sessions page, and booking confirmation. */
export const bookingStatusMeta: Record<BookingStatus, { tone: BadgeTone; label: string }> = {
  confirmed: { tone: "success", label: "Confirmed" },
  completed: { tone: "info", label: "Completed" },
  cancelled: { tone: "danger", label: "Cancelled" },
  no_show: { tone: "warning", label: "No show" },
};
