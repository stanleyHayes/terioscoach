package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/recording"
	"github.com/xcreativs/terios/api/internal/ports"
)

func WithRecordings(svc ports.RecordingService, auth ports.AuthService) Option {
	return func(s *Server) {
		if svc == nil {
			s.Router.HandleFunc("/v1/bookings/{id}/recordings", handleRecordingsUnavailable)
			return
		}
		h := &recordingHandler{svc: svc}
		s.Router.With(RequireAuth(auth)).Get("/v1/bookings/{id}/recordings", h.list)
		s.Router.With(RequireAuth(auth), RequireRole(identity.RolePractitioner)).
			Post("/v1/bookings/{id}/recordings/sign-upload", h.signUpload)
		s.Router.With(RequireAuth(auth), RequireRole(identity.RolePractitioner)).
			Post("/v1/bookings/{id}/recordings", h.create)
	}
}

func handleRecordingsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "session recordings are unavailable: database not connected")
}

type recordingHandler struct {
	svc ports.RecordingService
}

type recordingBody struct {
	ID              string    `json:"id"`
	BookingID       string    `json:"bookingId"`
	ClientID        string    `json:"clientId"`
	PractitionerID  string    `json:"practitionerId"`
	ContentType     string    `json:"contentType"`
	URL             string    `json:"url"`
	Bytes           int64     `json:"bytes"`
	DurationSec     int       `json:"durationSec"`
	StorageLocation string    `json:"storageLocation"`
	RetainUntil     time.Time `json:"retainUntil"`
	CreatedAt       time.Time `json:"createdAt"`
}

func newRecordingBody(item ports.PlayableRecording) recordingBody {
	rec := item.Recording
	return recordingBody{
		ID:              rec.ID,
		BookingID:       rec.BookingID,
		ClientID:        rec.ClientID,
		PractitionerID:  rec.PractitionerID,
		ContentType:     rec.ContentType,
		URL:             item.URL,
		Bytes:           rec.Bytes,
		DurationSec:     rec.DurationSec,
		StorageLocation: recording.StorageLocation,
		RetainUntil:     rec.RetainUntil.UTC(),
		CreatedAt:       rec.CreatedAt.UTC(),
	}
}

func (h *recordingHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	recordings, err := h.svc.ListForBooking(r.Context(), id, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]recordingBody, 0, len(recordings))
	for _, rec := range recordings {
		items = append(items, newRecordingBody(rec))
	}
	writeJSON(w, http.StatusOK, map[string][]recordingBody{"items": items})
}

func (h *recordingHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		ContentType string `json:"contentType"`
		PublicID    string `json:"publicId"`
		Bytes       int64  `json:"bytes"`
		DurationSec int    `json:"durationSec"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	rec, err := h.svc.CreateRecording(r.Context(), id.UserID, chi.URLParam(r, "id"), ports.RecordingUpload{
		ContentType: req.ContentType,
		PublicID:    req.PublicID,
		Bytes:       req.Bytes,
		DurationSec: req.DurationSec,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	// No delivery URL on the create response: the practitioner who just
	// uploaded it has the file, and the list endpoint mints one when it is
	// actually needed.
	writeJSON(w, http.StatusCreated, map[string]recordingBody{
		"recording": newRecordingBody(ports.PlayableRecording{Recording: rec}),
	})
}

// signUpload authorizes one direct upload of a recording, so the bytes go
// to the media store rather than through this process.
func (h *recordingHandler) signUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		ContentType string `json:"contentType"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	signed, err := h.svc.SignUpload(r.Context(), id.UserID, chi.URLParam(r, "id"), req.ContentType)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":       signed.URL,
		"fields":    signed.Fields,
		"expiresAt": signed.ExpiresAt.UTC(),
	})
}
