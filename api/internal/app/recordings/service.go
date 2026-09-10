package recordings

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
	"github.com/xcreativs/terios/api/internal/ports"
)

type Service struct {
	recordings ports.RecordingRepository
	bookings   ports.BookingRepository
	now        func() time.Time
}

var _ ports.RecordingService = (*Service)(nil)

func NewService(recordings ports.RecordingRepository, bookings ports.BookingRepository) *Service {
	return &Service{
		recordings: recordings,
		bookings:   bookings,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) CreateRecording(ctx context.Context, practitionerID, bookingID string, upload ports.RecordingUpload) (recording.SessionRecording, error) {
	b, err := s.loadAuthorized(ctx, identity.Identity{UserID: practitionerID, Role: identity.RolePractitioner}, bookingID)
	if err != nil {
		return recording.SessionRecording{}, err
	}
	rec, err := recording.New(b.ID, b.ClientID, b.PractitionerID, upload.ContentType, upload.DataURL, upload.Bytes, upload.DurationSec, s.now())
	if err != nil {
		return recording.SessionRecording{}, err
	}
	return s.recordings.Create(ctx, rec)
}

func (s *Service) ListForBooking(ctx context.Context, id identity.Identity, bookingID string) ([]recording.SessionRecording, error) {
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return nil, err
	}
	return s.recordings.ListByBookingID(ctx, b.ID)
}

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
