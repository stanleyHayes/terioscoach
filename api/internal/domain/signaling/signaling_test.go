package signaling

import (
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
)

var sessionStart = time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

func session(status booking.Status) booking.Booking {
	return booking.Booking{
		ID:             "booking-1",
		ClientID:       "client-1",
		PractitionerID: "prac-1",
		ServiceID:      "svc-1",
		StartAt:        sessionStart,
		EndAt:          sessionStart.Add(time.Hour),
		Status:         status,
	}
}

var (
	owner        = identity.Identity{UserID: "client-1", Role: identity.RoleClient}
	practitioner = identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}
	stranger     = identity.Identity{UserID: "client-2", Role: identity.RoleClient}
	otherPrac    = identity.Identity{UserID: "prac-2", Role: identity.RolePractitioner}
)

// TestBothPartiesMayJoin.
func TestBothPartiesMayJoin(t *testing.T) {
	b := session(booking.StatusConfirmed)
	policy := DefaultPolicy()
	during := sessionStart.Add(10 * time.Minute)

	for name, id := range map[string]identity.Identity{"client": owner, "practitioner": practitioner} {
		room, err := Authorize(b, id, policy, during)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if room.BookingID != "booking-1" {
			t.Errorf("%s room = %+v, want the booking's room", name, room)
		}
		if !room.OpensAt.Equal(sessionStart.Add(-DefaultOpenBefore)) {
			t.Errorf("%s opensAt = %v, want the lead time before the session", name, room.OpensAt)
		}
		if !room.ClosesAt.Equal(sessionStart.Add(time.Hour + DefaultCloseAfter)) {
			t.Errorf("%s closesAt = %v, want the grace after the session", name, room.ClosesAt)
		}
	}
}

// TestOutsidersGetNotFound: a stranger must not learn whose sessions exist.
func TestOutsidersGetNotFound(t *testing.T) {
	b := session(booking.StatusConfirmed)
	during := sessionStart.Add(10 * time.Minute)

	for name, id := range map[string]identity.Identity{
		"another client":       stranger,
		"another practitioner": otherPrac,
		// A party to the booking presenting the wrong role is equally not
		// a participant: role comes from the booking, not the request.
		"client id with practitioner role": {UserID: "client-1", Role: identity.RolePractitioner},
		"practitioner id with client role": {UserID: "prac-1", Role: identity.RoleClient},
	} {
		if _, err := Authorize(b, id, DefaultPolicy(), during); !errors.Is(err, booking.ErrBookingNotFound) {
			t.Errorf("%s err = %v, want ErrBookingNotFound", name, err)
		}
	}
}

// TestJoinWindowIsEnforcedAtBothEnds.
func TestJoinWindowIsEnforcedAtBothEnds(t *testing.T) {
	b := session(booking.StatusConfirmed)
	policy := DefaultPolicy()
	opensAt, closesAt := policy.Window(b)

	if _, err := Authorize(b, owner, policy, opensAt.Add(-time.Second)); !errors.Is(err, ErrRoomNotOpenYet) {
		t.Errorf("a second early err = %v, want ErrRoomNotOpenYet", err)
	}
	if _, err := Authorize(b, owner, policy, opensAt); err != nil {
		t.Errorf("at opening err = %v, want the room open", err)
	}
	if _, err := Authorize(b, owner, policy, closesAt.Add(-time.Second)); err != nil {
		t.Errorf("a second before closing err = %v, want the room still open", err)
	}
	if _, err := Authorize(b, owner, policy, closesAt); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("at closing err = %v, want ErrRoomClosed", err)
	}

	if policy.Open(b, opensAt.Add(-time.Second)) || !policy.Open(b, opensAt) || policy.Open(b, closesAt) {
		t.Error("Open disagrees with Authorize about the window")
	}
}

// TestOnlyConfirmedSessionsHaveARoom: there is nothing to join once a
// session is cancelled or closed off.
func TestOnlyConfirmedSessionsHaveARoom(t *testing.T) {
	during := sessionStart.Add(10 * time.Minute)
	for _, status := range []booking.Status{
		booking.StatusCancelled, booking.StatusCompleted, booking.StatusNoShow,
	} {
		if _, err := Authorize(session(status), owner, DefaultPolicy(), during); !errors.Is(err, ErrSessionNotActive) {
			t.Errorf("%s err = %v, want ErrSessionNotActive", status, err)
		}
	}
}

// TestZeroPolicyFallsBackToDefaults: a misconfigured policy must not leave
// a room permanently shut or permanently open.
func TestZeroPolicyFallsBackToDefaults(t *testing.T) {
	b := session(booking.StatusConfirmed)
	var zero JoinPolicy

	opensAt, closesAt := zero.Window(b)
	if !opensAt.Equal(sessionStart.Add(-DefaultOpenBefore)) {
		t.Errorf("opensAt = %v, want the default lead", opensAt)
	}
	if !closesAt.Equal(sessionStart.Add(time.Hour + DefaultCloseAfter)) {
		t.Errorf("closesAt = %v, want the default grace", closesAt)
	}
}

// TestRoleComesFromTheBooking.
func TestRoleComesFromTheBooking(t *testing.T) {
	b := session(booking.StatusConfirmed)

	if role, ok := RoleFor(b, owner); !ok || role != RoleClient {
		t.Errorf("owner role = %q (%v), want client", role, ok)
	}
	if role, ok := RoleFor(b, practitioner); !ok || role != RolePractitioner {
		t.Errorf("practitioner role = %q (%v), want practitioner", role, ok)
	}
	if _, ok := RoleFor(b, stranger); ok {
		t.Error("a stranger was given a role in the call")
	}
}

// TestOnlySessionTrafficIsRelayable is what stops the socket becoming a
// general-purpose message bus, and stops a participant forging room events.
func TestOnlySessionTrafficIsRelayable(t *testing.T) {
	for _, relayable := range []MessageType{
		TypeOffer, TypeAnswer, TypeCandidate,
		TypeChat, TypeState, TypeReaction, TypeCaption,
		TypeAdmissionRequest, TypeAdmissionGrant, TypeAdmissionDeny,
		TypeRecordingRequest, TypeRecordingConsent, TypeSessionEnd,
	} {
		if !relayable.Relayable() || !relayable.Valid() {
			t.Errorf("%q is not relayable, want it passed between peers", relayable)
		}
	}
	for _, serverOnly := range []MessageType{TypeJoined, TypePeerJoin, TypePeerLeave, TypeError} {
		if serverOnly.Relayable() {
			t.Errorf("%q is relayable — a participant could forge a room event", serverOnly)
		}
		if serverOnly.Valid() {
			t.Errorf("%q is accepted from a client, want it server-only", serverOnly)
		}
	}
	for _, liveness := range []MessageType{TypePing, TypePong} {
		if liveness.Relayable() {
			t.Errorf("%q is relayed to the peer, want it handled by the server", liveness)
		}
		if !liveness.Valid() {
			t.Errorf("%q is refused, want liveness accepted", liveness)
		}
	}
	if MessageType("file").Valid() || MessageType("").Valid() {
		t.Error("an unknown message type was accepted")
	}
}
