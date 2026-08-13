// Package resend is the outbound adapter for the Resend transactional
// email API — a plain net/http client, no SDK. It implements ports.Mailer
// and never sees a template: the email adapter has already rendered the
// message by the time it arrives here.
package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

// baseURL is the Resend API root. It is a var so tests can point the client
// at an httptest server.
var baseURL = "https://api.resend.com"

// requestTimeout bounds every provider call.
const requestTimeout = 10 * time.Second

// maxResponseBytes caps provider response bodies.
const maxResponseBytes = 1 << 20

// Client sends mail through Resend.
type Client struct {
	apiKey string
	from   string
	http   *http.Client
}

// Compile-time check: Client satisfies the mailer port.
var _ ports.Mailer = (*Client)(nil)

// NewClient builds a mailer. from is the verified sender, in either
// "Name <address>" or bare-address form.
func NewClient(apiKey, from string) *Client {
	return &Client{
		apiKey: apiKey,
		from:   from,
		http:   &http.Client{Timeout: requestTimeout},
	}
}

// sendRequest is POST /emails.
type sendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text,omitempty"`
}

// errorResponse is Resend's failure shape.
type errorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// Send delivers one message.
//
// Failures come back as *ports.GatewayError so the notifications service
// can tell "the provider is having a moment, retry" from a programming
// error. Which of those a 4xx is depends on the code: a 429 or a 5xx is
// worth retrying, a 422 (malformed address) never will be — but the retry
// budget in the domain bounds both, so the distinction costs a few wasted
// attempts at most and never a lost message.
func (c *Client) Send(ctx context.Context, msg ports.EmailMessage) error {
	if msg.To == "" {
		return fmt.Errorf("resend: recipient is required")
	}
	body := sendRequest{
		From:    c.from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		HTML:    msg.HTML,
		Text:    msg.Text,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/emails", bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "resend is unreachable: " + err.Error()}
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "read resend response: " + err.Error()}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := fmt.Sprintf("resend rejected the message (HTTP %d)", res.StatusCode)
		var errRes errorResponse
		if json.Unmarshal(payload, &errRes) == nil && errRes.Message != "" {
			message = errRes.Message
		}
		return &ports.GatewayError{StatusCode: res.StatusCode, Message: message}
	}
	return nil
}
