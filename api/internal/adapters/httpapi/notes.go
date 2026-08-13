package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithNotes mounts the /v1/bookings/{id}/notes routes backed by the note
// port. GET accepts the booking owner or the practitioner (the app layer
// enforces ownership and the sharing visibility rule); PUT and share are
// practitioner-only. A nil service keeps the routes mounted but answering
// 503. Routes register as full paths on the mux so they coexist with the
// booking slice's own /v1/bookings group.
func WithNotes(svc ports.NoteService, auth ports.AuthService) Option {
	return func(s *Server) {
		if svc == nil {
			s.Router.HandleFunc("/v1/bookings/{id}/notes", handleNotesUnavailable)
			s.Router.HandleFunc("/v1/bookings/{id}/notes/share", handleNotesUnavailable)
			return
		}
		h := &noteHandler{svc: svc}
		practitioner := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RolePractitioner)}
		s.Router.With(RequireAuth(auth)).Get("/v1/bookings/{id}/notes", h.get)
		s.Router.With(practitioner...).Put("/v1/bookings/{id}/notes", h.upsert)
		s.Router.With(practitioner...).Post("/v1/bookings/{id}/notes/share", h.share)
	}
}

// handleNotesUnavailable answers every session-notes route when the
// database — and therefore the note service — is not configured.
func handleNotesUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "session notes are unavailable: database not connected")
}

type noteHandler struct {
	svc ports.NoteService
}

// noteBody is the practitioner note shape — everything, including the
// private notes.
type noteBody struct {
	ID              string     `json:"id"`
	BookingID       string     `json:"bookingId"`
	ClientID        string     `json:"clientId"`
	PractitionerID  string     `json:"practitionerId"`
	PrivateNotes    string     `json:"privateNotes"`
	SharedFeedback  string     `json:"sharedFeedback"`
	SharedResources []string   `json:"sharedResources"`
	SharedAt        *time.Time `json:"sharedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func newNoteBody(n note.SessionNote) noteBody {
	return noteBody{
		ID:              n.ID,
		BookingID:       n.BookingID,
		ClientID:        n.ClientID,
		PractitionerID:  n.PractitionerID,
		PrivateNotes:    n.PrivateNotes,
		SharedFeedback:  n.SharedFeedback,
		SharedResources: n.SharedResources,
		SharedAt:        n.SharedAt,
		CreatedAt:       n.CreatedAt.UTC(),
		UpdatedAt:       n.UpdatedAt.UTC(),
	}
}

// sharedNoteBody is the client note shape — shared content only. There is
// deliberately no privateNotes field to populate.
type sharedNoteBody struct {
	BookingID       string    `json:"bookingId"`
	SharedFeedback  string    `json:"sharedFeedback"`
	SharedResources []string  `json:"sharedResources"`
	SharedAt        time.Time `json:"sharedAt"`
}

// get handles GET /v1/bookings/{id}/notes for the owner or the
// practitioner. The app layer has already filtered by role: the response
// serializes whichever view came back.
func (h *noteHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	view, err := h.svc.GetNotes(r.Context(), id, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	if view.Full != nil {
		writeJSON(w, http.StatusOK, map[string]noteBody{"note": newNoteBody(*view.Full)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]sharedNoteBody{"note": sharedNoteBody{
		BookingID:       view.Shared.BookingID,
		SharedFeedback:  view.Shared.Feedback,
		SharedResources: view.Shared.Resources,
		SharedAt:        view.Shared.SharedAt.UTC(),
	}})
}

// upsert handles PUT /v1/bookings/{id}/notes — create or replace the
// note's content, practitioner-only.
func (h *noteHandler) upsert(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		PrivateNotes    string   `json:"privateNotes"`
		SharedFeedback  string   `json:"sharedFeedback"`
		SharedResources []string `json:"sharedResources"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	n, err := h.svc.UpsertNotes(r.Context(), id.UserID, chi.URLParam(r, "id"), ports.NoteContent{
		PrivateNotes:    req.PrivateNotes,
		SharedFeedback:  req.SharedFeedback,
		SharedResources: req.SharedResources,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]noteBody{"note": newNoteBody(n)})
}

// share handles POST /v1/bookings/{id}/notes/share — practitioner-only,
// one-way and idempotent.
func (h *noteHandler) share(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ShareNotes(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]noteBody{"note": newNoteBody(res.Note)})
}
