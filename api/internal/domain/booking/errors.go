package booking

import "errors"

// Domain errors for the booking slice. Adapters and the HTTP layer map
// these to storage results and status codes via errors.Is.
var (
	// ErrBookingNotFound means no booking matches the lookup key (unknown
	// id, or owned by someone else — no cross-tenant leak).
	ErrBookingNotFound = errors.New("booking not found")
	// ErrSlotUnavailable means the requested start time is not a slot the
	// availability engine would offer, or a concurrent booking claimed it
	// first (storage duplicate-key on the confirmed-slot index).
	ErrSlotUnavailable = errors.New("slot is no longer available")
	// ErrCutoffPassed means a client tried to reschedule or cancel inside
	// the practice's modification cutoff.
	ErrCutoffPassed = errors.New("the modification cutoff for this booking has passed")
	// ErrInvalidTransition means a lifecycle change was attempted from a
	// terminal status, or on a booking in the wrong state.
	ErrInvalidTransition = errors.New("booking cannot change status from its current state")
	// ErrTooEarly means complete/no-show was attempted before the
	// appointment had ended/started.
	ErrTooEarly = errors.New("too early for this status change")
	// ErrInvalidDuration means a non-positive session duration was supplied.
	ErrInvalidDuration = errors.New("duration must be positive")
)
