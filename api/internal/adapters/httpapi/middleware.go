package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// identityKey is the context key RequireAuth stores the principal under.
type identityKey struct{}

// IdentityFromContext returns the principal RequireAuth attached, if any.
func IdentityFromContext(ctx context.Context) (identity.Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(identity.Identity)
	return id, ok
}

// RequireAuth parses the Bearer access token, authenticates it through the
// auth port, and puts the resulting identity in the request context.
func RequireAuth(svc ports.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			id, err := svc.Authenticate(r.Context(), token)
			if err != nil {
				writeDomainError(w, err)
				return
			}
			ctx := context.WithValue(r.Context(), identityKey{}, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole guards a route by RBAC role. It must run after RequireAuth.
func RequireRole(role identity.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := IdentityFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			if id.Role != role {
				writeError(w, http.StatusForbidden, "forbidden", "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerToken extracts the token from an `Authorization: Bearer <token>`
// header.
func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}
