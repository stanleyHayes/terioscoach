// Package notes is the application service for the session-notes slice. It
// implements the inbound ports.NoteService port purely against outbound
// ports — no framework, driver, or transport imports. The client-visibility
// rule lives in the domain: this service enforces it by only ever handing
// the transport the domain-filtered view for client reads.
package notes

import (
	"context"
	"errors"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the session-notes use cases over outbound ports.
type Service struct {
	notes    ports.SessionNoteRepository
	bookings ports.BookingRepository
	services ports.ServiceRepository
	users    ports.UserRepository
	notifier ports.Notifier
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.NoteService = (*Service)(nil)

// Option customizes a Service.
type Option func(*Service)

// WithNotifications makes the first share of a note email the client
// (BE-09). The user and service repositories resolve the names the message
// needs. Delivery failure never fails the share: the note is shared in the
// portal either way, and the email is the nudge to go and read it.
func WithNotifications(notifier ports.Notifier, users ports.UserRepository, services ports.ServiceRepository) Option {
	return func(s *Service) {
		s.notifier = notifier
		s.users = users
		s.services = services
	}
}

// NewService wires the use cases to their outbound ports. The booking
// repository is shared with the booking slice: ownership of the note
// follows ownership of the booking it records.
func NewService(notes ports.SessionNoteRepository, bookings ports.BookingRepository, opts ...Option) *Service {
	s := &Service{
		notes:    notes,
		bookings: bookings,
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// GetNotes returns the booking's notes filtered by the caller's role. The
// practitioner gets the full note; the owning client gets the shared
// content only — and only once shared, otherwise the answer is
// note.ErrNoteNotFound, indistinguishable from no note at all.
func (s *Service) GetNotes(ctx context.Context, id identity.Identity, bookingID string) (ports.NoteView, error) {
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return ports.NoteView{}, err
	}
	n, err := s.notes.FindByBookingID(ctx, b.ID)
	if err != nil {
		return ports.NoteView{}, err
	}
	if id.Role == identity.RoleClient {
		shared, visible := n.ClientView()
		if !visible {
			return ports.NoteView{}, note.ErrNoteNotFound
		}
		return ports.NoteView{Shared: &shared}, nil
	}
	return ports.NoteView{Full: &n}, nil
}

// UpsertNotes creates or replaces the note's content on one of the
// practitioner's bookings. Exactly one note exists per booking: the first
// PUT creates it, later PUTs replace the content wholesale. SharedAt is
// never touched here — content edits do not unshare or re-stamp.
func (s *Service) UpsertNotes(ctx context.Context, practitionerID, bookingID string, content ports.NoteContent) (note.SessionNote, error) {
	b, err := s.loadForPractitioner(ctx, practitionerID, bookingID)
	if err != nil {
		return note.SessionNote{}, err
	}

	existing, findErr := s.notes.FindByBookingID(ctx, b.ID)
	if findErr != nil && !errors.Is(findErr, note.ErrNoteNotFound) {
		return note.SessionNote{}, findErr
	}
	if findErr == nil {
		if err := existing.ReplaceContent(content.PrivateNotes, content.SharedFeedback, content.SharedResources, s.now()); err != nil {
			return note.SessionNote{}, err
		}
		return s.notes.Update(ctx, existing)
	}

	n := note.New(b.ID, b.ClientID, b.PractitionerID, s.now())
	if err := n.ReplaceContent(content.PrivateNotes, content.SharedFeedback, content.SharedResources, s.now()); err != nil {
		return note.SessionNote{}, err
	}
	return s.notes.Create(ctx, n)
}

// ShareNotes stamps sharedAt, making the shared fields visible to the
// client. The transition is one-way and idempotent: JustShared is true only
// on the first call, which is the signal the notifications slice (BE-09)
// uses to send the feedback email exactly once.
func (s *Service) ShareNotes(ctx context.Context, practitionerID, bookingID string) (ports.ShareResult, error) {
	b, err := s.loadForPractitioner(ctx, practitionerID, bookingID)
	if err != nil {
		return ports.ShareResult{}, err
	}
	n, err := s.notes.FindByBookingID(ctx, b.ID)
	if err != nil {
		return ports.ShareResult{}, err
	}
	if n.IsShared() {
		return ports.ShareResult{Note: n}, nil
	}
	n.Share(s.now())
	n, err = s.notes.Update(ctx, n)
	if err != nil {
		return ports.ShareResult{}, err
	}
	// Only the first share notifies: JustShared is exactly the "this is
	// new to the client" signal, and a repeat POST returned above.
	s.notifyShared(ctx, b, n)
	return ports.ShareResult{Note: n, JustShared: true}, nil
}

// notifyShared queues the client's "there is feedback waiting" email. Every
// lookup it needs is best-effort: a missing service name is worth a vaguer
// email, not a missing one, and a missing account means there is nowhere to
// send at all.
func (s *Service) notifyShared(ctx context.Context, b booking.Booking, n note.SessionNote) {
	if s.notifier == nil || s.users == nil {
		return
	}
	client, err := s.users.FindByID(ctx, n.ClientID)
	if err != nil {
		return
	}
	notice := ports.FeedbackNotice{
		BookingID:    b.ID,
		ClientName:   client.Name,
		ClientEmail:  client.Email,
		ServiceName:  "your session",
		SessionDate:  b.StartAt,
		HasResources: len(n.SharedResources) > 0,
	}
	if practitioner, err := s.users.FindByID(ctx, n.PractitionerID); err == nil {
		notice.PractitionerName = practitioner.Name
	}
	if s.services != nil {
		if svc, err := s.services.FindByID(ctx, b.ServiceID); err == nil {
			notice.ServiceName = svc.Name
		}
	}
	s.notifier.FeedbackShared(ctx, notice)
}

// loadAuthorized fetches a booking and enforces ownership: the owning
// client or the practitioner it belongs to. Cross-owner access is reported
// as not-found — no existence leak.
func (s *Service) loadAuthorized(ctx context.Context, id identity.Identity, bookingID string) (booking.Booking, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return booking.Booking{}, err
	}
	switch id.Role {
	case identity.RoleClient:
		if b.ClientID != id.UserID {
			return booking.Booking{}, booking.ErrBookingNotFound
		}
	case identity.RolePractitioner:
		if b.PractitionerID != id.UserID {
			return booking.Booking{}, booking.ErrBookingNotFound
		}
	default:
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

// loadForPractitioner fetches a booking owned by the practitioner.
func (s *Service) loadForPractitioner(ctx context.Context, practitionerID, bookingID string) (booking.Booking, error) {
	return s.loadAuthorized(ctx, identity.Identity{UserID: practitionerID, Role: identity.RolePractitioner}, bookingID)
}
