// Terios Wellness Spa — dev seed tool.
// Creates a practitioner account, two sample services, and the default
// weekly availability rules. Refuses to run in production.
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
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	practitionerEmail    = "practitioner@terioswellness.com"
	seedPassword         = "password123" // dev only, never ship to production
	seedAvailabilityFrom = "09:00"
	seedAvailabilityTo   = "17:00"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
	slog.Info("seed complete")
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.Env == "production" {
		return errors.New("refusing to seed: APP_ENV=production")
	}
	if cfg.MongoURI == "" {
		return errors.New("MONGODB_URI is required to seed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongodb.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return fmt.Errorf("connect mongodb: %w", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			slog.Error("mongodb disconnect", "error", err)
		}
	}()

	db := client.Database(cfg.MongoDBName)
	if err := mongodb.EnsureIndexes(ctx, db); err != nil {
		return fmt.Errorf("ensure indexes: %w", err)
	}

	practitionerID, err := seedPractitioner(ctx, db)
	if err != nil {
		return err
	}
	if err := seedServices(ctx, db, practitionerID); err != nil {
		return err
	}
	if err := seedAvailability(ctx, db, practitionerID); err != nil {
		return err
	}
	return nil
}

// seedPractitioner upserts the dev practitioner account by email.
func seedPractitioner(ctx context.Context, db *mongo.Database) (bson.ObjectID, error) {
	hash, err := security.NewArgon2Hasher().Hash(seedPassword)
	if err != nil {
		return bson.NilObjectID, fmt.Errorf("hash seed password: %w", err)
	}

	now := time.Now().UTC()
	doc := bson.M{
		"email":        practitionerEmail,
		"passwordHash": hash,
		"role":         "practitioner",
		"name":         "Dev Practitioner",
		"active":       true,
		"createdAt":    now,
		"updatedAt":    now,
	}

	opts := options.UpdateOne().SetUpsert(true)
	if _, err := db.Collection("users").UpdateOne(ctx,
		bson.M{"email": practitionerEmail},
		bson.M{"$set": doc},
		opts,
	); err != nil {
		return bson.NilObjectID, fmt.Errorf("upsert practitioner: %w", err)
	}

	var user struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := db.Collection("users").FindOne(ctx, bson.M{"email": practitionerEmail}).Decode(&user); err != nil {
		return bson.NilObjectID, fmt.Errorf("lookup practitioner: %w", err)
	}
	slog.Info("seeded practitioner", "email", practitionerEmail, "id", user.ID.Hex())
	return user.ID, nil
}

// seedServices inserts two sample services, skipping if the practitioner
// already has any.
func seedServices(ctx context.Context, db *mongo.Database, practitionerID bson.ObjectID) error {
	count, err := db.Collection("services").CountDocuments(ctx, bson.M{"practitionerId": practitionerID})
	if err != nil {
		return fmt.Errorf("count services: %w", err)
	}
	if count > 0 {
		slog.Info("services already present, skipping", "count", count)
		return nil
	}

	now := time.Now().UTC()
	services := []any{
		bson.M{
			"practitionerId": practitionerID,
			"name":           "Swedish Massage",
			"description":    "Classic full-body relaxation massage.",
			"durationMin":    60,
			"priceKobo":      25_000_00, // GHS 25,000.00 (minor units)
			"currency":       "GHS",
			"active":         true,
			"sortOrder":      1,
			"createdAt":      now,
			"updatedAt":      now,
		},
		bson.M{
			"practitionerId": practitionerID,
			"name":           "Deep Tissue Massage",
			"description":    "Targeted pressure for chronic muscle tension.",
			"durationMin":    90,
			"priceKobo":      40_000_00, // GHS 40,000.00
			"currency":       "GHS",
			"active":         true,
			"sortOrder":      2,
			"createdAt":      now,
			"updatedAt":      now,
		},
	}
	if _, err := db.Collection("services").InsertMany(ctx, services); err != nil {
		return fmt.Errorf("insert services: %w", err)
	}
	slog.Info("seeded services", "count", len(services))
	return nil
}

// seedAvailability upserts weekday (Mon–Fri) 09:00–17:00 opening hours in
// the availability_rules shape the scheduling slice reads: windows as
// minutes-since-midnight plus a per-rule buffer.
func seedAvailability(ctx context.Context, db *mongo.Database, practitionerID bson.ObjectID) error {
	now := time.Now().UTC()
	// 1 = Monday .. 5 = Friday; 0 = Sunday and 6 = Saturday stay closed.
	for weekday := 1; weekday <= 5; weekday++ {
		filter := bson.M{"practitionerId": practitionerID, "weekday": weekday}
		rule := bson.M{
			"practitionerId": practitionerID,
			"weekday":        weekday,
			"windows":        []bson.M{{"startMin": 9 * 60, "endMin": 17 * 60}},
			"bufferMinutes":  0,
			"createdAt":      now,
			"updatedAt":      now,
		}
		if _, err := db.Collection("availability_rules").UpdateOne(ctx,
			filter,
			bson.M{"$set": rule},
			options.UpdateOne().SetUpsert(true),
		); err != nil {
			return fmt.Errorf("upsert availability rule weekday %d: %w", weekday, err)
		}
	}
	slog.Info("seeded availability rules", "weekdays", "Mon-Fri", "hours", seedAvailabilityFrom+"-"+seedAvailabilityTo)
	return nil
}
