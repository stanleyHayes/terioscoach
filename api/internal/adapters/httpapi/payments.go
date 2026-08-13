package httpapi

import (
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
)

// maxWebhookBytes caps webhook bodies (1 MB — far beyond any legitimate
// Paystack event).
const maxWebhookBytes = 1 << 20

// WithPayments mounts the /v1/payments and /v1/webhooks/{paystack,stripe}
// routes backed by the payment port. Initialize and "mine" are
// client-only; the list and refund are practitioner-only; the webhooks
// are public and signature-verified inside the app layer. A nil service
// (database or gateway unconfigured) keeps every route — webhooks
// included — mounted but answering 503.
func WithPayments(svc ports.PaymentService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/payments", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handlePaymentsUnavailable)
				return
			}
			h := &paymentHandler{svc: svc}
			client := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RoleClient)}
			practitioner := []func(http.Handler) http.Handler{RequireAuth(auth), RequireRole(identity.RolePractitioner)}
			r.With(client...).Post("/initialize", h.initialize)
			r.With(client...).Get("/mine", h.listMine)
			r.With(practitioner...).Get("/", h.listForPractitioner)
			r.With(practitioner...).Post("/{id}/refund", h.refund)
		})
		s.Router.Route("/v1/webhooks", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handlePaymentsUnavailable)
				return
			}
			h := &paymentHandler{svc: svc}
			r.Post("/paystack", h.paystackWebhook)
			r.Post("/stripe", h.stripeWebhook)
		})
	}
}

// handlePaymentsUnavailable answers every payment and webhook route when
// the database — or the configured payment gateway — is not connected.
func handlePaymentsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "payments are unavailable: database or payment gateway not connected")
}

type paymentHandler struct {
	svc ports.PaymentService
}

// paymentBody is the contract payment shape.
type paymentBody struct {
	ID                string     `json:"id"`
	BookingID         string     `json:"bookingId"`
	ClientID          string     `json:"clientId"`
	AmountKobo        int64      `json:"amountKobo"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	PaystackReference string     `json:"paystackReference"`
	Channel           string     `json:"channel,omitempty"`
	PaidAt            *time.Time `json:"paidAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
}

func newPaymentBody(p payment.Payment) paymentBody {
	return paymentBody{
		ID:                p.ID,
		BookingID:         p.BookingID,
		ClientID:          p.ClientID,
		AmountKobo:        p.AmountKobo,
		Currency:          p.Currency,
		Status:            string(p.Status),
		PaystackReference: p.PaystackReference,
		Channel:           p.Channel,
		PaidAt:            p.PaidAt,
		CreatedAt:         p.CreatedAt.UTC(),
	}
}

func newPaymentList(payments []payment.Payment) map[string][]paymentBody {
	items := make([]paymentBody, 0, len(payments))
	for _, p := range payments {
		items = append(items, newPaymentBody(p))
	}
	return map[string][]paymentBody{"items": items}
}

// initialize handles POST /v1/payments/initialize — the client starts a
// hosted checkout for its own booking.
func (h *paymentHandler) initialize(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		BookingID string `json:"bookingId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.BookingID == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "bookingId is required")
		return
	}
	init, err := h.svc.InitializePayment(r.Context(), id, req.BookingID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"authorizationUrl": init.AuthorizationURL,
		"reference":        init.Payment.PaystackReference,
	})
}

// listMine handles GET /v1/payments/mine — the client's own payments.
func (h *paymentHandler) listMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	payments, err := h.svc.ListMine(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newPaymentList(payments))
}

// listForPractitioner handles GET /v1/payments with optional ?from=&to=
// filters on createdAt.
func (h *paymentHandler) listForPractitioner(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	var filter ports.PaymentFilter
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
	payments, err := h.svc.ListForPractitioner(r.Context(), id.UserID, filter)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newPaymentList(payments))
}

// refund handles POST /v1/payments/{id}/refund — practitioner-only,
// success-only (guarded in the app layer).
func (h *paymentHandler) refund(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	p, err := h.svc.RefundPayment(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]paymentBody{"payment": newPaymentBody(p)})
}

// paystackWebhook handles POST /v1/webhooks/paystack — public, with the
// raw body read before anything else so the HMAC can be verified over the
// exact bytes Paystack signed.
func (h *paymentHandler) paystackWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "webhook body is unreadable")
		return
	}
	if err := h.svc.HandlePaystackWebhook(r.Context(), body, r.Header.Get("x-paystack-signature")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{})
}

// stripeWebhook handles POST /v1/webhooks/stripe — public, with the raw
// body read before anything else so the signature can be verified over
// the exact bytes Stripe signed.
func (h *paymentHandler) stripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "webhook body is unreadable")
		return
	}
	if err := h.svc.HandleStripeWebhook(r.Context(), body, r.Header.Get("Stripe-Signature")); err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{})
}
