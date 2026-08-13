package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// LoginAttemptRepository persists failed-login accounting in the
// login_attempts collection, keyed on the submitted login identifier
// (normalized email). Records carry an expiresAt stamp that the TTL index
// reaps, so the collection self-cleans once a window and its cooldown have
// passed (see indexes.go).
//
// Nothing here identifies an account: the key is the string that was typed
// into the login form, which is deliberately recorded whether or not it
// matches a real user.
type LoginAttemptRepository struct {
	coll *mongo.Collection
}

var _ ports.LoginAttemptStore = (*LoginAttemptRepository)(nil)

// NewLoginAttemptRepository binds the repository to login_attempts.
func NewLoginAttemptRepository(db *mongo.Database) *LoginAttemptRepository {
	return &LoginAttemptRepository{coll: db.Collection("login_attempts")}
}

// loginAttemptDoc is the storage shape.
type loginAttemptDoc struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	Identifier string        `bson:"identifier"`
	Count      int           `bson:"count"`
	FirstAt    bson.DateTime `bson:"firstAt"`
	LastAt     bson.DateTime `bson:"lastAt"`
	ExpiresAt  bson.DateTime `bson:"expiresAt"`
}

// Get returns the current history. An identifier with no record yields a
// zero-count history — "never failed" is not an error.
func (r *LoginAttemptRepository) Get(ctx context.Context, identifier string) (identity.LoginAttempts, error) {
	var doc loginAttemptDoc
	err := r.coll.FindOne(ctx, bson.M{"identifier": identifier}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return identity.LoginAttempts{Identifier: identifier}, nil
		}
		return identity.LoginAttempts{}, fmt.Errorf("find login attempts: %w", err)
	}
	return identity.LoginAttempts{
		Identifier: doc.Identifier,
		Count:      doc.Count,
		FirstAt:    doc.FirstAt.Time().UTC(),
		LastAt:     doc.LastAt.Time().UTC(),
	}, nil
}

// Save upserts the history and refreshes its expiry stamp.
func (r *LoginAttemptRepository) Save(ctx context.Context, attempts identity.LoginAttempts, retainFor time.Duration) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"identifier": attempts.Identifier},
		bson.M{"$set": bson.M{
			"identifier": attempts.Identifier,
			"count":      attempts.Count,
			"firstAt":    bson.NewDateTimeFromTime(attempts.FirstAt),
			"lastAt":     bson.NewDateTimeFromTime(attempts.LastAt),
			"expiresAt":  bson.NewDateTimeFromTime(attempts.LastAt.Add(retainFor)),
		}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("save login attempts: %w", err)
	}
	return nil
}

// Reset clears the history after a successful login. A missing record is
// not an error — there was simply nothing to clear.
func (r *LoginAttemptRepository) Reset(ctx context.Context, identifier string) error {
	if _, err := r.coll.DeleteOne(ctx, bson.M{"identifier": identifier}); err != nil {
		return fmt.Errorf("reset login attempts: %w", err)
	}
	return nil
}
