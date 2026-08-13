// Package note is the domain core for session notes: the practitioner's
// treatment record for one booking, split into private notes and
// client-shareable feedback, plus the sharing visibility rule. It imports
// nothing outside the standard library — no frameworks, no drivers.
package note

import (
	"time"
	"unicode/utf8"
)

// Content limits for note fields.
const (
	MaxPrivateNotesLen   = 10000
	MaxSharedFeedbackLen = 5000
	MaxSharedResources   = 20
	MaxResourceLen       = 500
)

// SessionNote is the practitioner's record of one appointment. Exactly one
// note exists per booking (the storage layer enforces it with a unique
// index on bookingId). ClientID is denormalized from the booking so
// client-scoped history queries can lead with it (isolation), matching the
// session_notes index design. The content splits into two trust zones:
// PrivateNotes is practitioner-only forever; SharedFeedback and
// SharedResources become visible to the client once SharedAt is set.
type SessionNote struct {
	ID              string
	BookingID       string
	ClientID        string
	PractitionerID  string
	PrivateNotes    string
	SharedFeedback  string
	SharedResources []string
	SharedAt        *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// New builds an empty note for a booking. ClientID and PractitionerID come
// from the booking — the caller (app layer) has already proven ownership.
func New(bookingID, clientID, practitionerID string, now time.Time) SessionNote {
	now = now.UTC()
	return SessionNote{
		BookingID:       bookingID,
		ClientID:        clientID,
		PractitionerID:  practitionerID,
		SharedResources: []string{},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

// ReplaceContent upserts the three writable fields wholesale (PUT
// semantics). It deliberately never touches SharedAt: editing after sharing
// neither unshares nor re-stamps.
func (n *SessionNote) ReplaceContent(privateNotes, sharedFeedback string, sharedResources []string, now time.Time) error {
	if utf8.RuneCountInString(privateNotes) > MaxPrivateNotesLen {
		return ErrPrivateNotesTooLong
	}
	if utf8.RuneCountInString(sharedFeedback) > MaxSharedFeedbackLen {
		return ErrSharedFeedbackTooLong
	}
	if len(sharedResources) > MaxSharedResources {
		return ErrTooManyResources
	}
	for _, r := range sharedResources {
		if utf8.RuneCountInString(r) > MaxResourceLen {
			return ErrResourceTooLong
		}
	}
	n.PrivateNotes = privateNotes
	n.SharedFeedback = sharedFeedback
	n.SharedResources = append([]string{}, sharedResources...)
	n.UpdatedAt = now.UTC()
	return nil
}

// Share makes the shared fields visible to the client by stamping SharedAt.
// It is idempotent and one-way: a repeat call leaves the original stamp
// untouched, and there is no unshare.
func (n *SessionNote) Share(now time.Time) {
	if n.SharedAt != nil {
		return
	}
	stamped := now.UTC()
	n.SharedAt = &stamped
	n.UpdatedAt = stamped
}

// IsShared reports whether the shared fields are visible to the client.
func (n SessionNote) IsShared() bool {
	return n.SharedAt != nil
}

// SharedContent is the client-visible projection of a note. PrivateNotes is
// absent by construction — it cannot leak through this shape.
type SharedContent struct {
	BookingID string
	Feedback  string
	Resources []string
	SharedAt  time.Time
}

// ClientView is the visibility rule as domain logic: the client sees the
// shared content only once the practitioner has shared the note. The second
// return value is false while the note is unshared — the caller reports
// that as not-found, so an unshared note is indistinguishable from no note.
func (n SessionNote) ClientView() (SharedContent, bool) {
	if n.SharedAt == nil {
		return SharedContent{}, false
	}
	return SharedContent{
		BookingID: n.BookingID,
		Feedback:  n.SharedFeedback,
		Resources: append([]string{}, n.SharedResources...),
		SharedAt:  *n.SharedAt,
	}, true
}
