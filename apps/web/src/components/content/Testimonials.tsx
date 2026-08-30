import { Star } from "lucide-react";
import { cn } from "@/lib/cn";
import type { PublicReview, ReviewSummary, Testimonial } from "@/lib/content";

/**
 * Approved-only social proof (WEB-07).
 *
 * Two sources, one display: testimonials are quotes the practice curates,
 * client reviews are ratings people left after a session. Both reach this
 * component only through the API's public routes, which cannot return
 * anything unapproved — there is no "pending" state to render, by design.
 */

export interface TestimonialsProps {
  testimonials: Testimonial[];
  reviews?: PublicReview[];
  summary?: ReviewSummary;
  className?: string;
}

export function Testimonials({
  testimonials,
  reviews = [],
  summary,
  className,
}: TestimonialsProps) {
  if (testimonials.length === 0 && reviews.length === 0) {
    return null;
  }

  const hasSingleTestimonial = testimonials.length === 1;

  return (
    <div className={cn("flex flex-col gap-10", className)}>
      {summary && summary.count > 0 ? (
        <div className="flex flex-wrap items-center gap-4">
          <Stars rating={Math.round(summary.average)} />
          <p className="text-sm text-ink-muted">
            <span className="font-display text-xl font-medium tabular-nums text-ink">
              {summary.average.toFixed(1)}
            </span>{" "}
            from {summary.count} {summary.count === 1 ? "review" : "reviews"}
          </p>
        </div>
      ) : null}

      {testimonials.length > 0 ? (
        <ul
          className={cn(
            hasSingleTestimonial
              ? "grid"
              : "columns-1 gap-5 md:columns-2 lg:columns-3",
          )}
        >
          {testimonials.map((testimonial, index) => (
            <li
              key={testimonial.id}
              className={cn(!hasSingleTestimonial && "mb-5 break-inside-avoid")}
            >
              {hasSingleTestimonial ? (
                <FeaturedTestimonial testimonial={testimonial} />
              ) : (
                <figure
                  className={`flex flex-col rounded-[1.75rem] p-8 ${index % 3 === 0 ? "bg-eucalyptus-900 text-sand-0" : index % 3 === 1 ? "bg-clay-50" : "border border-border bg-surface-raised"}`}
                >
                  <blockquote className="flex-1">
                    <p
                      className={`font-display text-xl leading-[1.5] font-medium [text-wrap:pretty] ${index % 3 === 0 ? "text-sand-0" : "text-ink"}`}
                    >
                      &ldquo;{testimonial.quote}&rdquo;
                    </p>
                  </blockquote>
                  <figcaption
                    className={`mt-8 text-sm ${index % 3 === 0 ? "text-eucalyptus-200" : "text-ink-muted"}`}
                  >
                    <span
                      className={`font-medium ${index % 3 === 0 ? "text-sand-0" : "text-ink"}`}
                    >
                      {displayAuthor(testimonial)}
                    </span>
                    {testimonial.authorRole ? (
                      <>
                        <span
                          aria-hidden="true"
                          className="mx-2 text-ink-faint"
                        >
                          ·
                        </span>
                        {testimonial.authorRole}
                      </>
                    ) : null}
                  </figcaption>
                </figure>
              )}
            </li>
          ))}
        </ul>
      ) : null}

      {reviews.length > 0 ? (
        <ul className="grid gap-6 md:grid-cols-2">
          {reviews.map((review) => (
            <li key={review.id}>
              <figure className="terios-quote-card flex h-full flex-col overflow-hidden border border-border/70 bg-surface-raised p-7">
                <Stars rating={review.rating} />
                {review.comment ? (
                  <blockquote className="mt-4 flex-1">
                    <p className="text-base leading-[1.6] text-ink-muted [text-wrap:pretty]">
                      {review.comment}
                    </p>
                  </blockquote>
                ) : (
                  <div className="flex-1" />
                )}
                <figcaption className="mt-5 text-[13px] text-ink-faint">
                  {/* First name only — that is all the API publishes. */}
                  <span className="font-medium text-ink-muted">
                    {review.authorName}
                  </span>
                  {review.serviceName ? (
                    <>
                      <span aria-hidden="true" className="mx-2">
                        ·
                      </span>
                      {review.serviceName}
                    </>
                  ) : null}
                </figcaption>
              </figure>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function FeaturedTestimonial({ testimonial }: { testimonial: Testimonial }) {
  const author = displayAuthor(testimonial);
  const initial = author.charAt(0).toUpperCase();

  return (
    <figure className="terios-featured-testimonial relative isolate grid min-h-[25rem] overflow-hidden bg-eucalyptus-900 text-sand-0 md:grid-cols-[minmax(0,1fr)_15rem]">
      <div className="relative z-10 p-7 sm:p-10 lg:p-14">
        <div>
          <div className="flex items-center gap-3 text-[0.7rem] font-semibold tracking-[0.2em] text-eucalyptus-200 uppercase">
            <span className="h-px w-8 bg-clay-300" aria-hidden="true" />
            Client reflection
          </div>
          <blockquote className="mt-8 max-w-[30ch]">
            <p className="font-display text-[clamp(1.35rem,2.6vw,2rem)] leading-[1.4] font-medium tracking-[-0.015em] text-sand-0 [text-wrap:pretty]">
              &ldquo;{testimonial.quote}&rdquo;
            </p>
          </blockquote>
        </div>
      </div>

      <figcaption className="relative z-10 flex items-center gap-4 border-t border-white/15 bg-white/[0.035] p-7 sm:p-10 md:flex-col md:items-start md:justify-end md:gap-0 md:border-t-0 md:border-l md:p-8">
        <AuthorSeal initial={initial} />
        <div className="md:mt-5">
          <AuthorDetails author={author} role={testimonial.authorRole} />
        </div>
        <p className="mt-7 hidden text-xs leading-relaxed tracking-[0.12em] text-eucalyptus-200 uppercase md:block">
          Shared from the Terios community
        </p>
      </figcaption>
    </figure>
  );
}

function AuthorSeal({ initial }: { initial: string }) {
  return (
    <span
      aria-hidden="true"
      className="flex size-14 shrink-0 items-center justify-center rounded-full border border-clay-300/60 bg-clay-300/10 font-display text-xl font-semibold text-clay-200"
    >
      {initial}
    </span>
  );
}

function AuthorDetails({ author, role }: { author: string; role?: string }) {
  return (
    <span className="flex flex-col gap-1">
      <span className="font-medium text-sand-0">{author}</span>
      {role ? (
        <span className="text-sm text-eucalyptus-200">{role}</span>
      ) : null}
    </span>
  );
}

function displayAuthor(testimonial: Testimonial): string {
  return testimonial.authorName.trim() || "Terios client";
}

/** Five stars with the filled count set by the rating. The rating is stated
 * in text for anyone not seeing the stars. */
function Stars({ rating }: { rating: number }) {
  const filled = Math.max(0, Math.min(5, rating));
  return (
    <p className="flex items-center gap-1">
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
    </p>
  );
}
