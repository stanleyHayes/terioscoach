// Package identity is the domain core for accounts, roles, and sessions.
// It imports nothing outside the standard library — no frameworks, no drivers.
package identity

import (
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

// Role is the RBAC role an account holds. The platform has a single
// practitioner, but the role is modeled explicitly so authorization
// decisions never special-case "the one admin".
type Role string

const (
	RoleClient       Role = "client"
	RolePractitioner Role = "practitioner"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	return r == RoleClient || r == RolePractitioner
}

// MinPasswordLength is the domain policy for acceptable passwords.
const MinPasswordLength = 12

// User is an account. PasswordHash is always an encoded Argon2id hash —
// the domain never sees a plaintext password beyond validation.
type User struct {
	ID                     string
	Email                  string
	PasswordHash           string
	Role                   Role
	Name                   string
	CreatedAt              time.Time
	PasswordResetTokenHash string
	PasswordResetExpiresAt time.Time
}

// Identity is the authenticated principal carried through requests.
// It deliberately holds only role + id: client data isolation starts here,
// and nothing else about the account leaks into authorization decisions.
type Identity struct {
	UserID string
	Role   Role
}

// Identity returns the principal for this user.
func (u User) Identity() Identity {
	return Identity{UserID: u.ID, Role: u.Role}
}

// RefreshToken is the persisted side of an opaque refresh session. The
// plaintext token is only ever held by the client; the store keeps its
// SHA-256 hash so a database leak does not leak usable sessions.
type RefreshToken struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

// NewUser validates registration input and builds a User. The password must
// already be hashed by an outbound PasswordHasher port; the plaintext is
// validated separately via ValidatePassword before hashing.
func NewUser(email, name, passwordHash string, role Role, now time.Time) (User, error) {
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return User{}, err
	}
	if !role.Valid() {
		return User{}, ErrInvalidRole
	}
	if passwordHash == "" {
		return User{}, ErrPasswordHashRequired
	}
	return User{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         role,
		Name:         strings.TrimSpace(name),
		CreatedAt:    now.UTC(),
	}, nil
}

// NormalizeEmail canonicalizes an email for storage and lookup.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail enforces a single plain address (no display name).
func ValidateEmail(email string) error {
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return ErrInvalidEmail
	}
	return nil
}

// ValidatePassword enforces the minimum-length policy on plaintext input.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}
