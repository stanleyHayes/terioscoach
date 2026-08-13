package review

import "errors"

// Domain errors for the reviews slice.
var (
	// ErrReviewNotFound means no review matches the lookup — including a
	// review that belongs to someone else, which must be indistinguishable
	// from one that does not exist.
	ErrReviewNotFound = errors.New("review not found")
	// ErrReviewExists signals the one-review-per-booking rule.
	ErrReviewExists = errors.New("this session has already been reviewed")
	// ErrAlreadyModerated means the review has left the pending state, so
	// its content can no longer be revised by the client.
	ErrAlreadyModerated = errors.New("a moderated review can no longer be edited")
	// ErrSessionNotComplete means the booking has not happened yet.
	ErrSessionNotComplete = errors.New("only a completed session can be reviewed")

	// Validation errors.
	ErrInvalidBooking = errors.New("a booking is required")
	ErrInvalidRating  = errors.New("rating must be between 1 and 5")
	ErrCommentTooLong = errors.New("comment is too long")
	ErrInvalidStatus  = errors.New("invalid review status")
)
