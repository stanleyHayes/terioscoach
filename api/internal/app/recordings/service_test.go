package recordings

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var fixedNow = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)

// fakeRecordings is an in-memory recording store.
type fakeRecordings struct {
	byID   map[string]recording.SessionRecording
	next   int
	delErr error
}

func newFakeRecordings() *fakeRecordings {
	return &fakeRecordings{byID: map[string]recording.SessionRecording{}}
}

func (f *fakeRecordings) Create(_ context.Context, rec recording.SessionRecording) (recording.SessionRecording, error) {
	f.next++
	rec.ID = "rec-" + string(rune('0'+f.next))
	f.byID[rec.ID] = rec
	return rec, nil
}

func (f *fakeRecordings) ListByBookingID(_ context.Context, bookingID string) ([]recording.SessionRecording, error) {
	var out []recording.SessionRecording
	for _, rec := range f.byID {
		if rec.BookingID == bookingID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (f *fakeRecordings) ListExpired(_ context.Context, now time.Time, limit int) ([]recording.SessionRecording, error) {
	var out []recording.SessionRecording
	for _, rec := range f.byID {
		if !rec.RetainUntil.After(now) && len(out) < limit {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (f *fakeRecordings) Delete(_ context.Context, id string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.byID, id)
	return nil
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type rig struct {
	svc        *Service
	recordings *fakeRecordings
	bookings   *portstest.FakeBookingRepository
	media      *portstest.FakeMediaStore
	booking    booking.Booking
}

func newRig(t *testing.T) *rig {
	t.Helper()
	recs := newFakeRecordings()
	bookings := portstest.NewFakeBookingRepository()
	media := portstest.NewFakeMediaStore()

	b, err := booking.New("client-1", "prac-1", "svc-1", fixedNow.Add(24*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b, err = bookings.Create(context.Background(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	return &rig{
		svc: NewService(recs, bookings, media, Options{
			Log: quiet(),
			Now: func() time.Time { return fixedNow },
		}),
		recordings: recs,
		bookings:   bookings,
		media:      media,
		booking:    b,
	}
}

func (r *rig) store(t *testing.T, publicID string, retainUntil time.Time) recording.SessionRecording {
	t.Helper()
	rec, err := recording.New(r.booking.ID, "client-1", "prac-1", "video/mp4", publicID, 1024, 60, fixedNow)
	if err != nil {
		t.Fatalf("recording.New: %v", err)
	}
	if !retainUntil.IsZero() {
		rec.RetainUntil = retainUntil
	}
	stored, err := r.recordings.Create(context.Background(), rec)
	if err != nil {
		t.Fatalf("seed recording: %v", err)
	}
	return stored
}

var (
	practitioner = identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}
	client       = identity.Identity{UserID: "client-1", Role: identity.RoleClient}
	stranger     = identity.Identity{UserID: "client-2", Role: identity.RoleClient}
)

// The folder and the private flag are inside the signature, so a signed
// upload cannot be redirected into another client's folder or turned into a
// publicly reachable asset.
func TestSignUploadScopesTheUploadToTheClientAndKeepsItPrivate(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.SignUpload(context.Background(), "prac-1", r.booking.ID, "video/mp4"); err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	if len(r.media.Uploads) != 1 {
		t.Fatalf("uploads signed = %d, want 1", len(r.media.Uploads))
	}
	params := r.media.Uploads[0]
	if params.Folder != recording.Folder("client-1") {
		t.Errorf("Folder = %q, want the client's recordings folder", params.Folder)
	}
	if !params.Private {
		t.Error("a consultation recording must be signed as private")
	}
	if params.ResourceType != document.ResourceVideo {
		t.Errorf("ResourceType = %q, want video", params.ResourceType)
	}
}

func TestSignUploadRefusesAnotherPractitionersBooking(t *testing.T) {
	r := newRig(t)
	_, err := r.svc.SignUpload(context.Background(), "prac-2", r.booking.ID, "video/mp4")
	if !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("err = %v, want it reported as missing", err)
	}
}

func TestSignUploadNeedsAContentType(t *testing.T) {
	r := newRig(t)
	if _, err := r.svc.SignUpload(context.Background(), "prac-1", r.booking.ID, ""); !errors.Is(err, recording.ErrInvalidRecording) {
		t.Errorf("err = %v, want ErrInvalidRecording", err)
	}
}

func TestCreateRecordingStampsThePartiesFromTheBooking(t *testing.T) {
	r := newRig(t)
	rec, err := r.svc.CreateRecording(context.Background(), "prac-1", r.booking.ID, ports.RecordingUpload{
		ContentType: "video/mp4",
		PublicID:    "terios/clients/client-1/recordings/abc",
		Bytes:       5_200_000,
		DurationSec: 1800,
	})
	if err != nil {
		t.Fatalf("CreateRecording: %v", err)
	}
	if rec.ClientID != "client-1" || rec.PractitionerID != "prac-1" {
		t.Errorf("recording = %+v, want the booking's parties", rec)
	}
}

// The response carries a URL, not the file. This is the whole point of the
// change: listing a session's recordings used to be a multi-megabyte
// response because the bytes were inlined.
func TestListReturnsASignedURLRatherThanTheFile(t *testing.T) {
	r := newRig(t)
	r.store(t, "terios/clients/client-1/recordings/abc", time.Time{})

	items, err := r.svc.ListForBooking(context.Background(), practitioner, r.booking.ID)
	if err != nil {
		t.Fatalf("ListForBooking: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].URL == "" || strings.HasPrefix(items[0].URL, "data:") {
		t.Errorf("URL = %q, want a delivery URL rather than inline bytes", items[0].URL)
	}
	if items[0].Recording.PublicID == "" {
		t.Error("the record should still carry its store reference")
	}
}

// Recordings made before the move have no stored asset; the inline copy is
// the only one there is, and they have to keep playing.
func TestALegacyInlineRecordingStillPlays(t *testing.T) {
	r := newRig(t)
	legacy := recording.SessionRecording{
		BookingID: r.booking.ID, ClientID: "client-1", PractitionerID: "prac-1",
		ContentType: "video/webm", DataURL: "data:video/webm;base64,AAAA",
		Bytes: 1024, CreatedAt: fixedNow, RetainUntil: fixedNow.Add(time.Hour),
	}
	if _, err := r.recordings.Create(context.Background(), legacy); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	items, err := r.svc.ListForBooking(context.Background(), client, r.booking.ID)
	if err != nil {
		t.Fatalf("ListForBooking: %v", err)
	}
	if len(items) != 1 || items[0].URL != "data:video/webm;base64,AAAA" {
		t.Errorf("items = %+v, want the inline copy served", items)
	}
}

// One unsignable asset must not take the rest of the session's recordings
// down with it. The legacy row needs no signature, so it still comes back
// while the stored one is skipped.
func TestAnUnsignableRecordingIsSkippedNotFatal(t *testing.T) {
	r := newRig(t)
	r.store(t, "terios/clients/client-1/recordings/abc", time.Time{})
	legacy := recording.SessionRecording{
		BookingID: r.booking.ID, ClientID: "client-1", PractitionerID: "prac-1",
		ContentType: "video/webm", DataURL: "data:video/webm;base64,AAAA",
		Bytes: 1024, CreatedAt: fixedNow, RetainUntil: fixedNow.Add(time.Hour),
	}
	if _, err := r.recordings.Create(context.Background(), legacy); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	r.media.SignErr = errors.New("store unavailable")

	items, err := r.svc.ListForBooking(context.Background(), practitioner, r.booking.ID)
	if err != nil {
		t.Fatalf("ListForBooking = %v, want the list to survive", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want the signable-free legacy row to survive", len(items))
	}
	if items[0].URL != "data:video/webm;base64,AAAA" {
		t.Errorf("URL = %q, want the legacy row returned", items[0].URL)
	}
}

func TestListIsScopedToTheSessionsOwnParties(t *testing.T) {
	r := newRig(t)
	r.store(t, "terios/clients/client-1/recordings/abc", time.Time{})

	for name, id := range map[string]identity.Identity{
		"the client":       client,
		"the practitioner": practitioner,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := r.svc.ListForBooking(context.Background(), id, r.booking.ID); err != nil {
				t.Errorf("ListForBooking = %v, want nil", err)
			}
		})
	}

	// Reported as missing rather than forbidden — the isolation rule the
	// rest of the client-scoped API follows.
	if _, err := r.svc.ListForBooking(context.Background(), stranger, r.booking.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("err = %v, want it reported as missing", err)
	}
}

// Retention: the stored file goes first, then the row. The reverse would
// leave the recording in the media store with nothing pointing at it.
func TestPurgeDeletesTheStoredFileThenTheRecord(t *testing.T) {
	r := newRig(t)
	expired := r.store(t, "terios/clients/client-1/recordings/old", fixedNow.Add(-time.Hour))
	r.store(t, "terios/clients/client-1/recordings/current", fixedNow.Add(24*time.Hour))

	result, err := r.svc.PurgeExpired(context.Background(), 0)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if result.Deleted != 1 || result.Failed != 0 {
		t.Errorf("result = %+v, want one deleted", result)
	}
	if len(r.media.Deleted) != 1 || r.media.Deleted[0].PublicID != expired.PublicID {
		t.Errorf("deleted assets = %+v, want the expired one", r.media.Deleted)
	}
	if _, still := r.recordings.byID[expired.ID]; still {
		t.Error("the expired record should be gone")
	}
	if len(r.recordings.byID) != 1 {
		t.Errorf("records left = %d, want the in-date one kept", len(r.recordings.byID))
	}
}

// If the file cannot be deleted, the row stays so the next sweep tries
// again. Dropping it would lose the only reference to the file.
func TestPurgeKeepsTheRecordWhenTheFileCannotBeDeleted(t *testing.T) {
	r := newRig(t)
	expired := r.store(t, "terios/clients/client-1/recordings/old", fixedNow.Add(-time.Hour))
	r.media.DeleteErr = errors.New("store unavailable")

	result, err := r.svc.PurgeExpired(context.Background(), 0)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if result.Deleted != 0 || result.Failed != 1 {
		t.Errorf("result = %+v, want one failure and nothing deleted", result)
	}
	if _, still := r.recordings.byID[expired.ID]; !still {
		t.Error("the record must survive so the next sweep can retry")
	}
}

// A legacy inline recording has no asset to delete, and must still expire.
func TestPurgeRemovesALegacyInlineRecording(t *testing.T) {
	r := newRig(t)
	legacy := recording.SessionRecording{
		BookingID: r.booking.ID, ClientID: "client-1", PractitionerID: "prac-1",
		ContentType: "video/webm", DataURL: "data:video/webm;base64,AAAA",
		Bytes: 1024, CreatedAt: fixedNow, RetainUntil: fixedNow.Add(-time.Hour),
	}
	if _, err := r.recordings.Create(context.Background(), legacy); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	result, err := r.svc.PurgeExpired(context.Background(), 0)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if result.Deleted != 1 {
		t.Errorf("result = %+v, want the inline recording expired too", result)
	}
	if len(r.media.Deleted) != 0 {
		t.Error("there is no stored asset to delete for an inline recording")
	}
}
