package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/ports"
)

// SignalingUpgrader serves the WebSocket half of the video slice. It is an
// interface so this package keeps no dependency on the WebSocket library:
// the wsapi adapter satisfies it.
type SignalingUpgrader interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request, bookingID string)
}

// WithSessions mounts the video-session routes (CX-01/CX-02).
//
//	POST /v1/sessions/{bookingId}/join  — authenticated; authorizes and
//	                                      mints a connection ticket
//	GET  /v1/sessions/{bookingId}/signal?ticket=… — the socket
//
// Authorization happens on the POST, where a proper status code can
// explain why (too early, cancelled, not yours). By the time the socket is
// opened the decision is already made and carried by a single-use ticket.
// A nil service keeps the routes mounted but answering 503.
func WithSessions(svc ports.SignalingService, upgrader SignalingUpgrader, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/sessions", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleSessionsUnavailable)
				r.HandleFunc("/", handleSessionsUnavailable)
				return
			}
			h := &sessionHandler{svc: svc, upgrader: upgrader}
			r.With(RequireAuth(auth)).Post("/{bookingId}/join", h.join)
			// The socket carries its own credential: a ticket the POST
			// above issued. It must not sit behind RequireAuth, because a
			// browser cannot put a bearer token on a WebSocket handshake.
			r.Get("/{bookingId}/signal", h.signal)
		})
	}
}

// handleSessionsUnavailable answers every session route when signaling is
// not configured.
func handleSessionsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "video sessions are unavailable: database not connected")
}

type sessionHandler struct {
	svc      ports.SignalingService
	upgrader SignalingUpgrader
}

// join handles POST /v1/sessions/{bookingId}/join.
func (h *sessionHandler) join(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	access, err := h.svc.Authorize(r.Context(), id, chi.URLParam(r, "bookingId"))
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"bookingId":       access.Room.BookingID,
		"role":            string(access.Role),
		"ticket":          access.Ticket,
		"ticketExpiresIn": int(access.TicketTTL.Seconds()),
		"opensAt":         access.Room.OpensAt.UTC().Format(time.RFC3339),
		"closesAt":        access.Room.ClosesAt.UTC().Format(time.RFC3339),
		"iceServers":      access.ICEServers,
	})
}

// signal handles GET /v1/sessions/{bookingId}/signal — the upgrade.
func (h *sessionHandler) signal(w http.ResponseWriter, r *http.Request) {
	if h.upgrader == nil {
		handleSessionsUnavailable(w, r)
		return
	}
	h.upgrader.ServeHTTP(w, r, chi.URLParam(r, "bookingId"))
}
