// Package recordings is the application service for session recordings.
//
// The slice's job is to keep the bytes out of the API. An upload is signed
// and goes straight to the media store; a read hands back a short-lived
// delivery URL rather than the file. What the API decides is *who* may have
// a URL — and that decision is here, not in the store.
package recordings

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultPurgeBatch bounds one retention sweep, so a long-neglected backlog
// is worked through over several ticks rather than in one long transaction.
const defaultPurgeBatch = 50

// Options are the deployment-specific parts.
type Options struct {
	// DeliveryTTL is how long a playback URL lives. Long enough to watch a
	// consultation back, short enough that a copied link expires.
	DeliveryTTL time.Duration
	Log         *slog.Logger
	Now         func() time.Time
}

type Service struct {
	recordings ports.RecordingRepository
	bookings   ports.BookingRepository
	media      ports.MediaStore
	ttl        time.Duration
	log        *slog.Logger
	now        func() time.Time
}

var _ ports.RecordingService = (*Service)(nil)

func NewService(
	recordings ports.RecordingRepository,
	bookings ports.BookingRepository,
	media ports.MediaStore,
	opts Options,
) *Service {
	ttl := opts.DeliveryTTL
	if ttl <= 0 {
		ttl = ports.DefaultDeliveryTTL
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		recordings: recordings,
		bookings:   bookings,
		media:      media,
		ttl:        ttl,
		log:        log,
		now:        now,
	}
}

// SignUpload authorizes one upload into the client's own recordings folder.
//
// The folder and the private delivery type are inside the signature, so a
// practitioner holding it cannot redirect the upload into another client's
// folder or turn a consultation into a publicly reachable asset.
func (s *Service) SignUpload(ctx context.Context, practitionerID, bookingID, contentType string) (ports.SignedUpload, error) {
	b, err := s.loadAuthorized(ctx, identity.Identity{UserID: practitionerID, Role: identity.RolePractitioner}, bookingID)
	if err != nil {
		return ports.SignedUpload{}, err
	}
	if contentType == "" {
		return ports.SignedUpload{}, recording.ErrInvalidRecording
	}
	return s.media.SignUpload(ctx, ports.UploadParams{
		Folder:       recording.Folder(b.ClientID),
		ResourceType: document.ResourceVideo,
		Private:      true,
	})
}

// CreateRecording registers an upload that has landed.
func (s *Service) CreateRecording(ctx context.Context, practitionerID, bookingID string, upload ports.RecordingUpload) (recording.SessionRecording, error) {
	b, err := s.loadAuthorized(ctx, identity.Identity{UserID: practitionerID, Role: identity.RolePractitioner}, bookingID)
	if err != nil {
		return recording.SessionRecording{}, err
	}
	rec, err := recording.New(
		b.ID, b.ClientID, b.PractitionerID,
		upload.ContentType, upload.PublicID, upload.Bytes, upload.DurationSec, s.now(),
	)
	if err != nil {
		return recording.SessionRecording{}, err
	}
	return s.recordings.Create(ctx, rec)
}

// ListForBooking returns the session's recordings, each with a fresh
// delivery URL.
//
// Signing is local arithmetic, not a call to the store, so doing it per
// item costs nothing — and minting per request is what keeps the URL
// short-lived rather than something stored and shared around.
func (s *Service) ListForBooking(ctx context.Context, id identity.Identity, bookingID string) ([]ports.PlayableRecording, error) {
	b, err := s.loadAuthorized(ctx, id, bookingID)
	if err != nil {
		return nil, err
	}
	stored, err := s.recordings.ListByBookingID(ctx, b.ID)
	if err != nil {
		return nil, err
	}

	out := make([]ports.PlayableRecording, 0, len(stored))
	for _, rec := range stored {
		item := ports.PlayableRecording{Recording: rec}
		switch {
		case rec.Stored():
			url, err := s.media.SignedURL(ctx, ports.Asset{
				PublicID:     rec.PublicID,
				ResourceType: document.ResourceVideo,
				Private:      true,
			}, s.ttl)
			if err != nil {
				// One unsignable asset must not take the rest of the
				// session's recordings down with it.
				s.log.Error("sign recording url", "recordingId", rec.ID, "error", err)
				continue
			}
			item.URL = url
		case rec.DataURL != "":
			// Made before the move to the media store: the inline copy is
			// the only one there is.
			item.URL = rec.DataURL
		default:
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// PurgeExpired deletes recordings past their retention date.
//
// This replaces the TTL index that used to expire the row on its own. With
// the bytes in the media store, letting Mongo drop the record first would
// strand the file there with nothing left pointing at it — so the asset
// goes first, and the row only once it has. A recording that outlives its
// date because the sweep is not running is a visible, fixable problem; an
// orphaned consultation in object storage is not.
func (s *Service) PurgeExpired(ctx context.Context, limit int) (ports.PurgeResult, error) {
	if limit <= 0 {
		limit = defaultPurgeBatch
	}
	due, err := s.recordings.ListExpired(ctx, s.now(), limit)
	if err != nil {
		return ports.PurgeResult{}, fmt.Errorf("list expired recordings: %w", err)
	}

	var result ports.PurgeResult
	for _, rec := range due {
		if rec.Stored() {
			err := s.media.Delete(ctx, ports.Asset{
				PublicID:     rec.PublicID,
				ResourceType: document.ResourceVideo,
				Private:      true,
			})
			if err != nil {
				// The row stays, so the next sweep tries again. Deleting it
				// now would lose the only reference to the file.
				s.log.Error("delete expired recording asset", "recordingId", rec.ID, "error", err)
				result.Failed++
				continue
			}
		}
		if err := s.recordings.Delete(ctx, rec.ID); err != nil {
			s.log.Error("delete expired recording", "recordingId", rec.ID, "error", err)
			result.Failed++
			continue
		}
		result.Deleted++
	}
	return result, nil
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
