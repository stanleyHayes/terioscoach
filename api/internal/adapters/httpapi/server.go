// Package httpapi is the inbound HTTP adapter. It depends on ports only;
// it never talks to MongoDB, Paystack, Resend, or Cloudinary directly.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server wires the HTTP transport to the application ports.
type Server struct {
	Router    *chi.Mux
	readiness func(ctx context.Context) error
}

// Option customizes a Server.
type Option func(*Server)

// serverConfig holds settings that must be known before the router's own
// middleware stack is built, so they cannot be plain Options (those run
// after NewServer has already installed the stack).
type serverConfig struct {
	trustedProxyHops int
	cors             CORSPolicy
	production       bool
}

// BuildOption customizes the router stack itself.
type BuildOption func(*serverConfig)

// WithTrustedProxyHops sets how many reverse proxies sit in front of the
// API, which decides how X-Forwarded-For is read (see RealIP). Zero means
// the API is exposed directly and the header is ignored.
func WithTrustedProxyHops(hops int) BuildOption {
	return func(c *serverConfig) { c.trustedProxyHops = hops }
}

// WithCORS sets the cross-origin allowlist. Both apps are served from a
// different origin than the API, so this is what makes the browser calls
// work — and what stops every other page on the internet making them.
func WithCORS(policy CORSPolicy) BuildOption {
	return func(c *serverConfig) { c.cors = policy }
}

// WithProduction turns on the headers that only make sense over TLS.
func WithProduction(production bool) BuildOption {
	return func(c *serverConfig) { c.production = production }
}

// WithReadiness registers a dependency check (e.g. MongoDB ping) that
// /readyz must pass before the process reports ready.
func WithReadiness(fn func(ctx context.Context) error) Option {
	return func(s *Server) {
		s.readiness = fn
	}
}

// NewServer builds the router. Build options configure the middleware
// stack; the remaining options mount route groups onto it.
func NewServer(opts ...Option) *Server {
	return NewServerWith(nil, opts...)
}

// NewServerWith is NewServer with explicit control over the middleware
// stack. The composition root uses it; tests mostly use NewServer.
func NewServerWith(build []BuildOption, opts ...Option) *Server {
	cfg := serverConfig{trustedProxyHops: DefaultTrustedProxyHops}
	for _, opt := range build {
		opt(&cfg)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// Our own RealIP, not chi's: chi reads the leftmost X-Forwarded-For
	// entry, which the caller controls. See realip.go.
	r.Use(RealIP(cfg.trustedProxyHops))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Security headers before CORS so even a refused preflight carries
	// them, and both before any route.
	r.Use(SecurityHeaders(cfg.production))
	r.Use(CORS(cfg.cors))

	s := &Server{Router: r}
	for _, opt := range opts {
		opt(s)
	}

	r.Get("/healthz", handleHealth)
	r.Get("/readyz", s.handleReady)

	return s
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady reports dependency readiness. Without a registered check the
// process itself is the only signal; with one, its failure yields 503.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if s.readiness != nil {
		if err := s.readiness(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
