package resend

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xcreativs/terios/api/internal/ports"
)

// withServer points the adapter at a stub Resend for the duration of a test.
func withServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	original := baseURL
	baseURL = srv.URL
	t.Cleanup(func() {
		baseURL = original
		srv.Close()
	})
}

var testMessage = ports.EmailMessage{
	To:      "ama@example.com",
	Subject: "Your session is confirmed",
	HTML:    "<p>Hello Ama</p>",
	Text:    "Hello Ama",
}

// TestSendPostsTheRenderedMessage: the adapter authenticates and forwards
// exactly what it was given, adding only the verified sender.
func TestSendPostsTheRenderedMessage(t *testing.T) {
	var got sendRequest
	var authorization, path string

	withServer(t, func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_1"}`))
	})

	client := NewClient("re_test_key", "Terios Wellness Spa <no-reply@terioscoach.com>")
	if err := client.Send(context.Background(), testMessage); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if path != "/emails" {
		t.Errorf("path = %q, want /emails", path)
	}
	if authorization != "Bearer re_test_key" {
		t.Errorf("Authorization = %q, want the bearer secret", authorization)
	}
	if got.From != "Terios Wellness Spa <no-reply@terioscoach.com>" {
		t.Errorf("from = %q, want the configured sender", got.From)
	}
	if len(got.To) != 1 || got.To[0] != "ama@example.com" {
		t.Errorf("to = %v, want the single recipient", got.To)
	}
	if got.Subject != testMessage.Subject || got.HTML != testMessage.HTML || got.Text != testMessage.Text {
		t.Errorf("payload = %+v, want the rendered message forwarded unchanged", got)
	}
}

// TestSendRejectsMissingRecipient: nowhere to send is caught before a
// pointless round trip.
func TestSendRejectsMissingRecipient(t *testing.T) {
	called := false
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	err := NewClient("re_test_key", "sender@example.com").Send(context.Background(), ports.EmailMessage{Subject: "x"})
	if err == nil {
		t.Fatal("Send succeeded with no recipient")
	}
	if called {
		t.Error("the provider was called with no recipient")
	}
}

// TestSendSurfacesProviderErrors: a rejection becomes a typed gateway error
// carrying the provider's own explanation, so a failed job records why.
func TestSendSurfacesProviderErrors(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"name":"validation_error","message":"Invalid to field"}`))
	})

	err := NewClient("re_test_key", "sender@example.com").Send(context.Background(), testMessage)
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want a *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", gatewayErr.StatusCode)
	}
	if gatewayErr.Message != "Invalid to field" {
		t.Errorf("message = %q, want the provider's explanation", gatewayErr.Message)
	}
}

// TestSendHandlesUnparseableErrorBody: a provider failure with no usable
// body still reports the status rather than claiming success.
func TestSendHandlesUnparseableErrorBody(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<html>gateway timeout</html>`))
	})

	err := NewClient("re_test_key", "sender@example.com").Send(context.Background(), testMessage)
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want a *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", gatewayErr.StatusCode)
	}
	if gatewayErr.Message == "" {
		t.Error("empty message, want a fallback explanation")
	}
}

// TestSendHandlesUnreachableProvider: a transport failure is a gateway
// error with no status, which the retry policy treats as transient.
func TestSendHandlesUnreachableProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	original := baseURL
	baseURL = srv.URL
	srv.Close() // nothing is listening now
	t.Cleanup(func() { baseURL = original })

	err := NewClient("re_test_key", "sender@example.com").Send(context.Background(), testMessage)
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("err = %v, want a *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != 0 {
		t.Errorf("status = %d, want 0 for a transport failure", gatewayErr.StatusCode)
	}
}

// TestSendHonoursContextCancellation: a shutdown mid-send aborts rather
// than blocking the dispatcher.
func TestSendHonoursContextCancellation(t *testing.T) {
	withServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := NewClient("re_test_key", "sender@example.com").Send(ctx, testMessage); err == nil {
		t.Fatal("Send succeeded on a cancelled context")
	}
}
