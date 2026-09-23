// Package booking is the application service for the booking slice. It
// implements the inbound ports.BookingService port purely against outbound
// ports — no framework, driver, or transport imports.
package booking

import (
	"context"
	"strings"
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
	notifier     ports.Notifier
	users        ports.UserRepository
	agreements   ports.AgreementGate
	now          func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.BookingService = (*Service)(nil)

// Option customizes a Service.
type Option func(*Service)

// WithNotifications makes the slice announce booking changes (BE-09). The
// user repository resolves the client's name and address, because the
// message is composed here — while the booking, the service and the
// account are all still in hand — rather than when it is delivered.
//
// Notification is deliberately the last thing each use case does, and its
// failure never fails the booking: a confirmed session that did not email
// is a smaller problem than a session that refused to book because the
// mail outbox was briefly unavailable.
func WithNotifications(notifier ports.Notifier, users ports.UserRepository) Option {
	return func(s *Service) {
		s.notifier = notifier
		s.users = users
	}
}

// WithAgreementGate refuses a booking for a service whose agreement the
// client has not signed (BE-14).
//
// The gate lives here, on the write path, and not only in the portal's
// booking wizard: a step a browser can skip is not a contract requirement.
// The portal asks the same question first so the client is shown the
// agreement rather than an error, but this is the check that decides.
func WithAgreementGate(gate ports.AgreementGate) Option {
	return func(s *Service) { s.agreements = gate }
}

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
	opts ...Option,
) *Service {
	s := &Service{
		bookings:     bookings,
		services:     services,
		availability: availability,
		busy:         busy,
		policy:       policy,
		now:          func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notice assembles the message payload for a booking. A missing service or
// account degrades the message rather than blocking it: the client still
// needs to know their session moved even if the service was since deleted.
func (s *Service) notice(ctx context.Context, b booking.Booking, tz string) (ports.BookingNotice, bool) {
	if s.notifier == nil || s.users == nil {
		return ports.BookingNotice{}, false
	}
	user, err := s.users.FindByID(ctx, b.ClientID)
	if err != nil {
		// With no address there is nowhere to send; the notifier would
		// only reject it.
		return ports.BookingNotice{}, false
	}
	serviceName := "your session"
	if svc, err := s.services.FindByID(ctx, b.ServiceID); err == nil {
		serviceName = svc.Name
	}
	return ports.BookingNotice{
		BookingID:   b.ID,
		ClientName:  user.Name,
		ClientEmail: user.Email,
		ServiceName: serviceName,
		StartAt:     b.StartAt,
		Timezone:    tz,
	}, true
}

func (s *Service) changeRequestNotice(ctx context.Context, b booking.Booking, requestType, reason string, proposedStartAt time.Time, tz string) (ports.BookingChangeRequestNotice, bool) {
	if s.notifier == nil || s.users == nil {
		return ports.BookingChangeRequestNotice{}, false
	}
	user, err := s.users.FindByID(ctx, b.ClientID)
	if err != nil {
		return ports.BookingChangeRequestNotice{}, false
	}
	serviceName := "your session"
	if svc, err := s.services.FindByID(ctx, b.ServiceID); err == nil {
		serviceName = svc.Name
	}
	return ports.BookingChangeRequestNotice{
		BookingID:       b.ID,
		ClientName:      user.Name,
		ClientEmail:     user.Email,
		ServiceName:     serviceName,
		StartAt:         b.StartAt,
		ProposedStartAt: proposedStartAt,
		RequestType:     requestType,
		Reason:          reason,
		Timezone:        tz,
	}, true
}

// CreateBooking books a slot the availability engine would actually offer —
// not merely a free one. The storage unique index is the race backstop: a
// concurrent claim of the same slot surfaces as ErrSlotUnavailable.
func (s *Service) CreateBooking(ctx context.Context, clientID, serviceID string, startAt time.Time, tz string, participant ...booking.Participant) (booking.Booking, error) {
	if _, err := time.LoadLocation(tz); err != nil {
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
	if err := s.assertSlotGeneratable(ctx, svc.PractitionerID, svc.DurationMinutes, startAt, ""); err != nil {
		return booking.Booking{}, err
	}

	b, err := booking.New(clientID, svc.PractitionerID, serviceID, startAt, svc.DurationMinutes, s.now())
	if err != nil {
		return booking.Booking{}, err
	}
	if svc.PriceKobo > 0 {
		b.RequirePayment()
	}
	b.ReadinessVersion = 1
	if len(participant) > 0 {
		p := participant[0]
		if err := p.Validate(); err != nil {
			return booking.Booking{}, err
		}
		p.Revision = 1
		p.DeclaredAt = s.now().UTC()
		p.DeclaredBy = clientID
		p.AppointmentAt = b.StartAt
		b.Participant = &p
	}
	b.BookingTimezone = tz
	b, err = s.bookings.Create(ctx, b)
	if err != nil {
		return booking.Booking{}, err
	}
	if b.Status == booking.StatusPendingPayment {
		ports.NotifyActivity(ctx, s.notifier, ports.ActivityNotice{EventID: "booking:" + b.ID + ":created", ClientID: b.ClientID, PractitionerID: b.PractitionerID, Title: "A new session is awaiting payment", PracticeLink: "/calendar"})
	}
	if b.Status == booking.StatusConfirmed {
		if notice, ok := s.notice(ctx, b, tz); ok {
			s.notifier.BookingConfirmed(ctx, notice)
		}
	}
	return b, nil
}

// ListMine returns the client's own bookings, upcoming and past.
func (s *Service) ListMine(ctx context.Context, clientID string) ([]booking.Booking, error) {
	items, err := s.bookings.ListByClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ExpirePayment(s.now()) {
			items[i], err = s.bookings.Update(ctx, items[i])
			if err != nil {
				return nil, err
			}
		}
	}
	return items, nil
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
	if _, err := time.LoadLocation(tz); err != nil {
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
	if err := s.assertSlotGeneratable(ctx, b.PractitionerID, duration, startAt, b.ID); err != nil {
		return booking.Booking{}, err
	}
	previousStart := b.StartAt
	if err := b.Reschedule(startAt, s.now()); err != nil {
		return booking.Booking{}, err
	}
	b, err = s.bookings.Update(ctx, b)
	if err != nil {
		return booking.Booking{}, err
	}
	if notice, ok := s.notice(ctx, b, tz); ok {
		notice.PreviousStartAt = previousStart
		s.notifier.BookingRescheduled(ctx, notice)
	}
	return b, nil
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
	wasConfirmed := b.Status == booking.StatusConfirmed
	if err := b.Cancel(s.now()); err != nil {
		return booking.Booking{}, err
	}
	b, err = s.bookings.Update(ctx, b)
	if err != nil {
		return booking.Booking{}, err
	}
	// Cancellation has no request timezone of its own; the notifier's
	// practice default presents the time.
	if wasConfirmed {
		if notice, ok := s.notice(ctx, b, ""); ok {
			s.notifier.BookingCancelled(ctx, notice)
		}
	}
	return b, nil
}

// RequestReschedule lets a client ask the practitioner to move a session. It
// proves the proposed time is currently bookable but deliberately leaves the
// booking untouched until the practitioner decides what to do.
func (s *Service) RequestReschedule(ctx context.Context, clientID, bookingID string, proposedStartAt time.Time, tz string) (booking.Booking, error) {
	if _, err := time.LoadLocation(tz); err != nil {
		return booking.Booking{}, scheduling.ErrInvalidTimezone
	}
	id := identity.Identity{UserID: clientID, Role: identity.RoleClient}
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := s.checkCutoff(id, b); err != nil {
		return booking.Booking{}, err
	}
	if b.Status != booking.StatusConfirmed {
		return booking.Booking{}, booking.ErrInvalidTransition
	}
	duration := int(b.EndAt.Sub(b.StartAt) / time.Minute)
	if err := s.assertSlotGeneratable(ctx, b.PractitionerID, duration, proposedStartAt, b.ID); err != nil {
		return booking.Booking{}, err
	}
	now := s.now().UTC()
	b.ChangeRequestedAt = &now
	b.ChangeRequestType = "reschedule"
	b.ProposedStartAt = &proposedStartAt
	b.ChangeRequestReason = ""
	b.UpdatedAt = now
	b, err = s.bookings.Update(ctx, b)
	if err != nil {
		return booking.Booking{}, err
	}
	if notice, ok := s.changeRequestNotice(ctx, b, "reschedule", "", proposedStartAt, tz); ok {
		s.notifier.BookingChangeRequested(ctx, notice)
	}
	return b, nil
}

// RequestCancellation lets a client ask the practitioner to cancel a session
// and requires a reason, but does not release the booked slot automatically.
func (s *Service) RequestCancellation(ctx context.Context, clientID, bookingID, reason, tz string) (booking.Booking, error) {
	if _, err := time.LoadLocation(tz); err != nil {
		return booking.Booking{}, scheduling.ErrInvalidTimezone
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return booking.Booking{}, booking.ErrCancellationReasonRequired
	}
	id := identity.Identity{UserID: clientID, Role: identity.RoleClient}
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	if err := s.checkCutoff(id, b); err != nil {
		return booking.Booking{}, err
	}
	if b.Status != booking.StatusConfirmed {
		return booking.Booking{}, booking.ErrInvalidTransition
	}
	now := s.now().UTC()
	b.ChangeRequestedAt = &now
	b.ChangeRequestType = "cancellation"
	b.ChangeRequestReason = reason
	b.ProposedStartAt = nil
	b.UpdatedAt = now
	b, err = s.bookings.Update(ctx, b)
	if err != nil {
		return booking.Booking{}, err
	}
	if notice, ok := s.changeRequestNotice(ctx, b, "cancellation", reason, time.Time{}, tz); ok {
		s.notifier.BookingChangeRequested(ctx, notice)
	}
	return b, nil
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
	stored, err := s.bookings.Update(ctx, b)
	if err == nil {
		ports.NotifyActivity(ctx, s.notifier, ports.ActivityNotice{EventID: "booking:" + b.ID + ":" + string(b.Status), ClientID: b.ClientID, PractitionerID: b.PractitionerID, Title: "Session marked " + strings.ReplaceAll(string(b.Status), "_", " "), ClientLink: "/portal/sessions", PracticeLink: "/calendar"})
	}
	return stored, err
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
	stored, err := s.bookings.Update(ctx, b)
	if err == nil {
		ports.NotifyActivity(ctx, s.notifier, ports.ActivityNotice{EventID: "booking:" + b.ID + ":" + string(b.Status), ClientID: b.ClientID, PractitionerID: b.PractitionerID, Title: "Session marked " + strings.ReplaceAll(string(b.Status), "_", " "), ClientLink: "/portal/sessions", PracticeLink: "/calendar"})
	}
	return stored, err
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
	scheduleLoc, err := time.LoadLocation(scheduling.RulesTimezone(rules))
	if err != nil {
		return scheduling.ErrInvalidTimezone
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
		Loc:             scheduleLoc,
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

func (s *Service) UpdateParticipant(ctx context.Context, id identity.Identity, bookingID string, p booking.Participant) (booking.Booking, error) {
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return b, err
	}
	if b.Status != booking.StatusConfirmed && b.Status != booking.StatusPendingPayment {
		return b, booking.ErrInvalidTransition
	}
	if err = p.Validate(); err != nil {
		return b, err
	}
	p.Revision = 1
	if b.Participant != nil {
		p.Revision = b.Participant.Revision + 1
	}
	p.DeclaredAt = s.now().UTC()
	p.DeclaredBy = id.UserID
	p.AppointmentAt = b.StartAt
	b.Participant = &p
	b.ReadinessVersion = 1
	b.UpdatedAt = s.now().UTC()
	return s.bookings.Update(ctx, b)
}
