package client

import "errors"

// Domain errors for the client-records slice. Adapters and the HTTP layer
// map these to storage results and status codes via errors.Is.
var (
	// ErrClientNotFound means no client matches the lookup key (unknown user
	// id, or a user with no bookings at this practice — no cross-practice
	// leak).
	ErrClientNotFound = errors.New("client not found")
	// ErrProfileNotFound means no practice profile exists yet for the user
	// (it is created by the first practitioner PATCH).
	ErrProfileNotFound = errors.New("client profile not found")
	// ErrPhoneTooLong means the phone field exceeded MaxPhoneLen.
	ErrPhoneTooLong = errors.New("phone must be at most 40 characters")
	// ErrPracticeNotesTooLong means the summary exceeded MaxPracticeNotesLen.
	ErrPracticeNotesTooLong = errors.New("practice notes must be at most 5000 characters")
	// ErrTooManyTags means more than MaxTags tags were supplied.
	ErrTooManyTags = errors.New("at most 20 tags are allowed")
	// ErrTagTooLong means a single tag exceeded MaxTagLen.
	ErrTagTooLong = errors.New("tags must be at most 40 characters each")
)
