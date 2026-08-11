package scheduling

import "errors"

// Domain errors for the scheduling slice. Adapters and the HTTP layer map
// these to storage results and status codes via errors.Is.
var (
	// ErrInvalidWeekday means a weekday outside 0 (Sunday) - 6 (Saturday).
	ErrInvalidWeekday = errors.New("weekday must be between 0 (Sunday) and 6 (Saturday)")
	// ErrInvalidWindow means a window failed validation: out of range,
	// start not before end, overnight, or overlapping a sibling window.
	ErrInvalidWindow = errors.New("invalid window: require 0 <= startMin < endMin <= 1440, sorted and non-overlapping")
	// ErrInvalidBuffer means bufferMinutes fell outside 0-120.
	ErrInvalidBuffer = errors.New("buffer must be between 0 and 120 minutes")
	// ErrDuplicateWeekday means two rules were supplied for the same weekday.
	ErrDuplicateWeekday = errors.New("duplicate weekday rule")
	// ErrInvalidTimeOffRange means a time-off range was empty or inverted.
	ErrInvalidTimeOffRange = errors.New("time-off start must be before end")
	// ErrInvalidDuration means a non-positive session duration was supplied.
	ErrInvalidDuration = errors.New("duration must be positive")
	// ErrInvalidRange means the slot query range was inverted or too long.
	ErrInvalidRange = errors.New("invalid date range")
	// ErrInvalidTimezone means a tz name time.LoadLocation cannot resolve.
	ErrInvalidTimezone = errors.New("invalid timezone")
)
