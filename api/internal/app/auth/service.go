// Package auth is the application service for the identity slice. It
// implements the inbound ports.AuthService port purely against outbound
// ports — no framework, driver, or transport imports.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the auth use cases over outbound ports.
type Service struct {
	users      ports.UserRepository
	sessions   ports.RefreshTokenRepository
	hasher     ports.PasswordHasher
	tokens     ports.TokenIssuer
	refreshTTL time.Duration
	now        func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.AuthService = (*Service)(nil)

// NewService wires the use cases to their outbound ports.
func NewService(
	users ports.UserRepository,
	sessions ports.RefreshTokenRepository,
	hasher ports.PasswordHasher,
	tokens ports.TokenIssuer,
	refreshTTL time.Duration,
) *Service {
	return &Service{
		users:      users,
		sessions:   sessions,
		hasher:     hasher,
		tokens:     tokens,
		refreshTTL: refreshTTL,
		now:        func() time.Time { return time.Now().UTC() },
	}
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
func (s *Service) Login(ctx context.Context, email, password string) (ports.AuthResult, error) {
	user, err := s.users.FindByEmail(ctx, identity.NormalizeEmail(email))
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return ports.AuthResult{}, identity.ErrInvalidCredentials
		}
		return ports.AuthResult{}, err
	}
	ok, err := s.hasher.Verify(user.PasswordHash, password)
	if err != nil {
		return ports.AuthResult{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		return ports.AuthResult{}, identity.ErrInvalidCredentials
	}
	return s.openSession(ctx, user)
}

// Refresh rotates a session: validate the presented token, revoke it, then
// issue a fresh pair. A revoked or unknown token is ErrTokenInvalid; a
// well-formed but stale one is ErrTokenExpired.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (ports.AuthResult, error) {
	hash := s.tokens.HashRefreshToken(refreshToken)
	session, err := s.sessions.FindByHash(ctx, hash)
	if err != nil {
		return ports.AuthResult{}, err
	}
	if session.Revoked {
		return ports.AuthResult{}, identity.ErrTokenInvalid
	}
	if !s.now().Before(session.ExpiresAt) {
		return ports.AuthResult{}, identity.ErrTokenExpired
	}
	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return ports.AuthResult{}, err
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
