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

// UserRepository persists accounts in the users collection. Email is
// globally unique (enforced by the index in indexes.go).
type UserRepository struct {
	coll *mongo.Collection
}

var _ ports.UserRepository = (*UserRepository)(nil)

// NewUserRepository binds the repository to the users collection.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection("users")}
}

// userDoc is the storage shape; kept separate from the domain entity.
type userDoc struct {
	ID                     bson.ObjectID `bson:"_id,omitempty"`
	Email                  string        `bson:"email"`
	PasswordHash           string        `bson:"passwordHash"`
	Role                   string        `bson:"role"`
	Name                   string        `bson:"name"`
	CreatedAt              bson.DateTime `bson:"createdAt"`
	PasswordResetTokenHash string        `bson:"passwordResetTokenHash,omitempty"`
	PasswordResetExpiresAt bson.DateTime `bson:"passwordResetExpiresAt,omitempty"`
}

func (r *UserRepository) SetPasswordReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return identity.ErrUserNotFound
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"passwordResetTokenHash": tokenHash, "passwordResetExpiresAt": bson.NewDateTimeFromTime(expiresAt)}})
	if err != nil {
		return fmt.Errorf("store password reset: %w", err)
	}
	return nil
}

func (r *UserRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string, now time.Time) (string, error) {
	var doc userDoc
	err := r.coll.FindOneAndUpdate(ctx, bson.M{"passwordResetTokenHash": tokenHash, "passwordResetExpiresAt": bson.M{"$gt": bson.NewDateTimeFromTime(now)}}, bson.M{"$set": bson.M{"passwordHash": passwordHash}, "$unset": bson.M{"passwordResetTokenHash": "", "passwordResetExpiresAt": ""}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", identity.ErrPasswordResetInvalid
	}
	if err != nil {
		return "", fmt.Errorf("reset password: %w", err)
	}
	return doc.ID.Hex(), nil
}

// Create inserts a new account, mapping the unique-index violation to
// identity.ErrEmailTaken.
func (r *UserRepository) Create(ctx context.Context, user identity.User) (identity.User, error) {
	doc := userDoc{
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
		Name:         user.Name,
		CreatedAt:    bson.NewDateTimeFromTime(user.CreatedAt),
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return identity.User{}, identity.ErrEmailTaken
		}
		return identity.User{}, fmt.Errorf("insert user: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		user.ID = oid.Hex()
	}
	return user, nil
}

// FindByEmail looks up an account by normalized email.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (identity.User, error) {
	return r.findOne(ctx, bson.M{"email": email})
}

// FindFirstByRole looks up one account by role.
func (r *UserRepository) FindFirstByRole(ctx context.Context, role identity.Role) (identity.User, error) {
	return r.findOne(ctx, bson.M{"role": string(role)})
}

// FindByID looks up an account by hex ObjectID.
func (r *UserRepository) FindByID(ctx context.Context, id string) (identity.User, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return identity.User{}, identity.ErrUserNotFound
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

func (r *UserRepository) findOne(ctx context.Context, filter bson.M) (identity.User, error) {
	var doc userDoc
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return identity.User{}, identity.ErrUserNotFound
		}
		return identity.User{}, fmt.Errorf("find user: %w", err)
	}
	return identity.User{
		ID:           doc.ID.Hex(),
		Email:        doc.Email,
		PasswordHash: doc.PasswordHash,
		Role:         identity.Role(doc.Role),
		Name:         doc.Name,
		CreatedAt:    doc.CreatedAt.Time(),
	}, nil
}
