// Package signaling is the application service for the video slice. It
// implements the inbound ports.SignalingService port purely against
// outbound ports — no framework, no WebSocket library, no WebRTC.
//
// It answers one question: may this person open a socket for this session
// right now? The transport adapter does the rest, and does no
// authorization of its own.
package signaling

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	domain "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// ticketBytes is the entropy in a connection ticket. 32 bytes is far
// beyond guessing in the minute it lives.
const ticketBytes = 32

// Service authorizes video sessions.
type Service struct {
	bookings ports.BookingRepository
	tickets  ports.TicketStore
	policy   domain.JoinPolicy
	ice      ports.ICEProvider
	log      func(error)
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.SignalingService = (*Service)(nil)

// Options configure a Service.
type Options struct {
	// Policy is the room's opening hours relative to the appointment.
	Policy domain.JoinPolicy
	// ICE supplies the STUN/TURN servers handed to the browser. It is a
	// provider rather than a list because TURN credentials expire and are
	// minted on demand — see ports.ICEProvider.
	ICE ports.ICEProvider
	// Report receives ICE failures. They are reported and not returned:
	// see Authorize. Nil discards them, which is only right in a test.
	Report func(error)
}

// NewService wires the use cases to their outbound ports.
func NewService(bookings ports.BookingRepository, tickets ports.TicketStore, opts Options) *Service {
	report := opts.Report
	if report == nil {
		report = func(error) {}
	}
	return &Service{
		bookings: bookings,
		tickets:  tickets,
		policy:   opts.Policy,
		ice:      opts.ICE,
		log:      report,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Authorize checks that the caller may join and mints a connection ticket.
//
// Authorization happens here, before any socket exists, so an unauthorized
// caller never gets as far as an upgrade — and the reason (too early, too
// late, session cancelled) can be returned as a proper HTTP error the
// portal can explain, which a failed WebSocket handshake cannot.
func (s *Service) Authorize(ctx context.Context, id identity.Identity, bookingID string) (ports.SessionAccess, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return ports.SessionAccess{}, err
	}
	room, err := domain.Authorize(b, id, s.policy, s.now())
	if err != nil {
		return ports.SessionAccess{}, err
	}
	role, ok := domain.RoleFor(b, id)
	if !ok {
		// Authorize already proved membership; this cannot happen, and if
		// it ever did the safe answer is the same not-found.
		return ports.SessionAccess{}, domain.ErrSessionNotActive
	}

	value, err := newTicketValue()
	if err != nil {
		return ports.SessionAccess{}, err
	}
	ticket := ports.Ticket{
		Value:     value,
		UserID:    id.UserID,
		Role:      id.Role,
		BookingID: b.ID,
		ExpiresAt: s.now().Add(ports.TicketTTL),
	}
	if err := s.tickets.Issue(ctx, ticket); err != nil {
		return ports.SessionAccess{}, fmt.Errorf("issue signaling ticket: %w", err)
	}

	return ports.SessionAccess{
		Room:       room,
		Role:       role,
		Ticket:     value,
		TicketTTL:  ports.TicketTTL,
		ICEServers: s.iceFor(ctx, room),
	}, nil
}

// iceFor fetches ICE servers covering the rest of the room's window.
//
// A failure here is reported and swallowed. Refusing to open the session
// because the TURN provider is unreachable would take video down for
// everyone, including the majority who connect peer-to-peer and never
// touch the relay; returning an empty list lets those calls work and only
// the hard-NAT ones fail. That is a worse outcome for some clients and a
// better one overall, and it is a deliberate choice rather than an
// oversight — which is why it is logged rather than ignored.
func (s *Service) iceFor(ctx context.Context, room domain.Room) []ports.ICEServer {
	if s.ice == nil {
		return nil
	}
	// Credentials must outlive the room, not the moment of joining.
	ttl := room.ClosesAt.Sub(s.now())
	if ttl < 0 {
		ttl = 0
	}
	servers, err := s.ice.ICEServers(ctx, ttl)
	if err != nil {
		s.log(fmt.Errorf("ice servers for booking %s: %w", room.BookingID, err))
		return nil
	}
	return servers
}

// RedeemTicket validates a ticket at socket-open time.
//
// The booking is re-checked rather than trusted from the ticket: a minute
// is long enough for a session to be cancelled, and the room must not
// outlive the booking that justified it.
func (s *Service) RedeemTicket(ctx context.Context, value, bookingID string) (domain.Room, domain.Participant, error) {
	ticket, err := s.tickets.Redeem(ctx, value)
	if err != nil {
		return domain.Room{}, domain.Participant{}, err
	}
	// A ticket for one session must not open another's room.
	if ticket.BookingID != bookingID {
		return domain.Room{}, domain.Participant{}, domain.ErrTicketInvalid
	}

	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return domain.Room{}, domain.Participant{}, err
	}
	id := identity.Identity{UserID: ticket.UserID, Role: ticket.Role}
	room, err := domain.Authorize(b, id, s.policy, s.now())
	if err != nil {
		return domain.Room{}, domain.Participant{}, err
	}
	role, ok := domain.RoleFor(b, id)
	if !ok {
		return domain.Room{}, domain.Participant{}, domain.ErrTicketInvalid
	}

	peerID, err := newTicketValue()
	if err != nil {
		return domain.Room{}, domain.Participant{}, err
	}
	return room, domain.Participant{UserID: ticket.UserID, Role: role, PeerID: peerID}, nil
}

// newTicketValue mints an unguessable, URL-safe value.
func newTicketValue() (string, error) {
	buf := make([]byte, ticketBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate signaling ticket: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
