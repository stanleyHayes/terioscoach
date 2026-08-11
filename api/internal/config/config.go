// Package config loads runtime configuration from the environment.
// Secrets are never hard-coded; they come from env stores (Render/Vercel/local .env).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env         string
	Port        int
	MongoURI    string
	MongoDBName string

	JWTAccessSecret  string
	JWTRefreshSecret string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration

	ResendAPIKey string
	ResendFrom   string

	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string

	PaystackSecretKey string
	PaystackPublicKey string

	TURNUrls       []string
	TURNUsername   string
	TURNCredential string

	AllowedOrigins []string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		Env:                 getEnv("APP_ENV", "development"),
		Port:                getEnvInt("PORT", 8080),
		MongoURI:            os.Getenv("MONGODB_URI"),
		MongoDBName:         getEnv("MONGODB_DB", "terios"),
		JWTAccessSecret:     os.Getenv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret:    os.Getenv("JWT_REFRESH_SECRET"),
		AccessTokenTTL:      getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getEnvDuration("REFRESH_TOKEN_TTL", 30*24*time.Hour),
		ResendAPIKey:        os.Getenv("RESEND_API_KEY"),
		ResendFrom:          getEnv("RESEND_FROM", "Terios Wellness Spa <no-reply@terioswellness.com>"),
		CloudinaryCloudName: os.Getenv("CLOUDINARY_CLOUD_NAME"),
		CloudinaryAPIKey:    os.Getenv("CLOUDINARY_API_KEY"),
		CloudinaryAPISecret: os.Getenv("CLOUDINARY_API_SECRET"),
		PaystackSecretKey:   os.Getenv("PAYSTACK_SECRET_KEY"),
		PaystackPublicKey:   os.Getenv("PAYSTACK_PUBLIC_KEY"),
		TURNUsername:        os.Getenv("TURN_USERNAME"),
		TURNCredential:      os.Getenv("TURN_CREDENTIAL"),
	}

	if cfg.Env == "production" {
		if cfg.MongoURI == "" {
			return Config{}, fmt.Errorf("MONGODB_URI is required in production")
		}
		if cfg.JWTAccessSecret == "" || cfg.JWTRefreshSecret == "" {
			return Config{}, fmt.Errorf("JWT secrets are required in production")
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
