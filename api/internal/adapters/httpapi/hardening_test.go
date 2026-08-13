package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// newHardenedServer wires the auth routes with lockout on and a small rate
// limit, so both protections are observable in a handful of requests.
func newHardenedServer(limit int, policy identity.LockoutPolicy) *Server {
	svc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		portstest.NewFakeTokenIssuer(15*time.Minute),
		30*24*time.Hour,
		auth.WithLockout(portstest.NewFakeLoginAttemptStore(), policy),
	)
	return NewServer(WithAuth(svc, WithAuthRateLimit(RateLimitPolicy{
		Limit:  limit,
		Window: time.Minute,
	})))
}

// forwardedFrom builds the headers a request from one client address
// carries through the production proxy: the trusted hop appends the real
// address last.
func forwardedFrom(ip string) map[string]string {
	return map[string]string{"X-Forwarded-For": ip}
}

// TestAuthRateLimitReturns429: past the cap the credential routes answer
// 429 rate_limited with a Retry-After header.
func TestAuthRateLimitReturns429(t *testing.T) {
	const limit = 4
	srv := newHardenedServer(limit, identity.DefaultLockoutPolicy())
	body := map[string]any{"email": "ama@example.com", "password": "wrong-password-here"}

	for i := 1; i <= limit; i++ {
		rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("203.0.113.7"))
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d was limited, want the first %d to pass", i, limit)
		}
	}

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("203.0.113.7"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request %d status = %d, want 429", limit+1, rec.Code)
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "rate_limited" {
		t.Errorf("code = %q, want rate_limited", errRes.Error.Code)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 {
		t.Errorf("Retry-After = %q, want a positive whole number of seconds", rec.Header().Get("Retry-After"))
	}
}

// TestAuthRateLimitIsPerClient: one noisy address does not lock out the
// rest of the internet.
func TestAuthRateLimitIsPerClient(t *testing.T) {
	const limit = 2
	srv := newHardenedServer(limit, identity.DefaultLockoutPolicy())
	body := map[string]any{"email": "ama@example.com", "password": "wrong-password-here"}

	for i := 0; i <= limit; i++ {
		doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("203.0.113.7"))
	}
	if rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("203.0.113.7")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("noisy client status = %d, want 429", rec.Code)
	}

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("198.51.100.4"))
	if rec.Code == http.StatusTooManyRequests {
		t.Error("a different client was limited, want the cap scoped per address")
	}
}

// TestForgedForwardedForCannotResetTheLimit: prepending a fake
// X-Forwarded-For entry — the classic per-IP-limit bypass — must not buy a
// fresh bucket, because only the entry the trusted proxy appended counts.
func TestForgedForwardedForCannotResetTheLimit(t *testing.T) {
	const limit = 2
	srv := newHardenedServer(limit, identity.DefaultLockoutPolicy())
	body := map[string]any{"email": "ama@example.com", "password": "wrong-password-here"}

	for i := 0; i <= limit; i++ {
		doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, forwardedFrom("203.0.113.7"))
	}

	// The attacker sends its own X-Forwarded-For; the proxy appends the
	// address it actually saw, so the real one is still last.
	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/login", body, map[string]string{
		"X-Forwarded-For": "1.2.3.4, 203.0.113.7",
	})
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 — a forged leading entry must not mint a new rate-limit identity", rec.Code)
	}
}

// TestAuthMeIsNotRateLimited: the authenticated read is ordinary app
// traffic, not a credential-guessing surface.
func TestAuthMeIsNotRateLimited(t *testing.T) {
	const limit = 2
	srv := newHardenedServer(limit, identity.DefaultLockoutPolicy())

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", map[string]any{
		"email": "ama@example.com", "name": "Ama", "password": "a long enough password",
	}, forwardedFrom("203.0.113.7"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res authTestResponse
	decodeBody(t, rec, &res)

	for i := 0; i < limit+3; i++ {
		rec := doJSON(t, srv, http.MethodGet, "/v1/auth/me", nil, map[string]string{
			"Authorization":   "Bearer " + res.AccessToken,
			"X-Forwarded-For": "203.0.113.7",
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("me request %d status = %d, want 200 (body %s)", i+1, rec.Code, rec.Body.String())
		}
	}
}

// TestLoginLockoutReturns429WithoutLeakingExistence: the lockout answers
// too_many_attempts identically for a real and an unknown email.
func TestLoginLockoutReturns429WithoutLeakingExistence(t *testing.T) {
	policy := identity.LockoutPolicy{MaxAttempts: 3, Window: 10 * time.Minute, Cooldown: 5 * time.Minute}
	// Rate limit set high enough that the lockout, not the limiter, is what
	// trips — otherwise this test would prove the wrong control.
	srv := newHardenedServer(100, policy)

	rec := doJSON(t, srv, http.MethodPost, "/v1/auth/register", map[string]any{
		"email": "real@example.com", "name": "Ama", "password": "a long enough password",
	}, forwardedFrom("203.0.113.7"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", rec.Code, rec.Body.String())
	}

	lockedResponse := func(email string) (int, errorBody, string) {
		var last *httptest.ResponseRecorder
		for i := 0; i < policy.MaxAttempts; i++ {
			last = doJSON(t, srv, http.MethodPost, "/v1/auth/login", map[string]any{
				"email": email, "password": "wrong-password-here",
			}, forwardedFrom("203.0.113.7"))
		}
		var errRes errorBody
		decodeBody(t, last, &errRes)
		return last.Code, errRes, last.Header().Get("Retry-After")
	}

	realStatus, realErr, realRetry := lockedResponse("real@example.com")
	ghostStatus, ghostErr, ghostRetry := lockedResponse("ghost@example.com")

	if realStatus != http.StatusTooManyRequests {
		t.Fatalf("real account status = %d, want 429", realStatus)
	}
	if realErr.Error.Code != "too_many_attempts" {
		t.Errorf("code = %q, want too_many_attempts", realErr.Error.Code)
	}
	if realRetry == "" {
		t.Error("Retry-After missing on a lockout response")
	}
	if ghostStatus != realStatus || ghostErr.Error.Code != realErr.Error.Code || ghostErr.Error.Message != realErr.Error.Message {
		t.Errorf("unknown email answered %d/%+v, real answered %d/%+v — the difference leaks account existence",
			ghostStatus, ghostErr.Error, realStatus, realErr.Error)
	}
	if (ghostRetry == "") != (realRetry == "") {
		t.Error("Retry-After present for one email and not the other — that difference leaks account existence")
	}
}

// TestRealIPReadsTheTrustedHop: the resolver believes the entry the
// configured number of proxies from the right, and nothing else.
func TestRealIPReadsTheTrustedHop(t *testing.T) {
	for name, tc := range map[string]struct {
		hops   int
		header string
		want   string
	}{
		"single trusted hop":      {1, "1.2.3.4, 203.0.113.7", "203.0.113.7"},
		"two trusted hops":        {2, "1.2.3.4, 203.0.113.7, 10.0.0.1", "203.0.113.7"},
		"no header":               {1, "", ""},
		"proxies disabled":        {0, "203.0.113.7", ""},
		"shorter than hop count":  {3, "203.0.113.7", "203.0.113.7"},
		"non-address is rejected": {1, "not-an-ip", ""},
	} {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("X-Forwarded-For", tc.header)
			}
			if got := forwardedFor(req, tc.hops); got != tc.want {
				t.Errorf("forwardedFor = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRateLimiterWindowRolls: the cap is per window, not for all time.
func TestRateLimiterWindowRolls(t *testing.T) {
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(RateLimitPolicy{Limit: 2, Window: time.Minute})
	limiter.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if allowed, _ := limiter.allow("client"); !allowed {
			t.Fatalf("request %d blocked, want the first 2 allowed", i+1)
		}
	}
	allowed, retryAfter := limiter.allow("client")
	if allowed {
		t.Fatal("request 3 allowed, want it blocked")
	}
	if retryAfter != time.Minute {
		t.Errorf("retryAfter = %v, want the rest of the window", retryAfter)
	}

	now = now.Add(time.Minute)
	if allowed, _ := limiter.allow("client"); !allowed {
		t.Error("blocked after the window rolled, want a fresh allowance")
	}
}

// TestRateLimiterSweepsStaleKeys: the counter map must not grow forever
// under a stream of one-shot callers.
func TestRateLimiterSweepsStaleKeys(t *testing.T) {
	now := time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(RateLimitPolicy{Limit: 5, Window: time.Minute})
	limiter.now = func() time.Time { return now }

	for i := 0; i < 50; i++ {
		limiter.allow("client-" + strconv.Itoa(i))
	}
	if len(limiter.windows) != 50 {
		t.Fatalf("windows = %d, want 50 live keys", len(limiter.windows))
	}

	now = now.Add(2 * time.Minute)
	limiter.allow("fresh-client")
	if len(limiter.windows) != 1 {
		t.Errorf("windows = %d after the sweep, want only the fresh key", len(limiter.windows))
	}
}
