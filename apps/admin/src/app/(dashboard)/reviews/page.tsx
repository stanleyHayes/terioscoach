"use client";

import { CircleAlert, Star } from "lucide-react";
import { useState } from "react";
import { Badge, type BadgeVariant } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { EmptyState } from "@/components/content/states";
import { cn } from "@/lib/cn";
import {
  REVIEW_STATUSES,
  reviewsApi,
  type Review,
  type ReviewStatus,
} from "@/lib/inbox";
import { useAction, useResource } from "@/lib/use-resource";

/**
 * Review moderation (ADM-10).
 *
 * Nothing a client wrote appears on the public site until it is approved
 * here, and approval is reversible in both directions — a review rejected
 * in haste can be approved later, and one that turns out to be a mistake
 * can be taken down. That is why every row keeps both actions rather than
 * disappearing once it is handled.
 */

const statusTone: Record<ReviewStatus, BadgeVariant> = {
  pending: "warning",
  approved: "success",
  rejected: "neutral",
};

const statusLabel: Record<ReviewStatus, string> = {
  pending: "Awaiting review",
  approved: "On the site",
  rejected: "Not published",
};

export default function ReviewsPage() {
  // Pending first: it is the only state that needs an action.
  const [filter, setFilter] = useState<ReviewStatus | "all">("pending");

  const reviews = useResource<Review[]>(
    (session, callbacks) => reviewsApi.list(session, callbacks),
    [],
  );
  const action = useAction();

  const items = reviews.data ?? [];
  const visible =
    filter === "all" ? items : items.filter((r) => r.status === filter);
  const pendingCount = items.filter((r) => r.status === "pending").length;

  async function moderate(review: Review, approve: boolean) {
    const updated = await action.run(review.id, (session, callbacks) =>
      reviewsApi.moderate(session, callbacks, review.id, approve),
    );
    if (updated) {
      reviews.set((current) =>
        (current ?? []).map((r) => (r.id === updated.id ? updated : r)),
      );
    }
  }

  return (
    <div data-admin-page="reviews" className="flex flex-col gap-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="font-display text-[1.75rem] leading-[1.2] font-medium tracking-[-0.01em] text-ink">
            Reviews
          </h1>
          <p className="mt-1.5 text-sm text-ink-muted">
            {pendingCount > 0
              ? `${pendingCount} waiting for your decision`
              : "Nothing waiting — the queue is clear"}
          </p>
        </div>

        <div
          className="flex flex-wrap gap-2"
          role="group"
          aria-label="Filter by status"
        >
          {(["all", ...REVIEW_STATUSES] as const).map((value) => (
            <button
              key={value}
              type="button"
              aria-pressed={filter === value}
              onClick={() => setFilter(value)}
              className={cn(
                "rounded-full px-3 py-1.5 text-[13px] font-medium transition-colors duration-instant ease-out",
                filter === value
                  ? "bg-primary text-on-primary"
                  : "bg-surface-sunken text-ink-muted hover:text-ink",
              )}
            >
              {value === "all" ? "All" : statusLabel[value]}
            </button>
          ))}
        </div>
      </header>

      {action.error ? (
        <div
          role="alert"
          className="flex items-start gap-3 rounded-lg border border-danger-bg bg-danger-bg px-4 py-3"
        >
          <CircleAlert
            size={16}
            aria-hidden="true"
            className="mt-0.5 shrink-0 text-danger-ink"
          />
          <p className="text-sm text-danger-ink">{action.error}</p>
        </div>
      ) : null}

      {reviews.error ? (
        <div
          role="alert"
          className="rounded-lg border border-border bg-surface-raised p-8 text-center"
        >
          <p className="text-sm text-ink-muted">{reviews.error}</p>
          <div className="mt-4">
            <Button variant="secondary" size="sm" onClick={reviews.refresh}>
              Try again
            </Button>
          </div>
        </div>
      ) : reviews.data === null ? (
        <div role="status" aria-busy="true" className="flex flex-col gap-3">
          <span className="sr-only">Loading reviews…</span>
          {[0, 1, 2].map((i) => (
            <span
              key={i}
              aria-hidden="true"
              className="h-28 rounded-lg bg-surface-sunken"
            />
          ))}
        </div>
      ) : visible.length === 0 ? (
        <EmptyState
          icon={<Star size={26} />}
          title={filter === "pending" ? "Nothing waiting" : "No reviews here"}
          body="Clients can review a session once it is complete. Approved reviews appear on the public site; nothing appears until you say so."
        />
      ) : (
        <ul className="flex flex-col gap-3">
          {visible.map((review) => {
            const busy = action.pending === review.id;
            return (
              <li
                key={review.id}
                className="rounded-lg border border-border bg-surface-raised p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-3">
                      <Stars rating={review.rating} />
                      <Badge variant={statusTone[review.status]}>
                        {statusLabel[review.status]}
                      </Badge>
                    </div>
                    {review.comment ? (
                      <p className="mt-3 max-w-[68ch] text-sm leading-[1.6] whitespace-pre-line text-ink">
                        {review.comment}
                      </p>
                    ) : (
                      <p className="mt-3 text-sm italic text-ink-faint">
                        A rating with no comment.
                      </p>
                    )}
                  </div>
                  <time
                    dateTime={review.createdAt}
                    className="shrink-0 text-[13px] tabular-nums text-ink-faint"
                  >
                    {new Date(review.createdAt).toLocaleDateString("en-GB", {
                      day: "numeric",
                      month: "short",
                      year: "numeric",
                    })}
                  </time>
                </div>

                <div className="mt-5 flex flex-wrap gap-2">
                  {review.status !== "approved" ? (
                    <Button
                      size="sm"
                      disabled={busy}
                      onClick={() => void moderate(review, true)}
                    >
                      {review.status === "rejected"
                        ? "Publish after all"
                        : "Publish"}
                    </Button>
                  ) : null}
                  {review.status !== "rejected" ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy}
                      onClick={() => void moderate(review, false)}
                    >
                      {review.status === "approved"
                        ? "Take off the site"
                        : "Don't publish"}
                    </Button>
                  ) : null}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/** Five stars with the filled count set by the rating, and the number stated
 * for anyone not seeing them. */
function Stars({ rating }: { rating: number }) {
  const filled = Math.max(0, Math.min(5, rating));
  return (
    <span className="flex items-center gap-1">
      <span className="sr-only">{filled} out of 5</span>
      {[1, 2, 3, 4, 5].map((star) => (
        <Star
          key={star}
          size={16}
          aria-hidden="true"
          className={cn(
            star <= filled ? "fill-primary text-primary" : "text-border-strong",
          )}
        />
      ))}
    </span>
  );
}
