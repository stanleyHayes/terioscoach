import type { Booking } from "@/lib/bookings";
import type { ClientPayment, ClientReview, FormSubmission } from "@/lib/portal";

export type NotificationTone = "attention" | "care" | "neutral";

export interface PortalNotificationItem {
  id: string;
  title: string;
  description: string;
  href: string;
  tone: NotificationTone;
}

export function buildPortalNotifications({
  bookings,
  forms,
  payments,
  reviews,
  now = new Date(),
}: {
  bookings: Booking[];
  forms: FormSubmission[];
  payments: ClientPayment[];
  reviews: ClientReview[];
  now?: Date;
}): PortalNotificationItem[] {
  const items: PortalNotificationItem[] = forms
    .filter((form) => form.status === "assigned")
    .map((form) => ({
      id: `form-${form.id}`,
      title: form.formTitle,
      description: "A form from your care team is ready to complete.",
      href: `/portal/forms/${form.id}`,
      tone: "attention" as const,
    }));

  payments
    .filter((payment) => payment.status === "pending" || payment.status === "failed")
    .forEach((payment) =>
      items.push({
        id: `payment-${payment.id}`,
        title: payment.status === "failed" ? "Payment needs another try" : "Payment is awaiting completion",
        description: "Open payments to securely complete this booking.",
        href: "/portal/payments",
        tone: "attention",
      }),
    );

  const upcoming = bookings
    .filter((booking) => booking.status === "confirmed" && new Date(booking.endAt) > now)
    .sort((a, b) => a.startAt.localeCompare(b.startAt))[0];
  if (upcoming) {
    const time = new Intl.DateTimeFormat("en-GH", {
      timeZone: "Africa/Accra",
      weekday: "short",
      day: "numeric",
      month: "short",
      hour: "numeric",
      minute: "2-digit",
    }).format(new Date(upcoming.startAt));
    items.push({
      id: `booking-${upcoming.id}`,
      title: `Your next consultation is ${time}`,
      description: "Review the details and join from Consultations when the room opens.",
      href: "/portal/sessions",
      tone: "care",
    });
  }

  const reviewed = new Set(reviews.map((review) => review.bookingId));
  const reviewable = bookings.find(
    (booking) => booking.status === "completed" && !reviewed.has(booking.id),
  );
  if (reviewable) {
    items.push({
      id: `review-${reviewable.id}`,
      title: "How did your session feel?",
      description: "Share a private review of your completed consultation.",
      href: "/portal/reviews",
      tone: "neutral",
    });
  }
  return items;
}
