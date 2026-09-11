package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// stubGate answers for a fixed set of (client, service) pairs that have
// been signed. Everything else is unsigned.
type stubGate struct {
	signed map[string]bool // key: clientID + "|" + serviceID
	asked  int
}

func (g *stubGate) RequireSigned(_ context.Context, clientID, serviceID string) error {
	g.asked++
	if g.signed[clientID+"|"+serviceID] {
		return nil
	}
	return agreement.ErrAgreementRequired
}

func newGatedRig(gate *stubGate) testRig {
	rig := testRig{
		bookings: portstest.NewFakeBookingRepository(),
		services: portstest.NewFakeServiceRepository(),
		avail:    portstest.NewFakeAvailabilityRepository(),
	}
	rig.svc = NewService(rig.bookings, rig.services, rig.avail, rig.bookings,
		booking.DefaultPolicy(), WithAgreementGate(gate))
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

// bookingCount counts what actually reached storage.
func bookingCount(t *testing.T, rig testRig) int {
	t.Helper()
	all, err := rig.bookings.ListByPractitioner(context.Background(), "prac-1", ports.BookingFilter{})
	if err != nil {
		t.Fatalf("ListByPractitioner: %v", err)
	}
	return len(all)
}

// The gate has to be on the write path, not only in the portal wizard: a
// client who posts straight at the API must be refused too.
func TestBookingIsRefusedWithoutASignedAgreement(t *testing.T) {
	gate := &stubGate{signed: map[string]bool{}}
	rig := newGatedRig(gate)
	svc, day := seedBookable(t, rig)

	_, err := rig.svc.CreateBooking(context.Background(), "client-1", svc.ID, day.Add(9*time.Hour), "UTC")
	if !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("err = %v, want ErrAgreementRequired", err)
	}
	if got := bookingCount(t, rig); got != 0 {
		t.Errorf("bookings stored = %d, want 0 — nothing may be written before the gate passes", got)
	}
}

func TestBookingProceedsOnceTheAgreementIsSigned(t *testing.T) {
	gate := &stubGate{signed: map[string]bool{}}
	rig := newGatedRig(gate)
	svc, day := seedBookable(t, rig)
	gate.signed["client-1|"+svc.ID] = true

	b, err := rig.svc.CreateBooking(context.Background(), "client-1", svc.ID, day.Add(9*time.Hour), "UTC")
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	if b.Status != booking.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", b.Status)
	}
}

// A priced service must be refused before it ever reaches a payment
// intent: charging for a session the practice would then have to refuse is
// the failure mode this ordering exists to prevent.
func TestPaidBookingIsRefusedBeforeAnyPaymentIsRequired(t *testing.T) {
	gate := &stubGate{signed: map[string]bool{}}
	rig := newGatedRig(gate)
	svc, day := seedBookable(t, rig)
	svc.PriceKobo = 25000
	if _, err := rig.services.Update(context.Background(), svc); err != nil {
		t.Fatalf("price service: %v", err)
	}

	_, err := rig.svc.CreateBooking(context.Background(), "client-1", svc.ID, day.Add(9*time.Hour), "UTC")
	if !errors.Is(err, agreement.ErrAgreementRequired) {
		t.Fatalf("err = %v, want ErrAgreementRequired", err)
	}
	if got := bookingCount(t, rig); got != 0 {
		t.Errorf("bookings stored = %d, want 0", got)
	}
}

// A deployment that wires no gate must behave exactly as before.
func TestNoGateMeansNoChange(t *testing.T) {
	rig := newTestRig()
	svc, day := seedBookable(t, rig)
	if _, err := rig.svc.CreateBooking(context.Background(), "client-1", svc.ID, day.Add(9*time.Hour), "UTC"); err != nil {
		t.Errorf("CreateBooking = %v, want nil", err)
	}
}
