package paystack

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

const testSecret = "sk_test_abc123"

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

func TestInitialize(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody map[string]any
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":true,"message":"Authorization URL created","data":{"authorization_url":"https://checkout.paystack.com/xyz","reference":"terios_b_1"}}`))
	}))

	c := NewClient(testSecret)
	init, err := c.Initialize(context.Background(), ports.InitializeParams{
		Email: "client@example.com", AmountKobo: 25000, Currency: "GHS", Reference: "terios_b_1",
	})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if init.AuthorizationURL != "https://checkout.paystack.com/xyz" || init.Reference != "terios_b_1" {
		t.Errorf("init = %+v, want authorization URL and reference", init)
	}
	if gotAuth != "Bearer "+testSecret {
		t.Errorf("Authorization = %q, want Bearer secret", gotAuth)
	}
	if gotPath != "/transaction/initialize" {
		t.Errorf("path = %q", gotPath)
	}
	// Amount crosses the wire as integer minor units.
	if gotBody["amount"] != float64(25000) || gotBody["currency"] != "GHS" || gotBody["email"] != "client@example.com" {
		t.Errorf("body = %v, want email/amount/currency passed through", gotBody)
	}
}

func TestInitializeRejectedIsTypedGatewayError(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":false,"message":"Invalid amount"}`))
	}))

	c := NewClient(testSecret)
	_, err := c.Initialize(context.Background(), ports.InitializeParams{
		Email: "c@example.com", AmountKobo: -5, Currency: "GHS", Reference: "r",
	})
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != http.StatusBadRequest || gatewayErr.Message != "Invalid amount" {
		t.Errorf("gatewayErr = %+v, want status 400 with the provider message", gatewayErr)
	}
}

func TestInitializeUnreachableIsTypedGatewayError(t *testing.T) {
	// A closed port: the transport fails before any response.
	withServer(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	orig := baseURL
	baseURL = "http://127.0.0.1:1"
	t.Cleanup(func() { baseURL = orig })

	c := NewClient(testSecret)
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

func TestVerify(t *testing.T) {
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transaction/verify/terios_b_1" || r.Method != http.MethodGet {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":true,"message":"Verification successful","data":{
			"reference":"terios_b_1","status":"success","amount":25000,"currency":"GHS",
			"channel":"mobile_money","paid_at":"2026-03-02T10:15:00.000Z"}}`))
	}))

	c := NewClient(testSecret)
	vt, err := c.Verify(context.Background(), "terios_b_1")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	want := ports.VerifiedTransaction{
		Reference:  "terios_b_1",
		Status:     "success",
		AmountKobo: 25000,
		Currency:   "GHS",
		Channel:    "mobile_money",
		PaidAt:     time.Date(2026, 3, 2, 10, 15, 0, 0, time.UTC),
	}
	if vt != want {
		t.Errorf("verify = %+v, want %+v", vt, want)
	}
}

func TestRefund(t *testing.T) {
	var gotBody map[string]any
	withServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/refund" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"status":true,"message":"Refund has been queued","data":{}}`))
	}))

	c := NewClient(testSecret)
	if err := c.Refund(context.Background(), "terios_b_1"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if gotBody["transaction"] != "terios_b_1" {
		t.Errorf("body = %v, want the transaction reference", gotBody)
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	c := NewClient(testSecret)
	payload := []byte(`{"event":"charge.success","data":{"reference":"terios_b_1"}}`)

	mac := hmac.New(sha512.New, []byte(testSecret))
	mac.Write(payload)
	valid := hex.EncodeToString(mac.Sum(nil))

	if !c.VerifyWebhookSignature(payload, valid) {
		t.Error("valid signature rejected")
	}

	tampered := []byte(`{"event":"charge.success","data":{"reference":"forged"}}`)
	if c.VerifyWebhookSignature(tampered, valid) {
		t.Error("tampered body accepted under the original signature")
	}

	wrongKey := hmac.New(sha512.New, []byte("sk_test_wrong"))
	wrongKey.Write(payload)
	if c.VerifyWebhookSignature(payload, hex.EncodeToString(wrongKey.Sum(nil))) {
		t.Error("signature under a wrong key accepted")
	}

	if c.VerifyWebhookSignature(payload, "") {
		t.Error("missing signature accepted")
	}
	if c.VerifyWebhookSignature(payload, "zzzz-not-hex") {
		t.Error("garbage signature accepted")
	}
}
