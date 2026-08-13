package httpapi

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// seedSessionDurationMinutes is the session length a seeded booking gets
// when the catalog has no active service to borrow one from.
const seedSessionDurationMinutes = 60

// seedSessionServiceID labels a seeded booking whose catalog has no active
// service. The booking is real in every respect the video room checks; the
// service reference is display data only.
const seedSessionServiceID = "testing-seed-session"

// WithTestingSeed mounts the test-only seed routes used by the e2e suite:
//
//	POST /v1/testing/sessions — create a confirmed booking that is
//	                            immediately joinable in the video room
//
// The routes exist only when token is non-empty AND the server is not in
// production — both conditions are enforced here, at the mount, so a
// miswired composition root cannot expose them. When either fails this
// option is a no-op and the paths answer 404 like any unknown route.
// TESTING_SEED_TOKEN must never be set in production.
func WithTestingSeed(token string, production bool, users ports.UserRepository, bookings ports.BookingRepository, services ports.ServiceRepository) Option {
	return func(s *Server) {
		if token == "" || production || users == nil || bookings == nil {
			return
		}
		h := &testingSeedHandler{token: token, users: users, bookings: bookings, services: services}
		s.Router.Route("/v1/testing", func(r chi.Router) {
			r.Post("/sessions", h.createSession)
		})
	}
}

type testingSeedHandler struct {
	token    string
	users    ports.UserRepository
	bookings ports.BookingRepository
	services ports.ServiceRepository
}

// createSessionRequest is the POST /v1/testing/sessions body. StartingIn is
// seconds until the session starts; 0 means now.
type createSessionRequest struct {
	ClientEmail string `json:"clientEmail"`
	StartingIn  int    `json:"startingIn"`
}

// createSession handles POST /v1/testing/sessions. The bearer token is the
// shared TESTING_SEED_TOKEN, compared in constant time — this route is not
// behind RequireAuth because the e2e suite holds no user session for it.
func (h *testingSeedHandler) createSession(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r)
	if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(h.token)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid seed token")
		return
	}

	var req createSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ClientEmail == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "clientEmail is required")
		return
	}
	if req.StartingIn < 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "startingIn must not be negative")
		return
	}

	client, err := h.users.FindByEmail(r.Context(), identity.NormalizeEmail(req.ClientEmail))
	if err != nil {
		writeError(w, http.StatusNotFound, "client_not_found", "no account with that email")
		return
	}
	// The platform has a single practitioner; the seeded booking belongs to
	// them so both e2e browsers — client and practitioner — can join.
	practitioner, err := h.users.FindFirstByRole(r.Context(), identity.RolePractitioner)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "no_practitioner", "no practitioner account exists to host the session")
		return
	}

	serviceID, durationMinutes := h.seedService(r)
	now := time.Now().UTC()
	startAt := now.Add(time.Duration(req.StartingIn) * time.Second)

	b, err := domainbooking.New(client.ID, practitioner.ID, serviceID, startAt, durationMinutes, now)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	b, err = h.bookings.Create(r.Context(), b)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"bookingId": b.ID})
}

// seedService borrows the first active catalog service for its id and
// duration, so a seeded booking looks like a real one. A deployment with
// no catalog falls back to a labelled placeholder — the video room checks
// the booking's parties, status, and window, never the service.
func (h *testingSeedHandler) seedService(r *http.Request) (string, int) {
	if h.services != nil {
		if services, err := h.services.ListByPractitioner(r.Context(), "", true); err == nil && len(services) > 0 {
			return services[0].ID, services[0].DurationMinutes
		}
	}
	return seedSessionServiceID, seedSessionDurationMinutes
}
