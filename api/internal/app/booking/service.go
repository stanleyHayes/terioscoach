// Package booking is the application service for the booking slice. It
// implements the inbound ports.BookingService port purely against outbound
// ports — no framework, driver, or transport imports.
package booking

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the booking use cases over outbound ports.
type Service struct {
	bookings     ports.BookingRepository
	services     ports.ServiceRepository
	availability ports.AvailabilityRepository
	busy         ports.BusyIntervalReader
	policy       booking.ReschedulePolicy
	now          func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.BookingService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. The scheduling
// ports (availability rules, time-off, busy intervals) are shared with the
// availability slice: a booking is only created when the slot engine would
// have offered the slot.
func NewService(
	bookings ports.BookingRepository,
	services ports.ServiceRepository,
	availability ports.AvailabilityRepository,
	busy ports.BusyIntervalReader,
	policy booking.ReschedulePolicy,
) *Service {
	return &Service{
		bookings:     bookings,
		services:     services,
		availability: availability,
		busy:         busy,
		policy:       policy,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// CreateBooking books a slot the availability engine would actually offer —
// not merely a free one. The storage unique index is the race backstop: a
// concurrent claim of the same slot surfaces as ErrSlotUnavailable.
func (s *Service) CreateBooking(ctx context.Context, clientID, serviceID string, startAt time.Time, tz string) (booking.Booking, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return booking.Booking{}, scheduling.ErrInvalidTimezone
	}
	svc, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return booking.Booking{}, err
	}
	if !svc.Active {
		// Inactive services are not bookable — report as missing rather
		// than leaking that the id exists (mirrors GetSlots).
		return booking.Booking{}, catalog.ErrServiceNotFound
	}
	if err := s.assertSlotGeneratable(ctx, svc.PractitionerID, svc.DurationMinutes, startAt, loc, ""); err != nil {
		return booking.Booking{}, err
	}

	b, err := booking.New(clientID, svc.PractitionerID, serviceID, startAt, svc.DurationMinutes, s.now())
	if err != nil {
		return booking.Booking{}, err
	}
	return s.bookings.Create(ctx, b)
}

// ListMine returns the client's own bookings, upcoming and past.
func (s *Service) ListMine(ctx context.Context, clientID string) ([]booking.Booking, error) {
	return s.bookings.ListByClient(ctx, clientID)
}

// ListForPractitioner returns the practitioner's bookings, optionally
// narrowed by range and status.
func (s *Service) ListForPractitioner(ctx context.Context, practitionerID string, filter ports.BookingFilter) ([]booking.Booking, error) {
	return s.bookings.ListByPractitioner(ctx, practitionerID, filter)
}

// GetBooking returns one booking to its owner or its practitioner.
func (s *Service) GetBooking(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if !authorized(id, b) {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

// RescheduleBooking moves a booking to a new slot, validated exactly like a
// create. The booking's own current interval is excluded from the busy set
// so it cannot block its own neighbours; the update claims the new slot and
// frees the old one in a single write.
func (s *Service) RescheduleBooking(ctx context.Context, id identity.Identity, bookingID string, startAt time.Time, tz string) (booking.Booking, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return booking.Booking{}, scheduling.ErrInvalidTimezone
	}
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := s.checkCutoff(id, b); err != nil {
		return booking.Booking{}, err
	}
	// The duration never changes — same service, different slot.
	duration := int(b.EndAt.Sub(b.StartAt) / time.Minute)
	if err := s.assertSlotGeneratable(ctx, b.PractitionerID, duration, startAt, loc, b.ID); err != nil {
		return booking.Booking{}, err
	}
	if err := b.Reschedule(startAt, s.now()); err != nil {
		return booking.Booking{}, err
	}
	return s.bookings.Update(ctx, b)
}

// CancelBooking cancels a booking, freeing its slot immediately.
func (s *Service) CancelBooking(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error) {
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := s.checkCutoff(id, b); err != nil {
		return booking.Booking{}, err
	}
	if err := b.Cancel(s.now()); err != nil {
		return booking.Booking{}, err
	}
	return s.bookings.Update(ctx, b)
}

// CompleteBooking marks a booking completed — practitioner-only, after the
// appointment has ended.
func (s *Service) CompleteBooking(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error) {
	b, err := s.loadForPractitioner(ctx, practitionerID, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := b.Complete(s.now()); err != nil {
		return booking.Booking{}, err
	}
	return s.bookings.Update(ctx, b)
}

// MarkNoShow marks a booking no_show — practitioner-only, after the
// appointment has started.
func (s *Service) MarkNoShow(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error) {
	b, err := s.loadForPractitioner(ctx, practitionerID, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := b.MarkNoShow(s.now()); err != nil {
		return booking.Booking{}, err
	}
	return s.bookings.Update(ctx, b)
}

// authorized reports whether the principal may touch the booking: the
// owning client, or the practitioner it belongs to.
func authorized(id identity.Identity, b booking.Booking) bool {
	switch id.Role {
	case identity.RoleClient:
		return b.ClientID == id.UserID
	case identity.RolePractitioner:
		return b.PractitionerID == id.UserID
	}
	return false
}

// loadAuthorized fetches a booking and enforces ownership. Cross-owner
// access is reported as not-found — no existence leak.
func (s *Service) loadAuthorized(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if !authorized(id, b) {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

// loadForPractitioner fetches a booking owned by the practitioner.
func (s *Service) loadForPractitioner(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if b.PractitionerID != practitionerID {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

// checkCutoff enforces the client modification cutoff. Practitioners skip
// it; the cutoff is measured against the booking's current startAt so a
// client cannot hop a booking around inside the lockout window.
func (s *Service) checkCutoff(id identity.Identity, b booking.Booking) error {
	if id.Role == identity.RoleClient && !s.policy.ClientCanModify(b.StartAt, s.now()) {
		return booking.ErrCutoffPassed
	}
	return nil
}

// assertSlotGeneratable proves startAt is a slot the availability engine
// would return for the practitioner/duration on that day — same windows,
// step, time-off, and busy rules as the public slots route. excludeBookingID
// lets a reschedule ignore its own current interval.
func (s *Service) assertSlotGeneratable(
	ctx context.Context,
	practitionerID string,
	durationMinutes int,
	startAt time.Time,
	loc *time.Location,
	excludeBookingID string,
) error {
	startAt = startAt.UTC()
	// Fetch context wider than the target day so intervals overlapping its
	// edges still block edge slots (same convention as GetSlots).
	queryFrom := startAt.Add(-48 * time.Hour)
	queryTo := startAt.Add(48 * time.Hour)

	rules, err := s.availability.GetRules(ctx, practitionerID)
	if err != nil {
		return err
	}
	timeOff, err := s.availability.ListTimeOff(ctx, practitionerID, queryFrom, queryTo)
	if err != nil {
		return err
	}
	busy, err := s.busy.BusyIntervals(ctx, practitionerID, queryFrom, queryTo)
	if err != nil {
		return err
	}

	if excludeBookingID != "" {
		own, err := s.bookings.FindByID(ctx, excludeBookingID)
		if err != nil {
			return err
		}
		filtered := busy[:0]
		for _, iv := range busy {
			if iv.Start.Equal(own.StartAt) && iv.End.Equal(own.EndAt) {
				continue // the booking being rescheduled cannot block itself
			}
			filtered = append(filtered, iv)
		}
		busy = filtered
	}

	slots, err := scheduling.GenerateSlots(scheduling.SlotRequest{
		Rules:           rules,
		TimeOff:         timeOff,
		Busy:            busy,
		DurationMinutes: durationMinutes,
		From:            startAt,
		To:              startAt,
		Loc:             loc,
		Now:             s.now(),
	})
	if err != nil {
		return err
	}
	for _, slot := range slots {
		if slot.Start.Equal(startAt) {
			return nil
		}
	}
	return booking.ErrSlotUnavailable
}
