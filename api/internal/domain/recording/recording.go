// Package recording is the domain core for session recordings.
package recording

import (
	"strings"
	"time"
	"unicode/utf8"
)

const (
	MaxDataURLLen     = 25 * 1024 * 1024
	RetentionDuration = 30 * 24 * time.Hour
	StorageLocation   = "encrypted application storage"
)

type SessionRecording struct {
	ID             string
	BookingID      string
	ClientID       string
	PractitionerID string
	ContentType    string
	DataURL        string
	Bytes          int64
	DurationSec    int
	CreatedAt      time.Time
	RetainUntil    time.Time
}

func New(bookingID, clientID, practitionerID, contentType, dataURL string, bytes int64, durationSec int, now time.Time) (SessionRecording, error) {
	contentType = strings.TrimSpace(contentType)
	if bookingID == "" || clientID == "" || practitionerID == "" || contentType == "" || dataURL == "" || bytes < 1 || durationSec < 0 {
		return SessionRecording{}, ErrInvalidRecording
	}
	if utf8.RuneCountInString(dataURL) > MaxDataURLLen {
		return SessionRecording{}, ErrInvalidRecording
	}
	now = now.UTC()
	return SessionRecording{
		BookingID:      bookingID,
		ClientID:       clientID,
		PractitionerID: practitionerID,
		ContentType:    contentType,
		DataURL:        dataURL,
		Bytes:          bytes,
		DurationSec:    durationSec,
		CreatedAt:      now,
		RetainUntil:    now.Add(RetentionDuration),
	}, nil
}
