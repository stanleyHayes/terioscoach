package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

const testRefreshTTL = 30 * 24 * time.Hour

func newTestService() (*Service, *portstest.FakeUserRepository, *portstest.FakeRefreshTokenRepository) {
	users := portstest.NewFakeUserRepository()
	sessions := portstest.NewFakeRefreshTokenRepository()
	svc := NewService(users, sessions, portstest.FakeHasher{}, portstest.NewFakeTokenIssuer(15*time.Minute), testRefreshTTL)
	return svc, users, sessions
}

func registerClient(t *testing.T, svc *Service, email, password string) ports.AuthResult {
	t.Helper()
	res, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    email,
		Name:     "Test User",
		Password: password,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	return res
}

func TestRegisterHappyPath(t *testing.T) {
	svc, _, sessions := newTestService()
	res := registerClient(t, svc, "new@example.com", "a long enough password")

	if res.User.Role != identity.RoleClient {
		t.Errorf("role = %q, want client (self-registration never yields practitioner)", res.User.Role)
	}
	if res.User.ID == "" || res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected user + both tokens")
	}
	if res.User.Email != "new@example.com" || res.User.Name != "Test User" {
		t.Errorf("user = %+v, want full account shape (email + name)", res.User)
	}
	// The stored session must be the hash of the presented token.
	stored, err := sessions.FindByHash(context.Background(), svc.tokens.HashRefreshToken(res.RefreshToken))
	if err != nil {
		t.Fatalf("session not stored: %v", err)
	}
	if stored.UserID != res.User.ID || stored.Revoked {
		t.Errorf("stored session = %+v", stored)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, _, _ := newTestService()
	registerClient(t, svc, "dupe@example.com", "a long enough password")

	_, err := svc.Register(context.Background(), ports.RegisterInput{
		Email:    "Dupe@Example.com", // normalization must still collide
		Name:     "Second",
		Password: "another long password",
	})
	if !errors.Is(err, identity.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	svc, _, _ := newTestService()
	if _, err := svc.Register(context.Background(), ports.RegisterInput{
		Email: "not-an-email", Name: "X", Password: "a long enough password",
	}); !errors.Is(err, identity.ErrInvalidEmail) {
		t.Errorf("bad email: err = %v, want ErrInvalidEmail", err)
	}
	if _, err := svc.Register(context.Background(), ports.RegisterInput{
		Email: "ok@example.com", Name: "X", Password: "short",
	}); !errors.Is(err, identity.ErrPasswordTooShort) {
		t.Errorf("short password: err = %v, want ErrPasswordTooShort", err)
	}
}

func TestLoginHappyPath(t *testing.T) {
	svc, _, _ := newTestService()
	registerClient(t, svc, "login@example.com", "the correct password")

	res, err := svc.Login(context.Background(), " Login@Example.com ", "the correct password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected token pair")
	}
}

func TestLoginUniformFailure(t *testing.T) {
	svc, _, _ := newTestService()
	registerClient(t, svc, "known@example.com", "the correct password")

	// Unknown email and wrong password must fail identically.
	if _, err := svc.Login(context.Background(), "ghost@example.com", "whatever password"); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("unknown email: err = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Login(context.Background(), "known@example.com", "the wrong password"); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("wrong password: err = %v, want ErrInvalidCredentials", err)
	}
}

func TestRefreshRotatesSession(t *testing.T) {
	svc, _, sessions := newTestService()
	first := registerClient(t, svc, "rotate@example.com", "a long enough password")

	second, err := svc.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("rotation must issue a new refresh token")
	}
	if second.User != first.User {
		t.Errorf("user changed across refresh: %+v -> %+v", first.User, second.User)
	}
	if second.User.Email != "rotate@example.com" || second.User.Name == "" {
		t.Errorf("refresh must carry the full user shape, got %+v", second.User)
	}

	// The old session must be revoked...
	old, err := sessions.FindByHash(context.Background(), svc.tokens.HashRefreshToken(first.RefreshToken))
	if err != nil {
		t.Fatalf("old session missing: %v", err)
	}
	if !old.Revoked {
		t.Error("rotated session was not revoked")
	}

	// ...and reusing it must fail (theft detection surface).
	if _, err := svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Errorf("reused token: err = %v, want ErrTokenInvalid", err)
	}
}

func TestRefreshRejectsUnknownAndExpired(t *testing.T) {
	svc, users, sessions := newTestService()
	ctx := context.Background()

	if _, err := svc.Refresh(ctx, "never-issued"); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Errorf("unknown token: err = %v, want ErrTokenInvalid", err)
	}

	// Plant an expired session directly.
	user, _ := users.Create(ctx, identity.User{
		Email: "expired@example.com", PasswordHash: "fakehash:x", Role: identity.RoleClient, Name: "X", CreatedAt: time.Now().UTC(),
	})
	plain := "expired-token"
	sessions.Store(ctx, identity.RefreshToken{
		TokenHash: svc.tokens.HashRefreshToken(plain),
		UserID:    user.ID,
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
	})
	if _, err := svc.Refresh(ctx, plain); !errors.Is(err, identity.ErrTokenExpired) {
		t.Errorf("expired token: err = %v, want ErrTokenExpired", err)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	svc, _, sessions := newTestService()
	ctx := context.Background()
	res := registerClient(t, svc, "bye@example.com", "a long enough password")

	if err := svc.Logout(ctx, res.RefreshToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	stored, _ := sessions.FindByHash(ctx, svc.tokens.HashRefreshToken(res.RefreshToken))
	if !stored.Revoked {
		t.Error("session not revoked after logout")
	}
	// Second logout of the same (now revoked) token and of an unknown token
	// both succeed silently.
	if err := svc.Logout(ctx, res.RefreshToken); err != nil {
		t.Errorf("second logout: %v", err)
	}
	if err := svc.Logout(ctx, "never-issued"); err != nil {
		t.Errorf("unknown logout: %v", err)
	}
	// And the revoked token can no longer refresh.
	if _, err := svc.Refresh(ctx, res.RefreshToken); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Errorf("refresh after logout: err = %v, want ErrTokenInvalid", err)
	}
}

func TestAuthenticate(t *testing.T) {
	svc, _, _ := newTestService()
	res := registerClient(t, svc, "me@example.com", "a long enough password")

	id, err := svc.Authenticate(context.Background(), res.AccessToken)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id != res.User.Identity() {
		t.Errorf("identity = %+v, want %+v", id, res.User.Identity())
	}
	if _, err := svc.Authenticate(context.Background(), "forged-token"); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Errorf("forged token: err = %v, want ErrTokenInvalid", err)
	}
}

func TestCurrentUser(t *testing.T) {
	svc, _, _ := newTestService()
	ctx := context.Background()
	res := registerClient(t, svc, "full@example.com", "a long enough password")

	user, err := svc.CurrentUser(ctx, res.User.Identity())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if user.Email != "full@example.com" || user.Name != "Test User" || user.Role != identity.RoleClient {
		t.Errorf("user = %+v, want full account shape", user)
	}

	if _, err := svc.CurrentUser(ctx, identity.Identity{UserID: "ghost", Role: identity.RoleClient}); !errors.Is(err, identity.ErrUserNotFound) {
		t.Errorf("missing account: err = %v, want ErrUserNotFound", err)
	}
}
