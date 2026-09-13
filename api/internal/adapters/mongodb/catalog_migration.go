package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"time"
)

// RetireLaunchSample runs once per database. Only the unchanged launch seed
// matches: an administrator's renamed, repriced or rewritten offering is kept.
// Retirement preserves the service ID and all booking history. The migration
// marker also lets an administrator explicitly reactivate it afterwards.
func RetireLaunchSample(ctx context.Context, db *mongo.Database) error {
	const migration = "retire-unchanged-launch-service-2026-09"
	migrations := db.Collection("schema_migrations")
	count, err := migrations.CountDocuments(ctx, bson.M{"_id": migration})
	if err != nil {
		return fmt.Errorf("check catalog migration: %w", err)
	}
	if count > 0 {
		return nil
	}
	_, err = db.Collection("services").UpdateMany(ctx, bson.M{
		"name":        "Introductory wellness conversation",
		"description": "A private, obligation-free conversation to explore what support you are looking for and whether Terios is the right fit.",
		"durationMin": 30, "priceKobo": 0, "currency": "USD",
		"deletedAt":   bson.M{"$exists": false},
		"imageUrl":    bson.M{"$in": bson.A{"", nil}},
		"agreementId": bson.M{"$in": bson.A{"", nil}},
	}, bson.M{"$set": bson.M{"active": false, "updatedAt": time.Now().UTC()}})
	if err != nil {
		return fmt.Errorf("retire launch catalog sample: %w", err)
	}
	_, err = migrations.UpdateOne(ctx, bson.M{"_id": migration}, bson.M{"$setOnInsert": bson.M{"completedAt": time.Now().UTC()}}, options.UpdateOne().SetUpsert(true))
	return err
}
