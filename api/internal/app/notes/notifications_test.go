package notes

import (
	"context"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// notifiedRig is the notes service with the notifications option wired
// (BE-09), plus the accounts and catalog the notice is resolved from.
type notifiedRig struct {
	svc      *Service
	notes    *portstest.FakeSessionNoteRepository
	bookings *portstest.FakeBookingRepository
	users    *portstest.FakeUserRepository
	services *portstest.FakeServiceRepository
	notifier *portstest.FakeNotifier
}

func newNotifiedRig(t *testing.T) notifiedRig {
	t.Helper()
	rig := notifiedRig{
		notes:    portstest.NewFakeSessionNoteRepository(),
		bookings: portstest.NewFakeBookingRepository(),
		users:    portstest.NewFakeUserRepository(),
		services: portstest.NewFakeServiceRepository(),
		notifier: portstest.NewFakeNotifier(),
	}
	rig.svc = NewService(rig.notes, rig.bookings,
		WithNotifications(rig.notifier, rig.users, rig.services))
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

func seedUser(t *testing.T, rig notifiedRig, email, name string, role identity.Role) string {
	t.Helper()
	user, err := identity.NewUser(email, name, "hash", role, fixedNow)
	if err != nil {
		t.Fatalf("identity.NewUser: %v", err)
	}
	user, err = rig.users.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user.ID
}

// seedSharedBooking wires an account, a service, and a booking, and returns
// the stored booking plus the practitioner's id.
func seedSharedBooking(t *testing.T, rig notifiedRig) (booking.Booking, string, string) {
	t.Helper()
	ctx := context.Background()
	clientID := seedUser(t, rig, "ama@example.com", "Ama Serwaa", identity.RoleClient)
	practitionerID := seedUser(t, rig, "terios@example.com", "Terios Owusu", identity.RolePractitioner)

	svc, err := catalog.NewService(practitionerID, "Deep Tissue Massage", "", 60, 25000, "GHS", 1, fixedNow)
	if err != nil {
		t.Fatalf("catalog.NewService: %v", err)
	}
	svc, err = rig.services.Create(ctx, svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}

	b, err := booking.New(clientID, practitionerID, svc.ID, fixedNow.Add(24*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b, err = rig.bookings.Create(ctx, b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b, practitionerID, clientID
}

// TestFirstShareNotifiesTheClient: sharing feedback tells the client there
// is something to read, with the names resolved.
func TestFirstShareNotifiesTheClient(t *testing.T) {
	rig := newNotifiedRig(t)
	b, practitionerID, _ := seedSharedBooking(t, rig)

	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		PrivateNotes:    "tension in left shoulder",
		SharedFeedback:  "great progress",
		SharedResources: []string{"https://example.com/stretch"},
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	if _, err := rig.svc.ShareNotes(context.Background(), practitionerID, b.ID); err != nil {
		t.Fatalf("ShareNotes: %v", err)
	}

	if len(rig.notifier.Feedback) != 1 {
		t.Fatalf("feedback notices = %d, want 1", len(rig.notifier.Feedback))
	}
	notice := rig.notifier.Feedback[0]
	if notice.ClientEmail != "ama@example.com" || notice.ClientName != "Ama Serwaa" {
		t.Errorf("notice = %+v, want the client's identity", notice)
	}
	if notice.PractitionerName != "Terios Owusu" {
		t.Errorf("practitionerName = %q, want the sharing practitioner", notice.PractitionerName)
	}
	if notice.ServiceName != "Deep Tissue Massage" {
		t.Errorf("serviceName = %q, want the booked service", notice.ServiceName)
	}
	if !notice.HasResources {
		t.Error("hasResources = false, want true — resources were shared")
	}
	if !notice.SessionDate.Equal(b.StartAt) {
		t.Errorf("sessionDate = %v, want the session's start %v", notice.SessionDate, b.StartAt)
	}
}

// TestRepeatShareDoesNotNotifyAgain: share is idempotent, and so is the
// email — a client is told once.
func TestRepeatShareDoesNotNotifyAgain(t *testing.T) {
	rig := newNotifiedRig(t)
	b, practitionerID, _ := seedSharedBooking(t, rig)

	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		SharedFeedback: "great progress",
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := rig.svc.ShareNotes(context.Background(), practitionerID, b.ID); err != nil {
			t.Fatalf("ShareNotes %d: %v", i+1, err)
		}
	}

	if len(rig.notifier.Feedback) != 1 {
		t.Errorf("feedback notices = %d after three shares, want exactly 1", len(rig.notifier.Feedback))
	}
}

// TestEditingAfterSharingDoesNotNotify: revising the wording of shared
// feedback is not a new event for the client.
func TestEditingAfterSharingDoesNotNotify(t *testing.T) {
	rig := newNotifiedRig(t)
	b, practitionerID, _ := seedSharedBooking(t, rig)

	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		SharedFeedback: "great progress",
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	if _, err := rig.svc.ShareNotes(context.Background(), practitionerID, b.ID); err != nil {
		t.Fatalf("ShareNotes: %v", err)
	}
	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		SharedFeedback: "great progress — corrected typo",
	}); err != nil {
		t.Fatalf("second UpsertNotes: %v", err)
	}

	if len(rig.notifier.Feedback) != 1 {
		t.Errorf("feedback notices = %d, want the edit to stay silent", len(rig.notifier.Feedback))
	}
}

// TestUnsharedNotesNotifyNobody: writing private notes is not a client
// event at all.
func TestUnsharedNotesNotifyNobody(t *testing.T) {
	rig := newNotifiedRig(t)
	b, practitionerID, _ := seedSharedBooking(t, rig)

	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		PrivateNotes: "secret diagnosis",
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}

	if len(rig.notifier.Feedback) != 0 {
		t.Errorf("feedback notices = %d, want none before sharing", len(rig.notifier.Feedback))
	}
}

// TestShareWithoutResourcesClearsTheFlag: the email's optional clause is
// driven by what was actually shared.
func TestShareWithoutResourcesClearsTheFlag(t *testing.T) {
	rig := newNotifiedRig(t)
	b, practitionerID, _ := seedSharedBooking(t, rig)

	if _, err := rig.svc.UpsertNotes(context.Background(), practitionerID, b.ID, ports.NoteContent{
		SharedFeedback: "great progress",
	}); err != nil {
		t.Fatalf("UpsertNotes: %v", err)
	}
	if _, err := rig.svc.ShareNotes(context.Background(), practitionerID, b.ID); err != nil {
		t.Fatalf("ShareNotes: %v", err)
	}

	if rig.notifier.Feedback[0].HasResources {
		t.Error("hasResources = true, want false — no resources were shared")
	}
}
