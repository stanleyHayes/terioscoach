// workflowctl performs read-only rollout audits by default. Mutation commands
// require --apply and target only explicit IDs or a provenance-reviewed manifest.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type zoneEntry struct {
	BookingID string `json:"bookingId"`
	Timezone  string `json:"timezone"`
	Source    string `json:"source"`
}
type backup struct {
	Migration string      `json:"migration"`
	Entries   []zoneEntry `json:"entries"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	action := flag.String("action", "audit", "audit, backfill, rollback, replay-event, or replay-job")
	apply := flag.Bool("apply", false, "apply the explicit operation; otherwise dry-run")
	path := flag.String("manifest", "", "JSON provenance manifest for backfill or backup for rollback")
	backupPath := flag.String("backup-out", "", "new backup file required when applying backfill")
	target := flag.String("id", "", "single dead-letter event or failed job ID")
	flag.Parse()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return errors.New("MONGODB_URI is required")
	}
	name := os.Getenv("MONGODB_DB")
	if name == "" {
		name = "terios"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background())
	db := client.Database(name)
	switch *action {
	case "audit":
		return audit(ctx, db)
	case "backfill", "rollback":
		raw, err := os.ReadFile(*path)
		if err != nil {
			return err
		}
		data := backup{Migration: bson.NewObjectID().Hex()}
		if *action == "backfill" {
			err = json.Unmarshal(raw, &data.Entries)
		} else {
			err = json.Unmarshal(raw, &data)
		}
		if err != nil {
			return err
		}
		if data.Migration == "" {
			return errors.New("backup migration ID missing")
		}
		seen := map[string]bool{}
		eligible := []zoneEntry{}
		for _, entry := range data.Entries {
			oid, err := bson.ObjectIDFromHex(entry.BookingID)
			if err != nil {
				return err
			}
			if seen[entry.BookingID] {
				return errors.New("duplicate booking in manifest")
			}
			seen[entry.BookingID] = true
			if strings.TrimSpace(entry.Source) == "" || (entry.Timezone != "UTC" && !strings.Contains(entry.Timezone, "/")) {
				return errors.New("each entry requires an IANA timezone and provenance source")
			}
			if _, err = time.LoadLocation(entry.Timezone); err != nil {
				return err
			}
			var current struct {
				Timezone  string `bson:"bookingTimezone"`
				Migration string `bson:"timezoneMigration"`
			}
			if err = db.Collection("bookings").FindOne(ctx, bson.M{"_id": oid}).Decode(&current); err != nil {
				return err
			}
			if (*action == "backfill" && current.Timezone == "") || (*action == "rollback" && current.Timezone == entry.Timezone && current.Migration == data.Migration) {
				eligible = append(eligible, entry)
			}
		}
		fmt.Printf("%s: %d eligible records; apply=%t; appointment instants untouched\n", *action, len(eligible), *apply)
		if !*apply {
			return nil
		}
		if *action == "backfill" {
			if *backupPath == "" {
				return errors.New("--backup-out is required before applying backfill")
			}
			data.Entries = eligible
			bytes, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return err
			}
			file, err := os.OpenFile(*backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return err
			}
			_, err = file.Write(bytes)
			if err == nil {
				err = file.Sync()
			}
			closeErr := file.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		for _, entry := range eligible {
			oid, _ := bson.ObjectIDFromHex(entry.BookingID)
			filter := bson.M{"_id": oid}
			var update bson.M
			if *action == "backfill" {
				filter["$or"] = bson.A{bson.M{"bookingTimezone": bson.M{"$exists": false}}, bson.M{"bookingTimezone": ""}}
				update = bson.M{"$set": bson.M{"bookingTimezone": entry.Timezone, "timezoneMigration": data.Migration, "timezoneProvenance": entry.Source}}
			} else {
				filter["bookingTimezone"] = entry.Timezone
				filter["timezoneMigration"] = data.Migration
				update = bson.M{"$unset": bson.M{"bookingTimezone": "", "timezoneMigration": "", "timezoneProvenance": ""}}
			}
			if _, err = db.Collection("bookings").UpdateOne(ctx, filter, update); err != nil {
				return err
			}
		}
		return nil
	case "replay-event", "replay-job":
		oid, err := bson.ObjectIDFromHex(*target)
		if err != nil {
			return errors.New("--id must name one event or job")
		}
		collection, required := "workflow_events", "dead_letter"
		if *action == "replay-job" {
			collection, required = "notification_jobs", "failed"
		}
		filter := bson.M{"_id": oid, "status": required}
		count, err := db.Collection(collection).CountDocuments(ctx, filter)
		if err != nil {
			return err
		}
		if count != 1 {
			return errors.New("target is missing or not in the required terminal failure state; sent jobs cannot be replayed")
		}
		fmt.Printf("%s: %s eligible; apply=%t; original event/provider key preserved\n", collection, *target, *apply)
		if !*apply {
			return nil
		}
		_, err = db.Collection(collection).UpdateOne(ctx, filter, bson.M{"$set": bson.M{"status": "pending", "attempts": 0, "dueAt": time.Now().UTC(), "replayedAt": time.Now().UTC()}, "$unset": bson.M{"claimedAt": "", "claimToken": ""}})
		return err
	default:
		return errors.New("unknown action")
	}
}
func audit(ctx context.Context, db *mongo.Database) error {
	counts := map[string]int64{}
	samples := map[string][]string{}
	cursor, err := db.Collection("bookings").Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"startAt": 1, "endAt": 1, "bookingTimezone": 1, "participant": 1, "status": 1}))
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	add := func(key, id string) {
		counts[key]++
		if len(samples[key]) < 5 {
			samples[key] = append(samples[key], id)
		}
	}
	for cursor.Next(ctx) {
		var doc bson.M
		if err = cursor.Decode(&doc); err != nil {
			return err
		}
		id := fmt.Sprint(doc["_id"])
		counts["bookings"]++
		if doc["bookingTimezone"] == nil || doc["bookingTimezone"] == "" {
			add("missingBookingTimezone", id)
		}
		if doc["participant"] == nil {
			add("participantNotRecorded", id)
		}
		for _, field := range []string{"startAt", "endAt"} {
			switch v := doc[field].(type) {
			case bson.DateTime:
			case string:
				if _, err = time.Parse(time.RFC3339Nano, v); err != nil {
					add("invalid_"+field, id)
				} else {
					add("string_"+field, id)
				}
			default:
				add("invalid_"+field, id)
			}
		}
	}
	if err = cursor.Err(); err != nil {
		return err
	}
	for key, query := range map[string]struct {
		collection string
		filter     bson.M
	}{
		"accountsWithoutTimezone": {"users", bson.M{"$or": bson.A{bson.M{"timezone": bson.M{"$exists": false}}, bson.M{"timezone": ""}}}},
		"workflowDeadLetters":     {"workflow_events", bson.M{"status": "dead_letter"}},
		"workflowPending":         {"workflow_events", bson.M{"status": "pending"}},
		"failedDeliveries":        {"notification_jobs", bson.M{"status": "failed"}},
		"legacyUnsignedSOW":       {"agreement_signatures", bson.M{"statementOfWork": bson.M{"$exists": true}, "signedName": ""}},
	} {
		count, err := db.Collection(query.collection).CountDocuments(ctx, query.filter)
		if err != nil {
			return err
		}
		counts[key] = count
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"counts": counts, "sampleIds": samples, "readOnly": true})
}
