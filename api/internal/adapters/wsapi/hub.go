// Package wsapi is the inbound WebSocket adapter for video signaling. It
// owns the sockets and the room bookkeeping; every authorization decision
// belongs to the signaling service, which has already been consulted
// before a socket is opened.
//
// Scope note: rooms live in this process. That is sound for the current
// single-instance deployment — both peers reach the same instance, so both
// reach the same room. Scaling the API out would need the peers pinned to
// one instance or a shared relay; the ticket redemption is where that
// decision would land.
package wsapi

import (
	"encoding/json"
	"sync"

	domain "github.com/xcreativs/terios/api/internal/domain/signaling"
)

// Envelope is one signaling message on the wire.
//
// Payload is a raw JSON blob and stays that way: it carries SDP and ICE
// candidates, which are the peers' business. The server routes it without
// looking inside, which is both the privacy property and the reason this
// stays simple as WebRTC evolves.
type Envelope struct {
	Type domain.MessageType `json:"type"`
	// From is stamped by the server so a peer cannot claim to be the
	// other one.
	From string `json:"from,omitempty"`
	// Payload is opaque negotiation data.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Reason carries a human-readable explanation on error frames.
	Reason string `json:"reason,omitempty"`
	// Role tells a joining peer who the other party is, so the UI can
	// label the tile without another request.
	Role domain.Role `json:"role,omitempty"`
}

// peer is one live connection in a room.
type peer struct {
	participant domain.Participant
	// send is buffered: a slow reader must not block the peer sending to
	// it. A full buffer means the connection is not keeping up and is
	// closed rather than allowed to stall the other side.
	send chan Envelope
}

// sendBuffer is how many messages may queue for one peer. ICE candidates
// arrive in small bursts; anything beyond this is a stalled connection.
const sendBuffer = 32

// Hub holds the live rooms.
type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[string]*peer // bookingID -> peerID -> peer
}

// NewHub builds an empty hub.
func NewHub() *Hub {
	return &Hub{rooms: make(map[string]map[string]*peer)}
}

// Join adds a peer to a room, returning its send channel and the
// participants already present.
//
// A room holds at most two people. The third connection is refused rather
// than admitted, because these are one-to-one consultations and quietly
// turning one into a group call would be a privacy failure, not a feature.
func (h *Hub) Join(bookingID string, participant domain.Participant) (<-chan Envelope, []domain.Participant, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room := h.rooms[bookingID]
	if room == nil {
		room = make(map[string]*peer, domain.MaxParticipants)
		h.rooms[bookingID] = room
	}

	// A reconnecting person replaces their own earlier connection rather
	// than counting as a second occupant: a refreshed tab must not lock
	// someone out of their own session.
	for peerID, existing := range room {
		if existing.participant.UserID == participant.UserID {
			delete(room, peerID)
			close(existing.send)
		}
	}
	if len(room) >= domain.MaxParticipants {
		return nil, nil, domain.ErrRoomFull
	}

	joined := &peer{participant: participant, send: make(chan Envelope, sendBuffer)}
	room[participant.PeerID] = joined

	others := make([]domain.Participant, 0, len(room)-1)
	for peerID, other := range room {
		if peerID == participant.PeerID {
			continue
		}
		others = append(others, other.participant)
		// Tell the existing peer someone arrived, so it can start
		// negotiating without polling.
		deliver(other, Envelope{
			Type: domain.TypePeerJoin,
			From: participant.PeerID,
			Role: participant.Role,
		})
	}
	return joined.send, others, nil
}

// Leave removes a peer and tells whoever is left.
func (h *Hub) Leave(bookingID, peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room := h.rooms[bookingID]
	if room == nil {
		return
	}
	leaving, ok := room[peerID]
	if !ok {
		return
	}
	delete(room, peerID)
	close(leaving.send)

	for _, other := range room {
		deliver(other, Envelope{
			Type: domain.TypePeerLeave,
			From: peerID,
			Role: leaving.participant.Role,
		})
	}
	if len(room) == 0 {
		delete(h.rooms, bookingID)
	}
}

// Relay passes a message to the other participant in the room.
//
// The sender is stamped from the connection, never from the message, and
// only negotiation types are relayed — a peer cannot forge a room event or
// use the socket as a chat channel.
func (h *Hub) Relay(bookingID string, from domain.Participant, message Envelope) error {
	if !message.Type.Relayable() {
		return domain.ErrInvalidMessage
	}
	message.From = from.PeerID
	message.Role = from.Role
	message.Reason = ""

	h.mu.Lock()
	defer h.mu.Unlock()

	room := h.rooms[bookingID]
	for peerID, other := range room {
		if peerID == from.PeerID {
			continue
		}
		deliver(other, message)
	}
	return nil
}

// Occupancy reports how many peers are in a room. It exists for tests and
// for a health view; nothing in the protocol depends on it.
func (h *Hub) Occupancy(bookingID string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[bookingID])
}

// deliver queues a message for a peer, dropping it if that peer's buffer
// is full. A blocked send here would stall the whole hub behind one slow
// connection; the reader's own write loop notices the close and hangs up.
func deliver(to *peer, message Envelope) {
	select {
	case to.send <- message:
	default:
	}
}
