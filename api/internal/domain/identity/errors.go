package identity

import (
	"errors"
	"time"
)

// Domain errors for the identity slice. Adapters and the HTTP layer map
// these to storage results and status codes via errors.Is.
var (
	// ErrInvalidCredentials is the uniform login failure — it never reveals
	// whether the email exists (no user enumeration).
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrEmailTaken signals a unique-email conflict at registration.
	ErrEmailTaken = errors.New("email already registered")
	// ErrUserNotFound means no account matches the lookup key.
	ErrUserNotFound = errors.New("user not found")
	// ErrTokenExpired means a presented token was well-formed but stale.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid means a presented token is malformed, unknown,
	// revoked, or otherwise unusable.
	ErrTokenInvalid = errors.New("token invalid")
	// ErrInvalidEmail means the email failed domain validation.
	ErrInvalidEmail = errors.New("invalid email address")
	// ErrPasswordTooShort means the password failed the length policy.
	ErrPasswordTooShort = errors.New("password must be at least 12 characters")
	// ErrInvalidRole means a role outside the known set was supplied.
	ErrInvalidRole = errors.New("invalid role")
	// ErrPasswordHashRequired guards against storing unhashed credentials.
	ErrPasswordHashRequired = errors.New("password hash required")
	// ErrTooManyAttempts means the brute-force lockout is holding this
	// login identifier. It is returned for locked identifiers whether or
	// not an account exists, so it cannot be used to enumerate accounts.
	ErrTooManyAttempts      = errors.New("too many login attempts")
	ErrPasswordResetInvalid = errors.New("password reset link is invalid or expired")
	ErrMFARequired          = errors.New("multi-factor authentication code required")
	ErrMFAInvalid           = errors.New("multi-factor authentication code invalid")
	ErrMFANotPending        = errors.New("multi-factor authentication enrollment not pending")
	ErrAccountDisabled      = errors.New("account disabled")
	ErrLastOwner            = errors.New("the owner account cannot be changed or disabled")
	ErrInvalidPermission    = errors.New("invalid permission")
)

// RetryAfterError carries how long a caller must wait before retrying. The
// HTTP adapter reads it to set the Retry-After header; errors.Is still
// matches the wrapped sentinel, so every other layer treats it as the plain
// domain error.
type RetryAfterError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RetryAfterError) Error() string { return e.Err.Error() }
func (e *RetryAfterError) Unwrap() error { return e.Err }
