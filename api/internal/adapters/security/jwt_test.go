package security

import (
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

func TestJWTIssueAndVerify(t *testing.T) {
	issuer, err := NewJWTIssuer("test-secret", 15*time.Minute)
	if err != nil {
		t.Fatalf("NewJWTIssuer: %v", err)
	}
	want := identity.Identity{UserID: "user-1", Role: identity.RolePractitioner}

	token, expiresAt, err := issuer.IssueAccessToken(want)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if expiresAt.Before(time.Now().Add(14 * time.Minute)) {
		t.Errorf("expiry %v not ~15m out", expiresAt)
	}

	got, err := issuer.VerifyAccessToken(token)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if got != want {
		t.Errorf("identity = %+v, want %+v", got, want)
	}
}

func TestJWTRejectsExpiredToken(t *testing.T) {
	issuer, err := NewJWTIssuer("test-secret", time.Minute)
	if err != nil {
		t.Fatalf("NewJWTIssuer: %v", err)
	}
	// Backdate the TTL so the issued token is already expired.
	issuer.ttl = -time.Minute
	token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: "u", Role: identity.RoleClient})
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := issuer.VerifyAccessToken(token); !errors.Is(err, identity.ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	a, _ := NewJWTIssuer("secret-a", time.Minute)
	b, _ := NewJWTIssuer("secret-b", time.Minute)

	token, _, err := a.IssueAccessToken(identity.Identity{UserID: "u", Role: identity.RoleClient})
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := b.VerifyAccessToken(token); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Errorf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestJWTRejectsGarbage(t *testing.T) {
	issuer, _ := NewJWTIssuer("test-secret", time.Minute)
	for _, token := range []string{"", "abc", "a.b.c", "header.payload.signature"} {
		if _, err := issuer.VerifyAccessToken(token); !errors.Is(err, identity.ErrTokenInvalid) {
			t.Errorf("VerifyAccessToken(%q): err = %v, want ErrTokenInvalid", token, err)
		}
	}
}

func TestJWTIssuerRequiresSecret(t *testing.T) {
	if _, err := NewJWTIssuer("", time.Minute); err == nil {
		t.Error("empty secret accepted")
	}
	if _, err := NewJWTIssuer("x", 0); err == nil {
		t.Error("zero TTL accepted")
	}
}

func TestRefreshTokensAreOpaqueAndHashed(t *testing.T) {
	issuer, _ := NewJWTIssuer("test-secret", time.Minute)

	plain, hash, err := issuer.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	if plain == "" || hash == "" {
		t.Fatal("empty token or hash")
	}
	if plain == hash {
		t.Error("hash must differ from plaintext")
	}
	if got := issuer.HashRefreshToken(plain); got != hash {
		t.Error("HashRefreshToken not deterministic")
	}

	plain2, hash2, err := issuer.NewRefreshToken()
	if err != nil {
		t.Fatalf("NewRefreshToken: %v", err)
	}
	if plain2 == plain || hash2 == hash {
		t.Error("refresh tokens not unique")
	}
}
