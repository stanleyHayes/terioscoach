package httpapi

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	paymentsapp "github.com/xcreativs/terios/api/internal/app/payments"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// paymentTestRig bundles a fully wired server over in-memory fakes plus
// client and practitioner tokens, the seeded booking, and the gateway.
type paymentTestRig struct {
	srv               *Server
	services          *portstest.FakeServiceRepository
	bookings          *portstest.FakeBookingRepository
	payments          *portstest.FakePaymentRepository
	gateway           *portstest.FakePaymentGateway
	booking           booking.Booking
	clientToken       string
	otherClientToken  string
	practitionerToken string
}

const paymentTestWebhookSecret = "sk_test_webhook"

func newPaymentTestRig(t *testing.T) paymentTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	users := portstest.NewFakeUserRepository()
	authSvc := auth.NewService(
		users,
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)

	now := time.Now().UTC()
	// Seed the client account first so the token id matches a real user
	// (initialize reads the receipt email from the account).
	client, err := users.Create(t.Context(), mustHTTPUser(t, "client@example.com", identity.RoleClient, now))
	if err != nil {
		t.Fatalf("seed client: %v", err)
	}
	other, err := users.Create(t.Context(), mustHTTPUser(t, "other@example.com", identity.RoleClient, now))
	if err != nil {
		t.Fatalf("seed other client: %v", err)
	}

	services := portstest.NewFakeServiceRepository()
	svc, err := catalog.NewService("prac-1", "Massage", "", 60, 25000, "GHS", 1, now)
	if err != nil {
		t.Fatalf("domain NewService: %v", err)
	}
	if _, err := services.Create(t.Context(), svc); err != nil {
		t.Fatalf("seed service: %v", err)
	}
	stored, err := services.FindByID(t.Context(), "service-1")
	if err != nil {
		t.Fatalf("load service: %v", err)
	}

	bookings := portstest.NewFakeBookingRepository()
	b, err := booking.New(client.ID, "prac-1", stored.ID, now.Add(48*time.Hour), 60, now)
	if err != nil {
		t.Fatalf("domain booking New: %v", err)
	}
	b, err = bookings.Create(t.Context(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}

	payments := portstest.NewFakePaymentRepository()
	gateway := portstest.NewFakePaymentGateway(paymentTestWebhookSecret)
	paymentSvc := paymentsapp.NewService(payments, bookings, services, users, gateway)

	srv := NewServer(
		WithAuth(authSvc),
		WithPayments(paymentSvc, authSvc),
	)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return paymentTestRig{
		srv:               srv,
		services:          services,
		bookings:          bookings,
		payments:          payments,
		gateway:           gateway,
		booking:           b,
		clientToken:       issue(client.ID, identity.RoleClient),
		otherClientToken:  issue(other.ID, identity.RoleClient),
		practitionerToken: issue("prac-1", identity.RolePractitioner),
	}
}

func mustHTTPUser(t *testing.T, email string, role identity.Role, now time.Time) identity.User {
	t.Helper()
	u, err := identity.NewUser(email, "Test User", "fakehash:password", role, now)
	if err != nil {
		t.Fatalf("domain NewUser: %v", err)
	}
	return u
}

// paymentTestBody is the contract payment shape for assertions.
type paymentTestBody struct {
	ID                string     `json:"id"`
	BookingID         string     `json:"bookingId"`
	ClientID          string     `json:"clientId"`
	AmountKobo        int64      `json:"amountKobo"`
	Currency          string     `json:"currency"`
	Status            string     `json:"status"`
	PaystackReference string     `json:"paystackReference"`
	Channel           string     `json:"channel"`
	PaidAt            *time.Time `json:"paidAt"`
}

func initializeViaHTTP(t *testing.T, rig paymentTestRig, token string) map[string]string {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/payments/initialize",
		map[string]string{"bookingId": rig.booking.ID}, bearer(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res map[string]string
	decodeBody(t, rec, &res)
	return res
}

// deliverWebhook posts a raw, signed webhook body.
func deliverWebhook(t *testing.T, rig paymentTestRig, payload []byte, signature string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/paystack", bytes.NewReader(payload))
	if signature != "" {
		req.Header.Set("x-paystack-signature", signature)
	}
	rec := httptest.NewRecorder()
	rig.srv.Router.ServeHTTP(rec, req)
	return rec
}

func TestPaymentInitializeHappyPath(t *testing.T) {
	rig := newPaymentTestRig(t)
	res := initializeViaHTTP(t, rig, rig.clientToken)
	if res["authorizationUrl"] == "" || res["reference"] == "" {
		t.Errorf("response = %v, want authorizationUrl and reference", res)
	}
}

func TestPaymentInitializeMatrix(t *testing.T) {
	rig := newPaymentTestRig(t)

	cases := []struct {
		name   string
		method string
		path   string
		token  string
		want   int
	}{
		{"unauthenticated", http.MethodPost, "/v1/payments/initialize", "", http.StatusUnauthorized},
		{"practitioner cannot initialize", http.MethodPost, "/v1/payments/initialize", rig.practitionerToken, http.StatusForbidden},
		{"other client gets 404", http.MethodPost, "/v1/payments/initialize", rig.otherClientToken, http.StatusNotFound},
		{"mine unauthenticated", http.MethodGet, "/v1/payments/mine", "", http.StatusUnauthorized},
		{"mine practitioner forbidden", http.MethodGet, "/v1/payments/mine", rig.practitionerToken, http.StatusForbidden},
		{"list unauthenticated", http.MethodGet, "/v1/payments", "", http.StatusUnauthorized},
		{"list client forbidden", http.MethodGet, "/v1/payments", rig.clientToken, http.StatusForbidden},
		{"refund client forbidden", http.MethodPost, "/v1/payments/payment-1/refund", rig.clientToken, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]string{"bookingId": rig.booking.ID}
			if tc.method == http.MethodGet {
				body = nil
			}
			headers := map[string]string{}
			if tc.token != "" {
				headers = bearer(tc.token)
			}
			rec := doJSON(t, rig.srv, tc.method, tc.path, body, headers)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestPaymentInitializeValidationAndConflict(t *testing.T) {
	rig := newPaymentTestRig(t)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/payments/initialize", map[string]string{}, bearer(rig.clientToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing bookingId status = %d, want 400", rec.Code)
	}

	res := initializeViaHTTP(t, rig, rig.clientToken)

	// Pay it, then a second initialize answers 409 already_paid.
	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, res["reference"]))
	wh := deliverWebhook(t, rig, payload, rig.gateway.SignWebhook(payload))
	if wh.Code != http.StatusOK {
		t.Fatalf("webhook status = %d, body %s", wh.Code, wh.Body.String())
	}
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/payments/initialize",
		map[string]string{"bookingId": rig.booking.ID}, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Errorf("already paid status = %d, want 409", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "already_paid" {
		t.Errorf("code = %q, want already_paid", errRes.Error.Code)
	}
}

func TestPaymentWebhookEndToEnd(t *testing.T) {
	rig := newPaymentTestRig(t)
	res := initializeViaHTTP(t, rig, rig.clientToken)
	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, res["reference"]))

	// Bad signature → 401 invalid_signature, payment untouched.
	bad := deliverWebhook(t, rig, payload, "deadbeef")
	if bad.Code != http.StatusUnauthorized {
		t.Errorf("bad signature status = %d, want 401", bad.Code)
	}

	// Valid signature → 200; double delivery is also 200 and idempotent.
	for i := 0; i < 2; i++ {
		rec := deliverWebhook(t, rig, payload, rig.gateway.SignWebhook(payload))
		if rec.Code != http.StatusOK {
			t.Fatalf("delivery %d status = %d, body %s", i+1, rec.Code, rec.Body.String())
		}
	}

	// The client's list shows exactly one successful payment.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/payments/mine", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("mine status = %d", rec.Code)
	}
	var list struct {
		Items []paymentTestBody `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 {
		t.Fatalf("mine items = %d, want 1", len(list.Items))
	}
	got := list.Items[0]
	if got.Status != "success" || got.Channel != "card" || got.PaidAt == nil {
		t.Errorf("payment = %+v, want success with channel and paidAt", got)
	}
	if got.AmountKobo != 25000 || got.Currency != "GHS" || got.BookingID != rig.booking.ID {
		t.Errorf("payment = %+v, want booking snapshot fields", got)
	}

	// The practitioner's list sees the same payment; the booking carries
	// the denormalized paid stamp... (booking routes are not mounted in
	// this rig, so the stamp is asserted at the repository level).
	b, err := rig.bookings.FindByID(t.Context(), rig.booking.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if b.PaymentStatus != booking.PaymentPaid || b.PaidAt == nil {
		t.Errorf("booking paymentStatus = %q, want paid stamp", b.PaymentStatus)
	}

	// Practitioner list with a from/to window.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/payments", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 {
		t.Errorf("practitioner items = %d, want 1", len(list.Items))
	}

	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/payments?from="+future, nil, bearer(rig.practitionerToken))
	decodeBody(t, rec, &list)
	if len(list.Items) != 0 {
		t.Errorf("filtered items = %d, want 0", len(list.Items))
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/payments?from=not-a-time", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad from status = %d, want 400", rec.Code)
	}
}

func TestPaymentRefundEndToEnd(t *testing.T) {
	rig := newPaymentTestRig(t)
	res := initializeViaHTTP(t, rig, rig.clientToken)

	// Refund while pending → 409 invalid_status.
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/payments/payment-1/refund", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Errorf("refund pending status = %d, want 409", rec.Code)
	}

	payload := []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, res["reference"]))
	if rec := deliverWebhook(t, rig, payload, rig.gateway.SignWebhook(payload)); rec.Code != http.StatusOK {
		t.Fatalf("webhook status = %d", rec.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/payments/payment-1/refund", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("refund status = %d, body %s", rec.Code, rec.Body.String())
	}
	var refundRes struct {
		Payment paymentTestBody `json:"payment"`
	}
	decodeBody(t, rec, &refundRes)
	if refundRes.Payment.Status != "refunded" {
		t.Errorf("status = %q, want refunded", refundRes.Payment.Status)
	}

	// Unknown payment id → 404 payment_not_found.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/payments/payment-999/refund", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown payment status = %d, want 404", rec.Code)
	}
}

func TestPaymentGatewayErrorMapsTo502(t *testing.T) {
	rig := newPaymentTestRig(t)
	rig.gateway.InitializeErr = &ports.GatewayError{StatusCode: 500, Message: "paystack is down"}
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/payments/initialize",
		map[string]string{"bookingId": rig.booking.ID}, bearer(rig.clientToken))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "payment_gateway_error" {
		t.Errorf("code = %q, want payment_gateway_error", errRes.Error.Code)
	}
}

func TestPaymentsUnavailableWithoutService(t *testing.T) {
	srv := NewServer(WithPayments(nil, nil))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/v1/payments/initialize"},
		{http.MethodGet, "/v1/payments/mine"},
		{http.MethodGet, "/v1/payments"},
		{http.MethodPost, "/v1/payments/payment-1/refund"},
		{http.MethodPost, "/v1/webhooks/paystack"},
		{http.MethodPost, "/v1/webhooks/stripe"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := doJSON(t, srv, tc.method, tc.path, map[string]string{}, nil)
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("status = %d, want 503", rec.Code)
			}
			var errRes errorTestResponse
			decodeBody(t, rec, &errRes)
			if errRes.Error.Code != "service_unavailable" {
				t.Errorf("code = %q, want service_unavailable", errRes.Error.Code)
			}
		})
	}
}

// deliverStripeWebhook posts a raw, signed Stripe webhook body.
func deliverStripeWebhook(t *testing.T, rig paymentTestRig, payload []byte, signature string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/stripe", bytes.NewReader(payload))
	if signature != "" {
		req.Header.Set("Stripe-Signature", signature)
	}
	rec := httptest.NewRecorder()
	rig.srv.Router.ServeHTTP(rec, req)
	return rec
}

func TestStripeWebhookEndToEnd(t *testing.T) {
	rig := newPaymentTestRig(t)
	res := initializeViaHTTP(t, rig, rig.clientToken)
	payload := []byte(fmt.Sprintf(`{"type":"checkout.session.completed","data":{"object":{"id":%q}}}`, res["reference"]))

	// Bad signature → 401 invalid_signature, payment untouched.
	bad := deliverStripeWebhook(t, rig, payload, "t=1,v1=deadbeef")
	if bad.Code != http.StatusUnauthorized {
		t.Errorf("bad signature status = %d, want 401", bad.Code)
	}

	// Valid signature → 200; double delivery is also 200 and idempotent.
	for i := 0; i < 2; i++ {
		rec := deliverStripeWebhook(t, rig, payload, rig.gateway.SignWebhook(payload))
		if rec.Code != http.StatusOK {
			t.Fatalf("delivery %d status = %d, body %s", i+1, rec.Code, rec.Body.String())
		}
	}

	// The client's list shows exactly one successful payment.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/payments/mine", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("mine status = %d", rec.Code)
	}
	var list struct {
		Items []paymentTestBody `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 || list.Items[0].Status != "success" || list.Items[0].PaidAt == nil {
		t.Errorf("items = %+v, want exactly one success with paidAt", list.Items)
	}

	// The booking carries the denormalized paid stamp.
	b, err := rig.bookings.FindByID(t.Context(), rig.booking.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if b.PaymentStatus != booking.PaymentPaid || b.PaidAt == nil {
		t.Errorf("booking paymentStatus = %q, want paid stamp", b.PaymentStatus)
	}
}
