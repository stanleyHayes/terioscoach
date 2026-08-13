package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

const (
	testSecret        = "sk_test_abc123"
	testWebhookSecret = "whsec_test_abc123"
)

// withServer points the package base URL at an httptest server for the
// duration of one test (no parallel tests: baseURL is a package var).
func withServer(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(func() { srv.Close() })
	orig := baseURL
	baseURL = srv.URL
	t.Cleanup(func() { baseURL = orig })
}

func newTestClient() *Client {
	return NewClient(testSecret, testWebhookSecret, "https://portal.example.com/payments", "https://portal.example.com/payments")
}

// sign computes a Stripe-Signature header for payload at the given time,
// exactly as Stripe's webhook endpoint would.
func sign(t *testing.T, payload []byte, at time.Time) string {
	t.Helper()
	ts := fmt.Sprintf("%d", at.Unix())
	mac := hmac.New(sha256.New, []byte(testWebhookSecret))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(payload)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestInitialize(t *testing.T) {
	var gotAuth, gotPath, gotContentType string
	var gotForm url.Values
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotForm, _ = url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cs_test_123","url":"https://checkout.stripe.com/c/pay/cs_test_123"}`))
	}))

	c := newTestClient()
	init, err := c.Initialize(context.Background(), ports.InitializeParams{
		Email: "client@example.com", AmountKobo: 25000, Currency: "GHS", Reference: "terios_b_1",
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	// The session ID becomes the gateway reference — the webhook join key.
	if init.AuthorizationURL != "https://checkout.stripe.com/c/pay/cs_test_123" || init.Reference != "cs_test_123" {
		t.Errorf("init = %+v, want checkout URL and session ID", init)
	}
	if gotAuth != "Bearer "+testSecret {
		t.Errorf("Authorization = %q, want Bearer secret", gotAuth)
	}
	if gotPath != "/v1/checkout/sessions" {
		t.Errorf("path = %q", gotPath)
	}
	if gotContentType != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want form encoding", gotContentType)
	}
	// Amount crosses the wire as integer minor units, lowercased currency,
	// and the app reference rides along for traceability.
	want := map[string]string{
		"mode":                                   "payment",
		"customer_email":                         "client@example.com",
		"client_reference_id":                    "terios_b_1",
		"success_url":                            "https://portal.example.com/payments",
		"cancel_url":                             "https://portal.example.com/payments",
		"line_items[0][quantity]":                "1",
		"line_items[0][price_data][currency]":    "ghs",
		"line_items[0][price_data][unit_amount]": "25000",
	}
	for k, v := range want {
		if gotForm.Get(k) != v {
			t.Errorf("form[%q] = %q, want %q (form = %v)", k, gotForm.Get(k), v, gotForm)
		}
	}
}

func TestInitializeRejectedIsTypedGatewayError(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Invalid integer: -5"}}`))
	}))

	c := newTestClient()
	_, err := c.Initialize(context.Background(), ports.InitializeParams{
		Email: "c@example.com", AmountKobo: -5, Currency: "GHS", Reference: "r",
	})
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != http.StatusBadRequest || gatewayErr.Message != "Invalid integer: -5" {
		t.Errorf("gatewayErr = %+v, want status 400 with the provider message", gatewayErr)
	}
}

func TestInitializeUnreachableIsTypedGatewayError(t *testing.T) {
	// A closed port: the transport fails before any response.
	withServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	orig := baseURL
	baseURL = "http://127.0.0.1:1"
	t.Cleanup(func() { baseURL = orig })

	c := newTestClient()
	_, err := c.Initialize(context.Background(), ports.InitializeParams{
		Email: "c@example.com", AmountKobo: 100, Currency: "GHS", Reference: "r",
	})
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 for transport failures", gatewayErr.StatusCode)
	}
}

func TestVerifyPaidMapsToSuccess(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/checkout/sessions/cs_test_123" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"id":"cs_test_123","payment_status":"paid","amount_total":25000,
			"currency":"ghs","payment_method_types":["card"],"payment_intent":"pi_123"}`))
	}))

	c := newTestClient()
	vt, err := c.Verify(context.Background(), "cs_test_123")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	want := ports.VerifiedTransaction{
		Reference:  "cs_test_123",
		Status:     "success",
		AmountKobo: 25000,
		Currency:   "GHS",
		Channel:    "card",
	}
	if vt != want {
		t.Errorf("verify = %+v, want %+v", vt, want)
	}
}

func TestVerifyUnpaidStaysUnpaid(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cs_test_123","payment_status":"unpaid","amount_total":25000,"currency":"ghs"}`))
	}))

	c := newTestClient()
	vt, err := c.Verify(context.Background(), "cs_test_123")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if vt.Status == "success" {
		t.Errorf("status = %q, unpaid session must not map to success", vt.Status)
	}
}

func TestRefundResolvesPaymentIntent(t *testing.T) {
	var gotForm url.Values
	var refundCalled bool
	var gotIdempotencyKey string
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/checkout/sessions/cs_test_123":
			_, _ = w.Write([]byte(`{"id":"cs_test_123","payment_status":"paid","payment_intent":"pi_123"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/refunds":
			refundCalled = true
			gotIdempotencyKey = r.Header.Get("Idempotency-Key")
			raw, _ := io.ReadAll(r.Body)
			gotForm, _ = url.ParseQuery(string(raw))
			_, _ = w.Write([]byte(`{"id":"re_123","status":"succeeded"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))

	c := newTestClient()
	if err := c.Refund(context.Background(), "cs_test_123"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if !refundCalled {
		t.Fatal("POST /v1/refunds was never called")
	}
	if gotForm.Get("payment_intent") != "pi_123" {
		t.Errorf("form = %v, want payment_intent=pi_123", gotForm)
	}
	if gotIdempotencyKey == "" {
		t.Error("refund sent without an Idempotency-Key — a retry could double-refund")
	}
}

func TestRefundWithoutPaymentIntentFails(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"cs_test_123","payment_status":"unpaid","payment_intent":""}`))
	}))

	c := newTestClient()
	if err := c.Refund(context.Background(), "cs_test_123"); err == nil {
		t.Fatal("Refund: want an error when the session carries no payment intent")
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	c := newTestClient()
	payload := []byte(`{"type":"checkout.session.completed","data":{"object":{"id":"cs_test_123"}}}`)

	if !c.VerifyWebhookSignature(payload, sign(t, payload, time.Now())) {
		t.Error("valid signature rejected")
	}

	// Secret rotation: several v1 entries, any one match authenticates.
	ts := fmt.Sprintf("%d", time.Now().Unix())
	goodMAC := hmac.New(sha256.New, []byte(testWebhookSecret))
	goodMAC.Write([]byte(ts + "." + string(payload)))
	good := hex.EncodeToString(goodMAC.Sum(nil))
	other := strings.Repeat("ab", 32)
	if !c.VerifyWebhookSignature(payload, "t="+ts+",v1="+good+",v1="+other) {
		t.Error("valid v1 alongside another v1 rejected (rotation case)")
	}
	if !c.VerifyWebhookSignature(payload, "t="+ts+",v1="+other+",v1="+good) {
		t.Error("valid v1 in a later position rejected (rotation case)")
	}

	tampered := []byte(`{"type":"checkout.session.completed","data":{"object":{"id":"forged"}}}`)
	if c.VerifyWebhookSignature(tampered, sign(t, payload, time.Now())) {
		t.Error("tampered body accepted under the original signature")
	}

	// A signature over the payload under a different secret.
	mac := hmac.New(sha256.New, []byte("whsec_wrong"))
	mac.Write([]byte(ts + "." + string(payload)))
	if c.VerifyWebhookSignature(payload, "t="+ts+",v1="+hex.EncodeToString(mac.Sum(nil))) {
		t.Error("signature under a wrong key accepted")
	}

	if c.VerifyWebhookSignature(payload, sign(t, payload, time.Now().Add(-10*time.Minute))) {
		t.Error("stale timestamp accepted")
	}
	if c.VerifyWebhookSignature(payload, "") {
		t.Error("missing signature accepted")
	}
	if c.VerifyWebhookSignature(payload, "garbage") {
		t.Error("garbage signature accepted")
	}

	noSecret := NewClient(testSecret, "", "https://x", "https://x")
	if noSecret.VerifyWebhookSignature(payload, sign(t, payload, time.Now())) {
		t.Error("signature accepted without a webhook secret configured")
	}
}
