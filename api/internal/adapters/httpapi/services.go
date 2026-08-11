package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithCatalog mounts the /v1/services routes backed by the catalog port.
// Listing is public; mutations require the practitioner role. A nil
// service keeps the routes mounted but answering 503, matching the auth
// slice's no-database behavior.
func WithCatalog(svc ports.CatalogService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/services", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleCatalogUnavailable)
				return
			}
			h := &catalogHandler{svc: svc}
			practitioner := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RolePractitioner)}
			r.Get("/", h.listActive)
			r.With(practitioner...).Get("/all", h.listAll)
			r.With(practitioner...).Post("/", h.create)
			r.With(practitioner...).Patch("/{id}", h.update)
			r.With(practitioner...).Delete("/{id}", h.delete)
		})
	}
}

// handleCatalogUnavailable answers every services route when the database
// — and therefore the catalog service — is not configured.
func handleCatalogUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "services are unavailable: database not connected")
}

type catalogHandler struct {
	svc ports.CatalogService
}

// serviceBody is the contract service shape.
type serviceBody struct {
	ID              string    `json:"id"`
	PractitionerID  string    `json:"practitionerId"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DurationMinutes int       `json:"durationMinutes"`
	PriceKobo       int64     `json:"priceKobo"`
	Currency        string    `json:"currency"`
	Active          bool      `json:"active"`
	SortOrder       int       `json:"sortOrder"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func newServiceBody(s catalog.Service) serviceBody {
	return serviceBody{
		ID:              s.ID,
		PractitionerID:  s.PractitionerID,
		Name:            s.Name,
		Description:     s.Description,
		DurationMinutes: s.DurationMinutes,
		PriceKobo:       s.PriceKobo,
		Currency:        s.Currency,
		Active:          s.Active,
		SortOrder:       s.SortOrder,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func newServiceList(services []catalog.Service) map[string][]serviceBody {
	items := make([]serviceBody, 0, len(services))
	for _, s := range services {
		items = append(items, newServiceBody(s))
	}
	return map[string][]serviceBody{"items": items}
}

// listActive handles GET /v1/services — the public catalog.
func (h *catalogHandler) listActive(w http.ResponseWriter, r *http.Request) {
	services, err := h.svc.ListActive(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newServiceList(services))
}

// listAll handles GET /v1/services/all for the practitioner, including
// inactive services.
func (h *catalogHandler) listAll(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	services, err := h.svc.ListAll(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newServiceList(services))
}

// create handles POST /v1/services.
func (h *catalogHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		DurationMinutes int    `json:"durationMinutes"`
		PriceKobo       int64  `json:"priceKobo"`
		Currency        string `json:"currency"`
		SortOrder       int    `json:"sortOrder"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	svc, err := h.svc.CreateService(r.Context(), id.UserID, ports.ServiceInput{
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		PriceKobo:       req.PriceKobo,
		Currency:        req.Currency,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]serviceBody{"service": newServiceBody(svc)})
}

// update handles PATCH /v1/services/{id} with partial-update semantics:
// only supplied fields change — including activate/deactivate and reorder.
func (h *catalogHandler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		Name            *string `json:"name"`
		Description     *string `json:"description"`
		DurationMinutes *int    `json:"durationMinutes"`
		PriceKobo       *int64  `json:"priceKobo"`
		Currency        *string `json:"currency"`
		Active          *bool   `json:"active"`
		SortOrder       *int    `json:"sortOrder"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	svc, err := h.svc.UpdateService(r.Context(), id.UserID, chi.URLParam(r, "id"), ports.ServicePatch{
		Name:            req.Name,
		Description:     req.Description,
		DurationMinutes: req.DurationMinutes,
		PriceKobo:       req.PriceKobo,
		Currency:        req.Currency,
		Active:          req.Active,
		SortOrder:       req.SortOrder,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]serviceBody{"service": newServiceBody(svc)})
}

// delete handles DELETE /v1/services/{id}. Hard vs soft delete is decided
// by the use case (bookings exist or not); both answer 204.
func (h *catalogHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if err := h.svc.DeleteService(r.Context(), id.UserID, chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
