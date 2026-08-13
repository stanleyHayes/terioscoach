package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/review"
)

// ReviewFilter narrows a review listing.
type ReviewFilter struct {
	// Status, when set, narrows to one moderation state.
	Status review.Status
	// ApprovedOnly is the public read. It is separate from Status so a
	// public listing cannot be widened by passing a status.
	ApprovedOnly bool
}

// ReviewRepository is the outbound port for post-session reviews.
type ReviewRepository interface {
	// Create persists a new review. A second review for the same booking
	// returns review.ErrReviewExists (enforced by a unique index).
	Create(ctx context.Context, r review.Review) (review.Review, error)
	Update(ctx context.Context, r review.Review) (review.Review, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return review.ErrReviewNotFound.
	FindByID(ctx context.Context, id string) (review.Review, error)
	// FindByBookingID misses return review.ErrReviewNotFound.
	FindByBookingID(ctx context.Context, bookingID string) (review.Review, error)
	// ListByClient returns one client's own reviews, newest first.
	ListByClient(ctx context.Context, clientID string) ([]review.Review, error)
	// ListByPractitioner returns the practice's reviews, newest first.
	ListByPractitioner(ctx context.Context, practitionerID string, filter ReviewFilter) ([]review.Review, error)
	// ListPublic returns approved reviews for the public site.
	ListPublic(ctx context.Context, limit int) ([]review.Review, error)
}

// ReviewInput is the client's submission.
type ReviewInput struct {
	BookingID string
	Rating    int
	Comment   string
}

// PublicReview is one approved review as the public site shows it: the
// verdict and the author's first name, never the client's id or address.
type PublicReview struct {
	ID          string
	AuthorName  string
	ServiceName string
	Rating      int
	Comment     string
	CreatedAt   string
}

// ReviewService is the inbound port for the reviews slice (BE-14).
type ReviewService interface {
	// Submit records a client's review of their own completed session.
	Submit(ctx context.Context, clientID string, in ReviewInput) (review.Review, error)
	// UpdateMine revises the caller's own review while it is still pending.
	UpdateMine(ctx context.Context, clientID, reviewID string, patch review.Patch) (review.Review, error)
	// ListMine returns the caller's own reviews.
	ListMine(ctx context.Context, clientID string) ([]review.Review, error)
	// ListForPractitioner returns the moderation queue.
	ListForPractitioner(ctx context.Context, practitionerID string, filter ReviewFilter) ([]review.Review, error)
	// Moderate approves or rejects a review on the practitioner's own
	// bookings. Someone else's review is not-found, not forbidden.
	Moderate(ctx context.Context, practitionerID, reviewID string, approve bool) (review.Review, error)
	// PublicReviews returns approved reviews with display names resolved.
	PublicReviews(ctx context.Context, limit int) ([]PublicReview, error)
	// PublicSummary is the aggregate shown beside the list.
	PublicSummary(ctx context.Context) (review.Summary, error)
}
