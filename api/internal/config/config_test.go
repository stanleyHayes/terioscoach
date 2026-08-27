package config

import (
	"strings"
	"testing"
	"time"
)

// production sets the three variables every production boot needs, so each
// test can knock out exactly the one it is about.
func production(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("MONGODB_URI", "mongodb://localhost:27017")
	t.Setenv("JWT_ACCESS_SECRET", "access-secret-access-secret-1234")
	t.Setenv("JWT_REFRESH_SECRET", "refresh-secret-refresh-secret-12")
	t.Setenv("ALLOWED_ORIGINS", "https://terioswellness.com,https://admin.terioswellness.com")
}

func TestProductionRequiresItsSecretsAndOrigins(t *testing.T) {
	cases := []struct {
		name   string
		unset  string
		wantIn string
	}{
		{"no database", "MONGODB_URI", "MONGODB_URI"},
		{"no access secret", "JWT_ACCESS_SECRET", "JWT secrets"},
		{"no refresh secret", "JWT_REFRESH_SECRET", "JWT secrets"},
		// The one that fails silently otherwise: the API comes up healthy
		// and refuses every browser call from both apps.
		{"no allowed origins", "ALLOWED_ORIGINS", "ALLOWED_ORIGINS"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			production(t)
			t.Setenv(tc.unset, "")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load succeeded without %s; production must not start half-configured", tc.unset)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error = %q, want it to name %q", err, tc.wantIn)
			}
		})
	}
}

func TestProductionLoadsWithEverythingSet(t *testing.T) {
	production(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.AllowedOrigins; len(got) != 2 || got[0] != "https://terioswellness.com" {
		t.Errorf("AllowedOrigins = %v, want both app origins", got)
	}
}

// Development is deliberately permissive: the API boots without a database
// so the frontend work can run against it, and says so through readiness
// rather than by refusing to start.
func TestDevelopmentBootsWithNothingConfigured(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("MONGODB_URI", "")
	t.Setenv("JWT_ACCESS_SECRET", "")
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 || cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("defaults = port %d / ttl %s, want 8080 / 15m", cfg.Port, cfg.AccessTokenTTL)
	}
}

func TestPaymentProviderMustBeOneOfTwo(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("PAYMENT_PROVIDER", "PayStack") // case is not the caller's problem
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PaymentProvider != "paystack" {
		t.Errorf("PaymentProvider = %q, want normalized to paystack", cfg.PaymentProvider)
	}

	t.Setenv("PAYMENT_PROVIDER", "flutterwave")
	if _, err := Load(); err == nil {
		t.Error("Load accepted an unsupported payment provider")
	}
}

// A malformed list must not read as "no origins" — that is the outage the
// production guard exists to prevent, arriving by a different door.
func TestAllowedOriginsIgnoresBlankEntries(t *testing.T) {
	production(t)
	t.Setenv("ALLOWED_ORIGINS", " https://terioswellness.com , , https://admin.terioswellness.com ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"https://terioswellness.com", "https://admin.terioswellness.com"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i, origin := range want {
		if cfg.AllowedOrigins[i] != origin {
			t.Errorf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], origin)
		}
	}
}

// TrustedProxyHops decides which X-Forwarded-For entry is believed, so a
// typo must not silently become "trust the leftmost, forgeable one".
func TestTrustedProxyHopsDefaultsToRendersSingleEdge(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("TRUSTED_PROXY_HOPS", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TrustedProxyHops != 1 {
		t.Errorf("TrustedProxyHops = %d, want 1", cfg.TrustedProxyHops)
	}

	t.Setenv("TRUSTED_PROXY_HOPS", "not-a-number")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.TrustedProxyHops != 1 {
		t.Errorf("TrustedProxyHops = %d on a malformed value, want the default 1", cfg.TrustedProxyHops)
	}
}
