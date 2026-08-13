// Package signaling is the domain core for the video session: who may be
// in a room, when the room is open, and what one participant is allowed to
// send another. It imports nothing outside the standard library — no
// frameworks, no WebSocket library, no WebRTC.
//
// The server never looks inside an offer, an answer, or a candidate. Its
// whole job is to decide that two particular people are entitled to be in
// one particular room at one particular time, and then to pass opaque
// blobs between exactly those two. Everything that makes that decision
// lives here.
package signaling

import (
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// Join-window defaults. The room opens a little before the appointment so
// nobody is staring at a locked door, and stays open a little past the end
// so an overrunning session is not cut off mid-sentence.
const (
	DefaultOpenBefore = 10 * time.Minute
	DefaultCloseAfter = 15 * time.Minute
	// MaxParticipants is two: these are one-to-one sessions. A third
	// connection is refused rather than silently making a group call out
	// of a private consultation.
	MaxParticipants = 2
)

// JoinPolicy is the room's opening hours relative to the appointment.
type JoinPolicy struct {
	OpenBefore time.Duration
	CloseAfter time.Duration
}

// DefaultPolicy is the platform default.
func DefaultPolicy() JoinPolicy {
	return JoinPolicy{OpenBefore: DefaultOpenBefore, CloseAfter: DefaultCloseAfter}
}

// normalized fills zero fields with the defaults.
func (p JoinPolicy) normalized() JoinPolicy {
	if p.OpenBefore <= 0 {
		p.OpenBefore = DefaultOpenBefore
	}
	if p.CloseAfter <= 0 {
		p.CloseAfter = DefaultCloseAfter
	}
	return p
}

// Window is when a booking's room is open.
func (p JoinPolicy) Window(b booking.Booking) (opensAt, closesAt time.Time) {
	p = p.normalized()
	return b.StartAt.UTC().Add(-p.OpenBefore), b.EndAt.UTC().Add(p.CloseAfter)
}

// Open reports whether the room is open at `now`.
func (p JoinPolicy) Open(b booking.Booking, now time.Time) bool {
	opensAt, closesAt := p.Window(b)
	now = now.UTC()
	return !now.Before(opensAt) && now.Before(closesAt)
}

// Role is a participant's part in the call.
type Role string

const (
	RoleClient       Role = "client"
	RolePractitioner Role = "practitioner"
)

// Participant is one authorized member of a room.
type Participant struct {
	UserID string
	Role   Role
	// PeerID identifies this connection. One person may reconnect (a
	// dropped network, a refreshed tab), and the new connection is a new
	// peer even though the person is the same.
	PeerID string
}

// Room is an authorized session room, identified by its booking.
type Room struct {
	BookingID string
	// OpensAt and ClosesAt let the client show a countdown rather than
	// simply failing to connect.
	OpensAt  time.Time
	ClosesAt time.Time
}

// Authorize decides whether an identity may enter a booking's room, and
// returns the room if so.
//
// Four things must hold, and each failure is deliberately its own error so
// the portal can say something useful: the booking must exist and be
// theirs, it must still be confirmed, and the room must be open. A booking
// belonging to somebody else is reported as not-found — the same rule as
// everywhere else in this API, so a stranger cannot probe for whose
// sessions exist.
func Authorize(b booking.Booking, id identity.Identity, policy JoinPolicy, now time.Time) (Room, error) {
	if !owns(b, id) {
		return Room{}, booking.ErrBookingNotFound
	}
	if b.Status != booking.StatusConfirmed {
		return Room{}, ErrSessionNotActive
	}

	opensAt, closesAt := policy.Window(b)
	now = now.UTC()
	if now.Before(opensAt) {
		return Room{}, ErrRoomNotOpenYet
	}
	if !now.Before(closesAt) {
		return Room{}, ErrRoomClosed
	}

	return Room{BookingID: b.ID, OpensAt: opensAt, ClosesAt: closesAt}, nil
}

// RoleFor returns the caller's part in the call. It is derived from the
// booking, never from the request.
func RoleFor(b booking.Booking, id identity.Identity) (Role, bool) {
	switch {
	case b.ClientID == id.UserID && id.Role == identity.RoleClient:
		return RoleClient, true
	case b.PractitionerID == id.UserID && id.Role == identity.RolePractitioner:
		return RolePractitioner, true
	}
	return "", false
}

// owns reports whether the identity is a party to the booking.
func owns(b booking.Booking, id identity.Identity) bool {
	_, ok := RoleFor(b, id)
	return ok
}

// MessageType is the kind of signaling message. The set is closed: an
// unknown type is refused rather than relayed, so the socket carries only
// the negotiation, collaboration, and liveness traffic a video session
// needs — it is not a general-purpose message bus between two accounts.
type MessageType string

const (
	// Peer-to-peer negotiation, relayed opaquely.
	TypeOffer     MessageType = "offer"
	TypeAnswer    MessageType = "answer"
	TypeCandidate MessageType = "candidate"
	// In-call collaboration, also relayed opaquely. The server routes
	// these without inspecting their payloads; shape and size limits are
	// the clients' concern (bounded by the socket's frame cap).
	TypeChat     MessageType = "chat"     // {text} — one chat line
	TypeState    MessageType = "state"    // {micOn, cameraOn, handRaised, recording} — presence
	TypeReaction MessageType = "reaction" // {emoji} — one transient reaction
	TypeCaption  MessageType = "caption"  // {text, final} — own-mic transcription relay
	// Server-originated room events.
	TypeJoined    MessageType = "joined"
	TypePeerJoin  MessageType = "peer-joined"
	TypePeerLeave MessageType = "peer-left"
	TypeError     MessageType = "error"
	// Liveness, so a half-open connection is noticed.
	TypePing MessageType = "ping"
	TypePong MessageType = "pong"
)

// Relayable reports whether a client may send this type. Room events are
// the server's to announce; a participant that could forge "peer-left"
// could make the other side think the call had ended.
func (t MessageType) Relayable() bool {
	switch t {
	case TypeOffer, TypeAnswer, TypeCandidate,
		TypeChat, TypeState, TypeReaction, TypeCaption:
		return true
	}
	return false
}

// Valid reports whether a client may send this type at all, including the
// liveness messages that are not relayed.
func (t MessageType) Valid() bool {
	return t.Relayable() || t == TypePing || t == TypePong
}
