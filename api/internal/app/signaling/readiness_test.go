package signaling

import (
	"context"
	"errors"
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
	"testing"
	"time"
)

type testTickets struct{ ticket ports.Ticket }

func (t *testTickets) Issue(_ context.Context, value ports.Ticket) error {
	t.ticket = value
	return nil
}
func (t *testTickets) Redeem(context.Context, string) (ports.Ticket, error) { return t.ticket, nil }

type readinessGate struct {
	err   error
	calls int
}

func (g *readinessGate) RequireBookingReady(context.Context, string) error { g.calls++; return g.err }
func TestReadinessChecksBothTicketIssueAndRedemption(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	repo := portstest.NewFakeBookingRepository()
	b, err := repo.Create(ctx, booking.Booking{ClientID: "client", PractitionerID: "practitioner", Status: booking.StatusConfirmed, StartAt: now, EndAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []identity.Identity{{UserID: "client", Role: identity.RoleClient}, {UserID: "practitioner", Role: identity.RolePractitioner}} {
		gate := &readinessGate{err: agreement.ErrAgreementRequired}
		tickets := &testTickets{}
		svc := NewService(repo, tickets, Options{Readiness: gate})
		svc.now = func() time.Time { return now }
		if _, err = svc.Authorize(ctx, id, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
			t.Fatalf("issue bypass: %v", err)
		}
		if tickets.ticket.Value != "" {
			t.Fatal("ticket issued before consent")
		}
		gate.err = nil
		access, err := svc.Authorize(ctx, id, b.ID)
		if err != nil {
			t.Fatal(err)
		}
		gate.err = agreement.ErrAgreementRequired
		if _, _, err = svc.RedeemTicket(ctx, access.Ticket, b.ID); !errors.Is(err, agreement.ErrAgreementRequired) {
			t.Fatal("redemption bypassed changed readiness")
		}
	}
}
