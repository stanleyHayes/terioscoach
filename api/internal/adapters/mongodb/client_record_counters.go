package mongodb

import (
	"context"
	"fmt"

	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DocumentCounter counts a client's documents. The documents collection and
// its records are owned by the documents slice (BE-11) — this is a
// count-only reader for the client-record rollup, leading with clientId
// like every client-scoped query.
type DocumentCounter struct {
	coll *mongo.Collection
}

var _ ports.DocumentCounter = (*DocumentCounter)(nil)

// NewDocumentCounter binds the counter to the documents collection.
func NewDocumentCounter(db *mongo.Database) *DocumentCounter {
	return &DocumentCounter{coll: db.Collection("documents")}
}

// CountByClient returns how many documents the client has. A non-ObjectID
// client id counts nothing, mirroring the repository list conventions.
func (c *DocumentCounter) CountByClient(ctx context.Context, clientID string) (int, error) {
	oid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return 0, nil
	}
	n, err := c.coll.CountDocuments(ctx, bson.M{"clientId": oid})
	if err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return int(n), nil
}

// FormSubmissionCounter counts a client's form submissions. The
// form_submissions collection and its records are owned by the forms slice
// (BE-10) — this is a count-only reader for the client-record rollup.
type FormSubmissionCounter struct {
	coll *mongo.Collection
}

var _ ports.FormSubmissionCounter = (*FormSubmissionCounter)(nil)

// NewFormSubmissionCounter binds the counter to the form_submissions
// collection.
func NewFormSubmissionCounter(db *mongo.Database) *FormSubmissionCounter {
	return &FormSubmissionCounter{coll: db.Collection("form_submissions")}
}

// CountByClient returns how many form submissions the client has.
func (c *FormSubmissionCounter) CountByClient(ctx context.Context, clientID string) (int, error) {
	oid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return 0, nil
	}
	n, err := c.coll.CountDocuments(ctx, bson.M{"clientId": oid})
	if err != nil {
		return 0, fmt.Errorf("count form submissions: %w", err)
	}
	return int(n), nil
}
