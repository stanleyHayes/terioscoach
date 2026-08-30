package httpapi

import (
	"net/http"
	"testing"
)

// newSecuredServer builds a server with the production CORS allowlist.
func newSecuredServer(origins []string, production bool) *Server {
	return NewServerWith([]BuildOption{
		WithCORS(CORSPolicy{AllowedOrigins: origins}),
		WithProduction(production),
	})
}

const appOrigin = "https://terioscoach.com"

// TestAllowedOriginGetsCredentialedCORS: the apps are on another origin, so
// this is what makes every browser call work.
func TestAllowedOriginGetsCredentialedCORS(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	rec := doJSON(t, srv, http.MethodGet, "/healthz", nil, map[string]string{"Origin": appOrigin})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != appOrigin {
		t.Errorf("Allow-Origin = %q, want the caller's origin echoed", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
	// Without Vary, a cache could serve one origin's response to another.
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

// TestUnknownOriginGetsNoCORSHeaders is the whole point of the allowlist:
// any other page on the internet must not be able to make authenticated
// calls with a signed-in client's tokens.
func TestUnknownOriginGetsNoCORSHeaders(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	for _, origin := range []string{
		"https://attacker.test",
		// A suffix match would let this through — it must not.
		"https://terioscoach.com.attacker.test",
		"http://terioscoach.com",
		"null",
	} {
		rec := doJSON(t, srv, http.MethodGet, "/healthz", nil, map[string]string{"Origin": origin})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q got Allow-Origin %q, want none", origin, got)
		}
		if got := rec.Header().Get("Vary"); got != "Origin" {
			t.Errorf("origin %q: Vary = %q, want Origin even on a refusal", origin, got)
		}
	}
}

// TestNoWildcardEverAppears: `*` cannot carry credentials, and reflecting
// whatever arrived is the same as having no policy.
func TestNoWildcardEverAppears(t *testing.T) {
	for _, origins := range [][]string{nil, {}, {appOrigin}} {
		srv := newSecuredServer(origins, true)
		rec := doJSON(t, srv, http.MethodGet, "/healthz", nil, map[string]string{
			"Origin": "https://anything.test",
		})
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Fatalf("Allow-Origin = %q — a wildcard cannot be combined with credentials", got)
		}
	}
}

// TestPreflightIsAnsweredForAnAllowedOrigin.
func TestPreflightIsAnsweredForAnAllowedOrigin(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	rec := doJSON(t, srv, http.MethodOptions, "/v1/services", nil, map[string]string{
		"Origin":                        appOrigin,
		"Access-Control-Request-Method": "POST",
	})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("preflight allows no headers, so Authorization would be blocked")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Error("preflight has no max-age, so every request pays the extra round trip")
	}
}

// TestPreflightIsRefusedForAnUnknownOrigin.
func TestPreflightIsRefusedForAnUnknownOrigin(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	rec := doJSON(t, srv, http.MethodOptions, "/v1/services", nil, map[string]string{
		"Origin":                        "https://attacker.test",
		"Access-Control-Request-Method": "POST",
	})

	if rec.Code != http.StatusForbidden {
		t.Errorf("preflight status = %d, want 403", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q on a refused preflight, want none", got)
	}
}

// TestSameOriginCallsNeedNoCORS: a server-to-server call carries no Origin
// and must not be disturbed.
func TestSameOriginCallsNeedNoCORS(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	rec := doJSON(t, srv, http.MethodGet, "/healthz", nil, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q on a call with no Origin, want none", got)
	}
}

// TestSecurityHeadersOnEveryResponse.
func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin}, true)

	for _, path := range []string{"/healthz", "/readyz", "/v1/nope"} {
		rec := doJSON(t, srv, http.MethodGet, path, nil, nil)

		want := map[string]string{
			// This API serves no HTML and no scripts; nothing in a
			// response should ever be rendered or executed.
			"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
			"Referrer-Policy":         "no-referrer",
			// Bearer tokens and client records must not sit in a shared
			// proxy cache.
			"Cache-Control": "no-store",
		}
		for header, value := range want {
			if got := rec.Header().Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", path, header, got, value)
			}
		}
		if got := rec.Header().Get("Permissions-Policy"); got == "" {
			t.Errorf("%s: no Permissions-Policy", path)
		}
	}
}

// TestHSTSOnlyInProduction: sending it from a local http:// dev server
// would poison the browser for localhost.
func TestHSTSOnlyInProduction(t *testing.T) {
	production := doJSON(t, newSecuredServer([]string{appOrigin}, true), http.MethodGet, "/healthz", nil, nil)
	if got := production.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("no HSTS in production, want it set")
	}

	development := doJSON(t, newSecuredServer([]string{appOrigin}, false), http.MethodGet, "/healthz", nil, nil)
	if got := development.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS = %q in development, want none", got)
	}
}

// TestOriginsAreMatchedIgnoringATrailingSlash: an env var with a trailing
// slash is a configuration typo, not a different origin.
func TestOriginsAreMatchedIgnoringATrailingSlash(t *testing.T) {
	srv := newSecuredServer([]string{appOrigin + "/"}, true)

	rec := doJSON(t, srv, http.MethodGet, "/healthz", nil, map[string]string{"Origin": appOrigin})

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != appOrigin {
		t.Errorf("Allow-Origin = %q, want the origin matched despite the configured slash", got)
	}
}

// TestHeadersSurviveAnErrorResponse: a 401 carries client-relevant
// information too, and must not be cached or framed either.
func TestHeadersSurviveAnErrorResponse(t *testing.T) {
	srv := NewServerWith(
		[]BuildOption{WithCORS(CORSPolicy{AllowedOrigins: []string{appOrigin}}), WithProduction(true)},
		WithClients(nil, nil),
	)

	rec := doJSON(t, srv, http.MethodGet, "/v1/clients", nil, map[string]string{"Origin": appOrigin})

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q on an error response, want no-store", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != appOrigin {
		t.Errorf("Allow-Origin = %q on an error response, want it still set", got)
	}
}
