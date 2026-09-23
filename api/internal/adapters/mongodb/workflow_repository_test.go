package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWorkflowSnapshotDoesNotContainPrivateContent(t *testing.T) {
	snap := mutationSnapshot(bson.M{"clientId": bson.NewObjectID(), "status": "submitted", "answers": "private answers", "body": "clinical notes", "signedName": "signature", "guardianEmail": "private@example.com", "participant": bson.D{{Key: "revision", Value: int32(3)}, {Key: "name", Value: "child"}}})
	if snap["participantRevision"] != "3" {
		t.Fatal("revision lost")
	}
	for _, key := range []string{"answers", "body", "signedName", "guardianEmail", "participant"} {
		if _, ok := snap[key]; ok {
			t.Fatalf("private field %s leaked", key)
		}
	}
}

// Requires an explicitly supplied disposable replica-set connection. It creates
// and drops only a uniquely named test database, never the application database.
func TestBusinessWriteAndWorkflowEventCommitTogether(t *testing.T) {
	uri := os.Getenv("TERIOS_TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("set TERIOS_TEST_MONGODB_URI to a disposable replica set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database("terios_workflow_test_" + bson.NewObjectID().Hex())
	defer db.Drop(context.Background())
	if err = db.CreateCollection(ctx, "bookings"); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateCollection(ctx, "workflow_events", options.CreateCollection().SetValidator(bson.M{"requiredTestField": bson.M{"$exists": true}})); err != nil {
		t.Fatal(err)
	}
	oid := bson.NewObjectID()
	doc := bson.M{"_id": oid, "status": "confirmed", "startAt": time.Now().UTC()}
	if _, err = insertOneWithEvent(ctx, db.Collection("bookings"), doc); err == nil {
		t.Fatal("expected journal insertion to fail")
	}
	count, err := db.Collection("bookings").CountDocuments(ctx, bson.M{})
	if err != nil || count != 0 {
		t.Fatalf("business write survived failed journal: count=%d err=%v", count, err)
	}
	if err = db.RunCommand(ctx, bson.D{{Key: "collMod", Value: "workflow_events"}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	if _, err = insertOneWithEvent(ctx, db.Collection("bookings"), doc); err != nil {
		t.Fatal(err)
	}
	count, err = db.Collection("workflow_events").CountDocuments(ctx, bson.M{"entityId": oid, "status": "pending"})
	if err != nil || count != 1 {
		t.Fatalf("event missing: %d %v", count, err)
	}
	var event bson.M
	if err = db.Collection("workflow_events").FindOne(ctx, bson.M{"entityId": oid}).Decode(&event); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stringMustJSON(t, event), "requiredTestField") {
		t.Fatal("unexpected metadata")
	}
}
func stringMustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := bson.MarshalExtJSON(v, false, false)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
