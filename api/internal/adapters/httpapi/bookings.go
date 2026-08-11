package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultBookingTimezone matches the availability contract default.
const defaultBookingTimezone = "Africa/Accra"

// WithBooking mounts the /v1/bookings routes backed by the booking port.
// Creating and listing "mine" is client-only; the calendar list and
// complete/no-show are practitioner-only; get/reschedule/cancel accept both
// roles and the app layer enforces ownership. A nil service keeps the
// routes mounted but answering 503.
func WithBooking(svc ports.BookingService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/bookings", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleBookingUnavailable)
				return
			}
			h := &bookingHandler{svc: svc}
			client := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RoleClient)}
			practitioner := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RolePractitioner)}
			both := []func(http.Handler) http.Handler{RequireAuth(auth)}
			r.With(client...).Post("/", h.create)
			r.With(client...).Get("/mine", h.listMine)
			r.With(practitioner...).Get("/", h.listForPractitioner)
			r.With(both...).Get("/{id}", h.get)
			r.With(both...).Post("/{id}/reschedule", h.reschedule)
			r.With(both...).Post("/{id}/cancel", h.cancel)
			r.With(practitioner...).Post("/{id}/complete", h.complete)
			r.With(practitioner...).Post("/{id}/no-show", h.markNoShow)
		})
	}
}

// handleBookingUnavailable answers every bookings route when the database
// — and therefore the booking service — is not configured.
func handleBookingUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "bookings are unavailable: database not connected")
}

type bookingHandler struct {
	svc ports.BookingService
}

// bookingBody is the contract booking shape.
type bookingBody struct {
	ID             string     `json:"id"`
	ClientID       string     `json:"clientId"`
	PractitionerID string     `json:"practitionerId"`
	ServiceID      string     `json:"serviceId"`
	StartAt        time.Time  `json:"startAt"`
	EndAt          time.Time  `json:"endAt"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	CancelledAt    *time.Time `json:"cancelledAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	PaymentStatus  string     `json:"paymentStatus,omitempty"`
	PaidAt         *time.Time `json:"paidAt,omitempty"`
}

func newBookingBody(b booking.Booking) bookingBody {
	return bookingBody{
		ID:             b.ID,
		ClientID:       b.ClientID,
		PractitionerID: b.PractitionerID,
		ServiceID:      b.ServiceID,
		StartAt:        b.StartAt.UTC(),
		EndAt:          b.EndAt.UTC(),
		Status:         string(b.Status),
		CreatedAt:      b.CreatedAt.UTC(),
		UpdatedAt:      b.UpdatedAt.UTC(),
		CancelledAt:    b.CancelledAt,
		CompletedAt:    b.CompletedAt,
		PaymentStatus:  string(b.PaymentStatus),
		PaidAt:         b.PaidAt,
	}
}

func newBookingList(bookings []booking.Booking) map[string][]bookingBody {
	items := make([]bookingBody, 0, len(bookings))
	for _, b := range bookings {
		items = append(items, newBookingBody(b))
	}
	return map[string][]bookingBody{"items": items}
}

// identityOr401 pulls the principal RequireAuth attached.
func identityOr401(w http.ResponseWriter, r *http.Request) (identity.Identity, bool) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return identity.Identity{}, false
	}
	return id, true
}

// create handles POST /v1/bookings — a client claims a slot.
func (h *bookingHandler) create(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		ServiceID string    `json:"serviceId"`
		StartAt   time.Time `json:"startAt"`
		TZ        string    `json:"tz"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ServiceID == "" || req.StartAt.IsZero() {
		writeError(w, http.StatusBadRequest, "validation_error", "serviceId and startAt are required")
		return
	}
	tz := req.TZ
	if tz == "" {
		tz = defaultBookingTimezone
	}
	b, err := h.svc.CreateBooking(r.Context(), id.UserID, req.ServiceID, req.StartAt, tz)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]bookingBody{"booking": newBookingBody(b)})
}

// listMine handles GET /v1/bookings/mine — the client's own history.
func (h *bookingHandler) listMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	bookings, err := h.svc.ListMine(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newBookingList(bookings))
}

// listForPractitioner handles GET /v1/bookings with optional
// ?from=&to=&status= filters.
func (h *bookingHandler) listForPractitioner(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var filter ports.BookingFilter
	if raw := q.Get("from"); raw != "" {
		from, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "from must be an RFC 3339 timestamp")
			return
		}
		filter.From = &from
	}
	if raw := q.Get("to"); raw != "" {
		to, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "to must be an RFC 3339 timestamp")
			return
		}
		filter.To = &to
	}
	if raw := q.Get("status"); raw != "" {
		status := booking.Status(raw)
		if !status.Valid() {
			writeError(w, http.StatusBadRequest, "validation_error", "status must be one of confirmed, cancelled, completed, no_show")
			return
		}
		filter.Status = status
	}
	bookings, err := h.svc.ListForPractitioner(r.Context(), id.UserID, filter)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newBookingList(bookings))
}

// get handles GET /v1/bookings/{id} for the owner or the practitioner.
func (h *bookingHandler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	b, err := h.svc.GetBooking(r.Context(), id, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bookingBody{"booking": newBookingBody(b)})
}

// reschedule handles POST /v1/bookings/{id}/reschedule.
func (h *bookingHandler) reschedule(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		StartAt time.Time `json:"startAt"`
		TZ      string    `json:"tz"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StartAt.IsZero() {
		writeError(w, http.StatusBadRequest, "validation_error", "startAt is required")
		return
	}
	tz := req.TZ
	if tz == "" {
		tz = defaultBookingTimezone
	}
	b, err := h.svc.RescheduleBooking(r.Context(), id, chi.URLParam(r, "id"), req.StartAt, tz)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bookingBody{"booking": newBookingBody(b)})
}

// cancel handles POST /v1/bookings/{id}/cancel.
func (h *bookingHandler) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	b, err := h.svc.CancelBooking(r.Context(), id, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bookingBody{"booking": newBookingBody(b)})
}

// complete handles POST /v1/bookings/{id}/complete — practitioner-only.
func (h *bookingHandler) complete(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	b, err := h.svc.CompleteBooking(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bookingBody{"booking": newBookingBody(b)})
}

// markNoShow handles POST /v1/bookings/{id}/no-show — practitioner-only.
func (h *bookingHandler) markNoShow(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	b, err := h.svc.MarkNoShow(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bookingBody{"booking": newBookingBody(b)})
}
