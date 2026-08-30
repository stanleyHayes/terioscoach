package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// preflightMaxAge is how long a browser may cache a preflight result. Ten
// minutes keeps the extra round trip off most requests without pinning a
// stale CORS policy for long after a deploy changes it.
const preflightMaxAge = 10 * time.Minute

// CORSPolicy is the cross-origin rule for the API.
//
// The two apps are served from Vercel and the API from Render, so every
// browser call to this API is cross-origin and credentialed. That makes the
// allowlist load-bearing rather than boilerplate: it is the only thing
// stopping any page on the internet from making authenticated calls with a
// signed-in client's tokens.
type CORSPolicy struct {
	// AllowedOrigins is an exact-match allowlist. Empty means no browser
	// origin is allowed — which is the correct default for a
	// misconfigured deployment, not a reason to fall back to "*".
	AllowedOrigins []string
}

// CORS answers preflights and stamps the response headers.
//
// There is deliberately no wildcard support. `Access-Control-Allow-Origin:
// *` cannot be combined with credentials, and the version people reach for
// instead — reflecting whatever Origin arrived — is the same as having no
// policy at all. Origins are compared exactly; a suffix match would let
// `terioscoach.com.attacker.test` through.
func CORS(policy CORSPolicy) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(policy.AllowedOrigins))
	for _, origin := range policy.AllowedOrigins {
		if origin = strings.TrimSpace(strings.TrimRight(origin, "/")); origin != "" {
			allowed[origin] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// A same-origin or server-to-server call carries no Origin and
			// needs no CORS headers at all.
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Vary matters even on a refusal: a cache must not serve one
			// origin's response to another.
			w.Header().Add("Vary", "Origin")

			if !allowed[strings.TrimRight(origin, "/")] {
				// An unknown origin gets no CORS headers, so the browser
				// blocks it. A preflight ends here; a real request is
				// still refused browser-side, and answering it normally
				// keeps this from becoming an origin-probing oracle.
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(int(preflightMaxAge.Seconds())))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders sets the response headers that harden every reply.
//
// This is a JSON API, not a site: it serves no HTML and no scripts. The
// headers are chosen accordingly — a content policy that forbids everything
// (nothing legitimate is ever loaded from an API response), nosniff so a
// JSON body cannot be coerced into being treated as script, and a frame
// denial so no response can be embedded. HSTS is included because the API
// is HTTPS-only in production and a downgrade would expose bearer tokens.
func SecurityHeaders(production bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()

			// Nothing this API returns should ever be rendered, framed, or
			// executed. default-src 'none' says exactly that.
			header.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
			header.Set("X-Content-Type-Options", "nosniff")
			header.Set("X-Frame-Options", "DENY")
			// No referrer at all: API paths carry ids that are nobody
			// else's business.
			header.Set("Referrer-Policy", "no-referrer")
			// The API needs none of these; denying them costs nothing and
			// removes them from a compromised response's reach.
			header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			// Bearer tokens and client records must not be cached by any
			// shared proxy on the way back.
			header.Set("Cache-Control", "no-store")

			if production {
				// Only over TLS, and only in production: sending HSTS from
				// a local http:// dev server would poison the browser for
				// localhost.
				header.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			next.ServeHTTP(w, r)
		})
	}
}
