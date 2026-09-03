package booking

import (
	"context"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	domain "github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// notifiedRig is the booking service with the notifications option wired
// (BE-09), plus the user repository the notices are resolved from.
type notifiedRig struct {
	testRig
	notifier *portstest.FakeNotifier
	users    *portstest.FakeUserRepository
}

func newNotifiedRig(t *testing.T) notifiedRig {
	t.Helper()
	rig := notifiedRig{
		testRig: testRig{
			bookings: portstest.NewFakeBookingRepository(),
			services: portstest.NewFakeServiceRepository(),
			avail:    portstest.NewFakeAvailabilityRepository(),
		},
		notifier: portstest.NewFakeNotifier(),
		users:    portstest.NewFakeUserRepository(),
	}
	rig.svc = NewService(
		rig.bookings, rig.services, rig.avail, rig.bookings, booking.DefaultPolicy(),
		WithNotifications(rig.notifier, rig.users),
	)
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

// seedClient creates the account the notice's name and address come from.
func seedClient(t *testing.T, rig notifiedRig, email, name string) string {
	t.Helper()
	user, err := identity.NewUser(email, name, "hash", identity.RoleClient, fixedNow)
	if err != nil {
		t.Fatalf("identity.NewUser: %v", err)
	}
	user, err = rig.users.Create(context.Background(), user)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user.ID
}

// seedBookableFor mirrors seedBookable on the notified rig.
func seedBookableFor(t *testing.T, rig notifiedRig) (catalog.Service, time.Time) {
	t.Helper()
	ctx := context.Background()
	svc, err := catalog.NewService("prac-1", "Deep Tissue Massage", "", 60, 0, "USD", 1, fixedNow)
	if err != nil {
		t.Fatalf("domain NewService: %v", err)
	}
	svc, err = rig.services.Create(ctx, svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	day := fixedNow.Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	err = rig.avail.ReplaceRules(ctx, "prac-1", []domain.WeeklyRule{{
		Weekday: day.Weekday(),
		Windows: []domain.Window{{StartMin: 540, EndMin: 720}},
	}})
	if err != nil {
		t.Fatalf("seed rules: %v", err)
	}
	return svc, day
}

// TestCreateAnnouncesTheBooking: a confirmed booking tells the notifier who
// booked what, and when.
func TestCreateAnnouncesTheBooking(t *testing.T) {
	rig := newNotifiedRig(t)
	clientID := seedClient(t, rig, "ama@example.com", "Ama Serwaa")
	svc, day := seedBookableFor(t, rig)
	start := day.Add(9 * time.Hour)

	b, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, start, "Africa/Accra")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	if len(rig.notifier.Confirmed) != 1 {
		t.Fatalf("confirmations = %d, want 1", len(rig.notifier.Confirmed))
	}
	notice := rig.notifier.Confirmed[0]
	if notice.BookingID != b.ID {
		t.Errorf("bookingId = %q, want the stored booking %q", notice.BookingID, b.ID)
	}
	if notice.ClientName != "Ama Serwaa" || notice.ClientEmail != "ama@example.com" {
		t.Errorf("notice = %+v, want the client's resolved identity", notice)
	}
	if notice.ServiceName != "Deep Tissue Massage" {
		t.Errorf("serviceName = %q, want the booked service", notice.ServiceName)
	}
	if !notice.StartAt.Equal(start) || notice.Timezone != "Africa/Accra" {
		t.Errorf("notice = %+v, want the slot and the request timezone", notice)
	}
}

// TestFailedCreateAnnouncesNothing: a rejected booking must not email
// anyone about a session they do not have.
func TestFailedCreateAnnouncesNothing(t *testing.T) {
	rig := newNotifiedRig(t)
	clientID := seedClient(t, rig, "ama@example.com", "Ama Serwaa")
	svc, day := seedBookableFor(t, rig)
	taken := day.Add(9 * time.Hour)

	if _, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, taken, "UTC"); err != nil {
		t.Fatalf("first CreateBooking: %v", err)
	}
	otherID := seedClient(t, rig, "koffi@example.com", "Koffi")
	if _, err := rig.svc.CreateBooking(context.Background(), otherID, svc.ID, taken, "UTC"); err == nil {
		t.Fatal("second booking of the same slot succeeded, want ErrSlotUnavailable")
	}

	if len(rig.notifier.Confirmed) != 1 {
		t.Errorf("confirmations = %d, want only the booking that actually succeeded", len(rig.notifier.Confirmed))
	}

	// A slot that was never bookable at all is the same story.
	if _, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, day.Add(20*time.Hour), "UTC"); err == nil {
		t.Fatal("booking outside the working window succeeded")
	}
	if len(rig.notifier.Confirmed) != 1 {
		t.Errorf("confirmations = %d after a rejected booking, want 1", len(rig.notifier.Confirmed))
	}
}

// TestRescheduleAnnouncesBothTimes: the notice carries the old and the new
// slot, which is what the client's email compares.
func TestRescheduleAnnouncesBothTimes(t *testing.T) {
	rig := newNotifiedRig(t)
	clientID := seedClient(t, rig, "ama@example.com", "Ama Serwaa")
	svc, day := seedBookableFor(t, rig)
	original := day.Add(9 * time.Hour)
	moved := day.Add(11 * time.Hour)

	b, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, original, "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	caller := identity.Identity{UserID: clientID, Role: identity.RoleClient}
	if _, err := rig.svc.RescheduleBooking(context.Background(), caller, b.ID, moved, "UTC"); err != nil {
		t.Fatalf("RescheduleBooking: %v", err)
	}

	if len(rig.notifier.Rescheduled) != 1 {
		t.Fatalf("reschedule notices = %d, want 1", len(rig.notifier.Rescheduled))
	}
	notice := rig.notifier.Rescheduled[0]
	if !notice.PreviousStartAt.Equal(original) {
		t.Errorf("previousStartAt = %v, want the slot it moved from (%v)", notice.PreviousStartAt, original)
	}
	if !notice.StartAt.Equal(moved) {
		t.Errorf("startAt = %v, want the new slot (%v)", notice.StartAt, moved)
	}
}

// TestCancelAnnouncesTheCancellation.
func TestCancelAnnouncesTheCancellation(t *testing.T) {
	rig := newNotifiedRig(t)
	clientID := seedClient(t, rig, "ama@example.com", "Ama Serwaa")
	svc, day := seedBookableFor(t, rig)
	start := day.Add(9 * time.Hour)

	b, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, start, "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	caller := identity.Identity{UserID: clientID, Role: identity.RoleClient}
	if _, err := rig.svc.CancelBooking(context.Background(), caller, b.ID); err != nil {
		t.Fatalf("CancelBooking: %v", err)
	}

	if len(rig.notifier.Cancelled) != 1 {
		t.Fatalf("cancellation notices = %d, want 1", len(rig.notifier.Cancelled))
	}
	if rig.notifier.Cancelled[0].BookingID != b.ID {
		t.Errorf("bookingId = %q, want %q", rig.notifier.Cancelled[0].BookingID, b.ID)
	}
}

// TestCutoffRejectionAnnouncesNothing: a cancellation the rules refused
// must not tell the client their session is off.
func TestCutoffRejectionAnnouncesNothing(t *testing.T) {
	rig := newNotifiedRig(t)
	clientID := seedClient(t, rig, "ama@example.com", "Ama Serwaa")
	svc, day := seedBookableFor(t, rig)
	start := day.Add(9 * time.Hour)

	b, err := rig.svc.CreateBooking(context.Background(), clientID, svc.ID, start, "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}

	// Jump to inside the 24-hour cutoff.
	rig.svc.now = func() time.Time { return start.Add(-2 * time.Hour) }
	caller := identity.Identity{UserID: clientID, Role: identity.RoleClient}
	if _, err := rig.svc.CancelBooking(context.Background(), caller, b.ID); err == nil {
		t.Fatal("cancel inside the cutoff succeeded, want ErrCutoffPassed")
	}

	if len(rig.notifier.Cancelled) != 0 {
		t.Errorf("cancellation notices = %d, want none for a refused cancellation", len(rig.notifier.Cancelled))
	}
}

// TestNotificationsAreOptional: the slice works unchanged when no notifier
// is wired, which is how every other test in this package runs.
func TestNotificationsAreOptional(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)

	if _, err := rig.svc.CreateBooking(context.Background(), "client-1", svc.ID, day.Add(9*time.Hour), "UTC"); err != nil {
		t.Fatalf("CreateBooking without a notifier: %v", err)
	}
}

// TestMissingAccountSkipsTheNotice: with no account there is no address, so
// there is nothing to send — and the booking still succeeds.
func TestMissingAccountSkipsTheNotice(t *testing.T) {
	rig := newNotifiedRig(t)
	svc, day := seedBookableFor(t, rig)

	b, err := rig.svc.CreateBooking(context.Background(), "ghost-client", svc.ID, day.Add(9*time.Hour), "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	if b.ID == "" {
		t.Error("booking was not stored")
	}
	if len(rig.notifier.Confirmed) != 0 {
		t.Errorf("confirmations = %d, want none with no account to address", len(rig.notifier.Confirmed))
	}
}
