// Package ports defines the hexagonal boundaries of the API: inbound
// ports are the use cases the application exposes, outbound ports are the
// capabilities adapters must provide. Neither side knows the other's
// implementation.
package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// RegisterInput carries what a client self-registers with. Practitioners
// are never self-registered — they are provisioned via the seed tool.
type RegisterInput struct {
	Email    string
	Name     string
	Password string
}

// AuthResult is a successful authentication: the full account plus fresh
// credentials. The refresh token is opaque and shown to the client once.
type AuthResult struct {
	User              identity.User
	AccessToken       string
	AccessTokenExpiry time.Time
	RefreshToken      string
}

type MFAEnrollment struct {
	Secret     string
	OTPAuthURL string
}

// AuthService is the inbound port for authentication and authorization.
type AuthService interface {
	// Register creates a client account and returns a first session.
	Register(ctx context.Context, in RegisterInput) (AuthResult, error)
	// Login verifies credentials. Failures are uniformly
	// identity.ErrInvalidCredentials to prevent user enumeration.
	Login(ctx context.Context, email, password string) (AuthResult, error)
	LoginWithMFA(ctx context.Context, email, password, code string) (AuthResult, error)
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, password string) error
	// Refresh rotates a session: the presented refresh token is revoked and
	// a new token pair is issued. Reuse of a rotated token fails.
	Refresh(ctx context.Context, refreshToken string) (AuthResult, error)
	// Logout revokes the presented refresh token. It is idempotent.
	Logout(ctx context.Context, refreshToken string) error
	// Authenticate validates an access token and returns its principal.
	Authenticate(ctx context.Context, accessToken string) (identity.Identity, error)
	// CurrentUser loads the full account behind an authenticated identity.
	// Misses return identity.ErrUserNotFound.
	CurrentUser(ctx context.Context, id identity.Identity) (identity.User, error)
	UpdateProfile(ctx context.Context, id identity.Identity, name string) (identity.User, error)
	ChangePassword(ctx context.Context, id identity.Identity, currentPassword, newPassword string) error
	BeginMFA(ctx context.Context, id identity.Identity) (MFAEnrollment, error)
	ConfirmMFA(ctx context.Context, id identity.Identity, code string) error
	DisableMFA(ctx context.Context, id identity.Identity, code string) error
}

// SecretProtector encrypts reversible account secrets before persistence.
type SecretProtector interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// TOTPProvider owns authenticator-compatible secret generation and validation.
type TOTPProvider interface {
	Generate(issuer, account string) (secret, otpAuthURL string, err error)
	Validate(code, secret string, now time.Time) bool
}
