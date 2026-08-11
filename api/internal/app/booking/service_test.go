package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	domain "github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// fixedNow keeps cutoff and slot-futurity assertions deterministic.
var fixedNow = time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC) // a Monday

type testRig struct {
	svc      *Service
	bookings *portstest.FakeBookingRepository
	services *portstest.FakeServiceRepository
	avail    *portstest.FakeAvailabilityRepository
}

// newTestRig wires the service over in-memory fakes. The booking repository
// doubles as the busy-interval reader, so a fresh booking immediately
// blocks its own slot — exactly how the MongoDB wiring behaves.
func newTestRig() testRig {
	rig := testRig{
		bookings: portstest.NewFakeBookingRepository(),
		services: portstest.NewFakeServiceRepository(),
		avail:    portstest.NewFakeAvailabilityRepository(),
	}
	rig.svc = NewService(rig.bookings, rig.services, rig.avail, rig.bookings, booking.DefaultPolicy())
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

var (
	client1 = identity.Identity{UserID: "client-1", Role: identity.RoleClient}
	client2 = identity.Identity{UserID: "client-2", Role: identity.RoleClient}
	prac1   = identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}
	prac2   = identity.Identity{UserID: "prac-2", Role: identity.RolePractitioner}
)

// slotDay is seven days out from fixedNow; seedBookable opens 09:00-12:00
// that weekday (zero buffer) and returns the service and the day. Valid
// slots: 09:00, 10:00, 11:00 UTC.
func seedBookable(t *testing.T, rig testRig) (catalog.Service, time.Time) {
	t.Helper()
	ctx := context.Background()
	svc, err := catalog.NewService("prac-1", "Massage", "", 60, 25000, "GHS", 1, fixedNow)
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

func createBooking(t *testing.T, rig testRig, clientID, serviceID string, startAt time.Time) booking.Booking {
	t.Helper()
	b, err := rig.svc.CreateBooking(context.Background(), clientID, serviceID, startAt, "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	return b
}

func TestCreateBookingHappyPath(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	start := day.Add(9 * time.Hour)

	b := createBooking(t, rig, "client-1", svc.ID, start)
	if b.ID == "" || b.Status != booking.StatusConfirmed {
		t.Errorf("booking = %+v, want id assigned and status confirmed", b)
	}
	if b.ClientID != "client-1" || b.PractitionerID != "prac-1" || b.ServiceID != svc.ID {
		t.Errorf("booking = %+v, want parties stamped", b)
	}
	if !b.EndAt.Equal(start.Add(time.Hour)) {
		t.Errorf("endAt = %v, want startAt+60m", b.EndAt)
	}
}

func TestCreateBookingSlotMatchValidation(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()

	cases := []struct {
		name    string
		startAt time.Time
	}{
		{"misaligned start", day.Add(9*time.Hour + 30*time.Minute)},      // not on the step grid
		{"outside window", day.Add(14 * time.Hour)},                      // window closes 12:00
		{"closed weekday", day.Add(24*time.Hour + 9*time.Hour)},          // no rule next day
		{"in the past", fixedNow.Add(-time.Hour)},                        // slots must be future
		{"inside but too short", day.Add(11*time.Hour + 30*time.Minute)}, // would overrun the window
		{"valid next-day start on closed day", day.Add(26 * time.Hour)},  // only one weekday open
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rig.svc.CreateBooking(ctx, "client-1", svc.ID, tc.startAt, "UTC")
			if !errors.Is(err, booking.ErrSlotUnavailable) {
				t.Errorf("err = %v, want ErrSlotUnavailable", err)
			}
		})
	}

	// Time-off over the whole day removes every slot.
	if _, err := rig.avail.CreateTimeOff(ctx, domain.TimeOff{
		PractitionerID: "prac-1", StartAt: day, EndAt: day.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("seed time-off: %v", err)
	}
	if _, err := rig.svc.CreateBooking(ctx, "client-1", svc.ID, day.Add(9*time.Hour), "UTC"); !errors.Is(err, booking.ErrSlotUnavailable) {
		t.Errorf("time-off err = %v, want ErrSlotUnavailable", err)
	}
}

func TestCreateBookingBlocksBookedSlot(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	start := day.Add(9 * time.Hour)
	ctx := context.Background()

	createBooking(t, rig, "client-1", svc.ID, start)
	// The slot engine now sees the booking as busy: a second create fails
	// at validation with the same error a race loser would get.
	if _, err := rig.svc.CreateBooking(ctx, "client-2", svc.ID, start, "UTC"); !errors.Is(err, booking.ErrSlotUnavailable) {
		t.Fatalf("double-book err = %v, want ErrSlotUnavailable", err)
	}
}

func TestCreateBookingRaceLosesToStorageGuard(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	start := day.Add(9 * time.Hour)
	ctx := context.Background()

	// Simulate a race: both requests validate against a busy view that
	// does not yet include either booking (stale read), then both insert.
	rig.svc.busy = &portstest.FakeBusyIntervalReader{}
	createBooking(t, rig, "client-1", svc.ID, start)
	_, err := rig.svc.CreateBooking(ctx, "client-2", svc.ID, start, "UTC")
	if !errors.Is(err, booking.ErrSlotUnavailable) {
		t.Fatalf("race loser err = %v, want ErrSlotUnavailable (unique index guard)", err)
	}
}

func TestCreateBookingServiceMisses(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()

	if _, err := rig.svc.CreateBooking(ctx, "client-1", "unknown", day.Add(9*time.Hour), "UTC"); !errors.Is(err, catalog.ErrServiceNotFound) {
		t.Errorf("unknown service err = %v, want ErrServiceNotFound", err)
	}
	svc.Active = false
	if _, err := rig.services.Update(ctx, svc); err != nil {
		t.Fatalf("deactivate service: %v", err)
	}
	if _, err := rig.svc.CreateBooking(ctx, "client-1", svc.ID, day.Add(9*time.Hour), "UTC"); !errors.Is(err, catalog.ErrServiceNotFound) {
		t.Errorf("inactive service err = %v, want ErrServiceNotFound (no existence leak)", err)
	}
	if _, err := rig.svc.CreateBooking(ctx, "client-1", svc.ID, day.Add(9*time.Hour), "Mars/Olympus"); !errors.Is(err, domain.ErrInvalidTimezone) {
		t.Errorf("bad tz err = %v, want ErrInvalidTimezone", err)
	}
}

func TestRescheduleMovesSlotAndFreesOld(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()
	b := createBooking(t, rig, "client-1", svc.ID, day.Add(9*time.Hour))

	moved, err := rig.svc.RescheduleBooking(ctx, client1, b.ID, day.Add(11*time.Hour), "UTC")
	if err != nil {
		t.Fatalf("RescheduleBooking: %v", err)
	}
	if !moved.StartAt.Equal(day.Add(11*time.Hour)) || !moved.EndAt.Equal(day.Add(12*time.Hour)) {
		t.Errorf("moved = [%v, %v), want [11:00, 12:00)", moved.StartAt, moved.EndAt)
	}

	// The old slot is free again: another client can take it.
	if _, err := rig.svc.CreateBooking(ctx, "client-2", svc.ID, day.Add(9*time.Hour), "UTC"); err != nil {
		t.Errorf("old slot should be free after reschedule: %v", err)
	}

	// Rescheduling onto the booking's own current slot is a valid no-op —
	// the booking must not block itself.
	if _, err := rig.svc.RescheduleBooking(ctx, client1, b.ID, day.Add(11*time.Hour), "UTC"); err != nil {
		t.Errorf("self-reschedule err = %v, want nil (own interval excluded)", err)
	}

	// Misaligned target is rejected.
	if _, err := rig.svc.RescheduleBooking(ctx, client1, b.ID, day.Add(11*time.Hour+30*time.Minute), "UTC"); !errors.Is(err, booking.ErrSlotUnavailable) {
		t.Errorf("misaligned reschedule err = %v, want ErrSlotUnavailable", err)
	}
}

func TestRescheduleAndCancelCutoff(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()

	// Inside the 24h cutoff (slot day is 7 days out; move now close to it).
	near := day.Add(-12 * time.Hour)
	rig.svc.now = func() time.Time { return near }

	b := createBooking(t, rig, "client-1", svc.ID, day.Add(9*time.Hour))

	if _, err := rig.svc.RescheduleBooking(ctx, client1, b.ID, day.Add(10*time.Hour), "UTC"); !errors.Is(err, booking.ErrCutoffPassed) {
		t.Errorf("client reschedule inside cutoff err = %v, want ErrCutoffPassed", err)
	}
	if _, err := rig.svc.CancelBooking(ctx, client1, b.ID); !errors.Is(err, booking.ErrCutoffPassed) {
		t.Errorf("client cancel inside cutoff err = %v, want ErrCutoffPassed", err)
	}

	// The practitioner is never cutoff-restricted.
	if _, err := rig.svc.RescheduleBooking(ctx, prac1, b.ID, day.Add(10*time.Hour), "UTC"); err != nil {
		t.Errorf("practitioner reschedule inside cutoff: %v", err)
	}
	cancelled, err := rig.svc.CancelBooking(ctx, prac1, b.ID)
	if err != nil {
		t.Fatalf("practitioner cancel inside cutoff: %v", err)
	}
	if cancelled.Status != booking.StatusCancelled || cancelled.CancelledAt == nil {
		t.Errorf("cancelled = %+v, want status cancelled with timestamp", cancelled)
	}

	// Terminal state: nothing further allowed.
	if _, err := rig.svc.CancelBooking(ctx, prac1, b.ID); !errors.Is(err, booking.ErrInvalidTransition) {
		t.Errorf("double cancel err = %v, want ErrInvalidTransition", err)
	}
}

func TestCancelFreesSlot(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()
	start := day.Add(9 * time.Hour)

	b := createBooking(t, rig, "client-1", svc.ID, start)
	if _, err := rig.svc.CancelBooking(ctx, client1, b.ID); err != nil {
		t.Fatalf("CancelBooking: %v", err)
	}
	if _, err := rig.svc.CreateBooking(ctx, "client-2", svc.ID, start, "UTC"); err != nil {
		t.Errorf("cancelled slot should be bookable again: %v", err)
	}
}

func TestCrossOwnerAccessIsNotFound(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()
	b := createBooking(t, rig, "client-1", svc.ID, day.Add(9*time.Hour))

	// Another client: read, reschedule, and cancel all report not-found —
	// no existence leak.
	if _, err := rig.svc.GetBooking(ctx, client2, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("client2 get err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.RescheduleBooking(ctx, client2, b.ID, day.Add(10*time.Hour), "UTC"); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("client2 reschedule err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.CancelBooking(ctx, client2, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("client2 cancel err = %v, want ErrBookingNotFound", err)
	}

	// A different practitioner is equally isolated.
	if _, err := rig.svc.GetBooking(ctx, prac2, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("prac2 get err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.CompleteBooking(ctx, "prac-2", b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("prac2 complete err = %v, want ErrBookingNotFound", err)
	}

	// Owner and own practitioner see it fine.
	if _, err := rig.svc.GetBooking(ctx, client1, b.ID); err != nil {
		t.Errorf("owner get: %v", err)
	}
	if _, err := rig.svc.GetBooking(ctx, prac1, b.ID); err != nil {
		t.Errorf("practitioner get: %v", err)
	}
}

func TestCompleteAndNoShow(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()

	// Seed a past booking directly — the API can only create future ones.
	past, err := booking.New("client-1", "prac-1", "svc-1", fixedNow.Add(-3*time.Hour), 60, fixedNow.Add(-4*time.Hour))
	if err != nil {
		t.Fatalf("domain New: %v", err)
	}
	past, err = rig.bookings.Create(ctx, past)
	if err != nil {
		t.Fatalf("seed past booking: %v", err)
	}

	completed, err := rig.svc.CompleteBooking(ctx, "prac-1", past.ID)
	if err != nil {
		t.Fatalf("CompleteBooking: %v", err)
	}
	if completed.Status != booking.StatusCompleted || completed.CompletedAt == nil {
		t.Errorf("completed = %+v, want status completed with timestamp", completed)
	}
	// Terminal.
	if _, err := rig.svc.MarkNoShow(ctx, "prac-1", past.ID); !errors.Is(err, booking.ErrInvalidTransition) {
		t.Errorf("no-show after complete err = %v, want ErrInvalidTransition", err)
	}

	// A future booking cannot be completed early.
	future, err := booking.New("client-1", "prac-1", "svc-1", fixedNow.Add(3*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("domain New: %v", err)
	}
	future, err = rig.bookings.Create(ctx, future)
	if err != nil {
		t.Fatalf("seed future booking: %v", err)
	}
	if _, err := rig.svc.CompleteBooking(ctx, "prac-1", future.ID); !errors.Is(err, booking.ErrTooEarly) {
		t.Errorf("early complete err = %v, want ErrTooEarly", err)
	}
	if _, err := rig.svc.MarkNoShow(ctx, "prac-1", future.ID); !errors.Is(err, booking.ErrTooEarly) {
		t.Errorf("early no-show err = %v, want ErrTooEarly", err)
	}

	// In-progress booking can be marked no-show.
	running, err := booking.New("client-1", "prac-1", "svc-1", fixedNow.Add(-30*time.Minute), 60, fixedNow.Add(-time.Hour))
	if err != nil {
		t.Fatalf("domain New: %v", err)
	}
	running, err = rig.bookings.Create(ctx, running)
	if err != nil {
		t.Fatalf("seed running booking: %v", err)
	}
	noShow, err := rig.svc.MarkNoShow(ctx, "prac-1", running.ID)
	if err != nil {
		t.Fatalf("MarkNoShow: %v", err)
	}
	if noShow.Status != booking.StatusNoShow {
		t.Errorf("status = %q, want no_show", noShow.Status)
	}
}

func TestLists(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	ctx := context.Background()

	first := createBooking(t, rig, "client-1", svc.ID, day.Add(9*time.Hour))
	second := createBooking(t, rig, "client-1", svc.ID, day.Add(10*time.Hour))
	other := createBooking(t, rig, "client-2", svc.ID, day.Add(11*time.Hour))

	mine, err := rig.svc.ListMine(ctx, "client-1")
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(mine) != 2 || mine[0].ID != first.ID || mine[1].ID != second.ID {
		t.Errorf("mine = %+v, want client-1's two bookings, startAt ascending", mine)
	}

	all, err := rig.svc.ListForPractitioner(ctx, "prac-1", ports.BookingFilter{})
	if err != nil {
		t.Fatalf("ListForPractitioner: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("all = %d bookings, want 3", len(all))
	}

	from := day.Add(10 * time.Hour)
	window, err := rig.svc.ListForPractitioner(ctx, "prac-1", ports.BookingFilter{From: &from})
	if err != nil {
		t.Fatalf("ListForPractitioner from: %v", err)
	}
	if len(window) != 2 || window[0].ID != second.ID || window[1].ID != other.ID {
		t.Errorf("from-filtered = %+v, want the 10:00 and 11:00 bookings", window)
	}

	if _, err := rig.svc.CancelBooking(ctx, client1, first.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cancelled, err := rig.svc.ListForPractitioner(ctx, "prac-1", ports.BookingFilter{Status: booking.StatusCancelled})
	if err != nil {
		t.Fatalf("ListForPractitioner status: %v", err)
	}
	if len(cancelled) != 1 || cancelled[0].ID != first.ID {
		t.Errorf("status-filtered = %+v, want only the cancelled one", cancelled)
	}
}

// Compile-time check that the fakes satisfy the ports they stand in for.
var _ ports.BookingRepository = (*portstest.FakeBookingRepository)(nil)
var _ ports.BusyIntervalReader = (*portstest.FakeBookingRepository)(nil)
