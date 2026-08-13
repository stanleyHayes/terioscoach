// Package review is the domain core for post-session reviews: the rating
// and comment a client leaves after a completed session, and the moderation
// state that decides whether it appears on the public site. It imports
// nothing outside the standard library — no frameworks, no drivers.
//
// A review is client-authored content published under the practice's name,
// so it carries the same approve-before-publish rule as a testimonial, plus
// one the CMS does not need: it must be attached to a session that actually
// happened.
package review

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Rating bounds and comment limit.
const (
	MinRating     = 1
	MaxRating     = 5
	MaxCommentLen = 2000
)

// Status is the moderation state of a review.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected:
		return true
	}
	return false
}

// Review is one client's verdict on one session. The parties and the
// service are denormalized from the booking so the practice can report on
// ratings per service without a join, and so a review survives the
// service's later renaming or deletion.
type Review struct {
	ID             string
	BookingID      string
	ClientID       string
	PractitionerID string
	ServiceID      string
	Rating         int
	Comment        string
	Status         Status
	ModeratedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// New builds a pending review. Whether the booking may be reviewed at all
// is the application layer's call — it holds the booking; the domain's job
// is the content rules and the fact that nothing starts approved.
func New(bookingID, clientID, practitionerID, serviceID string, rating int, comment string, now time.Time) (Review, error) {
	if bookingID == "" || clientID == "" {
		return Review{}, ErrInvalidBooking
	}
	if err := ValidateRating(rating); err != nil {
		return Review{}, err
	}
	comment = strings.TrimSpace(comment)
	if utf8.RuneCountInString(comment) > MaxCommentLen {
		return Review{}, ErrCommentTooLong
	}
	now = now.UTC()
	return Review{
		BookingID:      bookingID,
		ClientID:       clientID,
		PractitionerID: practitionerID,
		ServiceID:      serviceID,
		Rating:         rating,
		Comment:        comment,
		Status:         StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Patch is the set of fields a client may revise on their own review.
type Patch struct {
	Rating  *int
	Comment *string
}

// Apply revises the review's content.
//
// It is allowed only while the review is still pending. Once a review is
// live on the public site, silently swapping five stars for one — or a kind
// comment for an abusive one — would publish text nobody approved. A client
// who wants to change an approved review has to have it moderated again,
// which is the practitioner's call, not a side effect of an edit.
func (r *Review) Apply(patch Patch, now time.Time) error {
	if r.Status != StatusPending {
		return ErrAlreadyModerated
	}
	if patch.Rating != nil {
		if err := ValidateRating(*patch.Rating); err != nil {
			return err
		}
		r.Rating = *patch.Rating
	}
	if patch.Comment != nil {
		comment := strings.TrimSpace(*patch.Comment)
		if utf8.RuneCountInString(comment) > MaxCommentLen {
			return ErrCommentTooLong
		}
		r.Comment = comment
	}
	r.UpdatedAt = now.UTC()
	return nil
}

// Approve publishes the review. It is idempotent and keeps the first
// moderation stamp.
func (r *Review) Approve(now time.Time) {
	now = now.UTC()
	if r.Status != StatusApproved {
		r.Status = StatusApproved
		if r.ModeratedAt == nil {
			r.ModeratedAt = &now
		}
	}
	r.UpdatedAt = now
}

// Reject keeps the review off the site. Like testimonial moderation it is
// reversible — a practitioner who rejects in haste can approve later.
func (r *Review) Reject(now time.Time) {
	now = now.UTC()
	r.Status = StatusRejected
	if r.ModeratedAt == nil {
		r.ModeratedAt = &now
	}
	r.UpdatedAt = now
}

// Public reports whether the review may be shown to an anonymous visitor.
func (r Review) Public() bool { return r.Status == StatusApproved }

// ValidateRating enforces the 1–5 scale.
func ValidateRating(rating int) error {
	if rating < MinRating || rating > MaxRating {
		return ErrInvalidRating
	}
	return nil
}

// Summary is the aggregate the public site and the reporting slice show:
// how many approved reviews there are and what they average.
type Summary struct {
	Count   int
	Average float64
	// Distribution counts approved reviews per star, indexed 1–5.
	Distribution map[int]int
}

// Summarize aggregates approved reviews only — an average that counted
// unmoderated or rejected reviews would not match the list beside it.
// Average is rounded to one decimal, which is all a star display uses.
func Summarize(reviews []Review) Summary {
	summary := Summary{Distribution: map[int]int{}}
	total := 0
	for _, r := range reviews {
		if !r.Public() {
			continue
		}
		summary.Count++
		total += r.Rating
		summary.Distribution[r.Rating]++
	}
	if summary.Count > 0 {
		summary.Average = float64(int(float64(total)/float64(summary.Count)*10+0.5)) / 10
	}
	return summary
}
