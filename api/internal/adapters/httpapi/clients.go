package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	domainclient "github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithClients mounts the /v1/clients routes backed by the client port. The
// list, record, and practice-field PATCH are practitioner-only; "me" is
// client-only. A nil service keeps the routes mounted but answering 503.
func WithClients(svc ports.ClientService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/clients", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleClientsUnavailable)
				return
			}
			h := &clientHandler{svc: svc}
			client := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RoleClient)}
			practitioner := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RolePractitioner)}
			r.With(practitioner...).Get("/", h.list)
			r.With(client...).Get("/me", h.me)
			r.With(practitioner...).Get("/{id}", h.record)
			r.With(practitioner...).Patch("/{id}", h.patch)
		})
	}
}

// handleClientsUnavailable answers every clients route when the database —
// and therefore the client service — is not configured.
func handleClientsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "client records are unavailable: database not connected")
}

type clientHandler struct {
	svc ports.ClientService
}

// clientSummaryBody is the contract list-row shape.
type clientSummaryBody struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Phone         string     `json:"phone,omitempty"`
	Tags          []string   `json:"tags"`
	TotalSessions int        `json:"totalSessions"`
	LastSessionAt *time.Time `json:"lastSessionAt,omitempty"`
}

// clientMeBody is the client's own view — practice-side fields are absent
// by construction.
type clientMeBody struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// paymentSummaryBody is the contract payment rollup inside the record.
type paymentSummaryBody struct {
	TotalPaidKobo     int64  `json:"totalPaidKobo"`
	TotalRefundedKobo int64  `json:"totalRefundedKobo"`
	PaymentCount      int    `json:"paymentCount"`
	Currency          string `json:"currency,omitempty"`
}

// clientRecordBody is the contract full-record shape.
type clientRecordBody struct {
	ID                  string             `json:"id"`
	Name                string             `json:"name"`
	Email               string             `json:"email"`
	Phone               string             `json:"phone,omitempty"`
	Tags                []string           `json:"tags"`
	PracticeNotes       string             `json:"practiceNotes"`
	ProfileCreatedAt    *time.Time         `json:"profileCreatedAt,omitempty"`
	ProfileUpdatedAt    *time.Time         `json:"profileUpdatedAt,omitempty"`
	RecentBookings      []bookingBody      `json:"recentBookings"`
	Payments            paymentSummaryBody `json:"payments"`
	DocumentCount       int                `json:"documentCount"`
	FormSubmissionCount int                `json:"formSubmissionCount"`
}

// clientProfileBody is the PATCH response shape — the practice profile.
type clientProfileBody struct {
	ID            string    `json:"id"`
	Phone         string    `json:"phone,omitempty"`
	Tags          []string  `json:"tags"`
	PracticeNotes string    `json:"practiceNotes"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func newClientSummaryBody(s domainclient.Summary) clientSummaryBody {
	return clientSummaryBody{
		ID:            s.Profile.UserID,
		Name:          s.Name,
		Email:         s.Email,
		Phone:         s.Profile.Phone,
		Tags:          s.Profile.Tags,
		TotalSessions: s.TotalSessions,
		LastSessionAt: s.LastSessionAt,
	}
}

func newClientRecordBody(rec ports.ClientRecord) clientRecordBody {
	body := clientRecordBody{
		ID:            rec.Profile.UserID,
		Name:          rec.Name,
		Email:         rec.Email,
		Phone:         rec.Profile.Phone,
		Tags:          rec.Profile.Tags,
		PracticeNotes: rec.Profile.PracticeNotes,
		Payments: paymentSummaryBody{
			TotalPaidKobo:     rec.Payments.TotalPaidKobo,
			TotalRefundedKobo: rec.Payments.TotalRefundedKobo,
			PaymentCount:      rec.Payments.PaymentCount,
			Currency:          rec.Payments.Currency,
		},
		DocumentCount:       rec.DocumentCount,
		FormSubmissionCount: rec.FormSubmissionCount,
	}
	if rec.Profile.ID != "" {
		created := rec.Profile.CreatedAt
		updated := rec.Profile.UpdatedAt
		body.ProfileCreatedAt = &created
		body.ProfileUpdatedAt = &updated
	}
	body.RecentBookings = make([]bookingBody, 0, len(rec.RecentBookings))
	for _, b := range rec.RecentBookings {
		body.RecentBookings = append(body.RecentBookings, newBookingBody(b))
	}
	return body
}

// list handles GET /v1/clients — the practice's client list with rollups.
func (h *clientHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	summaries, err := h.svc.ListClients(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]clientSummaryBody, 0, len(summaries))
	for _, s := range summaries {
		items = append(items, newClientSummaryBody(s))
	}
	writeJSON(w, http.StatusOK, map[string][]clientSummaryBody{"items": items})
}

// me handles GET /v1/clients/me — the client's own profile.
func (h *clientHandler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	me, err := h.svc.GetMe(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]clientMeBody{"profile": clientMeBody{
		ID:        me.ID,
		Name:      me.Name,
		Email:     me.Email,
		Phone:     me.Phone,
		CreatedAt: me.CreatedAt.UTC(),
	}})
}

// record handles GET /v1/clients/{id} — the full practice-side record.
func (h *clientHandler) record(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	rec, err := h.svc.GetClientRecord(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]clientRecordBody{"record": newClientRecordBody(rec)})
}

// patch handles PATCH /v1/clients/{id} — practice-side fields only.
func (h *clientHandler) patch(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		Phone         *string   `json:"phone"`
		PracticeNotes *string   `json:"practiceNotes"`
		Tags          *[]string `json:"tags"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	profile, err := h.svc.UpdatePracticeFields(r.Context(), id.UserID, chi.URLParam(r, "id"), domainclient.PracticePatch{
		Phone:         req.Phone,
		PracticeNotes: req.PracticeNotes,
		Tags:          req.Tags,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]clientProfileBody{"profile": clientProfileBody{
		ID:            profile.UserID,
		Phone:         profile.Phone,
		Tags:          profile.Tags,
		PracticeNotes: profile.PracticeNotes,
		CreatedAt:     profile.CreatedAt.UTC(),
		UpdatedAt:     profile.UpdatedAt.UTC(),
	}})
}
