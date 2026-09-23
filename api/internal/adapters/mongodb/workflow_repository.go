package mongodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Every covered business write and its recovery event commit in the same Mongo
// transaction. This requires Atlas/a replica set. No provider call runs inside it.
// A crash after commit is recoverable; a rejected/aborted mutation has no event.
func mutationTransaction(ctx context.Context, coll *mongo.Collection, fn func(context.Context) (any, error)) (any, error) {
	session, err := coll.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)
	return session.WithTransaction(ctx, fn)
}
func mutationSnapshot(doc bson.M) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"clientId", "practitionerId", "agreementId", "agreementIds", "agreementCollectionId", "uploadedBy", "bookingId", "serviceId", "formId", "status", "startAt", "endAt", "sharedAt", "visibleToClient", "submittedAt", "practitionerSignedAt", "contextId", "changeRequestedAt", "changeRequestType", "proposedStartAt", "paymentStatus", "paymentExpired", "participantRevision", "participantAppointmentAt", "version", "key", "active", "updatedAt", "deleted"} {
		switch value := doc[key].(type) {
		case bson.A:
			raw, _ := json.Marshal(value)
			out[key] = string(raw)
		case bson.ObjectID:
			out[key] = value.Hex()
		case bson.DateTime:
			out[key] = value.Time().UTC().Format(time.RFC3339Nano)
		case time.Time:
			out[key] = value.UTC().Format(time.RFC3339Nano)
		case string:
			out[key] = value
		case bool:
			out[key] = fmt.Sprint(value)
		case int32:
			out[key] = fmt.Sprint(value)
		case int64:
			out[key] = fmt.Sprint(value)
		}
	}
	if fields, ok := doc["participant"].(bson.D); ok {
		nested := bson.M{}
		for _, field := range fields {
			nested[field.Key] = field.Value
		}
		doc["participant"] = nested
	}
	if p, ok := doc["participant"].(bson.M); ok {
		out["participantRevision"] = fmt.Sprint(p["revision"])
	}
	return out
}
func recordMutation(ctx context.Context, coll *mongo.Collection, before, after bson.M) error {
	switch coll.Name() {
	case "services", "agreements", "bookings", "agreement_executions", "agreement_signatures", "form_submissions", "documents", "session_notes", "payments", "session_recordings", "enquiries", "reviews":
	default:
		return nil
	}
	old, next := mutationSnapshot(before), mutationSnapshot(after)
	if next["practitionerId"] == "" && (coll.Name() == "agreement_signatures" || coll.Name() == "agreement_executions") {
		var owner struct {
			PractitionerID bson.ObjectID `bson:"practitionerId"`
		}
		if err := coll.Database().Collection("agreements").FindOne(ctx, bson.M{"_id": after["agreementId"]}).Decode(&owner); err != nil {
			return err
		}
		next["practitionerId"] = owner.PractitionerID.Hex()
		if len(before) > 0 {
			old["practitionerId"] = next["practitionerId"]
		}
	}
	if len(before) > 0 && reflect.DeepEqual(old, next) {
		return nil
	}
	_, err := coll.Database().Collection("workflow_events").InsertOne(ctx, bson.M{"_id": bson.NewObjectID(), "collection": coll.Name(), "entityId": after["_id"], "before": old, "after": next, "createdAt": time.Now().UTC(), "status": "pending", "attempts": 0, "dueAt": time.Now().UTC()})
	return err
}
func insertOneWithEvent(ctx context.Context, coll *mongo.Collection, doc any) (*mongo.InsertOneResult, error) {
	value, err := mutationTransaction(ctx, coll, func(tx context.Context) (any, error) {
		result, err := coll.InsertOne(tx, doc)
		if err != nil {
			return nil, err
		}
		var after bson.M
		if err = coll.FindOne(tx, bson.M{"_id": result.InsertedID}).Decode(&after); err != nil {
			return nil, err
		}
		if err = recordMutation(tx, coll, nil, after); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*mongo.InsertOneResult), nil
}
func updateOneWithEvent(ctx context.Context, coll *mongo.Collection, filter, update any) (*mongo.UpdateResult, error) {
	return updateWithEvent(ctx, coll, filter, update, false)
}
func replaceOneWithEvent(ctx context.Context, coll *mongo.Collection, filter, replacement any) (*mongo.UpdateResult, error) {
	return updateWithEvent(ctx, coll, filter, replacement, true)
}
func updateWithEvent(ctx context.Context, coll *mongo.Collection, filter, update any, replace bool) (*mongo.UpdateResult, error) {
	value, err := mutationTransaction(ctx, coll, func(tx context.Context) (any, error) {
		var before bson.M
		err := coll.FindOne(tx, filter).Decode(&before)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		var result *mongo.UpdateResult
		if replace {
			result, err = coll.ReplaceOne(tx, filter, update)
		} else {
			result, err = coll.UpdateOne(tx, filter, update)
		}
		if err != nil {
			return nil, err
		}
		if result.MatchedCount == 0 {
			return result, nil
		}
		var after bson.M
		if err = coll.FindOne(tx, bson.M{"_id": before["_id"]}).Decode(&after); err != nil {
			return nil, err
		}
		if err = recordMutation(tx, coll, before, after); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return value.(*mongo.UpdateResult), nil
}

type WorkflowEventRepository struct{ coll *mongo.Collection }

func NewWorkflowEventRepository(db *mongo.Database) *WorkflowEventRepository {
	return &WorkflowEventRepository{db.Collection("workflow_events")}
}
func (r *WorkflowEventRepository) Pending(ctx context.Context, limit int) ([]ports.WorkflowEvent, error) {
	if err := r.expirePayments(ctx, limit); err != nil {
		return nil, err
	}
	cursor, err := r.coll.Find(ctx, bson.M{"status": "pending", "dueAt": bson.M{"$lte": time.Now().UTC()}}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	var docs []struct {
		ID         bson.ObjectID     `bson:"_id"`
		Collection string            `bson:"collection"`
		EntityID   bson.ObjectID     `bson:"entityId"`
		Before     map[string]string `bson:"before"`
		After      map[string]string `bson:"after"`
		CreatedAt  time.Time         `bson:"createdAt"`
	}
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]ports.WorkflowEvent, 0, len(docs))
	for _, d := range docs {
		out = append(out, ports.WorkflowEvent{ID: d.ID.Hex(), Collection: d.Collection, EntityID: d.EntityID.Hex(), Before: d.Before, After: d.After, CreatedAt: d.CreatedAt})
	}
	return out, nil
}
func (r *WorkflowEventRepository) Complete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"status": "queued", "queuedAt": time.Now().UTC()}})
	return err
}
func (r *WorkflowEventRepository) Failed(ctx context.Context, id, reason string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	var doc struct {
		Attempts int `bson:"attempts"`
	}
	if err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		return err
	}
	status := "pending"
	if doc.Attempts >= 9 {
		status = "dead_letter"
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid, "status": "pending"}, bson.M{"$inc": bson.M{"attempts": 1}, "$set": bson.M{"status": status, "lastError": reason, "dueAt": time.Now().UTC().Add(time.Minute * time.Duration(1<<min(doc.Attempts, 6)))}})
	return err
}

func (r *WorkflowEventRepository) expirePayments(ctx context.Context, limit int) error {
	coll := r.coll.Database().Collection("bookings")
	now := time.Now().UTC()
	cursor, err := coll.Find(ctx, bson.M{"status": "pending_payment", "startAt": bson.M{"$lte": now}}, options.Find().SetLimit(int64(limit)))
	if err != nil {
		return err
	}
	var docs []bookingDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return err
	}
	for _, doc := range docs {
		_, err := updateOneWithEvent(ctx, coll, bson.M{"_id": doc.ID, "status": "pending_payment"}, bson.M{"$set": bson.M{"status": "cancelled", "paymentExpired": true, "cancelledAt": now, "updatedAt": now}, "$inc": bson.M{"revision": 1}})
		if err != nil {
			return err
		}
	}
	return nil
}

// Deletion keeps a metadata-only tombstone event in the same transaction.
func deleteOneWithEvent(ctx context.Context, coll *mongo.Collection, filter any) (*mongo.DeleteResult, error) {
	result, err := mutationTransaction(ctx, coll, func(tx context.Context) (any, error) {
		var before bson.M
		if err := coll.FindOne(tx, filter).Decode(&before); errors.Is(err, mongo.ErrNoDocuments) {
			return &mongo.DeleteResult{}, nil
		} else if err != nil {
			return nil, err
		}
		result, err := coll.DeleteOne(tx, filter)
		if err != nil {
			return nil, err
		}
		after := bson.M{}
		for k, v := range before {
			after[k] = v
		}
		after["deleted"] = true
		if err = recordMutation(tx, coll, before, after); err != nil {
			return nil, err
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*mongo.DeleteResult), nil
}
