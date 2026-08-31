// Package config loads runtime configuration from the environment.
// Secrets are never hard-coded; they come from env stores (Render/Vercel/local .env).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/domain/signaling"
	"github.com/xcreativs/terios/api/internal/ports"
)

type Config struct {
	Env         string
	Port        int
	MongoURI    string
	MongoDBName string

	JWTAccessSecret  string
	JWTRefreshSecret string
	MFAEncryptionKey string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	ResendAPIKey string
	ResendFrom   string

	// Notifications (BE-09).
	// PracticeEmail receives practice-facing alerts (new enquiries).
	PracticeEmail string
	// PortalURL and DashboardURL are what the emails link to.
	PortalURL    string
	DashboardURL string
	// ReminderLead is how far ahead of a session its reminder goes out.
	ReminderLead time.Duration
	// NotificationPollInterval is how often the dispatcher drains the outbox.
	NotificationPollInterval time.Duration
	// DefaultTimezone presents times when a message carries none.
	DefaultTimezone string

	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
	// DocumentURLTTL is how long a signed download link lives.
	DocumentURLTTL time.Duration

	StripeSecretKey string
	// StripeWebhookSecret is the signing secret (whsec_...) of the Stripe
	// webhook endpoint — separate from the API key. Without it every
	// Stripe webhook delivery is rejected.
	StripeWebhookSecret string

	// TURNKeyID and TURNAPIToken are the two values a Cloudflare Realtime
	// TURN key yields. They are a long-term secret held server-side and
	// exchanged for short-lived credentials per session — there is no
	// static TURN username or password to configure, and a deployment
	// that sets one has configured nothing.
	TURNKeyID    string
	TURNAPIToken string
	// STUNUrls are the public STUN servers tried before TURN. STUN is
	// enough for most networks; TURN is the relay for the ones it is not.
	STUNUrls []string
	// RoomOpenBefore / RoomCloseAfter are the video room's opening hours
	// relative to the appointment.
	RoomOpenBefore time.Duration
	RoomCloseAfter time.Duration

	// AllowedOrigins is the exact set of sites permitted to open a
	// signaling socket. It is never "any": a WebSocket handshake is not
	// covered by the browser's same-origin policy.
	AllowedOrigins []string

	// Auth hardening (BE-02). Defaults come from the identity domain; the
	// env vars exist so a live incident can tighten them without a deploy.
	LoginMaxAttempts     int
	LoginAttemptWindow   time.Duration
	LoginLockoutCooldown time.Duration
	AuthRateLimit        int
	AuthRateLimitWindow  time.Duration
	// TrustedProxyHops is how many reverse proxies sit in front of the API.
	// It decides which X-Forwarded-For entry is believed, so it must match
	// the deployment exactly: too high and a caller can forge its own
	// address, too low and every caller shares the proxy's address.
	TrustedProxyHops int

	// TestingSeedToken authorizes the test-only seed routes
	// (/v1/testing/*) used by the e2e suite. The routes exist only when
	// this is set AND the server is not in production — see
	// httpapi.WithTestingSeed. It must never be set in production.
	TestingSeedToken string
}

// LockoutPolicy is the brute-force rule assembled from configuration. The
// domain fills in any field left at its zero value.
func (c Config) LockoutPolicy() identity.LockoutPolicy {
	return identity.LockoutPolicy{
		MaxAttempts: c.LoginMaxAttempts,
		Window:      c.LoginAttemptWindow,
		Cooldown:    c.LoginLockoutCooldown,
	}
}

// JoinPolicy is the video room's opening hours, assembled from
// configuration. The domain fills in any field left at its zero value.
func (c Config) JoinPolicy() signaling.JoinPolicy {
	return signaling.JoinPolicy{OpenBefore: c.RoomOpenBefore, CloseAfter: c.RoomCloseAfter}
}

// StaticICEServers is the STUN-only list used when no TURN key is set.
//
// STUN alone is enough for most calls: two peers on ordinary networks
// connect directly and never touch a relay. It is not enough for two peers
// behind symmetric NAT, which is common enough on home broadband that a
// deployment running on this is running a video feature that fails for
// some clients and works for others, with no pattern they can see.
func (c Config) StaticICEServers() []ports.ICEServer {
	if len(c.STUNUrls) == 0 {
		return nil
	}
	return []ports.ICEServer{{URLs: c.STUNUrls}}
}

// TURNConfigured reports whether a Cloudflare TURN key is available.
func (c Config) TURNConfigured() bool {
	return c.TURNKeyID != "" && c.TURNAPIToken != ""
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Env:              getEnv("APP_ENV", "development"),
		Port:             getEnvInt("PORT", 8080),
		MongoURI:         os.Getenv("MONGODB_URI"),
		MongoDBName:      getEnv("MONGODB_DB", "terios"),
		JWTAccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		MFAEncryptionKey: os.Getenv("MFA_ENCRYPTION_KEY"),
		AccessTokenTTL:   getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:  getEnvDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		ResendFrom:       getEnv("RESEND_FROM", "Terios Wellness Spa <no-reply@terioscoach.com>"),

		PracticeEmail:            getEnv("PRACTICE_EMAIL", "hello@terioscoach.com"),
		PortalURL:                getEnv("PORTAL_URL", "https://terioscoach.com/portal"),
		DashboardURL:             getEnv("DASHBOARD_URL", "https://practice.terioscoach.com"),
		ReminderLead:             getEnvDuration("REMINDER_LEAD", notification.DefaultReminderLead),
		NotificationPollInterval: getEnvDuration("NOTIFICATION_POLL_INTERVAL", time.Minute),
		DefaultTimezone:          getEnv("DEFAULT_TIMEZONE", "Africa/Accra"),
		CloudinaryCloudName:      os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:         os.Getenv("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret:      os.Getenv("CLOUDINARY_API_SECRET"),
		DocumentURLTTL:           getEnvDuration("DOCUMENT_URL_TTL", time.Hour),
		StripeSecretKey:          os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:      os.Getenv("STRIPE_WEBHOOK_SECRET"),
		TURNKeyID:                os.Getenv("TURN_KEY_ID"),
		TURNAPIToken:             os.Getenv("TURN_API_TOKEN"),

		LoginMaxAttempts:     getEnvInt("LOGIN_MAX_ATTEMPTS", identity.DefaultMaxLoginAttempts),
		LoginAttemptWindow:   getEnvDuration("LOGIN_ATTEMPT_WINDOW", identity.DefaultAttemptWindow),
		LoginLockoutCooldown: getEnvDuration("LOGIN_LOCKOUT_COOLDOWN", identity.DefaultLockoutCooldown),
		AuthRateLimit:        getEnvInt("AUTH_RATE_LIMIT", 0),
		AuthRateLimitWindow:  getEnvDuration("AUTH_RATE_LIMIT_WINDOW", 0),
		TrustedProxyHops:     getEnvInt("TRUSTED_PROXY_HOPS", 1),
		TestingSeedToken:     os.Getenv("TESTING_SEED_TOKEN"),

		STUNUrls:       getEnvList("STUN_URLS", []string{"stun:stun.cloudflare.com:3478"}),
		RoomOpenBefore: getEnvDuration("ROOM_OPEN_BEFORE", signaling.DefaultOpenBefore),
		RoomCloseAfter: getEnvDuration("ROOM_CLOSE_AFTER", signaling.DefaultCloseAfter),
		AllowedOrigins: getEnvList("ALLOWED_ORIGINS", nil),
	}

	if cfg.Env == "production" {
		if cfg.MongoURI == "" {
			return Config{}, fmt.Errorf("MONGODB_URI is required in production")
		}
		if cfg.JWTAccessSecret == "" || cfg.JWTRefreshSecret == "" {
			return Config{}, fmt.Errorf("JWT secrets are required in production")
		}
		if cfg.MFAEncryptionKey == "" {
			return Config{}, fmt.Errorf("MFA_ENCRYPTION_KEY is required in production")
		}
		if cfg.StripeSecretKey == "" || cfg.StripeWebhookSecret == "" {
			return Config{}, fmt.Errorf("STRIPE_SECRET_KEY and STRIPE_WEBHOOK_SECRET are required in production")
		}
		// Without this the API is up, healthy, and useless: CORS permits no
		// browser origin and the signaling socket refuses every handshake,
		// so both apps fail on every call while every probe stays green.
		// Refusing to start is the loud version of the same outcome — and
		// the only one that says why.
		if len(cfg.AllowedOrigins) == 0 {
			return Config{}, fmt.Errorf(
				"ALLOWED_ORIGINS is required in production: set it to the exact web and admin origins, comma-separated")
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// getEnvList reads a comma-separated list, trimming blanks.
func getEnvList(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
