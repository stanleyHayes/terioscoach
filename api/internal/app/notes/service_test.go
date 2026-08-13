package notes

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var fixedNow = time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC)

type testRig struct {
	svc      *Service
	notes    *portstest.FakeSessionNoteRepository
	bookings *portstest.FakeBookingRepository
}

func newTestRig() testRig {
	rig := testRig{
		notes:    portstest.NewFakeSessionNoteRepository(),
		bookings: portstest.NewFakeBookingRepository(),
	}
	rig.svc = NewService(rig.notes, rig.bookings)
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

var (
	pracID  = "prac-1"
	prac2ID = "prac-2"

	clientOwner   = identity.Identity{UserID: "client-1", Role: identity.RoleClient}
	clientOther   = identity.Identity{UserID: "client-2", Role: identity.RoleClient}
	practitioner  = identity.Identity{UserID: pracID, Role: identity.RolePractitioner}
	practitioner2 = identity.Identity{UserID: prac2ID, Role: identity.RolePractitioner}
)

// seedBooking inserts a confirmed booking for client-1 with prac-1.
func seedBooking(t *testing.T, rig testRig) booking.Booking {
	t.Helper()
	b, err := booking.New("client-1", pracID, "svc-1", fixedNow.Add(24*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b, err = rig.bookings.Create(context.Background(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b
}

func upsert(t *testing.T, rig testRig, bookingID string, content ports.NoteContent) note.SessionNote {
	t.Helper()
	n, err := rig.svc.UpsertNotes(context.Background(), pracID, bookingID, content)
	if err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	return n
}

// TestUpsertCreatesThenReplaces: one note per booking; a repeat PUT swaps
// the content wholesale and keeps identity and stamps.
func TestUpsertCreatesThenReplaces(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)

	n := upsert(t, rig, b.ID, ports.NoteContent{
		PrivateNotes:    "tension in left shoulder",
		SharedFeedback:  "great progress",
		SharedResources: []string{"https://example.com/stretch"},
	})
	if n.ID == "" || n.BookingID != b.ID || n.ClientID != "client-1" || n.PractitionerID != pracID {
		t.Errorf("note = %+v, want parties stamped from the booking", n)
	}
	if !n.CreatedAt.Equal(fixedNow) || n.SharedAt != nil {
		t.Errorf("note = %+v, want created stamp and no sharedAt", n)
	}

	n2 := upsert(t, rig, b.ID, ports.NoteContent{
		PrivateNotes:   "revised private",
		SharedFeedback: "revised feedback",
	})
	if n2.ID != n.ID {
		t.Errorf("second PUT created a new note (%s), want upsert of %s", n2.ID, n.ID)
	}
	if n2.PrivateNotes != "revised private" || n2.SharedFeedback != "revised feedback" || len(n2.SharedResources) != 0 {
		t.Errorf("note = %+v, want content replaced wholesale", n2)
	}
	if !n2.CreatedAt.Equal(n.CreatedAt) {
		t.Errorf("createdAt = %v, want original %v kept", n2.CreatedAt, n.CreatedAt)
	}
}

// TestUpsertKeepsSharedAt: editing after sharing neither unshares nor
// re-stamps.
func TestUpsertKeepsSharedAt(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	upsert(t, rig, b.ID, ports.NoteContent{SharedFeedback: "v1"})

	res, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID)
	if err != nil {
		t.Fatalf("ShareNotes: %v", err)
	}
	stamp := *res.Note.SharedAt

	rig.svc.now = func() time.Time { return fixedNow.Add(2 * time.Hour) }
	n := upsert(t, rig, b.ID, ports.NoteContent{SharedFeedback: "v2"})
	if n.SharedAt == nil || !n.SharedAt.Equal(stamp) {
		t.Errorf("sharedAt = %v, want untouched %v", n.SharedAt, stamp)
	}
}

// TestUpsertOwnership: notes on another practitioner's booking are
// not-found — no existence leak.
func TestUpsertOwnership(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	if _, err := rig.svc.UpsertNotes(context.Background(), prac2ID, b.ID, ports.NoteContent{}); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.UpsertNotes(context.Background(), pracID, "no-such-booking", ports.NoteContent{}); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("err = %v, want ErrBookingNotFound", err)
	}
}

// TestShareIdempotent: the first share transitions (JustShared — BE-09's
// email signal), repeats return the note unchanged.
func TestShareIdempotent(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	upsert(t, rig, b.ID, ports.NoteContent{SharedFeedback: "well done"})

	first, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID)
	if err != nil {
		t.Fatalf("first share: %v", err)
	}
	if !first.JustShared || first.Note.SharedAt == nil || !first.Note.SharedAt.Equal(fixedNow) {
		t.Errorf("first = %+v, want JustShared with the stamp", first)
	}

	rig.svc.now = func() time.Time { return fixedNow.Add(72 * time.Hour) }
	second, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID)
	if err != nil {
		t.Fatalf("second share: %v", err)
	}
	if second.JustShared {
		t.Error("repeat share must not report JustShared — the email would re-send")
	}
	if !second.Note.SharedAt.Equal(*first.Note.SharedAt) {
		t.Errorf("sharedAt = %v, want the original stamp kept", second.Note.SharedAt)
	}
}

func TestShareWithoutNote(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	if _, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Errorf("err = %v, want ErrNoteNotFound", err)
	}
	if _, err := rig.svc.ShareNotes(context.Background(), prac2ID, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("cross-practitioner err = %v, want ErrBookingNotFound", err)
	}
}

// TestGetNotesVisibility: the client sees nothing before sharing and the
// shared projection after; the practitioner always sees the full note.
func TestGetNotesVisibility(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	upsert(t, rig, b.ID, ports.NoteContent{
		PrivateNotes:    "secret diagnosis",
		SharedFeedback:  "great progress",
		SharedResources: []string{"https://example.com/aftercare"},
	})

	// Unshared: the owner's read is not-found, indistinguishable from no note.
	if _, err := rig.svc.GetNotes(context.Background(), clientOwner, b.ID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Errorf("unshared client read err = %v, want ErrNoteNotFound", err)
	}

	if _, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID); err != nil {
		t.Fatalf("share: %v", err)
	}

	view, err := rig.svc.GetNotes(context.Background(), clientOwner, b.ID)
	if err != nil {
		t.Fatalf("shared client read: %v", err)
	}
	if view.Shared == nil || view.Full != nil {
		t.Fatalf("client view = %+v, want the shared projection only", view)
	}
	if view.Shared.Feedback != "great progress" || len(view.Shared.Resources) != 1 || view.Shared.BookingID != b.ID {
		t.Errorf("shared = %+v, want shared content", view.Shared)
	}

	full, err := rig.svc.GetNotes(context.Background(), practitioner, b.ID)
	if err != nil {
		t.Fatalf("practitioner read: %v", err)
	}
	if full.Full == nil || full.Shared != nil || full.Full.PrivateNotes != "secret diagnosis" {
		t.Errorf("practitioner view = %+v, want the full note including private notes", full)
	}
}

// TestGetNotesIsolation: cross-owner and cross-practitioner reads are
// booking-not-found — no existence leak.
func TestGetNotesIsolation(t *testing.T) {
	rig := newTestRig()
	b := seedBooking(t, rig)
	upsert(t, rig, b.ID, ports.NoteContent{SharedFeedback: "x"})
	if _, err := rig.svc.ShareNotes(context.Background(), pracID, b.ID); err != nil {
		t.Fatalf("share: %v", err)
	}

	if _, err := rig.svc.GetNotes(context.Background(), clientOther, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("other client err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.GetNotes(context.Background(), practitioner2, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("other practitioner err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.GetNotes(context.Background(), clientOwner, "no-such-booking"); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("unknown booking err = %v, want ErrBookingNotFound", err)
	}

	// No note yet on a fresh booking: not-found for both roles.
	b2, err := booking.New("client-1", pracID, "svc-1", fixedNow.Add(48*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b2, err = rig.bookings.Create(context.Background(), b2)
	if err != nil {
		t.Fatalf("seed booking 2: %v", err)
	}
	if _, err := rig.svc.GetNotes(context.Background(), practitioner, b2.ID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Errorf("practitioner err = %v, want ErrNoteNotFound", err)
	}
	if _, err := rig.svc.GetNotes(context.Background(), clientOwner, b2.ID); !errors.Is(err, note.ErrNoteNotFound) {
		t.Errorf("client err = %v, want ErrNoteNotFound", err)
	}
}
