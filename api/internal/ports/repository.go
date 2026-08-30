package ports

import (
	"context"
	"time"

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
	// FindFirstByRole returns one account holding the role — used where the
	// platform's single-practitioner model makes "the practitioner" a
	// meaningful lookup. Misses return identity.ErrUserNotFound.
	FindFirstByRole(ctx context.Context, role identity.Role) (identity.User, error)
	// FindByID looks up by ID; misses return identity.ErrUserNotFound.
	FindByID(ctx context.Context, id string) (identity.User, error)
	SetPasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error
	ResetPassword(ctx context.Context, tokenHash, passwordHash string, now time.Time) (string, error)
	SetMFAPending(ctx context.Context, userID, encryptedSecret string) error
	EnableMFA(ctx context.Context, userID string) error
	DisableMFA(ctx context.Context, userID string) error
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
	// RevokeAllForUser revokes every live session of one account. It backs
	// refresh-token reuse detection: replaying a rotated token means the
	// token leaked, so the whole family dies at once (BE-02).
	RevokeAllForUser(ctx context.Context, userID string) error
}

// LoginAttemptStore is the outbound port for brute-force accounting. The
// key is the submitted login identifier (normalized email), recorded
// whether or not an account exists — see identity.LockoutPolicy.
type LoginAttemptStore interface {
	// Get returns the current history. An identifier with no failures
	// returns a zero-count record and no error.
	Get(ctx context.Context, identifier string) (identity.LoginAttempts, error)
	// Save persists the updated history, replacing any previous record.
	Save(ctx context.Context, attempts identity.LoginAttempts, retainFor time.Duration) error
	// Reset clears the history after a successful login. Missing records
	// are not an error.
	Reset(ctx context.Context, identifier string) error
}
