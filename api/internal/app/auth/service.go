// Package auth is the application service for the identity slice. It
// implements the inbound ports.AuthService port purely against outbound
// ports — no framework, driver, or transport imports.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the auth use cases over outbound ports.
type Service struct {
	users         ports.UserRepository
	sessions      ports.RefreshTokenRepository
	hasher        ports.PasswordHasher
	tokens        ports.TokenIssuer
	refreshTTL    time.Duration
	attempts      ports.LoginAttemptStore
	lockout       identity.LockoutPolicy
	now           func() time.Time
	resetMailer   ports.Mailer
	resetURL      string
	resetAdminURL string
	resetTTL      time.Duration
	protector     ports.SecretProtector
	totp          ports.TOTPProvider
}

func WithMFA(protector ports.SecretProtector, provider ports.TOTPProvider) Option {
	return func(s *Service) { s.protector, s.totp = protector, provider }
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.AuthService = (*Service)(nil)

// Option customizes a Service.
type Option func(*Service)

// WithLockout turns on brute-force lockout (BE-02), backed by the given
// attempt store. Without it the service still authenticates correctly but
// performs no per-identifier throttling — the composition root is expected
// to wire it, and the transport-level rate limiter is a second, independent
// layer either way.
func WithLockout(store ports.LoginAttemptStore, policy identity.LockoutPolicy) Option {
	return func(s *Service) {
		s.attempts = store
		s.lockout = policy
	}
}

func WithPasswordReset(mailer ports.Mailer, portalURL, dashboardURL string, ttl time.Duration) Option {
	return func(s *Service) {
		s.resetMailer = mailer
		s.resetURL = strings.TrimRight(strings.TrimSuffix(portalURL, "/portal"), "/") + "/reset-password"
		s.resetAdminURL = strings.TrimRight(dashboardURL, "/") + "/reset-password"
		if ttl <= 0 {
			ttl = time.Hour
		}
		s.resetTTL = ttl
	}
}

// NewService wires the use cases to their outbound ports.
func NewService(
	users ports.UserRepository,
	sessions ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	tokens ports.TokenIssuer,
	refreshTTL time.Duration,
	opts ...Option,
) *Service {
	s := &Service{
		users:      users,
		sessions:   sessions,
		hasher:     hasher,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		lockout:    identity.DefaultLockoutPolicy(),
		now:        func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func resetTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	user, err := s.users.FindByEmail(ctx, identity.NormalizeEmail(email))
	if errors.Is(err, identity.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.resetMailer == nil || s.resetURL == "" {
		return fmt.Errorf("password reset email is unavailable")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate password reset token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.users.SetPasswordReset(ctx, user.ID, resetTokenHash(token), s.now().Add(s.resetTTL)); err != nil {
		return err
	}
	resetURL := s.resetURL
	if user.Role == identity.RolePractitioner && s.resetAdminURL != "" {
		resetURL = s.resetAdminURL
	}
	link := resetURL + "?token=" + token
	// The public response stays identical for every address, including when
	// the provider is briefly unavailable, so this route cannot disclose
	// which emails own accounts.
	_ = s.resetMailer.Send(ctx, ports.EmailMessage{To: user.Email, Subject: "Reset your Terios password", Text: "Use this link within one hour to reset your password: " + link, HTML: `<p>You asked to reset your Terios password.</p><p><a href="` + html.EscapeString(link) + `">Choose a new password</a></p><p>This link expires in one hour and can only be used once.</p>`})
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if strings.TrimSpace(token) == "" {
		return identity.ErrPasswordResetInvalid
	}
	if err := identity.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	userID, err := s.users.ResetPassword(ctx, resetTokenHash(token), hash, s.now())
	if err != nil {
		return err
	}
	if err := s.sessions.RevokeAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("revoke sessions after password reset: %w", err)
	}
	return nil
}

// Register creates a client account and opens its first session.
func (s *Service) Register(ctx context.Context, in ports.RegisterInput) (ports.AuthResult, error) {
	if err := identity.ValidatePassword(in.Password); err != nil {
		return ports.AuthResult{}, err
	}
	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return ports.AuthResult{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := identity.NewUser(in.Email, in.Name, hash, identity.RoleClient, s.now())
	if err != nil {
		return ports.AuthResult{}, err
	}
	user, err = s.users.Create(ctx, user)
	if err != nil {
		return ports.AuthResult{}, err
	}
	return s.openSession(ctx, user)
}

// Login verifies credentials and opens a session. Every failure mode —
// unknown email, bad password — reports identity.ErrInvalidCredentials so
// the endpoint cannot be used to enumerate accounts.
//
// Brute-force lockout (BE-02) wraps the whole check: failures are counted
// per submitted email, and once the policy trips, the identifier is refused
// with identity.ErrTooManyAttempts until the cooldown elapses. Counting
// happens for unknown emails too, so the locked answer is not an
// account-existence oracle.
func (s *Service) Login(ctx context.Context, email, password string) (ports.AuthResult, error) {
	return s.LoginWithMFA(ctx, email, password, "")
}

func (s *Service) LoginWithMFA(ctx context.Context, email, password, code string) (ports.AuthResult, error) {
	identifier := identity.NormalizeEmail(email)
	if err := s.guardLockout(ctx, identifier); err != nil {
		return ports.AuthResult{}, err
	}

	user, err := s.users.FindByEmail(ctx, identifier)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return ports.AuthResult{}, s.recordFailure(ctx, identifier)
		}
		return ports.AuthResult{}, err
	}
	ok, err := s.hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return ports.AuthResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ports.AuthResult{}, s.recordFailure(ctx, identifier)
	}
	if user.Disabled {
		return ports.AuthResult{}, identity.ErrAccountDisabled
	}
	if user.MFAEnabled {
		if strings.TrimSpace(code) == "" {
			return ports.AuthResult{}, identity.ErrMFARequired
		}
		if s.protector == nil || s.totp == nil {
			return ports.AuthResult{}, fmt.Errorf("MFA is unavailable")
		}
		secret, err := s.protector.Decrypt(user.MFASecret)
		if err != nil {
			return ports.AuthResult{}, err
		}
		if !s.totp.Validate(strings.TrimSpace(code), secret, s.now()) {
			return ports.AuthResult{}, identity.ErrMFAInvalid
		}
	}

	// A correct password clears the history — a person who finally
	// remembers their password is not left locked out by earlier typos.
	if s.attempts != nil {
		if err := s.attempts.Reset(ctx, identifier); err != nil {
			return ports.AuthResult{}, fmt.Errorf("reset login attempts: %w", err)
		}
	}
	return s.openSession(ctx, user)
}

func (s *Service) BeginMFA(ctx context.Context, id identity.Identity) (ports.MFAEnrollment, error) {
	if s.protector == nil || s.totp == nil {
		return ports.MFAEnrollment{}, fmt.Errorf("MFA is unavailable")
	}
	user, err := s.users.FindByID(ctx, id.UserID)
	if err != nil {
		return ports.MFAEnrollment{}, err
	}
	secret, uri, err := s.totp.Generate("Terios Practice", user.Email)
	if err != nil {
		return ports.MFAEnrollment{}, fmt.Errorf("generate MFA enrollment: %w", err)
	}
	encrypted, err := s.protector.Encrypt(secret)
	if err != nil {
		return ports.MFAEnrollment{}, err
	}
	if err := s.users.SetMFAPending(ctx, user.ID, encrypted); err != nil {
		return ports.MFAEnrollment{}, err
	}
	return ports.MFAEnrollment{Secret: secret, OTPAuthURL: uri}, nil
}

func (s *Service) ConfirmMFA(ctx context.Context, id identity.Identity, code string) error {
	user, err := s.users.FindByID(ctx, id.UserID)
	if err != nil {
		return err
	}
	if user.MFASecret == "" || s.protector == nil || s.totp == nil {
		return identity.ErrMFANotPending
	}
	secret, err := s.protector.Decrypt(user.MFASecret)
	if err != nil {
		return err
	}
	if !s.totp.Validate(strings.TrimSpace(code), secret, s.now()) {
		return identity.ErrMFAInvalid
	}
	return s.users.EnableMFA(ctx, user.ID)
}

func (s *Service) DisableMFA(ctx context.Context, id identity.Identity, code string) error {
	user, err := s.users.FindByID(ctx, id.UserID)
	if err != nil {
		return err
	}
	if !user.MFAEnabled || user.MFASecret == "" {
		return nil
	}
	secret, err := s.protector.Decrypt(user.MFASecret)
	if err != nil {
		return err
	}
	if !s.totp.Validate(strings.TrimSpace(code), secret, s.now()) {
		return identity.ErrMFAInvalid
	}
	if err := s.users.DisableMFA(ctx, user.ID); err != nil {
		return err
	}
	return s.sessions.RevokeAllForUser(ctx, user.ID)
}

// guardLockout refuses a login identifier that is currently locked out,
// carrying the remaining cooldown so the transport can set Retry-After.
func (s *Service) guardLockout(ctx context.Context, identifier string) error {
	if s.attempts == nil {
		return nil
	}
	history, err := s.attempts.Get(ctx, identifier)
	if err != nil {
		return fmt.Errorf("read login attempts: %w", err)
	}
	if locked, retryAfter := s.lockout.Locked(history, s.now()); locked {
		return &identity.RetryAfterError{Err: identity.ErrTooManyAttempts, RetryAfter: retryAfter}
	}
	return nil
}

// recordFailure folds one failed login into the identifier's history and
// returns the error the caller should surface: the uniform credentials
// failure, or the lockout error once this attempt trips the policy.
func (s *Service) recordFailure(ctx context.Context, identifier string) error {
	if s.attempts == nil {
		return identity.ErrInvalidCredentials
	}
	history, err := s.attempts.Get(ctx, identifier)
	if err != nil {
		return fmt.Errorf("read login attempts: %w", err)
	}
	history.Identifier = identifier
	history = s.lockout.Record(history, s.now())
	if err := s.attempts.Save(ctx, history, s.lockout.RetentionAfter()); err != nil {
		return fmt.Errorf("record login attempt: %w", err)
	}
	if locked, retryAfter := s.lockout.Locked(history, s.now()); locked {
		return &identity.RetryAfterError{Err: identity.ErrTooManyAttempts, RetryAfter: retryAfter}
	}
	return identity.ErrInvalidCredentials
}

// Refresh rotates a session: validate the presented token, revoke it, then
// issue a fresh pair. A revoked or unknown token is ErrTokenInvalid; a
// well-formed but stale one is ErrTokenExpired.
//
// Reuse detection (BE-02): a refresh token is single-use, so seeing an
// already-rotated one means the token was captured and replayed — either
// the attacker's copy or the victim's. The holder cannot be told apart, so
// every session of that account is revoked at once and both parties are
// forced to log in again. This is the OAuth 2.0 security BCP rule for
// public clients, and it is what turns a stolen refresh token from
// indefinite access into one detected round trip.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (ports.AuthResult, error) {
	hash := s.tokens.HashRefreshToken(refreshToken)
	session, err := s.sessions.FindByHash(ctx, hash)
	if err != nil {
		return ports.AuthResult{}, err
	}
	if session.Revoked {
		if err := s.sessions.RevokeAllForUser(ctx, session.UserID); err != nil {
			return ports.AuthResult{}, fmt.Errorf("revoke reused session family: %w", err)
		}
		return ports.AuthResult{}, identity.ErrTokenInvalid
	}
	if !s.now().Before(session.ExpiresAt) {
		return ports.AuthResult{}, identity.ErrTokenExpired
	}
	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return ports.AuthResult{}, err
	}
	if user.Disabled {
		_ = s.sessions.RevokeAllForUser(ctx, user.ID)
		return ports.AuthResult{}, identity.ErrAccountDisabled
	}
	if err := s.sessions.Revoke(ctx, hash); err != nil {
		return ports.AuthResult{}, fmt.Errorf("revoke rotated session: %w", err)
	}
	return s.openSession(ctx, user)
}

// Logout revokes the presented refresh token. Unknown tokens are ignored so
// logout stays idempotent.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	hash := s.tokens.HashRefreshToken(refreshToken)
	if _, err := s.sessions.FindByHash(ctx, hash); err != nil {
		if errors.Is(err, identity.ErrTokenInvalid) {
			return nil
		}
		return err
	}
	return s.sessions.Revoke(ctx, hash)
}

// Authenticate validates an access token and returns its principal.
func (s *Service) Authenticate(_ context.Context, accessToken string) (identity.Identity, error) {
	return s.tokens.VerifyAccessToken(accessToken)
}

// CurrentUser loads the full account behind an authenticated identity.
func (s *Service) CurrentUser(ctx context.Context, id identity.Identity) (identity.User, error) {
	return s.users.FindByID(ctx, id.UserID)
}

// openSession issues a fresh access token and a rotated refresh session.
func (s *Service) openSession(ctx context.Context, user identity.User) (ports.AuthResult, error) {
	id := user.Identity()

	accessToken, accessExpiry, err := s.tokens.IssueAccessToken(id)
	if err != nil {
		return ports.AuthResult{}, fmt.Errorf("issue access token: %w", err)
	}
	plain, hash, err := s.tokens.NewRefreshToken()
	if err != nil {
		return ports.AuthResult{}, fmt.Errorf("issue refresh token: %w", err)
	}

	now := s.now()
	session := identity.RefreshToken{
		TokenHash: hash,
		UserID:    user.ID,
		ExpiresAt: now.Add(s.refreshTTL),
		CreatedAt: now,
	}
	if err := s.sessions.Store(ctx, session); err != nil {
		return ports.AuthResult{}, fmt.Errorf("store refresh session: %w", err)
	}

	return ports.AuthResult{
		User:              user,
		AccessToken:       accessToken,
		AccessTokenExpiry: accessExpiry,
		RefreshToken:      plain,
	}, nil
}
