package identity

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewUserNormalizesEmail(t *testing.T) {
	user, err := NewUser("  Alice@Example.COM ", "Alice", "hash", RoleClient, time.Now())
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email not normalized: %q", user.Email)
	}
	if user.Role != RoleClient {
		t.Errorf("role = %q, want client", user.Role)
	}
}

func TestNewUserRejectsBadInput(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name         string
		email        string
		passwordHash string
		role         Role
		wantErr      error
	}{
		{"bad email", "not-an-email", "hash", RoleClient, ErrInvalidEmail},
		{"display name email", "Alice <alice@example.com>", "hash", RoleClient, ErrInvalidEmail},
		{"empty email", "", "hash", RoleClient, ErrInvalidEmail},
		{"unknown role", "a@b.com", "hash", Role("owner"), ErrInvalidRole},
		{"empty hash", "a@b.com", "", RoleClient, ErrPasswordHashRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewUser(tc.email, "Alice", tc.passwordHash, tc.role, now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	valid := []string{"a@b.co", "first.last+tag@example.org", "practitioner@terioscoach.com"}
	for _, email := range valid {
		if err := ValidateEmail(email); err != nil {
			t.Errorf("ValidateEmail(%q) = %v, want nil", email, err)
		}
	}
	invalid := []string{"", "plain", "a b@c.com", "@c.com", "a@", "Alice <a@c.com>"}
	for _, email := range invalid {
		if err := ValidateEmail(email); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("ValidateEmail(%q) = %v, want ErrInvalidEmail", email, err)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword(strings.Repeat("x", MinPasswordLength)); err != nil {
		t.Errorf("min-length password rejected: %v", err)
	}
	if err := ValidatePassword(strings.Repeat("x", MinPasswordLength-1)); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("short password accepted: %v", err)
	}
	// Length counts runes, not bytes.
	if err := ValidatePassword(strings.Repeat("ö", MinPasswordLength)); err != nil {
		t.Errorf("multibyte min-length password rejected: %v", err)
	}
}

func TestRoleValid(t *testing.T) {
	if !RoleClient.Valid() || !RolePractitioner.Valid() {
		t.Error("known roles must be valid")
	}
	if Role("admin").Valid() {
		t.Error("unknown role must be invalid")
	}
}
