package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RefreshTokenRepository persists refresh sessions in the refresh_tokens
// collection. Only token hashes are stored; the TTL index on expiresAt
// reaps expired sessions (see indexes.go).
type RefreshTokenRepository struct {
	coll *mongo.Collection
}

var _ ports.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

// NewRefreshTokenRepository binds the repository to refresh_tokens.
func NewRefreshTokenRepository(db *mongo.Database) *RefreshTokenRepository {
	return &RefreshTokenRepository{coll: db.Collection("refresh_tokens")}
}

// refreshTokenDoc is the storage shape; userId is an ObjectID reference.
type refreshTokenDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	TokenHash string        `bson:"tokenHash"`
	UserID    bson.ObjectID `bson:"userId"`
	ExpiresAt bson.DateTime `bson:"expiresAt"`
	Revoked   bool          `bson:"revoked"`
	CreatedAt bson.DateTime `bson:"createdAt"`
}

// Store inserts a new session keyed by token hash.
func (r *RefreshTokenRepository) Store(ctx context.Context, token identity.RefreshToken) error {
	userID, err := bson.ObjectIDFromHex(token.UserID)
	if err != nil {
		return fmt.Errorf("refresh token user id: %w", err)
	}
	doc := refreshTokenDoc{
		TokenHash: token.TokenHash,
		UserID:    userID,
		ExpiresAt: bson.NewDateTimeFromTime(token.ExpiresAt),
		Revoked:   token.Revoked,
		CreatedAt: bson.NewDateTimeFromTime(token.CreatedAt),
	}
	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// FindByHash looks up a session; misses return identity.ErrTokenInvalid
// (an unknown token is simply an unusable one).
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (identity.RefreshToken, error) {
	var doc refreshTokenDoc
	err := r.coll.FindOne(ctx, bson.M{"tokenHash": tokenHash}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return identity.RefreshToken{}, identity.ErrTokenInvalid
		}
		return identity.RefreshToken{}, fmt.Errorf("find refresh token: %w", err)
	}
	return identity.RefreshToken{
		TokenHash: doc.TokenHash,
		UserID:    doc.UserID.Hex(),
		ExpiresAt: doc.ExpiresAt.Time(),
		Revoked:   doc.Revoked,
		CreatedAt: doc.CreatedAt.Time(),
	}, nil
}

// Revoke marks a session revoked. Missing sessions are not an error, so
// logout and rotation stay idempotent.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, tokenHash string) error {
	_, err := r.coll.UpdateOne(ctx,
		bson.M{"tokenHash": tokenHash},
		bson.M{"$set": bson.M{"revoked": true}},
	)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
