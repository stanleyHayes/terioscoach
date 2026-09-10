package recording

import "errors"

var (
	ErrRecordingNotFound = errors.New("recording not found")
	ErrRecordingExists   = errors.New("recording already exists")
	ErrInvalidRecording  = errors.New("invalid recording")
)
