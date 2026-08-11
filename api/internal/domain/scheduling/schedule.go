// Package scheduling is the domain core for availability: weekly opening
// rules, time-off, and the slot-generation engine. It imports nothing
// outside the standard library — no frameworks, no drivers.
package scheduling

import "time"

const (
	// minutesPerDay bounds window endpoints; a window may end at midnight
	// (1440) but never cross it — overnight windows are rejected.
	minutesPerDay = 24 * 60

	maxBufferMinutes = 120

	// MaxSlotRangeDays caps how far a single slot query may span.
	MaxSlotRangeDays = 62
)

// Window is a contiguous opening span within one local day, expressed as
// minutes since local midnight. EndMin may be 1440 (midnight close).
type Window struct {
	StartMin int
	EndMin   int
}

// Validate enforces 0 <= StartMin < EndMin <= 1440.
func (w Window) Validate() error {
	if w.StartMin < 0 || w.EndMin > minutesPerDay || w.StartMin >= w.EndMin {
		return ErrInvalidWindow
	}
	return nil
}

// WeeklyRule is the opening schedule for one weekday. A weekday with no
// rule is closed. BufferMinutes is the recovery gap the slot engine keeps
// free around every busy interval on this weekday.
type WeeklyRule struct {
	PractitionerID string
	Weekday        time.Weekday
	Windows        []Window
	BufferMinutes  int
}

// Validate enforces weekday, window, and buffer invariants: windows
// sorted and non-overlapping, buffer within bounds.
func (r WeeklyRule) Validate() error {
	if r.Weekday < time.Sunday || r.Weekday > time.Saturday {
		return ErrInvalidWeekday
	}
	if r.BufferMinutes < 0 || r.BufferMinutes > maxBufferMinutes {
		return ErrInvalidBuffer
	}
	prevEnd := -1
	for _, w := range r.Windows {
		if err := w.Validate(); err != nil {
			return err
		}
		if w.StartMin < prevEnd {
			return ErrInvalidWindow
		}
		prevEnd = w.EndMin
	}
	return nil
}

// ValidateRules validates a full weekly schedule: every rule valid and at
// most one rule per weekday.
func ValidateRules(rules []WeeklyRule) error {
	seen := make(map[time.Weekday]bool, len(rules))
	for _, r := range rules {
		if err := r.Validate(); err != nil {
			return err
		}
		if seen[r.Weekday] {
			return ErrDuplicateWeekday
		}
		seen[r.Weekday] = true
	}
	return nil
}

// Interval is a half-open UTC time span [Start, End). Bookings (BE-05)
// reach the slot engine purely as busy Intervals — the engine never sees
// the booking entity.
type Interval struct {
	Start time.Time
	End   time.Time
}

// Overlaps reports whether two half-open intervals share any time.
func (i Interval) Overlaps(o Interval) bool {
	return i.Start.Before(o.End) && o.Start.Before(i.End)
}

// TimeOff is a practitioner-blocked period (holiday, appointment, ...).
type TimeOff struct {
	ID             string
	PractitionerID string
	StartAt        time.Time
	EndAt          time.Time
	Reason         string
	CreatedAt      time.Time
}

// NewTimeOff validates the range and builds a TimeOff.
func NewTimeOff(practitionerID string, startAt, endAt time.Time, reason string, now time.Time) (TimeOff, error) {
	if !startAt.Before(endAt) {
		return TimeOff{}, ErrInvalidTimeOffRange
	}
	return TimeOff{
		PractitionerID: practitionerID,
		StartAt:        startAt.UTC(),
		EndAt:          endAt.UTC(),
		Reason:         reason,
		CreatedAt:      now.UTC(),
	}, nil
}
