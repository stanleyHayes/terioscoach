package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
)

type RecordingUpload struct {
	ContentType string
	DataURL     string
	Bytes       int64
	DurationSec int
}

type RecordingService interface {
	CreateRecording(ctx context.Context, practitionerID, bookingID string, upload RecordingUpload) (recording.SessionRecording, error)
	ListForBooking(ctx context.Context, id identity.Identity, bookingID string) ([]recording.SessionRecording, error)
}

type RecordingRepository interface {
	Create(ctx context.Context, rec recording.SessionRecording) (recording.SessionRecording, error)
	ListByBookingID(ctx context.Context, bookingID string) ([]recording.SessionRecording, error)
}
