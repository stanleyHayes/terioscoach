package wsapi

import (
	"encoding/json"
	"errors"
	"testing"

	domain "github.com/xcreativs/terios/api/internal/domain/signaling"
)

func client(peerID string) domain.Participant {
	return domain.Participant{UserID: "client-1", Role: domain.RoleClient, PeerID: peerID}
}

func practitioner(peerID string) domain.Participant {
	return domain.Participant{UserID: "prac-1", Role: domain.RolePractitioner, PeerID: peerID}
}

// drain reads whatever is queued for a peer without blocking.
func drain(ch <-chan Envelope) []Envelope {
	var out []Envelope
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, msg)
		default:
			return out
		}
	}
}

func admit(t *testing.T, hub *Hub, bookingID string, from domain.Participant, recipient <-chan Envelope) {
	t.Helper()
	if err := hub.Relay(bookingID, from, Envelope{Type: domain.TypeAdmissionGrant}); err != nil {
		t.Fatalf("admit room: %v", err)
	}
	drain(recipient)
}

// TestJoinAnnouncesEachArrival: the peer already in the room is told
// someone arrived, so it can start negotiating without polling.
func TestJoinAnnouncesEachArrival(t *testing.T) {
	hub := NewHub()

	first, others, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	if len(others) != 0 {
		t.Errorf("others = %+v, want an empty room", others)
	}

	_, others, err = hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	if len(others) != 1 || others[0].PeerID != "peer-1" {
		t.Errorf("others = %+v, want the first peer", others)
	}

	announcements := drain(first)
	if len(announcements) != 1 {
		t.Fatalf("announcements = %+v, want one peer-joined", announcements)
	}
	if announcements[0].Type != domain.TypePeerJoin || announcements[0].From != "peer-2" {
		t.Errorf("announcement = %+v, want peer-joined from peer-2", announcements[0])
	}
	if announcements[0].Role != domain.RolePractitioner {
		t.Errorf("role = %q, want the arriving peer's role", announcements[0].Role)
	}
}

// TestRoomHoldsTwo: these are one-to-one consultations, so a third
// connection is refused rather than quietly making it a group call.
func TestRoomHoldsTwo(t *testing.T) {
	hub := NewHub()

	if _, _, err := hub.Join("booking-1", client("peer-1")); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, _, err := hub.Join("booking-1", practitioner("peer-2")); err != nil {
		t.Fatalf("second join: %v", err)
	}

	third := domain.Participant{UserID: "someone-else", Role: domain.RoleClient, PeerID: "peer-3"}
	if _, _, err := hub.Join("booking-1", third); !errors.Is(err, domain.ErrRoomFull) {
		t.Fatalf("third join = %v, want ErrRoomFull", err)
	}
	if got := hub.Occupancy("booking-1"); got != 2 {
		t.Errorf("occupancy = %d, want 2", got)
	}
}

// TestReconnectionReplacesYourOwnConnection: a refreshed tab must not lock
// someone out of their own session.
func TestReconnectionReplacesYourOwnConnection(t *testing.T) {
	hub := NewHub()

	stale, _, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, _, err := hub.Join("booking-1", practitioner("peer-2")); err != nil {
		t.Fatalf("practitioner join: %v", err)
	}

	// The client comes back on a new connection while the room is full.
	fresh, others, err := hub.Join("booking-1", client("peer-3"))
	if err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if got := hub.Occupancy("booking-1"); got != 2 {
		t.Errorf("occupancy = %d, want the stale connection replaced", got)
	}
	if len(others) != 1 || others[0].PeerID != "peer-2" {
		t.Errorf("others = %+v, want the practitioner", others)
	}

	// The stale channel is closed, which is how its write loop learns to
	// hang up. Anything already queued on it drains first, so the check is
	// that the channel ends rather than that it is empty.
	for range stale { //nolint:revive // draining to the close
	}
	admit(t, hub, "booking-1", practitioner("peer-2"), fresh)

	if err := hub.Relay("booking-1", practitioner("peer-2"), Envelope{Type: domain.TypeOffer}); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if len(drain(fresh)) == 0 {
		t.Error("the reconnected peer received nothing")
	}
}

// TestRelayReachesOnlyTheOtherPeer.
func TestRelayReachesOnlyTheOtherPeer(t *testing.T) {
	hub := NewHub()
	first, _, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("first join: %v", err)
	}
	second, _, err := hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	drain(first) // the peer-joined announcement
	admit(t, hub, "booking-1", practitioner("peer-2"), first)

	payload := json.RawMessage(`{"sdp":"v=0"}`)
	if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: domain.TypeOffer, Payload: payload}); err != nil {
		t.Fatalf("relay: %v", err)
	}

	if got := drain(first); len(got) != 0 {
		t.Errorf("sender received its own message: %+v", got)
	}
	received := drain(second)
	if len(received) != 1 {
		t.Fatalf("received = %+v, want the offer", received)
	}
	if received[0].Type != domain.TypeOffer || string(received[0].Payload) != string(payload) {
		t.Errorf("message = %+v, want the offer relayed unchanged", received[0])
	}
}

// TestSenderIsStampedByTheServer: a peer cannot claim to be the other one.
func TestSenderIsStampedByTheServer(t *testing.T) {
	hub := NewHub()
	if _, _, err := hub.Join("booking-1", client("peer-1")); err != nil {
		t.Fatalf("first join: %v", err)
	}
	second, _, err := hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("second join: %v", err)
	}
	drain(second)
	admit(t, hub, "booking-1", practitioner("peer-2"), firstChannel(hub, "booking-1", "peer-1"))

	forged := Envelope{
		Type:   domain.TypeOffer,
		From:   "peer-2", // claiming to be the recipient
		Role:   domain.RolePractitioner,
		Reason: "spoofed",
	}
	if err := hub.Relay("booking-1", client("peer-1"), forged); err != nil {
		t.Fatalf("relay: %v", err)
	}

	received := drain(second)
	if len(received) != 1 {
		t.Fatalf("received = %+v, want one message", received)
	}
	if received[0].From != "peer-1" {
		t.Errorf("from = %q, want the real sender", received[0].From)
	}
	if received[0].Role != domain.RoleClient {
		t.Errorf("role = %q, want the real sender's role", received[0].Role)
	}
	if received[0].Reason != "" {
		t.Errorf("reason = %q, want the server-only field cleared", received[0].Reason)
	}
}

func firstChannel(hub *Hub, bookingID, peerID string) <-chan Envelope {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	return hub.rooms[bookingID][peerID].send
}

func TestClinicalRoomControlsAreRoleRestricted(t *testing.T) {
	hub := NewHub()
	clientMessages, _, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("client join: %v", err)
	}
	practitionerMessages, _, err := hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("practitioner join: %v", err)
	}
	drain(clientMessages)
	drain(practitionerMessages)

	for _, messageType := range []domain.MessageType{
		domain.TypeAdmissionGrant, domain.TypeAdmissionDeny, domain.TypeSessionEnd,
	} {
		if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: messageType}); !errors.Is(err, domain.ErrInvalidMessage) {
			t.Errorf("client relay %q = %v, want ErrInvalidMessage", messageType, err)
		}
	}
	if err := hub.Relay("booking-1", practitioner("peer-2"), Envelope{Type: domain.TypeAdmissionRequest}); !errors.Is(err, domain.ErrInvalidMessage) {
		t.Errorf("practitioner admission request = %v, want ErrInvalidMessage", err)
	}

	for _, allowed := range []struct {
		from  domain.Participant
		type_ domain.MessageType
	}{
		{client("peer-1"), domain.TypeAdmissionRequest},
		{practitioner("peer-2"), domain.TypeAdmissionGrant},
		{client("peer-1"), domain.TypeRecordingRequest},
		{practitioner("peer-2"), domain.TypeRecordingConsent},
	} {
		if err := hub.Relay("booking-1", allowed.from, Envelope{Type: allowed.type_}); err != nil {
			t.Errorf("allowed relay %q = %v", allowed.type_, err)
		}
	}
}

func TestNegotiationRequiresPractitionerAdmission(t *testing.T) {
	hub := NewHub()
	clientMessages, _, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("client join: %v", err)
	}
	practitionerMessages, _, err := hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("practitioner join: %v", err)
	}
	drain(clientMessages)
	drain(practitionerMessages)

	if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: domain.TypeOffer}); !errors.Is(err, domain.ErrInvalidMessage) {
		t.Fatalf("offer before admission = %v, want ErrInvalidMessage", err)
	}
	admit(t, hub, "booking-1", practitioner("peer-2"), clientMessages)
	if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: domain.TypeOffer}); err != nil {
		t.Fatalf("offer after admission: %v", err)
	}
}

// TestRoomEventsCannotBeRelayed: a participant that could forge peer-left
// would make the other side think the call had ended.
func TestRoomEventsCannotBeRelayed(t *testing.T) {
	hub := NewHub()
	if _, _, err := hub.Join("booking-1", client("peer-1")); err != nil {
		t.Fatalf("join: %v", err)
	}
	second, _, err := hub.Join("booking-1", practitioner("peer-2"))
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	drain(second)

	for _, forged := range []domain.MessageType{
		domain.TypePeerLeave, domain.TypePeerJoin, domain.TypeJoined, domain.TypeError, "file",
	} {
		if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: forged}); !errors.Is(err, domain.ErrInvalidMessage) {
			t.Errorf("relay %q = %v, want ErrInvalidMessage", forged, err)
		}
	}
	if got := drain(second); len(got) != 0 {
		t.Errorf("a forged room event was delivered: %+v", got)
	}
}

// TestRoomsAreIsolated: a message in one session never reaches another.
func TestRoomsAreIsolated(t *testing.T) {
	hub := NewHub()
	if _, _, err := hub.Join("booking-1", client("peer-1")); err != nil {
		t.Fatalf("join: %v", err)
	}
	otherRoom, _, err := hub.Join("booking-2", client("peer-9"))
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: domain.TypeReaction}); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if got := drain(otherRoom); len(got) != 0 {
		t.Errorf("another session received %+v, want nothing", got)
	}
}

// TestLeaveTellsWhoeverIsLeft.
func TestLeaveTellsWhoeverIsLeft(t *testing.T) {
	hub := NewHub()
	first, _, err := hub.Join("booking-1", client("peer-1"))
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, _, err := hub.Join("booking-1", practitioner("peer-2")); err != nil {
		t.Fatalf("join: %v", err)
	}
	admit(t, hub, "booking-1", practitioner("peer-2"), firstChannel(hub, "booking-1", "peer-1"))
	drain(first)

	hub.Leave("booking-1", "peer-2")

	received := drain(first)
	if len(received) != 1 || received[0].Type != domain.TypePeerLeave || received[0].From != "peer-2" {
		t.Errorf("received = %+v, want peer-left from peer-2", received)
	}
	if got := hub.Occupancy("booking-1"); got != 1 {
		t.Errorf("occupancy = %d, want 1", got)
	}

	hub.Leave("booking-1", "peer-1")
	if got := hub.Occupancy("booking-1"); got != 0 {
		t.Errorf("occupancy = %d, want the empty room dropped", got)
	}
	// Leaving twice is harmless.
	hub.Leave("booking-1", "peer-1")
}

// TestSlowPeerDoesNotBlockTheHub: a stalled connection must not hold up
// the other side of the call.
func TestSlowPeerDoesNotBlockTheHub(t *testing.T) {
	hub := NewHub()
	if _, _, err := hub.Join("booking-1", client("peer-1")); err != nil {
		t.Fatalf("join: %v", err)
	}
	if _, _, err := hub.Join("booking-1", practitioner("peer-2")); err != nil {
		t.Fatalf("join: %v", err)
	}
	admit(t, hub, "booking-1", practitioner("peer-2"), firstChannel(hub, "booking-1", "peer-1"))

	// Nobody is reading peer-2's channel. Far more than its buffer is
	// sent; every call must still return.
	for i := 0; i < sendBuffer*4; i++ {
		if err := hub.Relay("booking-1", client("peer-1"), Envelope{Type: domain.TypeCandidate}); err != nil {
			t.Fatalf("relay %d: %v", i, err)
		}
	}
}
