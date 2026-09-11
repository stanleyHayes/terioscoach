// Package recording is the domain core for session recordings.
//
// A recording is a reference, not a payload. The bytes live in the media
// store under a private, authenticated asset id; this record holds who the
// session belonged to, how big the file is, and how long it is kept. An
// earlier version inlined the whole file as a base64 data URL, which made
// listing one session's recordings a multi-megabyte response and put a
// 7 MB request body on the upload path.
package recording

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxBytes bounds a single recording. A consultation hour at the
	// bitrates MediaRecorder produces sits well inside this.
	MaxBytes = 512 << 20
	// MaxPublicIDLen mirrors the media store's own limit.
	MaxPublicIDLen = 500

	RetentionDuration = 30 * 24 * time.Hour
	StorageLocation   = "encrypted application storage"
)

// SessionRecording is one recording of one session.
type SessionRecording struct {
	ID             string
	BookingID      string
	ClientID       string
	PractitionerID string
	ContentType    string
	// PublicID is the media store's identifier. It is never given to a
	// caller: delivery goes through the API, which checks who is asking and
	// signs a short-lived URL.
	PublicID string
	// DataURL is the inline copy kept by recordings made before the move to
	// the media store. Nothing writes it any more; it is read so those
	// recordings keep playing, and it empties out on its own as they age
	// past retention.
	DataURL     string
	Bytes       int64
	DurationSec int
	CreatedAt   time.Time
	RetainUntil time.Time
}

// New builds a recording of an uploaded asset.
func New(bookingID, clientID, practitionerID, contentType, publicID string, bytes int64, durationSec int, now time.Time) (SessionRecording, error) {
	contentType = strings.TrimSpace(contentType)
	publicID = strings.TrimSpace(publicID)
	if bookingID == "" || clientID == "" || practitionerID == "" || contentType == "" || publicID == "" {
		return SessionRecording{}, ErrInvalidRecording
	}
	if bytes < 1 || bytes > MaxBytes || durationSec < 0 {
		return SessionRecording{}, ErrInvalidRecording
	}
	if utf8.RuneCountInString(publicID) > MaxPublicIDLen {
		return SessionRecording{}, ErrInvalidRecording
	}
	now = now.UTC()
	return SessionRecording{
		BookingID:      bookingID,
		ClientID:       clientID,
		PractitionerID: practitionerID,
		ContentType:    contentType,
		PublicID:       publicID,
		Bytes:          bytes,
		DurationSec:    durationSec,
		CreatedAt:      now,
		RetainUntil:    now.Add(RetentionDuration),
	}, nil
}

// Stored reports whether the bytes are in the media store, as opposed to
// inlined on a record made before the move.
func (r SessionRecording) Stored() bool { return r.PublicID != "" }

// Folder is where a client's recordings live in the media store. One folder
// per client, so an asset's path alone never implies it is shareable.
func Folder(clientID string) string {
	return "terios/clients/" + clientID + "/recordings"
}

// Extension is the file extension implied by a recorded MIME type. The
// media store keys delivery off it, so a WebM stored as .mp4 would not
// play.
func Extension(contentType string) string {
	switch {
	case strings.Contains(contentType, "mp4"):
		return "mp4"
	case strings.Contains(contentType, "webm"):
		return "webm"
	default:
		return "bin"
	}
}
