package recording

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)

func TestNewStampsThePartiesAndRetention(t *testing.T) {
	rec, err := New("booking-1", "client-1", "prac-1", "video/mp4", "terios/clients/client-1/recordings/abc", 5_200_000, 1800, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if rec.BookingID != "booking-1" || rec.ClientID != "client-1" || rec.PractitionerID != "prac-1" {
		t.Errorf("recording = %+v, want the parties stamped", rec)
	}
	if !rec.RetainUntil.Equal(fixedNow.Add(RetentionDuration)) {
		t.Errorf("RetainUntil = %v, want %v out", rec.RetainUntil, RetentionDuration)
	}
	if !rec.Stored() {
		t.Error("a recording with a public id should report as stored")
	}
}

func TestNewRejectsUnusableInput(t *testing.T) {
	valid := func() (string, string, string, string, string, int64, int) {
		return "booking-1", "client-1", "prac-1", "video/mp4", "abc", 10, 60
	}
	for name, mutate := range map[string]func() (string, string, string, string, string, int64, int){
		"no booking": func() (string, string, string, string, string, int64, int) {
			return "", "c", "p", "video/mp4", "abc", 10, 60
		},
		"no client": func() (string, string, string, string, string, int64, int) {
			return "b", "", "p", "video/mp4", "abc", 10, 60
		},
		"no practitioner": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "", "video/mp4", "abc", 10, 60
		},
		"no content type": func() (string, string, string, string, string, int64, int) { return "b", "c", "p", "  ", "abc", 10, 60 },
		// The whole point of the record: with no reference to the stored
		// file there is nothing to play or to delete later.
		"no public id": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "p", "video/mp4", "  ", 10, 60
		},
		"empty file": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "p", "video/mp4", "abc", 0, 60
		},
		"absurdly large": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "p", "video/mp4", "abc", MaxBytes + 1, 60
		},
		"negative duration": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "p", "video/mp4", "abc", 10, -1
		},
		"public id too long": func() (string, string, string, string, string, int64, int) {
			return "b", "c", "p", "video/mp4", strings.Repeat("a", MaxPublicIDLen+1), 10, 60
		},
	} {
		t.Run(name, func(t *testing.T) {
			b, c, p, ct, pid, bytes, dur := mutate()
			if _, err := New(b, c, p, ct, pid, bytes, dur, fixedNow); !errors.Is(err, ErrInvalidRecording) {
				t.Errorf("err = %v, want ErrInvalidRecording", err)
			}
		})
	}
	// Sanity: the unmutated fixture is accepted.
	b, c, p, ct, pid, bytes, dur := valid()
	if _, err := New(b, c, p, ct, pid, bytes, dur, fixedNow); err != nil {
		t.Errorf("the valid fixture was rejected: %v", err)
	}
}

// A recording made before the move to the media store has no public id, but
// still has to play.
func TestALegacyInlineRecordingIsNotStored(t *testing.T) {
	legacy := SessionRecording{DataURL: "data:video/webm;base64,AAAA"}
	if legacy.Stored() {
		t.Error("an inline recording should not report as stored")
	}
}

// One folder per client: an asset's path alone must never imply that
// someone else may see it.
func TestFolderIsScopedToTheClient(t *testing.T) {
	if got := Folder("client-1"); got != "terios/clients/client-1/recordings" {
		t.Errorf("Folder = %q", got)
	}
	if Folder("client-1") == Folder("client-2") {
		t.Error("two clients share a recordings folder")
	}
}

// The store keys delivery off the extension, so a WebM stored as .mp4 would
// not play back.
func TestExtensionFollowsTheRecordedType(t *testing.T) {
	for contentType, want := range map[string]string{
		"video/mp4":                    "mp4",
		"video/mp4;codecs=avc1.42E01E": "mp4",
		"video/webm":                   "webm",
		"video/webm;codecs=vp9,opus":   "webm",
		"application/octet-stream":     "bin",
	} {
		if got := Extension(contentType); got != want {
			t.Errorf("Extension(%q) = %q, want %q", contentType, got, want)
		}
	}
}
