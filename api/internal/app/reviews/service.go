// Package reviews is the application service for the post-session review
// slice. It implements the inbound ports.ReviewService port purely against
// outbound ports — no framework, driver, or transport imports.
//
// Two rules shape everything here. A review must belong to a session that
// actually happened and to the person who attended it — proven against the
// booking, never taken from the request. And nothing reaches the public
// site unapproved, exactly as in the CMS slice.
package reviews

import (
	"context"
	"strings"
	"time"

	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/review"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultPublicLimit bounds the public review list.
const defaultPublicLimit = 20

// Service orchestrates the review use cases over outbound ports.
type Service struct {
	reviews  ports.ReviewRepository
	bookings ports.BookingRepository
	users    ports.UserRepository
	services ports.ServiceRepository
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.ReviewService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. The booking
// repository is what proves a review is earned; the user and service
// repositories only resolve display names for the public list.
func NewService(
	reviews ports.ReviewRepository,
	bookings ports.BookingRepository,
	users ports.UserRepository,
	services ports.ServiceRepository,
) *Service {
	return &Service{
		reviews:  reviews,
		bookings: bookings,
		users:    users,
		services: services,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Submit records a review of the caller's own completed session.
//
// Both guards matter. Ownership is checked against the booking's clientId,
// so a client cannot review a stranger's session — and a booking that is
// not theirs answers not-found, never "forbidden", which would confirm the
// booking exists. Completion is checked because a review is a verdict on
// something that happened; letting one be written in advance would make the
// public rating a wish list.
func (s *Service) Submit(ctx context.Context, clientID string, in ports.ReviewInput) (review.Review, error) {
	b, err := s.ownedBooking(ctx, clientID, in.BookingID)
	if err != nil {
		return review.Review{}, err
	}
	if b.Status != domainbooking.StatusCompleted {
		return review.Review{}, review.ErrSessionNotComplete
	}

	// The unique index is the race backstop; this is the friendly answer.
	if _, err := s.reviews.FindByBookingID(ctx, b.ID); err == nil {
		return review.Review{}, review.ErrReviewExists
	}

	r, err := review.New(b.ID, b.ClientID, b.PractitionerID, b.ServiceID, in.Rating, in.Comment, s.now())
	if err != nil {
		return review.Review{}, err
	}
	return s.reviews.Create(ctx, r)
}

// UpdateMine revises the caller's own review while it is still pending.
func (s *Service) UpdateMine(ctx context.Context, clientID, reviewID string, patch review.Patch) (review.Review, error) {
	r, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return review.Review{}, err
	}
	if r.ClientID != clientID {
		return review.Review{}, review.ErrReviewNotFound
	}
	if err := r.Apply(patch, s.now()); err != nil {
		return review.Review{}, err
	}
	return s.reviews.Update(ctx, r)
}

// ListMine returns the caller's own reviews, whatever their state — a
// client can see that their own review is still awaiting moderation.
func (s *Service) ListMine(ctx context.Context, clientID string) ([]review.Review, error) {
	return s.reviews.ListByClient(ctx, clientID)
}

// ListForPractitioner returns the moderation queue for the practice.
func (s *Service) ListForPractitioner(ctx context.Context, practitionerID string, filter ports.ReviewFilter) ([]review.Review, error) {
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, review.ErrInvalidStatus
	}
	return s.reviews.ListByPractitioner(ctx, practitionerID, filter)
}

// Moderate approves or rejects a review on the practitioner's own bookings.
func (s *Service) Moderate(ctx context.Context, practitionerID, reviewID string, approve bool) (review.Review, error) {
	r, err := s.reviews.FindByID(ctx, reviewID)
	if err != nil {
		return review.Review{}, err
	}
	if r.PractitionerID != practitionerID {
		return review.Review{}, review.ErrReviewNotFound
	}
	if approve {
		r.Approve(s.now())
	} else {
		r.Reject(s.now())
	}
	return s.reviews.Update(ctx, r)
}

// PublicReviews returns approved reviews with display names resolved.
//
// The author is shown by first name only. A review is a person saying
// something about their own health care under their own name: the full
// name plus a service and a date is more identifying than the practice
// needs to publish, and the client never chose to publish it.
func (s *Service) PublicReviews(ctx context.Context, limit int) ([]ports.PublicReview, error) {
	if limit <= 0 || limit > defaultPublicLimit {
		limit = defaultPublicLimit
	}
	reviews, err := s.reviews.ListPublic(ctx, limit)
	if err != nil {
		return nil, err
	}

	out := make([]ports.PublicReview, 0, len(reviews))
	for _, r := range reviews {
		// Belt and braces: the repository filtered, and so does this. A
		// storage bug must not put an unapproved review on the site.
		if !r.Public() {
			continue
		}
		out = append(out, ports.PublicReview{
			ID:          r.ID,
			AuthorName:  s.displayName(ctx, r.ClientID),
			ServiceName: s.serviceName(ctx, r.ServiceID),
			Rating:      r.Rating,
			Comment:     r.Comment,
			CreatedAt:   r.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// PublicSummary aggregates the approved reviews.
func (s *Service) PublicSummary(ctx context.Context) (review.Summary, error) {
	// The summary counts every approved review, not just the page-worth
	// the list shows, so "4.6 from 38 reviews" stays true.
	reviews, err := s.reviews.ListPublic(ctx, 0)
	if err != nil {
		return review.Summary{}, err
	}
	return review.Summarize(reviews), nil
}

// ownedBooking loads a booking that belongs to the client. Someone else's
// booking is reported as missing — the same isolation rule as the booking
// slice, with no existence leak.
func (s *Service) ownedBooking(ctx context.Context, clientID, bookingID string) (domainbooking.Booking, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domainbooking.Booking{}, err
	}
	if b.ClientID != clientID {
		return domainbooking.Booking{}, domainbooking.ErrBookingNotFound
	}
	return b, nil
}

// displayName resolves a client's first name for public display, falling
// back to a neutral label when the account is gone.
func (s *Service) displayName(ctx context.Context, clientID string) string {
	user, err := s.users.FindByID(ctx, clientID)
	if err != nil || strings.TrimSpace(user.Name) == "" {
		return "A client"
	}
	fields := strings.Fields(user.Name)
	return fields[0]
}

// serviceName resolves the reviewed service's name, empty when it is gone.
func (s *Service) serviceName(ctx context.Context, serviceID string) string {
	svc, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return ""
	}
	return svc.Name
}
