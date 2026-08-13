package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xcreativs/terios/api/internal/adapters/memory"
	"github.com/xcreativs/terios/api/internal/adapters/wsapi"
	"github.com/xcreativs/terios/api/internal/app/auth"
	signalingapp "github.com/xcreativs/terios/api/internal/app/signaling"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	domainsignaling "github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// sessionTestRig bundles a live HTTP server (a real one, because the
// socket needs a real connection) with the video routes mounted.
type sessionTestRig struct {
	srv               *Server
	http              *httptest.Server
	bookings          *portstest.FakeBookingRepository
	practitionerToken string
	clientToken       string
	otherClientToken  string
}

func newSessionTestRig(t *testing.T) sessionTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	bookings := portstest.NewFakeBookingRepository()
	svc := signalingapp.NewService(bookings, memory.NewTicketStore(), signalingapp.Options{
		ICE: nil,
	})
	// Origin checking is exercised separately; the test client sends no
	// Origin header, which the library treats as same-origin.
	upgrader := wsapi.NewHandler(svc, wsapi.NewHub(), nil, nil)

	srv := NewServer(WithAuth(authSvc), WithSessions(svc, upgrader, authSvc))
	httpSrv := httptest.NewServer(srv.Router)
	t.Cleanup(httpSrv.Close)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return sessionTestRig{
		srv:               srv,
		http:              httpSrv,
		bookings:          bookings,
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
		otherClientToken:  issue("client-2", identity.RoleClient),
	}
}

// seedSession stores a booking starting `in` from now.
func seedSession(t *testing.T, rig sessionTestRig, in time.Duration, status domainbooking.Status) string {
	t.Helper()
	start := time.Now().UTC().Add(in)
	b, err := domainbooking.New("client-1", "prac-1", "svc-1", start, 60, time.Now().UTC().Add(-48*time.Hour))
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b.Status = status
	b, err = rig.bookings.Create(t.Context(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b.ID
}

type joinResponse struct {
	BookingID       string `json:"bookingId"`
	Role            string `json:"role"`
	Ticket          string `json:"ticket"`
	TicketExpiresIn int    `json:"ticketExpiresIn"`
	OpensAt         string `json:"opensAt"`
	ClosesAt        string `json:"closesAt"`
}

func join(t *testing.T, rig sessionTestRig, bookingID, token string) (int, joinResponse) {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil, bearer(token))
	var res joinResponse
	if rec.Code == http.StatusOK {
		decodeBody(t, rec, &res)
	}
	return rec.Code, res
}

// dial opens the signaling socket with a ticket.
func dial(t *testing.T, rig sessionTestRig, bookingID, ticket string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(rig.http.URL, "http") +
		"/v1/sessions/" + bookingID + "/signal?ticket=" + ticket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, url, nil)
}

// readEnvelope reads one frame with a timeout.
func readEnvelope(t *testing.T, conn *websocket.Conn) wsapi.Envelope {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var envelope wsapi.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	return envelope
}

func writeEnvelope(t *testing.T, conn *websocket.Conn, envelope wsapi.Envelope) {
	t.Helper()
	raw, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestJoinAuthorizesInsideTheWindow.
func TestJoinAuthorizesInsideTheWindow(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)

	code, res := join(t, rig, bookingID, rig.clientToken)
	if code != http.StatusOK {
		t.Fatalf("join status = %d, want 200", code)
	}
	if res.Ticket == "" || res.TicketExpiresIn <= 0 {
		t.Errorf("response = %+v, want a ticket with an expiry", res)
	}
	if res.Role != "client" {
		t.Errorf("role = %q, want client", res.Role)
	}
	if res.OpensAt == "" || res.ClosesAt == "" {
		t.Errorf("response = %+v, want the window stated so the UI can count down", res)
	}

	code, practitionerRes := join(t, rig, bookingID, rig.practitionerToken)
	if code != http.StatusOK {
		t.Fatalf("practitioner join status = %d, want 200", code)
	}
	if practitionerRes.Role != "practitioner" {
		t.Errorf("role = %q, want practitioner", practitionerRes.Role)
	}
	if practitionerRes.Ticket == res.Ticket {
		t.Error("both parties were given the same ticket")
	}
}

// TestJoinWindowIsEnforced: too early and too late are distinct, explained
// failures rather than a socket that will not open.
func TestJoinWindowIsEnforced(t *testing.T) {
	rig := newSessionTestRig(t)

	early := seedSession(t, rig, 2*time.Hour, domainbooking.StatusConfirmed)
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+early+"/join", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("early join = %d, want 403 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "room_not_open" {
		t.Errorf("code = %q, want room_not_open", errRes.Error.Code)
	}

	late := seedSession(t, rig, -3*time.Hour, domainbooking.StatusConfirmed)
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+late+"/join", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("late join = %d, want 403", rec.Code)
	}
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "room_closed" {
		t.Errorf("code = %q, want room_closed", errRes.Error.Code)
	}

	cancelled := seedSession(t, rig, 5*time.Minute, domainbooking.StatusCancelled)
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+cancelled+"/join", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Errorf("cancelled join = %d, want 409", rec.Code)
	}
}

// TestOutsidersCannotJoin: a stranger gets not-found, so they cannot probe
// for whose sessions exist.
func TestOutsidersCannotJoin(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("stranger join = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "booking_not_found" {
		t.Errorf("code = %q, want booking_not_found", errRes.Error.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/sessions/"+bookingID+"/join", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("anonymous join = %d, want 401", rec.Code)
	}
}

// TestSocketRelaysBetweenBothParties is the end-to-end path: two real
// sockets, one offer, one answer.
func TestSocketRelaysBetweenBothParties(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)

	_, clientAccess := join(t, rig, bookingID, rig.clientToken)
	_, practitionerAccess := join(t, rig, bookingID, rig.practitionerToken)

	clientConn, _, err := dial(t, rig, bookingID, clientAccess.Ticket)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer func() { _ = clientConn.CloseNow() }()

	first := readEnvelope(t, clientConn)
	if first.Type != domainsignaling.TypeJoined || first.From == "" {
		t.Fatalf("first frame = %+v, want a joined frame with this peer's id", first)
	}

	practitionerConn, _, err := dial(t, rig, bookingID, practitionerAccess.Ticket)
	if err != nil {
		t.Fatalf("practitioner dial: %v", err)
	}
	defer func() { _ = practitionerConn.CloseNow() }()

	joined := readEnvelope(t, practitionerConn)
	if joined.Type != domainsignaling.TypeJoined {
		t.Fatalf("practitioner first frame = %+v, want joined", joined)
	}
	if !strings.Contains(string(joined.Payload), "peers") {
		t.Errorf("joined payload = %s, want the peer already present listed", joined.Payload)
	}

	// The client is told the practitioner arrived.
	announcement := readEnvelope(t, clientConn)
	if announcement.Type != domainsignaling.TypePeerJoin {
		t.Fatalf("announcement = %+v, want peer-joined", announcement)
	}

	// Offer travels client → practitioner.
	writeEnvelope(t, clientConn, wsapi.Envelope{
		Type:    domainsignaling.TypeOffer,
		Payload: json.RawMessage(`{"sdp":"v=0 offer"}`),
	})
	offer := readEnvelope(t, practitionerConn)
	if offer.Type != domainsignaling.TypeOffer || !strings.Contains(string(offer.Payload), "v=0 offer") {
		t.Fatalf("offer = %+v, want it relayed unchanged", offer)
	}
	if offer.Role != domainsignaling.RoleClient {
		t.Errorf("role = %q, want the sender's real role", offer.Role)
	}

	// Answer travels back.
	writeEnvelope(t, practitionerConn, wsapi.Envelope{
		Type:    domainsignaling.TypeAnswer,
		Payload: json.RawMessage(`{"sdp":"v=0 answer"}`),
	})
	answer := readEnvelope(t, clientConn)
	if answer.Type != domainsignaling.TypeAnswer || !strings.Contains(string(answer.Payload), "v=0 answer") {
		t.Fatalf("answer = %+v, want it relayed unchanged", answer)
	}
}

// TestTicketIsSingleUse: a captured ticket is worthless once the real
// holder has connected.
func TestTicketIsSingleUse(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)
	_, access := join(t, rig, bookingID, rig.clientToken)

	conn, _, err := dial(t, rig, bookingID, access.Ticket)
	if err != nil {
		t.Fatalf("first dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	readEnvelope(t, conn)

	_, res, err := dial(t, rig, bookingID, access.Ticket)
	if err == nil {
		t.Fatal("a spent ticket opened a second socket")
	}
	if res != nil && res.StatusCode != http.StatusUnauthorized {
		t.Errorf("replay status = %d, want 401", res.StatusCode)
	}
}

// TestSocketRefusesBadTickets.
func TestSocketRefusesBadTickets(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)
	otherBooking := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)
	_, access := join(t, rig, bookingID, rig.clientToken)

	for name, tc := range map[string]struct{ booking, ticket string }{
		"no ticket":       {bookingID, ""},
		"unknown ticket":  {bookingID, "not-a-real-ticket"},
		"other session":   {otherBooking, access.Ticket},
		"unknown session": {"no-such-booking", access.Ticket},
	} {
		t.Run(name, func(t *testing.T) {
			conn, res, err := dial(t, rig, tc.booking, tc.ticket)
			if err == nil {
				_ = conn.CloseNow()
				t.Fatal("the socket opened, want it refused")
			}
			if res != nil && res.StatusCode == http.StatusSwitchingProtocols {
				t.Errorf("status = %d, want the upgrade refused", res.StatusCode)
			}
		})
	}
}

// TestSocketRefusesUnknownMessageTypes: the socket is for negotiating a
// call, not a general channel between two accounts.
func TestSocketRefusesUnknownMessageTypes(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)
	_, access := join(t, rig, bookingID, rig.clientToken)

	conn, _, err := dial(t, rig, bookingID, access.Ticket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.CloseNow() }()
	readEnvelope(t, conn)

	writeEnvelope(t, conn, wsapi.Envelope{Type: "file", Payload: json.RawMessage(`"hello"`)})
	refusal := readEnvelope(t, conn)
	if refusal.Type != domainsignaling.TypeError {
		t.Fatalf("response = %+v, want an error frame", refusal)
	}
	if refusal.Reason == "" {
		t.Error("the refusal carries no explanation")
	}

	// The connection survives a refused message — one bad frame is not
	// grounds for dropping a call.
	writeEnvelope(t, conn, wsapi.Envelope{Type: domainsignaling.TypePing})
	if pong := readEnvelope(t, conn); pong.Type != domainsignaling.TypePong {
		t.Errorf("response = %+v, want a pong", pong)
	}
}

// TestPeerLeaveIsAnnounced.
func TestPeerLeaveIsAnnounced(t *testing.T) {
	rig := newSessionTestRig(t)
	bookingID := seedSession(t, rig, 5*time.Minute, domainbooking.StatusConfirmed)
	_, clientAccess := join(t, rig, bookingID, rig.clientToken)
	_, practitionerAccess := join(t, rig, bookingID, rig.practitionerToken)

	clientConn, _, err := dial(t, rig, bookingID, clientAccess.Ticket)
	if err != nil {
		t.Fatalf("client dial: %v", err)
	}
	defer func() { _ = clientConn.CloseNow() }()
	readEnvelope(t, clientConn)

	practitionerConn, _, err := dial(t, rig, bookingID, practitionerAccess.Ticket)
	if err != nil {
		t.Fatalf("practitioner dial: %v", err)
	}
	readEnvelope(t, clientConn) // peer-joined

	_ = practitionerConn.Close(websocket.StatusNormalClosure, "done")

	left := readEnvelope(t, clientConn)
	if left.Type != domainsignaling.TypePeerLeave {
		t.Errorf("frame = %+v, want peer-left", left)
	}
}

// TestSessionsUnavailableWithoutDatabase.
func TestSessionsUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithSessions(nil, nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/v1/sessions/abc/join"},
		{http.MethodGet, "/v1/sessions/abc/signal"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
