// Package paystack is the outbound adapter for the Paystack REST API —
// a plain net/http client, no SDK. It implements ports.PaymentGateway.
// Card and mobile-money details only ever exist on Paystack's hosted
// checkout; this adapter deals in references and minor-unit amounts only.
package paystack

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

// baseURL is the Paystack API root. It is a var so tests can point the
// client at an httptest server.
var baseURL = "https://api.paystack.co"

// requestTimeout bounds every gateway call.
const requestTimeout = 10 * time.Second

// maxResponseBytes caps gateway response bodies (1 MB — far beyond any
// legitimate Paystack payload).
const maxResponseBytes = 1 << 20

// Client calls the Paystack REST API with the secret key.
type Client struct {
	secretKey string
	http      *http.Client
}

// Compile-time check: Client satisfies the gateway port.
var _ ports.PaymentGateway = (*Client)(nil)

// NewClient builds a gateway client for the given secret key.
func NewClient(secretKey string) *Client {
	return &Client{
		secretKey: secretKey,
		http:      &http.Client{Timeout: requestTimeout},
	}
}

// envelope is Paystack's standard response wrapper.
type envelope struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// initializeRequest is POST /transaction/initialize. Amount is integer
// minor units, as Paystack expects.
type initializeRequest struct {
	Email     string `json:"email"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Reference string `json:"reference"`
}

type initializeData struct {
	AuthorizationURL string `json:"authorization_url"`
	Reference        string `json:"reference"`
}

// Initialize creates a transaction and returns the hosted checkout URL.
func (c *Client) Initialize(ctx context.Context, params ports.InitializeParams) (ports.InitializedTransaction, error) {
	body := initializeRequest{
		Email:     params.Email,
		Amount:    params.AmountKobo,
		Currency:  params.Currency,
		Reference: params.Reference,
	}
	var data initializeData
	if err := c.do(ctx, http.MethodPost, "/transaction/initialize", body, &data); err != nil {
		return ports.InitializedTransaction{}, err
	}
	return ports.InitializedTransaction{AuthorizationURL: data.AuthorizationURL, Reference: data.Reference}, nil
}

// verifyData is the transaction object from GET /transaction/verify/{ref}.
type verifyData struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Channel   string `json:"channel"`
	PaidAt    string `json:"paid_at"`
}

// Verify fetches the provider-side truth about a transaction.
func (c *Client) Verify(ctx context.Context, reference string) (ports.VerifiedTransaction, error) {
	var data verifyData
	if err := c.do(ctx, http.MethodGet, "/transaction/verify/"+reference, nil, &data); err != nil {
		return ports.VerifiedTransaction{}, err
	}
	vt := ports.VerifiedTransaction{
		Reference:  data.Reference,
		Status:     data.Status,
		AmountKobo: data.Amount,
		Currency:   data.Currency,
		Channel:    data.Channel,
	}
	if data.PaidAt != "" {
		if paidAt, err := time.Parse(time.RFC3339, data.PaidAt); err == nil {
			vt.PaidAt = paidAt.UTC()
		}
	}
	return vt, nil
}

// refundRequest is POST /refund — the transaction reference is the only
// required field.
type refundRequest struct {
	Transaction string `json:"transaction"`
}

// Refund refunds a successful transaction by reference.
func (c *Client) Refund(ctx context.Context, reference string) error {
	return c.do(ctx, http.MethodPost, "/refund", refundRequest{Transaction: reference}, nil)
}

// VerifyWebhookSignature reports whether signature is the hex HMAC-SHA512
// of payload under the secret key, compared in constant time.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha512.New, []byte(c.secretKey))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// do performs one authenticated JSON call and decodes the envelope's data
// into out (nil to ignore). Any non-2xx or status:false response becomes a
// typed ports.GatewayError; transport failures become one with StatusCode 0.
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode paystack request: %w", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, body)
	if err != nil {
		return fmt.Errorf("build paystack request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "paystack is unreachable: " + err.Error()}
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "read paystack response: " + err.Error()}
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return &ports.GatewayError{StatusCode: res.StatusCode, Message: "malformed paystack response"}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || !env.Status {
		msg := env.Message
		if msg == "" {
			msg = "paystack rejected the request"
		}
		return &ports.GatewayError{StatusCode: res.StatusCode, Message: msg}
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return &ports.GatewayError{StatusCode: res.StatusCode, Message: "malformed paystack data"}
		}
	}
	return nil
}
