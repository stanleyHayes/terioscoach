// Package memory holds in-process stores for state that is genuinely
// short-lived and instance-local. Everything here is deliberately not in
// MongoDB, and each type documents why that is the right call.
package memory

import (
	"context"
	"sync"
	"time"

	domain "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// sweepEvery bounds how often expired tickets are cleared out. Tickets
// live a minute, so a sweep at that cadence keeps the map from growing
// under a stream of connection attempts.
const sweepEvery = time.Minute

// TicketStore holds signaling tickets in memory.
//
// In-process is correct here rather than a compromise: a ticket lives for
// sixty seconds and is redeemed by the very next request, which — because
// the socket must reach the process holding the room — has to arrive at
// this same instance anyway. Putting it in MongoDB would add a round trip
// and a collection to reap for no gain. If the API is ever scaled out, the
// rooms themselves are the thing that needs a shared backend; the tickets
// follow whatever that decision is.
type TicketStore struct {
	mu        sync.Mutex
	byValue   map[string]ports.Ticket
	lastSweep time.Time
	now       func() time.Time
}

var _ ports.TicketStore = (*TicketStore)(nil)

// NewTicketStore builds an empty store.
func NewTicketStore() *TicketStore {
	return &TicketStore{
		byValue: make(map[string]ports.Ticket),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// Issue stores a ticket.
func (s *TicketStore) Issue(_ context.Context, ticket ports.Ticket) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep(s.now())
	s.byValue[ticket.Value] = ticket
	return nil
}

// Redeem consumes a ticket, returning it exactly once.
//
// The delete happens whether or not the ticket turned out to be valid, so
// a replayed value is spent even if the first use failed — single-use is
// the whole point, and "spend on read" is what makes a captured ticket
// worthless the moment the legitimate holder uses it.
func (s *TicketStore) Redeem(_ context.Context, value string) (ports.Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ticket, ok := s.byValue[value]
	if !ok {
		return ports.Ticket{}, domain.ErrTicketInvalid
	}
	delete(s.byValue, value)

	if !s.now().Before(ticket.ExpiresAt) {
		return ports.Ticket{}, domain.ErrTicketInvalid
	}
	return ticket, nil
}

// sweep drops expired tickets. It runs at most once per window and holds
// the lock its caller already took.
func (s *TicketStore) sweep(now time.Time) {
	if now.Sub(s.lastSweep) < sweepEvery {
		return
	}
	s.lastSweep = now
	for value, ticket := range s.byValue {
		if !now.Before(ticket.ExpiresAt) {
			delete(s.byValue, value)
		}
	}
}
