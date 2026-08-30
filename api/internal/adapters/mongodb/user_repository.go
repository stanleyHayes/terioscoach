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
	MFASecret              string        `bson:"mfaSecret,omitempty"`
	MFAEnabled             bool          `bson:"mfaEnabled,omitempty"`
	RoleName               string        `bson:"roleName,omitempty"`
	Permissions            []string      `bson:"permissions,omitempty"`
	Disabled               bool          `bson:"disabled,omitempty"`
}

func (r *UserRepository) SetMFAPending(ctx context.Context, userID, encryptedSecret string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return identity.ErrUserNotFound
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"mfaSecret": encryptedSecret, "mfaEnabled": false}})
	if err != nil {
		return fmt.Errorf("store MFA enrollment: %w", err)
	}
	if res.MatchedCount == 0 {
		return identity.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) EnableMFA(ctx context.Context, userID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return identity.ErrUserNotFound
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid, "mfaSecret": bson.M{"$ne": ""}}, bson.M{"$set": bson.M{"mfaEnabled": true}})
	if err != nil {
		return fmt.Errorf("enable MFA: %w", err)
	}
	if res.MatchedCount == 0 {
		return identity.ErrMFANotPending
	}
	return nil
}

func (r *UserRepository) DisableMFA(ctx context.Context, userID string) error {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return identity.ErrUserNotFound
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$unset": bson.M{"mfaSecret": "", "mfaEnabled": ""}})
	if err != nil {
		return fmt.Errorf("disable MFA: %w", err)
	}
	if res.MatchedCount == 0 {
		return identity.ErrUserNotFound
	}
	return nil
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
		MFASecret:    user.MFASecret,
		MFAEnabled:   user.MFAEnabled,
		RoleName:     user.RoleName,
		Permissions:  permissionStrings(user.Permissions.List()),
		Disabled:     user.Disabled,
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

func (r *UserRepository) ListStaff(ctx context.Context) ([]identity.User, error) {
	cursor, err := r.coll.Find(ctx, bson.M{"role": bson.M{"$in": []string{string(identity.RolePractitioner), string(identity.RoleStaff)}}}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	defer cursor.Close(ctx)
	var docs []userDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode staff: %w", err)
	}
	users := make([]identity.User, 0, len(docs))
	for _, doc := range docs {
		users = append(users, userFromDoc(doc))
	}
	return users, nil
}

func (r *UserRepository) UpdateStaffAccess(ctx context.Context, userID, name, roleName string, permissions []identity.Permission, disabled bool) (identity.User, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return identity.User{}, identity.ErrUserNotFound
	}
	var doc userDoc
	err = r.coll.FindOneAndUpdate(ctx, bson.M{"_id": oid, "role": string(identity.RoleStaff)}, bson.M{"$set": bson.M{"name": name, "roleName": roleName, "permissions": permissionStrings(permissions), "disabled": disabled}}, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return identity.User{}, identity.ErrUserNotFound
	}
	if err != nil {
		return identity.User{}, fmt.Errorf("update staff access: %w", err)
	}
	return userFromDoc(doc), nil
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
	return userFromDoc(doc), nil
}

func userFromDoc(doc userDoc) identity.User {
	return identity.User{
		ID:           doc.ID.Hex(),
		Email:        doc.Email,
		PasswordHash: doc.PasswordHash,
		Role:         identity.Role(doc.Role),
		Name:         doc.Name,
		CreatedAt:    doc.CreatedAt.Time(),
		MFASecret:    doc.MFASecret,
		MFAEnabled:   doc.MFAEnabled,
		RoleName:     doc.RoleName,
		Permissions:  identity.NewPermissionSet(permissionsFromStrings(doc.Permissions)...),
		Disabled:     doc.Disabled,
	}
}

func permissionStrings(values []identity.Permission) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	return out
}
func permissionsFromStrings(values []string) []identity.Permission {
	out := make([]identity.Permission, 0, len(values))
	for _, value := range values {
		permission := identity.Permission(value)
		if permission.Valid() {
			out = append(out, permission)
		}
	}
	return out
}
