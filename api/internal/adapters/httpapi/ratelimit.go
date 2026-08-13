package httpapi

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Rate-limit defaults for the credential routes. Twenty attempts per minute
// per source address is far above what a person types and far below what a
// password-spraying script wants. It is the first of two independent
// layers: this one is per-IP and stops the flood; the per-identifier
// lockout in the auth service (identity.LockoutPolicy) stops the patient
// attacker who spreads attempts across addresses.
const (
	DefaultAuthRateLimit  = 20
	DefaultAuthRateWindow = time.Minute
)

// RateLimitPolicy is a fixed-window request cap per client key.
type RateLimitPolicy struct {
	// Limit is the number of requests allowed inside one window.
	Limit int
	// Window is how long each counting window lasts.
	Window time.Duration
}

// DefaultAuthRateLimitPolicy is the platform default for /v1/auth.
func DefaultAuthRateLimitPolicy() RateLimitPolicy {
	return RateLimitPolicy{Limit: DefaultAuthRateLimit, Window: DefaultAuthRateWindow}
}

// normalized substitutes the defaults for zero/negative fields so a
// half-configured policy can never disable the limiter.
func (p RateLimitPolicy) normalized() RateLimitPolicy {
	if p.Limit < 1 {
		p.Limit = DefaultAuthRateLimit
	}
	if p.Window <= 0 {
		p.Window = DefaultAuthRateWindow
	}
	return p
}

// rateLimiter is a fixed-window counter keyed by client address.
//
// Scope note: the counters live in this process. The API runs as a single
// Render service, so that is the whole edge; if it is ever scaled out, each
// instance enforces the limit independently and the effective cap
// multiplies by the instance count. The per-identifier lockout is the
// backstop that does not have this property, because it is stored in
// MongoDB.
type rateLimiter struct {
	policy RateLimitPolicy
	now    func() time.Time

	mu      sync.Mutex
	windows map[string]*rateWindow
	// lastSweep bounds map growth: stale keys are dropped in batches
	// rather than by a background goroutine.
	lastSweep time.Time
}

type rateWindow struct {
	count   int
	startAt time.Time
}

func newRateLimiter(policy RateLimitPolicy) *rateLimiter {
	return &rateLimiter{
		policy:  policy.normalized(),
		now:     func() time.Time { return time.Now().UTC() },
		windows: make(map[string]*rateWindow),
	}
}

// allow records one request against key and reports whether it fits inside
// the current window. When it does not, the second return is how long the
// caller must wait for the window to roll.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	window, ok := l.windows[key]
	if !ok || now.Sub(window.startAt) >= l.policy.Window {
		l.windows[key] = &rateWindow{count: 1, startAt: now}
		return true, 0
	}
	if window.count >= l.policy.Limit {
		return false, window.startAt.Add(l.policy.Window).Sub(now)
	}
	window.count++
	return true, 0
}

// sweep drops windows that have already rolled. It runs at most once per
// window and holds the lock its caller already took.
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.policy.Window {
		return
	}
	l.lastSweep = now
	for key, window := range l.windows {
		if now.Sub(window.startAt) >= l.policy.Window {
			delete(l.windows, key)
		}
	}
}

// RateLimit caps requests per client address, answering 429 rate_limited
// with a Retry-After header once the cap is hit. Chi's RealIP middleware
// has already resolved the forwarded address by the time this runs.
func RateLimit(policy RateLimitPolicy) func(http.Handler) http.Handler {
	limiter := newRateLimiter(policy)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter := limiter.allow(clientKey(r))
			if !allowed {
				writeRetryAfter(w, retryAfter)
				writeError(w, http.StatusTooManyRequests, "rate_limited",
					"too many requests — please wait a moment and try again")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientKey identifies the caller for rate-limiting purposes: the source
// address without its ephemeral port, so one client is one key.
func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// writeRetryAfter sets the Retry-After header in whole seconds, rounding up
// so it never advertises a moment that is still too early.
func writeRetryAfter(w http.ResponseWriter, d time.Duration) {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
