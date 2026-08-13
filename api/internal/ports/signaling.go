package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/signaling"
)

// TicketTTL is how long a connection ticket stays usable. Long enough to
// open a socket, short enough that a ticket which ended up in a proxy log
// is worthless by the time anyone reads it.
const TicketTTL = 60 * time.Second

// Ticket is a one-time credential for opening a signaling socket.
//
// It exists because a browser cannot set an Authorization header on a
// WebSocket handshake. The alternatives are putting the access token in
// the URL — where it lands in every access log and referrer — or in a
// subprotocol header. A single-use, minute-long ticket bound to one
// booking and one user leaks nothing worth having.
type Ticket struct {
	Value     string
	UserID    string
	Role      identity.Role
	BookingID string
	ExpiresAt time.Time
}

// TicketStore is the outbound port for connection tickets.
type TicketStore interface {
	// Issue mints and stores a ticket.
	Issue(ctx context.Context, ticket Ticket) error
	// Redeem consumes a ticket, returning it exactly once. An unknown,
	// expired, or already-spent value returns
	// signaling.ErrTicketInvalid.
	Redeem(ctx context.Context, value string) (Ticket, error)
}

// ICEServer is one STUN or TURN server the browser should try.
type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// ICEProvider supplies the STUN/TURN servers a browser should try.
//
// It is a port rather than a config value because Cloudflare's TURN — the
// service this platform uses — does not issue a static username and
// password. The two values a deployment holds are a long-term key that is
// exchanged, per session, for credentials that expire. Reading a fixed
// username out of the environment and handing it to the browser looks
// identical in a test and connects nobody in production.
type ICEProvider interface {
	// ICEServers returns servers valid for at least ttl. Implementations
	// may return credentials valid for longer, never shorter.
	//
	// A failure here must not be fatal to joining a session: STUN-only is
	// degraded, not broken, and a client on an unrestricted network still
	// connects. The caller decides what to do with the error.
	ICEServers(ctx context.Context, ttl time.Duration) ([]ICEServer, error)
}

// SessionAccess is everything a client needs to join a video room.
type SessionAccess struct {
	Room       signaling.Room
	Role       signaling.Role
	Ticket     string
	TicketTTL  time.Duration
	ICEServers []ICEServer
}

// SignalingService is the inbound port for the video slice (CX-01/CX-02).
type SignalingService interface {
	// Authorize checks that the caller may join the booking's room and
	// mints a connection ticket plus the ICE servers to use.
	Authorize(ctx context.Context, id identity.Identity, bookingID string) (SessionAccess, error)
	// RedeemTicket validates a ticket at socket-open time and returns the
	// participant it authorizes.
	RedeemTicket(ctx context.Context, value, bookingID string) (signaling.Room, signaling.Participant, error)
}
