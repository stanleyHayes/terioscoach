// Package scheduling is the application service for the availability
// slice. It implements the inbound ports.SchedulingService port purely
// against outbound ports — no framework, driver, or transport imports.
package scheduling

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/catalog"
	domain "github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the availability use cases over outbound ports.
type Service struct {
	services     ports.ServiceRepository
	availability ports.AvailabilityRepository
	busy         ports.BusyIntervalReader
	now          func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.SchedulingService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. The service
// repository is needed because the public slots route keys off a service
// — its duration and owning practitioner drive the computation.
func NewService(
	services ports.ServiceRepository,
	availability ports.AvailabilityRepository,
	busy ports.BusyIntervalReader,
) *Service {
	return &Service{
		services:     services,
		availability: availability,
		busy:         busy,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// GetRules returns the practitioner's weekly schedule ordered by weekday.
func (s *Service) GetRules(ctx context.Context, practitionerID string) ([]domain.WeeklyRule, error) {
	return s.availability.GetRules(ctx, practitionerID)
}

// ReplaceRules validates the full weekly schedule, then swaps it in.
func (s *Service) ReplaceRules(ctx context.Context, practitionerID string, rules []domain.WeeklyRule) ([]domain.WeeklyRule, error) {
	timezone := domain.RulesTimezone(rules)
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, domain.ErrInvalidTimezone
	}
	for i := range rules {
		rules[i].PractitionerID = practitionerID
		rules[i].Timezone = timezone
	}
	if err := domain.ValidateRules(rules); err != nil {
		return nil, err
	}
	if err := s.availability.ReplaceRules(ctx, practitionerID, rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// AddTimeOff validates and persists a blocked period.
func (s *Service) AddTimeOff(ctx context.Context, practitionerID string, startAt, endAt time.Time, reason string) (domain.TimeOff, error) {
	off, err := domain.NewTimeOff(practitionerID, startAt, endAt, reason, s.now())
	if err != nil {
		return domain.TimeOff{}, err
	}
	return s.availability.CreateTimeOff(ctx, off)
}

// GetSlots computes bookable slots for an active service: the schedule is
// evaluated in the practitioner's authored availability timezone, then
// filtered to the viewer's requested calendar dates and emitted in UTC.
func (s *Service) GetSlots(ctx context.Context, serviceID string, from, to time.Time, tz string) (ports.SlotsResult, error) {
	viewerLoc, err := time.LoadLocation(tz)
	if err != nil {
		return ports.SlotsResult{}, domain.ErrInvalidTimezone
	}

	svc, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return ports.SlotsResult{}, err
	}
	if !svc.Active {
		// Inactive services are not publicly bookable — report as missing
		// rather than leaking that the id exists.
		return ports.SlotsResult{}, catalog.ErrServiceNotFound
	}

	// Fetch context slightly wider than the query range so intervals
	// overlapping its edges still block edge slots.
	queryFrom := from.Add(-48 * time.Hour)
	queryTo := to.Add(48 * time.Hour)

	rules, err := s.availability.GetRules(ctx, svc.PractitionerID)
	if err != nil {
		return ports.SlotsResult{}, err
	}
	scheduleTimezone := domain.RulesTimezone(rules)
	scheduleLoc, err := time.LoadLocation(scheduleTimezone)
	if err != nil {
		return ports.SlotsResult{}, domain.ErrInvalidTimezone
	}
	timeOff, err := s.availability.ListTimeOff(ctx, svc.PractitionerID, queryFrom, queryTo)
	if err != nil {
		return ports.SlotsResult{}, err
	}
	busy, err := s.busy.BusyIntervals(ctx, svc.PractitionerID, queryFrom, queryTo)
	if err != nil {
		return ports.SlotsResult{}, err
	}

	viewerFrom := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, viewerLoc)
	viewerToExclusive := time.Date(to.Year(), to.Month(), to.Day()+1, 0, 0, 0, 0, viewerLoc)
	slots, err := domain.GenerateSlots(domain.SlotRequest{
		Rules:           rules,
		TimeOff:         timeOff,
		Busy:            busy,
		DurationMinutes: svc.DurationMinutes,
		From:            viewerFrom.In(scheduleLoc).AddDate(0, 0, -1),
		To:              viewerToExclusive.In(scheduleLoc).AddDate(0, 0, 1),
		Loc:             scheduleLoc,
		Now:             s.now(),
	})
	if err != nil {
		return ports.SlotsResult{}, err
	}
	filtered := slots[:0]
	for _, slot := range slots {
		if !slot.Start.Before(viewerFrom) && slot.Start.Before(viewerToExclusive) {
			filtered = append(filtered, slot)
		}
	}
	return ports.SlotsResult{DurationMinutes: svc.DurationMinutes, Slots: filtered}, nil
}
