package identity

import "errors"

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
)
