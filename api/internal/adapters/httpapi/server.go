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

// WithReadiness registers a dependency check (e.g. MongoDB ping) that
// /readyz must pass before the process reports ready.
func WithReadiness(fn func(ctx context.Context) error) Option {
	return func(s *Server) {
		s.readiness = fn
	}
}

func NewServer(opts ...Option) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

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
