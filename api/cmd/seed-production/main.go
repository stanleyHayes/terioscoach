// Command seed-production provisions only the named practitioner accounts.
// It is intentionally separate from cmd/seed: no demo content is ever written.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/mongodb"
	"github.com/xcreativs/terios/api/internal/adapters/security"
	"github.com/xcreativs/terios/api/internal/config"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const confirmation = "seed-terios-production"

type account struct{ email, name, passwordEnv string }

var accounts = []account{
	{"admin@terioscoach.com", "Terios Administrator", "TERIOS_ADMIN_PASSWORD"},
	{"hayfordstanley@gmail.com", "Hayford Stanley", "TERIOS_OWNER_PASSWORD"},
}

func main() {
	if err := run(); err != nil {
		slog.Error("production seed failed", "error", err)
		os.Exit(1)
	}
	slog.Info("production practitioner seed complete", "accounts", len(accounts))
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Env != "production" {
		return errors.New("refusing production seed: APP_ENV must equal production")
	}
	if os.Getenv("CONFIRM_PRODUCTION_SEED") != confirmation {
		return fmt.Errorf("refusing production seed: set CONFIRM_PRODUCTION_SEED=%s", confirmation)
	}
	if cfg.MongoURI == "" {
		return errors.New("MONGODB_URI is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client, err := mongodb.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("connect MongoDB: %w", err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(cfg.MongoDBName)
	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}
	for _, a := range accounts {
		if err := ensureAccount(ctx, db, a); err != nil {
			return err
		}
	}
	return nil
}

func ensureAccount(ctx context.Context, db *mongo.Database, a account) error {
	password := os.Getenv(a.passwordEnv)
	if err := identity.ValidatePassword(password); err != nil {
		return fmt.Errorf("%s: %w", a.passwordEnv, err)
	}
	hash, err := security.NewArgon2Hasher().Hash(password)
	if err != nil {
		return fmt.Errorf("hash %s: %w", a.email, err)
	}
	now := time.Now().UTC()
	res, err := db.Collection("users").UpdateOne(ctx,
		bson.M{"email": identity.NormalizeEmail(a.email)},
		bson.M{"$setOnInsert": bson.M{"email": identity.NormalizeEmail(a.email), "passwordHash": hash, "role": string(identity.RolePractitioner), "name": a.name, "active": true, "createdAt": now, "updatedAt": now, "mfaEnabled": false}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("provision %s: %w", a.email, err)
	}
	if res.UpsertedCount == 0 {
		slog.Info("practitioner already exists; password and MFA state preserved", "email", a.email)
	} else {
		slog.Info("created practitioner", "email", a.email)
	}
	return nil
}
