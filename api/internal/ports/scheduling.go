package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/scheduling"
)

// SlotsResult is the outcome of a slot computation: the service's session
// length plus the bookable intervals (possibly empty).
type SlotsResult struct {
	DurationMinutes int
	Slots           []scheduling.Interval
}

// SchedulingService is the inbound port for the availability slice.
type SchedulingService interface {
	// GetRules returns the practitioner's weekly schedule, ordered by
	// weekday. Weekdays without a rule are closed.
	GetRules(ctx context.Context, practitionerID string) ([]scheduling.WeeklyRule, error)
	// ReplaceRules validates and atomically replaces the whole weekly
	// schedule (PUT semantics). Returns the stored set.
	ReplaceRules(ctx context.Context, practitionerID string, rules []scheduling.WeeklyRule) ([]scheduling.WeeklyRule, error)
	// AddTimeOff blocks a period out of the schedule.
	AddTimeOff(ctx context.Context, practitionerID string, startAt, endAt time.Time, reason string) (scheduling.TimeOff, error)
	// GetSlots computes bookable slots for a service. tz is an IANA name;
	// misses on the service (unknown or inactive) return
	// catalog.ErrServiceNotFound.
	GetSlots(ctx context.Context, serviceID string, from, to time.Time, tz string) (SlotsResult, error)
}

// AvailabilityRepository is the outbound port for weekly rules and
// time-off persistence.
type AvailabilityRepository interface {
	// GetRules returns the practitioner's rules ordered by weekday.
	GetRules(ctx context.Context, practitionerID string) ([]scheduling.WeeklyRule, error)
	// ReplaceRules swaps the full weekly schedule for a practitioner.
	ReplaceRules(ctx context.Context, practitionerID string, rules []scheduling.WeeklyRule) error
	// CreateTimeOff persists a blocked period, assigning its ID.
	CreateTimeOff(ctx context.Context, t scheduling.TimeOff) (scheduling.TimeOff, error)
	// ListTimeOff returns the blocked periods overlapping [from, to) as
	// plain intervals.
	ListTimeOff(ctx context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error)
}

// BusyIntervalReader is the outbound port that feeds existing bookings
// into slot generation as plain busy intervals. The bookings schema lands
// in BE-05; until then implementations read an empty collection.
type BusyIntervalReader interface {
	// BusyIntervals returns booked intervals overlapping [from, to).
	BusyIntervals(ctx context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error)
}
