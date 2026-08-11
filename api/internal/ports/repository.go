package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// UserRepository is the outbound port for account persistence.
type UserRepository interface {
	// Create persists a new user, assigning its ID. A duplicate email
	// returns identity.ErrEmailTaken.
	Create(ctx context.Context, user identity.User) (identity.User, error)
	// FindByEmail looks up by normalized email; misses return
	// identity.ErrUserNotFound.
	FindByEmail(ctx context.Context, email string) (identity.User, error)
	// FindByID looks up by ID; misses return identity.ErrUserNotFound.
	FindByID(ctx context.Context, id string) (identity.User, error)
}

// RefreshTokenRepository is the outbound port for refresh-session
// persistence. Tokens are stored by hash only.
type RefreshTokenRepository interface {
	// Store persists a new session keyed by its token hash.
	Store(ctx context.Context, token identity.RefreshToken) error
	// FindByHash looks up a session; misses return identity.ErrTokenInvalid.
	FindByHash(ctx context.Context, tokenHash string) (identity.RefreshToken, error)
	// Revoke marks a session revoked. Missing sessions are not an error.
	Revoke(ctx context.Context, tokenHash string) error
}
