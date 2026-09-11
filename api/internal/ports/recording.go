package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
)

// RecordingUpload registers a recording already uploaded to the media
// store. The bytes never pass through the API: the browser uploads them
// directly under a signature, exactly as documents do.
type RecordingUpload struct {
	ContentType string
	PublicID    string
	Bytes       int64
	DurationSec int
}

// PlayableRecording is one recording with a delivery URL attached. The URL
// is short-lived and minted per request, so a link that leaves the page
// stops working rather than becoming a standing handle on a consultation.
type PlayableRecording struct {
	Recording recording.SessionRecording
	URL       string
}

// PurgeResult reports what a retention sweep removed.
type PurgeResult struct {
	Deleted int
	Failed  int
}

type RecordingService interface {
	// SignUpload authorizes one direct upload of a recording for a booking
	// — practitioner only.
	SignUpload(ctx context.Context, practitionerID, bookingID, contentType string) (SignedUpload, error)
	CreateRecording(ctx context.Context, practitionerID, bookingID string, upload RecordingUpload) (recording.SessionRecording, error)
	// ListForBooking returns the session's recordings with a delivery URL
	// for each, to whoever is entitled to them.
	ListForBooking(ctx context.Context, id identity.Identity, bookingID string) ([]PlayableRecording, error)
	// PurgeExpired deletes recordings past their retention date, stored
	// bytes first. Safe to run on a timer.
	PurgeExpired(ctx context.Context, limit int) (PurgeResult, error)
}

type RecordingRepository interface {
	Create(ctx context.Context, rec recording.SessionRecording) (recording.SessionRecording, error)
	ListByBookingID(ctx context.Context, bookingID string) ([]recording.SessionRecording, error)
	// ListExpired returns recordings whose retention has run out, oldest
	// first, so the sweep is bounded and resumable.
	ListExpired(ctx context.Context, now time.Time, limit int) ([]recording.SessionRecording, error)
	// Delete removes the record. Misses are not an error.
	Delete(ctx context.Context, id string) error
}
