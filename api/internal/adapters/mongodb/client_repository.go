package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ClientProfileRepository persists practice-side client profiles in the
// client_profiles collection. One profile per client account — the unique
// index on userId is the upsert key.
type ClientProfileRepository struct {
	coll *mongo.Collection
}

var _ ports.ClientProfileRepository = (*ClientProfileRepository)(nil)

// NewClientProfileRepository binds the repository to the client_profiles
// collection.
func NewClientProfileRepository(db *mongo.Database) *ClientProfileRepository {
	return &ClientProfileRepository{coll: db.Collection("client_profiles")}
}

// clientProfileDoc is the storage shape; kept separate from the domain
// entity.
type clientProfileDoc struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	UserID        bson.ObjectID `bson:"userId"`
	Phone         string        `bson:"phone"`
	Tags          []string      `bson:"tags"`
	PracticeNotes string        `bson:"practiceNotes"`
	CreatedAt     bson.DateTime `bson:"createdAt"`
	UpdatedAt     bson.DateTime `bson:"updatedAt"`
}

// Upsert persists the profile keyed on the user id: the mutable practice
// fields are always set; userId and createdAt are written only on insert,
// so an existing profile keeps its original creation stamp.
func (r *ClientProfileRepository) Upsert(ctx context.Context, profile client.Profile) (client.Profile, error) {
	userOID, err := bson.ObjectIDFromHex(profile.UserID)
	if err != nil {
		return client.Profile{}, fmt.Errorf("profile userId %q is not an ObjectID: %w", profile.UserID, err)
	}
	update := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "phone", Value: profile.Phone},
			{Key: "tags", Value: profile.Tags},
			{Key: "practiceNotes", Value: profile.PracticeNotes},
			{Key: "updatedAt", Value: bson.NewDateTimeFromTime(profile.UpdatedAt)},
		}},
		{Key: "$setOnInsert", Value: bson.D{
			{Key: "userId", Value: userOID},
			{Key: "createdAt", Value: bson.NewDateTimeFromTime(profile.CreatedAt)},
		}},
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"userId": userOID}, update, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return client.Profile{}, fmt.Errorf("upsert client profile: %w", err)
	}
	if oid, ok := res.UpsertedID.(bson.ObjectID); ok {
		profile.ID = oid.Hex()
	}
	if profile.ID == "" {
		// Matched an existing document the caller didn't know the id of —
		// read it back so the returned profile is complete.
		existing, err := r.FindByUserID(ctx, profile.UserID)
		if err != nil {
			return client.Profile{}, err
		}
		profile.ID = existing.ID
		profile.CreatedAt = existing.CreatedAt
	}
	return profile, nil
}

// FindByUserID looks up the profile for a client account; misses return
// client.ErrProfileNotFound.
func (r *ClientProfileRepository) FindByUserID(ctx context.Context, userID string) (client.Profile, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return client.Profile{}, client.ErrProfileNotFound
	}
	var doc clientProfileDoc
	err = r.coll.FindOne(ctx, bson.M{"userId": oid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return client.Profile{}, client.ErrProfileNotFound
		}
		return client.Profile{}, fmt.Errorf("find client profile: %w", err)
	}
	return clientProfileFromDoc(doc), nil
}

func clientProfileFromDoc(doc clientProfileDoc) client.Profile {
	tags := doc.Tags
	if tags == nil {
		tags = []string{}
	}
	return client.Profile{
		ID:            doc.ID.Hex(),
		UserID:        doc.UserID.Hex(),
		Phone:         doc.Phone,
		Tags:          tags,
		PracticeNotes: doc.PracticeNotes,
		CreatedAt:     doc.CreatedAt.Time(),
		UpdatedAt:     doc.UpdatedAt.Time(),
	}
}
