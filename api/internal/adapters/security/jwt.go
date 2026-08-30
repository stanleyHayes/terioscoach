package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// refreshTokenBytes is the entropy of an opaque refresh token.
const refreshTokenBytes = 32

// JWTIssuer signs and verifies HS256 access tokens and mints opaque
// refresh tokens. Refresh tokens are never JWTs: they are random values
// looked up by SHA-256 hash so sessions can be revoked and rotated.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// accessClaims carries the identity — sub (user id) and role, nothing more.
type accessClaims struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// NewJWTIssuer builds an issuer for the given HMAC secret and access TTL.
func NewJWTIssuer(secret string, ttl time.Duration) (*JWTIssuer, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt: secret must not be empty")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("jwt: access TTL must be positive")
	}
	return &JWTIssuer{
		secret: []byte(secret),
		ttl:    ttl,
		now:    func() time.Time { return time.Now().UTC() },
	}, nil
}

// IssueAccessToken signs a token carrying the identity's id and role.
func (i *JWTIssuer) IssueAccessToken(id identity.Identity) (string, time.Time, error) {
	now := i.now()
	expiresAt := now.Add(i.ttl)
	claims := accessClaims{
		Role: string(id.Role),
		Permissions: func() []string {
			list := id.Permissions.List()
			values := make([]string, len(list))
			for index, permission := range list {
				values[index] = string(permission)
			}
			return values
		}(),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("jwt sign: %w", err)
	}
	return token, expiresAt, nil
}

// VerifyAccessToken validates signature and expiry, mapping failures to the
// domain errors ErrTokenExpired / ErrTokenInvalid.
func (i *JWTIssuer) VerifyAccessToken(token string) (identity.Identity, error) {
	claims := &accessClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("jwt: unexpected signing method %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return identity.Identity{}, identity.ErrTokenExpired
		}
		return identity.Identity{}, identity.ErrTokenInvalid
	}
	role := identity.Role(claims.Role)
	if claims.Subject == "" || !role.Valid() {
		return identity.Identity{}, identity.ErrTokenInvalid
	}
	permissions := make([]identity.Permission, 0, len(claims.Permissions))
	for _, raw := range claims.Permissions {
		permission := identity.Permission(raw)
		if !permission.Valid() {
			return identity.Identity{}, identity.ErrTokenInvalid
		}
		permissions = append(permissions, permission)
	}
	return identity.Identity{UserID: claims.Subject, Role: role, Permissions: identity.NewPermissionSet(permissions...)}, nil
}

// NewRefreshToken returns a fresh opaque token and the SHA-256 hash to
// persist. The plaintext is never stored server-side.
func (i *JWTIssuer) NewRefreshToken() (plain string, hash string, err error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("refresh token entropy: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, i.HashRefreshToken(plain), nil
}

// HashRefreshToken hashes a presented opaque token for session lookup.
func (i *JWTIssuer) HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
