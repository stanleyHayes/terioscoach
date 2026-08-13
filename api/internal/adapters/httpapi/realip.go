package httpapi

import (
	"net"
	"net/http"
	"strings"
)

// DefaultTrustedProxyHops is how many reverse proxies sit in front of the
// API in production: one, Render's edge.
const DefaultTrustedProxyHops = 1

// RealIP resolves the caller's address from X-Forwarded-For and rewrites
// RemoteAddr with it, so downstream middleware (rate limiting, logging)
// sees the client rather than the load balancer.
//
// It reads the header from the RIGHT, not the left. Each proxy appends the
// address it received the request from, so with `hops` trusted proxies in
// front, the entry `hops` from the end is the address the outermost trusted
// proxy actually observed. Everything to its left was supplied by the
// caller and is forgeable: a client that sends its own X-Forwarded-For
// simply prepends to the list. Trusting the leftmost entry — the common
// default, including chi's own RealIP — would let anyone mint a fresh
// rate-limit identity per request by varying one header, which would make
// the per-IP limiter decorative.
//
// hops < 1 disables header parsing entirely (direct exposure, no proxy).
func RealIP(hops int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ip := forwardedFor(r, hops); ip != "" {
				r.RemoteAddr = ip
			}
			next.ServeHTTP(w, r)
		})
	}
}

// forwardedFor extracts the client address the outermost trusted proxy saw,
// or "" when there is nothing trustworthy to read.
func forwardedFor(r *http.Request, hops int) string {
	if hops < 1 {
		return ""
	}
	header := r.Header.Get("X-Forwarded-For")
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ",")

	// Index `hops` from the right. A list shorter than the configured hop
	// count means fewer proxies appended than expected (a direct call, or a
	// misconfiguration); the leftmost entry is then the safest available
	// answer and is no worse than not parsing at all.
	index := len(parts) - hops
	if index < 0 {
		index = 0
	}

	candidate := strings.TrimSpace(parts[index])
	if net.ParseIP(candidate) == nil {
		return ""
	}
	return candidate
}
