package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/review"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithReviews mounts the review routes backed by the review port (BE-14).
//
// Three surfaces, split by who may see what: /v1/content/reviews is public
// and approved-only, /v1/reviews is the client's own submissions, and
// /v1/admin/reviews is the practitioner's moderation queue. A nil service
// keeps the routes mounted but answering 503.
func WithReviews(svc ports.ReviewService, auth ports.AuthService) Option {
	return func(s *Server) {
		if svc == nil {
			s.Router.HandleFunc("/v1/content/reviews", handleReviewsUnavailable)
			s.Router.HandleFunc("/v1/content/reviews/summary", handleReviewsUnavailable)
			s.Router.Route("/v1/reviews", func(r chi.Router) {
				r.HandleFunc("/*", handleReviewsUnavailable)
				r.HandleFunc("/", handleReviewsUnavailable)
			})
			s.Router.Route("/v1/admin/reviews", func(r chi.Router) {
				r.HandleFunc("/*", handleReviewsUnavailable)
				r.HandleFunc("/", handleReviewsUnavailable)
			})
			return
		}
		h := &reviewHandler{svc: svc}

		// Public. Registered as full paths so they coexist with the CMS
		// slice's own /v1/content group.
		s.Router.Get("/v1/content/reviews", h.publicList)
		s.Router.Get("/v1/content/reviews/summary", h.publicSummary)

		s.Router.Route("/v1/reviews", func(r chi.Router) {
			r.Use(RequireAuth(auth), RequireRole(identity.RoleClient))
			r.Post("/", h.submit)
			r.Get("/mine", h.listMine)
			r.Patch("/{id}", h.updateMine)
		})

		s.Router.Route("/v1/admin/reviews", func(r chi.Router) {
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/", h.list)
			r.Post("/{id}/approve", h.approve)
			r.Post("/{id}/reject", h.reject)
		})
	}
}

// handleReviewsUnavailable answers every review route when the database is
// not configured.
func handleReviewsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "reviews are unavailable: database not connected")
}

type reviewHandler struct {
	svc ports.ReviewService
}

// reviewBody is the authenticated shape — the client's own review, or the
// practitioner's queue row.
type reviewBody struct {
	ID          string     `json:"id"`
	BookingID   string     `json:"bookingId"`
	ClientID    string     `json:"clientId"`
	ServiceID   string     `json:"serviceId,omitempty"`
	Rating      int        `json:"rating"`
	Comment     string     `json:"comment,omitempty"`
	Status      string     `json:"status"`
	ModeratedAt *time.Time `json:"moderatedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func newReviewBody(r review.Review) reviewBody {
	return reviewBody{
		ID:          r.ID,
		BookingID:   r.BookingID,
		ClientID:    r.ClientID,
		ServiceID:   r.ServiceID,
		Rating:      r.Rating,
		Comment:     r.Comment,
		Status:      string(r.Status),
		ModeratedAt: r.ModeratedAt,
		CreatedAt:   r.CreatedAt.UTC(),
		UpdatedAt:   r.UpdatedAt.UTC(),
	}
}

// publicReviewBody is what a stranger sees: the verdict and a first name.
// No client id, no booking id, no address — a review is someone talking
// about their own health care, and the practice publishes the minimum.
type publicReviewBody struct {
	ID          string `json:"id"`
	AuthorName  string `json:"authorName"`
	ServiceName string `json:"serviceName,omitempty"`
	Rating      int    `json:"rating"`
	Comment     string `json:"comment,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

// submit handles POST /v1/reviews.
func (h *reviewHandler) submit(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		BookingID string `json:"bookingId"`
		Rating    int    `json:"rating"`
		Comment   string `json:"comment"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	rev, err := h.svc.Submit(r.Context(), id.UserID, ports.ReviewInput{
		BookingID: req.BookingID,
		Rating:    req.Rating,
		Comment:   req.Comment,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]reviewBody{"review": newReviewBody(rev)})
}

// listMine handles GET /v1/reviews/mine.
func (h *reviewHandler) listMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListMine(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]reviewBody, 0, len(items))
	for _, rev := range items {
		out = append(out, newReviewBody(rev))
	}
	writeJSON(w, http.StatusOK, map[string][]reviewBody{"items": out})
}

// updateMine handles PATCH /v1/reviews/{id} — pending reviews only.
func (h *reviewHandler) updateMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		Rating  *int    `json:"rating"`
		Comment *string `json:"comment"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	rev, err := h.svc.UpdateMine(r.Context(), id.UserID, chi.URLParam(r, "id"), review.Patch{
		Rating:  req.Rating,
		Comment: req.Comment,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]reviewBody{"review": newReviewBody(rev)})
}

// list handles GET /v1/admin/reviews — the moderation queue.
func (h *reviewHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	status := review.Status(r.URL.Query().Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "validation_error", "status must be pending, approved, or rejected")
		return
	}
	items, err := h.svc.ListForPractitioner(r.Context(), id.UserID, ports.ReviewFilter{Status: status})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]reviewBody, 0, len(items))
	for _, rev := range items {
		out = append(out, newReviewBody(rev))
	}
	writeJSON(w, http.StatusOK, map[string][]reviewBody{"items": out})
}

func (h *reviewHandler) approve(w http.ResponseWriter, r *http.Request) {
	h.moderate(w, r, true)
}

func (h *reviewHandler) reject(w http.ResponseWriter, r *http.Request) {
	h.moderate(w, r, false)
}

func (h *reviewHandler) moderate(w http.ResponseWriter, r *http.Request, approve bool) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rev, err := h.svc.Moderate(r.Context(), id.UserID, chi.URLParam(r, "id"), approve)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]reviewBody{"review": newReviewBody(rev)})
}

// publicList handles GET /v1/content/reviews — approved reviews only.
func (h *reviewHandler) publicList(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "validation_error", "limit must be a positive whole number")
			return
		}
		limit = parsed
	}
	items, err := h.svc.PublicReviews(r.Context(), limit)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]publicReviewBody, 0, len(items))
	for _, item := range items {
		out = append(out, publicReviewBody{
			ID:          item.ID,
			AuthorName:  item.AuthorName,
			ServiceName: item.ServiceName,
			Rating:      item.Rating,
			Comment:     item.Comment,
			CreatedAt:   item.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]publicReviewBody{"items": out})
}

// publicSummary handles GET /v1/content/reviews/summary.
func (h *reviewHandler) publicSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.svc.PublicSummary(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	distribution := make(map[string]int, review.MaxRating)
	for star := review.MinRating; star <= review.MaxRating; star++ {
		distribution[strconv.Itoa(star)] = summary.Distribution[star]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":        summary.Count,
		"average":      summary.Average,
		"distribution": distribution,
	})
}
