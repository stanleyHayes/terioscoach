package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/memory"
	signalingapp "github.com/xcreativs/terios/api/internal/app/signaling"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

const seedTestToken = "test-seed-token"

// seedTestRig bundles the seed route's dependencies: a user store holding
// one client and the practitioner, the booking store the route writes, and
// a catalog with one active service to borrow.
type seedTestRig struct {
	users        *portstest.FakeUserRepository
	bookings     *portstest.FakeBookingRepository
	services     *portstest.FakeServiceRepository
	client       identity.User
	practitioner identity.User
}

func newSeedTestRig(t *testing.T) seedTestRig {
	t.Helper()
	users := portstest.NewFakeUserRepository()
	client, err := users.Create(t.Context(), identity.User{
		Email: "jane@example.com", Name: "Jane", Role: identity.RoleClient,
	})
	if err != nil {
		t.Fatalf("seed client: %v", err)
	}
	practitioner, err := users.Create(t.Context(), identity.User{
		Email: "prac@example.com", Name: "Prac", Role: identity.RolePractitioner,
	})
	if err != nil {
		t.Fatalf("seed practitioner: %v", err)
	}

	bookings := portstest.NewFakeBookingRepository()
	services := portstest.NewFakeServiceRepository()
	if _, err := services.Create(t.Context(), catalog.Service{
		PractitionerID:  practitioner.ID,
		Name:            "Massage",
		DurationMinutes: 45,
		Active:          true,
	}); err != nil {
		t.Fatalf("seed service: %v", err)
	}

	return seedTestRig{
		users:        users,
		bookings:     bookings,
		services:     services,
		client:       client,
		practitioner: practitioner,
	}
}

// server mounts the seed route over the rig's stores, enabled.
func (rig seedTestRig) server() *Server {
	return NewServer(WithTestingSeed(seedTestToken, false, rig.users, rig.bookings, rig.services))
}

func postSeed(t *testing.T, srv *Server, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var headers map[string]string
	if token != "" {
		headers = bearer(token)
	}
	return doJSON(t, srv, http.MethodPost, "/v1/testing/sessions", body, headers)
}

// TestSeedRouteAbsentWithoutToken: with no TESTING_SEED_TOKEN the routes
// are not mounted at all — an absent route, not a refused one.
func TestSeedRouteAbsentWithoutToken(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := NewServer(WithTestingSeed("", false, rig.users, rig.bookings, rig.services))

	rec := postSeed(t, srv, map[string]any{"clientEmail": "jane@example.com", "startingIn": 0}, seedTestToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}

// TestSeedRouteAbsentInProduction: even with a token set, production never
// mounts the seed routes.
func TestSeedRouteAbsentInProduction(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := NewServer(WithTestingSeed(seedTestToken, true, rig.users, rig.bookings, rig.services))

	rec := postSeed(t, srv, map[string]any{"clientEmail": "jane@example.com", "startingIn": 0}, seedTestToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}

// TestSeedRequiresTheToken: a missing or wrong bearer token answers 401.
func TestSeedRequiresTheToken(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := rig.server()
	body := map[string]any{"clientEmail": "jane@example.com", "startingIn": 0}

	rec := postSeed(t, srv, body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rec.Code)
	}

	rec = postSeed(t, srv, body, "wrong-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", rec.Code)
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "unauthorized" {
		t.Errorf("code = %q, want unauthorized", errRes.Error.Code)
	}
}

// TestSeedCreatesJoinableSession is the contract the e2e suite depends on:
// the returned bookingId belongs to the named client and the practitioner,
// and passes the signaling join check right now.
func TestSeedCreatesJoinableSession(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := rig.server()

	rec := postSeed(t, srv, map[string]any{"clientEmail": "jane@example.com", "startingIn": 0}, seedTestToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var res struct {
		BookingID string `json:"bookingId"`
	}
	decodeBody(t, rec, &res)
	if res.BookingID == "" {
		t.Fatal("response carries no bookingId")
	}

	b, err := rig.bookings.FindByID(t.Context(), res.BookingID)
	if err != nil {
		t.Fatalf("booking not retrievable: %v", err)
	}
	if b.Status != domainbooking.StatusConfirmed {
		t.Errorf("status = %q, want confirmed", b.Status)
	}
	if b.ClientID != rig.client.ID || b.PractitionerID != rig.practitioner.ID {
		t.Errorf("parties = client %q / practitioner %q, want %q / %q",
			b.ClientID, b.PractitionerID, rig.client.ID, rig.practitioner.ID)
	}
	// The duration is borrowed from the catalog's active service.
	if got := b.EndAt.Sub(b.StartAt); got != 45*time.Minute {
		t.Errorf("duration = %v, want 45m (the active service's)", got)
	}

	// The join check itself: both parties may enter the room now.
	signaling := signalingapp.NewService(rig.bookings, memory.NewTicketStore(), signalingapp.Options{})
	for _, id := range []identity.Identity{
		{UserID: rig.client.ID, Role: identity.RoleClient},
		{UserID: rig.practitioner.ID, Role: identity.RolePractitioner},
	} {
		if _, err := signaling.Authorize(t.Context(), id, res.BookingID); err != nil {
			t.Errorf("Authorize(%s) = %v, want the room open", id.Role, err)
		}
	}
}

// TestSeedHonorsStartingIn: a future session is created at the requested
// offset. (Joinability then is the room window's business, not the seed's.)
func TestSeedHonorsStartingIn(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := rig.server()
	before := time.Now().UTC()

	rec := postSeed(t, srv, map[string]any{"clientEmail": "jane@example.com", "startingIn": 300}, seedTestToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var res struct {
		BookingID string `json:"bookingId"`
	}
	decodeBody(t, rec, &res)

	b, err := rig.bookings.FindByID(t.Context(), res.BookingID)
	if err != nil {
		t.Fatalf("booking not retrievable: %v", err)
	}
	if b.StartAt.Before(before.Add(300*time.Second)) || b.StartAt.After(time.Now().UTC().Add(300*time.Second)) {
		t.Errorf("startAt = %v, want ~5m from the request", b.StartAt)
	}
}

// TestSeedUnknownEmail answers 404 in the standard error envelope.
func TestSeedUnknownEmail(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := rig.server()

	rec := postSeed(t, srv, map[string]any{"clientEmail": "nobody@example.com", "startingIn": 0}, seedTestToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "client_not_found" {
		t.Errorf("code = %q, want client_not_found", errRes.Error.Code)
	}
}

// TestSeedValidatesInput.
func TestSeedValidatesInput(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := rig.server()

	for name, body := range map[string]any{
		"missing email":  map[string]any{"startingIn": 0},
		"negative start": map[string]any{"clientEmail": "jane@example.com", "startingIn": -5},
	} {
		t.Run(name, func(t *testing.T) {
			rec := postSeed(t, srv, body, seedTestToken)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestSeedWithoutACatalog: a deployment with no active service still gets
// a working session — a labelled placeholder id and the 60-minute default.
func TestSeedWithoutACatalog(t *testing.T) {
	rig := newSeedTestRig(t)
	srv := NewServer(WithTestingSeed(seedTestToken, false, rig.users, rig.bookings, nil))

	rec := postSeed(t, srv, map[string]any{"clientEmail": "jane@example.com", "startingIn": 0}, seedTestToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	var res struct {
		BookingID string `json:"bookingId"`
	}
	decodeBody(t, rec, &res)

	b, err := rig.bookings.FindByID(t.Context(), res.BookingID)
	if err != nil {
		t.Fatalf("booking not retrievable: %v", err)
	}
	if b.ServiceID != seedSessionServiceID {
		t.Errorf("serviceId = %q, want the placeholder %q", b.ServiceID, seedSessionServiceID)
	}
	if got := b.EndAt.Sub(b.StartAt); got != seedSessionDurationMinutes*time.Minute {
		t.Errorf("duration = %v, want the default %dm", got, seedSessionDurationMinutes)
	}
}
