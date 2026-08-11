package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithAuth mounts the /v1/auth routes backed by the auth port. A nil
// service keeps the routes mounted but answering 503, so a dev process
// without MongoDB still boots and the gap is explicit rather than 404.
func WithAuth(svc ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/auth", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleAuthUnavailable)
				return
			}
			h := &authHandler{svc: svc}
			r.Post("/register", h.register)
			r.Post("/login", h.login)
			r.Post("/refresh", h.refresh)
			r.Post("/logout", h.logout)
			r.With(RequireAuth(svc)).Get("/me", h.me)
		})
	}
}

// handleAuthUnavailable answers every auth route when the database — and
// therefore the auth service — is not configured.
func handleAuthUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "authentication is unavailable: database not connected")
}

type authHandler struct {
	svc ports.AuthService
}

// authResponse is the token-pair envelope for register/login/refresh.
type authResponse struct {
	User               userBody  `json:"user"`
	AccessToken        string    `json:"accessToken"`
	AccessTokenExpires time.Time `json:"accessTokenExpiresAt"`
	RefreshToken       string    `json:"refreshToken"`
}

// userBody is the contract identity shape: {id, email, role, name}.
type userBody struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
	Name  string `json:"name"`
}

func newUserBody(u identity.User) userBody {
	return userBody{ID: u.ID, Email: u.Email, Role: string(u.Role), Name: u.Name}
}

func newAuthResponse(res ports.AuthResult) authResponse {
	return authResponse{
		User:               newUserBody(res.User),
		AccessToken:        res.AccessToken,
		AccessTokenExpires: res.AccessTokenExpiry,
		RefreshToken:       res.RefreshToken,
	}
}

// register handles POST /v1/auth/register. Only clients self-register;
// the practitioner account is provisioned by the seed tool.
func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Register(r.Context(), ports.RegisterInput{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, newAuthResponse(res))
}

// login handles POST /v1/auth/login.
func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(res))
}

// refresh handles POST /v1/auth/refresh with rotation: the presented token
// is revoked and a fresh pair returned.
func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(res))
}

// logout handles POST /v1/auth/logout. It is idempotent: revoking an
// unknown token still answers 204.
func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// me handles GET /v1/auth/me for an authenticated principal, loading the
// full account behind the token's identity.
func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	user, err := h.svc.CurrentUser(r.Context(), id)
	if err != nil {
		// A valid token whose account has disappeared can no longer act.
		writeError(w, http.StatusUnauthorized, "unauthorized", "account no longer exists")
		return
	}
	writeJSON(w, http.StatusOK, map[string]userBody{"user": newUserBody(user)})
}

// decodeJSON decodes a request body, writing a 400 on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "request body must be valid JSON")
		return false
	}
	return true
}
