"use client";

import { Star } from "lucide-react";
import { useState } from "react";
import { Badge, type BadgeTone } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import {
  PortalEmpty,
  PortalError,
  PortalLoading,
  PortalPage,
} from "@/components/portal/PortalPage";
import { useMyBookings } from "@/components/booking/use-my-bookings";
import { cn } from "@/lib/cn";
import {
  reviewableBookings,
  reviewsApi,
  type ClientReview,
  type ReviewStatus,
} from "@/lib/portal";
import { usePortalAction, usePortalData } from "@/lib/use-portal-data";

/**
 * Review submission (CX-11).
 *
 * A review can only be left for a session that actually happened, and only
 * once — both enforced by the API, and both reflected here so a client is
 * never offered a button that will fail. Editing is possible only while the
 * review is still waiting: once it is on the site, the text is fixed, and
 * the page says so rather than letting someone discover it by trying.
 */

const statusTone: Record<ReviewStatus, BadgeTone> = {
  pending: "warning",
  approved: "success",
  rejected: "neutral",
};

const statusLabel: Record<ReviewStatus, string> = {
  pending: "Waiting to be published",
  approved: "On the website",
  rejected: "Not published",
};

export default function ReviewsPage() {
  const { bookings, servicesById } = useMyBookings();
  const reviews = usePortalData<ClientReview[]>(
    (session, callbacks) => reviewsApi.listMine(session, callbacks),
    [],
  );
  const action = usePortalAction();

  const [draftFor, setDraftFor] = useState<string | null>(null);
  const [rating, setRating] = useState(5);
  const [comment, setComment] = useState("");

  const mine = reviews.data ?? [];
  const canReview = reviewableBookings(bookings ?? [], mine);

  async function submit(bookingId: string) {
    const created = await action.run(bookingId, (session, callbacks) =>
      reviewsApi.submit(session, callbacks, {
        bookingId,
        rating,
        comment: comment.trim() || undefined,
      }),
    );
    if (created) {
      reviews.set((current) => [created, ...(current ?? [])]);
      setDraftFor(null);
      setRating(5);
      setComment("");
    }
  }

  return (
    <PortalPage
      title="Your reviews"
      intro="Say how a session went. Reviews are read by your practitioner before anything appears on the website."
    >
      {action.error ? (
        <Card>
          <p role="alert" className="text-sm text-danger-ink">
            {action.error}
          </p>
        </Card>
      ) : null}

      {/* Sessions still waiting for a review. */}
      {canReview.length > 0 ? (
        <section aria-labelledby="reviewable-heading" className="flex flex-col gap-3">
          <h2
            id="reviewable-heading"
            className="font-display text-[1.5rem] leading-[1.2] font-medium text-ink"
          >
            Sessions you can review
          </h2>
          {canReview.map((booking) => (
            <Card key={booking.id} className="terios-record-card">
              {draftFor === booking.id ? (
                <div className="flex flex-col gap-4">
                  <p className="text-sm font-medium text-ink">
                    {servicesById.get(booking.serviceId)?.name ?? "Your session"}
                  </p>

                  <fieldset className="border-0 p-0">
                    <legend className="text-sm font-medium text-ink">Your rating</legend>
                    <div className="mt-2 flex gap-1">
                      {[1, 2, 3, 4, 5].map((star) => (
                        <button
                          key={star}
                          type="button"
                          aria-label={`${star} ${star === 1 ? "star" : "stars"}`}
                          aria-pressed={rating === star}
                          onClick={() => setRating(star)}
                          className="rounded-sm p-1 transition-colors duration-instant ease-out"
                        >
                          <Star
                            size={24}
                            aria-hidden="true"
                            className={cn(
                              star <= rating ? "fill-primary text-primary" : "text-border-strong",
                            )}
                          />
                        </button>
                      ))}
                    </div>
                  </fieldset>

                  <div>
                    <label
                      htmlFor="review-comment"
                      className="block text-sm font-medium text-ink"
                    >
                      Anything you would like to add?
                    </label>
                    <textarea
                      id="review-comment"
                      rows={4}
                      value={comment}
                      onChange={(event) => setComment(event.target.value)}
                      className="mt-1.5 w-full rounded-lg border border-border bg-surface-raised px-3 py-2.5 text-base leading-[1.6] text-ink placeholder:text-ink-faint focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20"
                      placeholder="Optional — a sentence is plenty."
                    />
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <Button
                      loading={action.pending === booking.id}
                      onClick={() => void submit(booking.id)}
                    >
                      Send review
                    </Button>
                    <Button variant="ghost" onClick={() => setDraftFor(null)}>
                      Cancel
                    </Button>
                  </div>
                </div>
              ) : (
                <div className="flex flex-wrap items-center justify-between gap-4">
                  <div>
                    <p className="text-base font-semibold text-ink">
                      {servicesById.get(booking.serviceId)?.name ?? "Your session"}
                    </p>
                    <p className="mt-1 text-[13px] tabular-nums text-ink-muted">
                      <time dateTime={booking.startAt}>
                        {new Date(booking.startAt).toLocaleDateString("en-GB", {
                          day: "numeric",
                          month: "short",
                          year: "numeric",
                        })}
                      </time>
                    </p>
                  </div>
                  <Button size="sm" onClick={() => setDraftFor(booking.id)}>
                    Leave a review
                  </Button>
                </div>
              )}
            </Card>
          ))}
        </section>
      ) : null}

      {/* Reviews already left. */}
      <section aria-labelledby="my-reviews-heading" className="flex flex-col gap-3">
        <h2
          id="my-reviews-heading"
          className="font-display text-[1.5rem] leading-[1.2] font-medium text-ink"
        >
          Reviews you have left
        </h2>

        {reviews.error ? (
          <PortalError message={reviews.error} onRetry={reviews.refresh} />
        ) : reviews.data === null ? (
          <PortalLoading label="Loading your reviews…" rows={2} />
        ) : mine.length === 0 ? (
          <PortalEmpty
            icon={<Star size={32} />}
            title="No reviews yet"
            body="Once a session is complete you can say how it went — it means a great deal."
          />
        ) : (
          <ul className="flex flex-col gap-3">
            {mine.map((review) => (
              <li key={review.id}>
                <Card>
                  <div className="flex flex-wrap items-start justify-between gap-4">
                    <div className="min-w-0">
                      <p className="flex items-center gap-1">
                        <span className="sr-only">{review.rating} out of 5</span>
                        {[1, 2, 3, 4, 5].map((star) => (
                          <Star
                            key={star}
                            size={16}
                            aria-hidden="true"
                            className={cn(
                              star <= review.rating
                                ? "fill-primary text-primary"
                                : "text-border-strong",
                            )}
                          />
                        ))}
                      </p>
                      {review.comment ? (
                        <p className="mt-3 max-w-[60ch] text-sm leading-[1.6] text-ink">
                          {review.comment}
                        </p>
                      ) : null}
                      {review.status !== "pending" ? (
                        <p className="mt-3 text-[13px] text-ink-faint">
                          Your practitioner has read this, so it can no longer be edited.
                        </p>
                      ) : null}
                    </div>
                    <Badge tone={statusTone[review.status]}>{statusLabel[review.status]}</Badge>
                  </div>
                </Card>
              </li>
            ))}
          </ul>
        )}
      </section>
    </PortalPage>
  );
}
