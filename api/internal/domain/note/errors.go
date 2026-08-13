package note

import "errors"

// Domain errors for the session-notes slice. Adapters and the HTTP layer
// map these to storage results and status codes via errors.Is.
var (
	// ErrNoteNotFound means no note matches the lookup key — unknown
	// booking, no note written yet, or (for client reads) a note that has
	// not been shared. The cases are indistinguishable on purpose.
	ErrNoteNotFound = errors.New("session note not found")
	// ErrNoteExists means a second note was created for the same booking
	// (storage duplicate-key on the bookingId unique index).
	ErrNoteExists = errors.New("a session note already exists for this booking")
	// ErrPrivateNotesTooLong means the private notes exceeded the limit.
	ErrPrivateNotesTooLong = errors.New("private notes must be at most 10000 characters")
	// ErrSharedFeedbackTooLong means the shared feedback exceeded the limit.
	ErrSharedFeedbackTooLong = errors.New("shared feedback must be at most 5000 characters")
	// ErrTooManyResources means more than MaxSharedResources were supplied.
	ErrTooManyResources = errors.New("at most 20 shared resources are allowed")
	// ErrResourceTooLong means a single resource entry exceeded the limit.
	ErrResourceTooLong = errors.New("shared resources must be at most 500 characters each")
)
