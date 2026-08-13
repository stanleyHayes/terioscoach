package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/enquiry"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// DefaultEnquiryRateLimit caps contact-form submissions per client address.
// Five an hour is far beyond anyone with something to say and well below
// what a spam script wants. The form is the only unauthenticated write in
// the API, so it is the one that needs a cap of its own.
const (
	DefaultEnquiryRateLimit  = 5
	DefaultEnquiryRateWindow = time.Hour
)

// EnquiryOption customizes the enquiry route group.
type EnquiryOption func(*enquiryRoutes)

type enquiryRoutes struct {
	rateLimit RateLimitPolicy
}

// WithEnquiryRateLimit overrides the per-IP cap on the public form.
func WithEnquiryRateLimit(policy RateLimitPolicy) EnquiryOption {
	return func(e *enquiryRoutes) { e.rateLimit = policy }
}

// WithEnquiries mounts the enquiry routes backed by the enquiry port
// (BE-13). POST /v1/enquiries is public and rate-limited; the inbox under
// /v1/admin/enquiries is practitioner-only. A nil service keeps the routes
// mounted but answering 503.
func WithEnquiries(svc ports.EnquiryService, auth ports.AuthService, opts ...EnquiryOption) Option {
	cfg := enquiryRoutes{rateLimit: RateLimitPolicy{
		Limit:  DefaultEnquiryRateLimit,
		Window: DefaultEnquiryRateWindow,
	}}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(s *Server) {
		s.Router.Route("/v1/enquiries", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleEnquiriesUnavailable)
				r.HandleFunc("/", handleEnquiriesUnavailable)
				return
			}
			h := &enquiryHandler{svc: svc}
			r.With(RateLimit(cfg.rateLimit)).Post("/", h.submit)
		})

		s.Router.Route("/v1/admin/enquiries", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleEnquiriesUnavailable)
				r.HandleFunc("/", handleEnquiriesUnavailable)
				return
			}
			h := &enquiryHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/", h.list)
			r.Get("/unread-count", h.unreadCount)
			r.Get("/{id}", h.get)
			r.Patch("/{id}", h.patch)
			r.Delete("/{id}", h.delete)
		})
	}
}

// handleEnquiriesUnavailable answers every enquiry route when the database
// is not configured.
func handleEnquiriesUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "enquiries are unavailable: database not connected")
}

type enquiryHandler struct {
	svc ports.EnquiryService
}

// enquiryBody is the practice-side shape. sourceIp is deliberately absent:
// it exists for abuse triage in the database, not for the inbox UI.
type enquiryBody struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	Subject   string    `json:"subject,omitempty"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newEnquiryBody(e enquiry.Enquiry) enquiryBody {
	return enquiryBody{
		ID:        e.ID,
		Name:      e.Name,
		Email:     e.Email,
		Phone:     e.Phone,
		Subject:   e.Subject,
		Message:   e.Message,
		Status:    string(e.Status),
		CreatedAt: e.CreatedAt.UTC(),
		UpdatedAt: e.UpdatedAt.UTC(),
	}
}

// submit handles POST /v1/enquiries — the public contact form.
//
// The response deliberately carries only an acknowledgement, not the stored
// enquiry: the sender already knows what they typed, and echoing an id back
// to an anonymous caller hands them a handle they have no route to use.
func (h *enquiryHandler) submit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Email   string `json:"email"`
		Phone   string `json:"phone"`
		Subject string `json:"subject"`
		Message string `json:"message"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if _, err := h.svc.Submit(r.Context(), ports.EnquiryInput{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Subject:  req.Subject,
		Message:  req.Message,
		SourceIP: clientKey(r),
	}); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bool{"received": true})
}

// list handles GET /v1/admin/enquiries.
func (h *enquiryHandler) list(w http.ResponseWriter, r *http.Request) {
	status := enquiry.Status(r.URL.Query().Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "validation_error", "status must be new, read, replied, or archived")
		return
	}
	items, err := h.svc.List(r.Context(), ports.EnquiryFilter{Status: status})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]enquiryBody, 0, len(items))
	for _, e := range items {
		out = append(out, newEnquiryBody(e))
	}
	writeJSON(w, http.StatusOK, map[string][]enquiryBody{"items": out})
}

// unreadCount handles GET /v1/admin/enquiries/unread-count.
func (h *enquiryHandler) unreadCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.svc.UnreadCount(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

// get handles GET /v1/admin/enquiries/{id}.
func (h *enquiryHandler) get(w http.ResponseWriter, r *http.Request) {
	e, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]enquiryBody{"enquiry": newEnquiryBody(e)})
}

// patch handles PATCH /v1/admin/enquiries/{id} — triage only.
func (h *enquiryHandler) patch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	e, err := h.svc.SetStatus(r.Context(), chi.URLParam(r, "id"), enquiry.Status(req.Status))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]enquiryBody{"enquiry": newEnquiryBody(e)})
}

// delete handles DELETE /v1/admin/enquiries/{id}.
func (h *enquiryHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
