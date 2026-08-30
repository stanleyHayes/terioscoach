package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// AuthOption customizes the auth route group.
type AuthOption func(*authRoutes)

// authRoutes holds the mount-time settings for the /v1/auth group.
type authRoutes struct {
	rateLimit RateLimitPolicy
}

// WithAuthRateLimit overrides the per-IP cap on the credential routes
// (BE-02). Tests use it to make the limit observable without sending the
// production number of requests.
func WithAuthRateLimit(policy RateLimitPolicy) AuthOption {
	return func(a *authRoutes) { a.rateLimit = policy }
}

// WithAuth mounts the /v1/auth routes backed by the auth port. A nil
// service keeps the routes mounted but answering 503, so a dev process
// without MongoDB still boots and the gap is explicit rather than 404.
//
// Every credential route sits behind a per-IP rate limit (BE-02). /me is
// excluded: it is a plain authenticated read whose token was already
// issued, and rate-limiting it would throttle normal app use.
func WithAuth(svc ports.AuthService, opts ...AuthOption) Option {
	cfg := authRoutes{rateLimit: DefaultAuthRateLimitPolicy()}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(s *Server) {
		s.Router.Route("/v1/auth", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleAuthUnavailable)
				return
			}
			h := &authHandler{svc: svc}
			r.Group(func(r chi.Router) {
				r.Use(RateLimit(cfg.rateLimit))
				r.Post("/register", h.register)
				r.Post("/login", h.login)
				r.Post("/refresh", h.refresh)
				r.Post("/logout", h.logout)
				r.Post("/forgot-password", h.forgotPassword)
				r.Post("/reset-password", h.resetPassword)
			})
			r.With(RequireAuth(svc)).Get("/me", h.me)
			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(svc))
				r.Post("/mfa/enroll", h.beginMFA)
				r.Post("/mfa/confirm", h.confirmMFA)
				r.Post("/mfa/disable", h.disableMFA)
			})
		})
	}
}

func (h *authHandler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ForgotPassword(r.Context(), req.Email); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ResetPassword(r.Context(), req.Token, req.Password); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Name        string   `json:"name"`
	MFAEnabled  bool     `json:"mfaEnabled"`
	RoleName    string   `json:"roleName,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

func newUserBody(u identity.User) userBody {
	permissionList := u.Permissions.List()
	permissions := make([]string, len(permissionList))
	for index, permission := range permissionList {
		permissions[index] = string(permission)
	}
	return userBody{ID: u.ID, Email: u.Email, Role: string(u.Role), Name: u.Name, MFAEnabled: u.MFAEnabled, RoleName: u.RoleName, Permissions: permissions}
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
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := h.svc.LoginWithMFA(r.Context(), req.Email, req.Password, req.Code)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newAuthResponse(res))
}

func (h *authHandler) beginMFA(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	enrollment, err := h.svc.BeginMFA(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"secret": enrollment.Secret, "otpAuthUrl": enrollment.OTPAuthURL})
}

func (h *authHandler) confirmMFA(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ConfirmMFA(r.Context(), id, req.Code); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) disableMFA(w http.ResponseWriter, r *http.Request) {
	id, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.DisableMFA(r.Context(), id, req.Code); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
