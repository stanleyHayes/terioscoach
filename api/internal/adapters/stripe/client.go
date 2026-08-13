// Package stripe is the outbound adapter for the Stripe REST API — a plain
// net/http client, no SDK, like the Paystack adapter. It implements
// ports.PaymentGateway over Stripe Checkout Sessions: card details only
// ever exist on Stripe's hosted checkout; this adapter deals in session
// references and minor-unit amounts only.
package stripe

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

// baseURL is the Stripe API root. It is a var so tests can point the
// client at an httptest server.
var baseURL = "https://api.stripe.com"

// requestTimeout bounds every gateway call.
const requestTimeout = 10 * time.Second

// maxResponseBytes caps gateway response bodies (1 MB — far beyond any
// legitimate Stripe payload).
const maxResponseBytes = 1 << 20

// webhookTolerance is how old a webhook timestamp may be before the
// delivery is rejected as a possible replay.
const webhookTolerance = 5 * time.Minute

// Client calls the Stripe REST API with the secret key. The webhook
// signing secret is separate from the API key (whsec_...) and is only
// used to authenticate webhook deliveries.
type Client struct {
	secretKey     string
	webhookSecret string
	successURL    string
	cancelURL     string
	http          *http.Client
}

// Compile-time check: Client satisfies the gateway port.
var _ ports.PaymentGateway = (*Client)(nil)

// NewClient builds a gateway client. successURL/cancelURL are where
// Stripe's hosted checkout returns the customer; they are required by
// Checkout Sessions and are derived from the portal URL by the caller.
func NewClient(secretKey, webhookSecret, successURL, cancelURL string) *Client {
	return &Client{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		successURL:    successURL,
		cancelURL:     cancelURL,
		http:          &http.Client{Timeout: requestTimeout},
	}
}

// checkoutSession is the subset of the Session object this adapter reads.
type checkoutSession struct {
	ID                 string   `json:"id"`
	URL                string   `json:"url"`
	PaymentStatus      string   `json:"payment_status"`
	AmountTotal        int64    `json:"amount_total"`
	Currency           string   `json:"currency"`
	PaymentMethodTypes []string `json:"payment_method_types"`
	PaymentIntent      string   `json:"payment_intent"`
}

// Initialize creates a Checkout Session and returns its hosted URL. The
// session ID becomes the gateway reference stored on the payment — the
// webhook join key. The app-generated reference rides along as
// client_reference_id for dashboard traceability.
func (c *Client) Initialize(ctx context.Context, params ports.InitializeParams) (ports.InitializedTransaction, error) {
	form := url.Values{
		"mode":                                   {"payment"},
		"customer_email":                         {params.Email},
		"client_reference_id":                    {params.Reference},
		"success_url":                            {c.successURL},
		"cancel_url":                             {c.cancelURL},
		"line_items[0][quantity]":                {"1"},
		"line_items[0][price_data][currency]":    {strings.ToLower(params.Currency)},
		"line_items[0][price_data][unit_amount]": {strconv.FormatInt(params.AmountKobo, 10)},
		"line_items[0][price_data][product_data][name]": {"Terios Wellness booking"},
	}
	var session checkoutSession
	if err := c.do(ctx, http.MethodPost, "/v1/checkout/sessions", form, &session); err != nil {
		return ports.InitializedTransaction{}, err
	}
	return ports.InitializedTransaction{AuthorizationURL: session.URL, Reference: session.ID}, nil
}

// Verify fetches the provider-side truth about a Checkout Session. A paid
// session maps to the gateway-neutral status "success" the app layer
// compares against; the session carries no paid-at timestamp, so PaidAt
// is left zero and the app layer stamps its own.
func (c *Client) Verify(ctx context.Context, reference string) (ports.VerifiedTransaction, error) {
	var session checkoutSession
	if err := c.do(ctx, http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(reference), nil, &session); err != nil {
		return ports.VerifiedTransaction{}, err
	}
	vt := ports.VerifiedTransaction{
		Reference:  session.ID,
		Status:     session.PaymentStatus,
		AmountKobo: session.AmountTotal,
		Currency:   strings.ToUpper(session.Currency),
	}
	if session.PaymentStatus == "paid" {
		vt.Status = "success"
	}
	if len(session.PaymentMethodTypes) > 0 {
		vt.Channel = session.PaymentMethodTypes[0]
	}
	return vt, nil
}

// Refund refunds a successful Checkout Session in full. Refunds attach to
// the payment intent, so the session is fetched first to resolve it.
func (c *Client) Refund(ctx context.Context, reference string) error {
	var session checkoutSession
	if err := c.do(ctx, http.MethodGet, "/v1/checkout/sessions/"+url.PathEscape(reference), nil, &session); err != nil {
		return err
	}
	if session.PaymentIntent == "" {
		return &ports.GatewayError{StatusCode: 0, Message: "stripe session has no payment intent to refund"}
	}
	form := url.Values{"payment_intent": {session.PaymentIntent}}
	// Keyed on the session: a retry of this same refund — after a client-
	// side timeout on a request that landed — replays instead of
	// double-refunding.
	return c.doWithIdempotencyKey(ctx, http.MethodPost, "/v1/refunds", form, nil, "refund-"+session.ID)
}

// VerifyWebhookSignature reports whether the Stripe-Signature header is
// valid for payload under the webhook signing secret: a v1 value must be
// the hex HMAC-SHA256 of "t.payload", compared in constant time, and the
// t timestamp must be within the replay tolerance. Stripe sends several
// v1 entries while an endpoint secret is being rolled; any one match
// authenticates the delivery.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) bool {
	if c.webhookSecret == "" {
		return false
	}
	var timestamp string
	var v1s []string
	for _, part := range strings.Split(signature, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			v1s = append(v1s, kv[1])
		}
	}
	if timestamp == "" || len(v1s) == 0 {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if age := time.Since(time.Unix(ts, 0)); age > webhookTolerance || age < -webhookTolerance {
		return false
	}
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, v1 := range v1s {
		if hmac.Equal([]byte(expected), []byte(v1)) {
			return true
		}
	}
	return false
}

// stripeError is Stripe's error envelope: {"error": {"message": ...}}.
type stripeError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// do performs one authenticated call. POST bodies are form-encoded, as
// the Stripe API expects; responses decode as JSON into out (nil to
// ignore). Any non-2xx response becomes a typed ports.GatewayError;
// transport failures become one with StatusCode 0.
func (c *Client) do(ctx context.Context, method, path string, form url.Values, out any) error {
	return c.doWithIdempotencyKey(ctx, method, path, form, out, "")
}

// doWithIdempotencyKey is do with an Idempotency-Key header, making a
// retry of the same mutation safe — Stripe replays the first result
// instead of applying the change twice.
func (c *Client) doWithIdempotencyKey(ctx context.Context, method, path string, form url.Values, out any, idempotencyKey string) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build stripe request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "stripe is unreachable: " + err.Error()}
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "read stripe response: " + err.Error()}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := "stripe rejected the request"
		var serr stripeError
		if json.Unmarshal(raw, &serr) == nil && serr.Error.Message != "" {
			msg = serr.Error.Message
		}
		return &ports.GatewayError{StatusCode: res.StatusCode, Message: msg}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return &ports.GatewayError{StatusCode: res.StatusCode, Message: "malformed stripe response"}
		}
	}
	return nil
}
