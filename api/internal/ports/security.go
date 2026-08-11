package ports

import (
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// PasswordHasher is the outbound port for password hashing. Implementations
// must be memory-hard and compare in constant time.
type PasswordHasher interface {
	// Hash encodes a password into a self-describing hash string.
	Hash(password string) (string, error)
	// Verify reports whether the password matches the encoded hash. A
	// malformed encoded hash is an error; a mismatch is (false, nil).
	Verify(encodedHash, password string) (bool, error)
}

// TokenIssuer is the outbound port for credential issuance. Access tokens
// are signed JWTs; refresh tokens are opaque random values persisted by
// hash so the store never holds a usable token.
type TokenIssuer interface {
	// IssueAccessToken signs a short-lived token carrying the identity.
	IssueAccessToken(id identity.Identity) (token string, expiresAt time.Time, err error)
	// VerifyAccessToken validates a token and returns its identity,
	// mapping failures to identity.ErrTokenExpired / ErrTokenInvalid.
	VerifyAccessToken(token string) (identity.Identity, error)
	// NewRefreshToken returns a fresh opaque token plus the hash to persist.
	NewRefreshToken() (plain string, hash string, err error)
	// HashRefreshToken hashes a presented opaque token for lookup.
	HashRefreshToken(plain string) string
}
