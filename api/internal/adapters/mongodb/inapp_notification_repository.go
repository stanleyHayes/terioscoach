package mongodb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// InAppNotificationRepository persists the in-app notification feed in the
// in_app_notifications collection.
type InAppNotificationRepository struct {
	coll *mongo.Collection
}

var _ ports.InAppNotificationRepository = (*InAppNotificationRepository)(nil)

// NewInAppNotificationRepository binds the repository to
// in_app_notifications.
func NewInAppNotificationRepository(db *mongo.Database) *InAppNotificationRepository {
	return &InAppNotificationRepository{coll: db.Collection("in_app_notifications")}
}

type inAppNotificationDoc struct {
	EventID        string        `bson:"eventId,omitempty"`
	ID             bson.ObjectID `bson:"_id,omitempty"`
	RecipientEmail string        `bson:"recipientEmail"`
	Kind           string        `bson:"kind"`
	Title          string        `bson:"title"`
	Body           string        `bson:"body,omitempty"`
	Link           string        `bson:"link,omitempty"`
	BookingID      string        `bson:"bookingId,omitempty"`
	Read           bool          `bson:"read"`
	CreatedAt      bson.DateTime `bson:"createdAt"`
}

func newInAppNotificationDoc(n notification.InApp) inAppNotificationDoc {
	return inAppNotificationDoc{
		EventID:        n.EventID,
		RecipientEmail: strings.ToLower(strings.TrimSpace(n.RecipientEmail)),
		Kind:           string(n.Kind),
		Title:          n.Title,
		Body:           n.Body,
		Link:           n.Link,
		BookingID:      n.BookingID,
		Read:           n.Read,
		CreatedAt:      bson.NewDateTimeFromTime(n.CreatedAt),
	}
}

func (d inAppNotificationDoc) toDomain() notification.InApp {
	return notification.InApp{
		ID:             d.ID.Hex(),
		EventID:        d.EventID,
		RecipientEmail: d.RecipientEmail,
		Kind:           notification.Kind(d.Kind),
		Title:          d.Title,
		Body:           d.Body,
		Link:           d.Link,
		BookingID:      d.BookingID,
		Read:           d.Read,
		CreatedAt:      d.CreatedAt.Time().UTC(),
	}
}

// Create inserts a new unread notification, assigning its id.
func (r *InAppNotificationRepository) Create(ctx context.Context, n notification.InApp) (notification.InApp, error) {
	doc := newInAppNotificationDoc(n)
	if n.EventID != "" {
		var stored inAppNotificationDoc
		err := r.coll.FindOneAndUpdate(ctx, bson.M{"eventId": n.EventID, "recipientEmail": doc.RecipientEmail}, bson.M{"$setOnInsert": doc}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&stored)
		if mongo.IsDuplicateKeyError(err) {
			err = r.coll.FindOne(ctx, bson.M{"eventId": n.EventID, "recipientEmail": doc.RecipientEmail}).Decode(&stored)
		}
		if err != nil {
			return notification.InApp{}, fmt.Errorf("upsert in-app notification: %w", err)
		}
		return stored.toDomain(), nil
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return notification.InApp{}, fmt.Errorf("insert in-app notification: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		n.ID = oid.Hex()
	}
	return n, nil
}

// ListForRecipient returns recipientEmail's notifications, most recent
// first, capped at limit.
func (r *InAppNotificationRepository) ListForRecipient(ctx context.Context, recipientEmail string, limit int) ([]notification.InApp, error) {
	if limit <= 0 {
		limit = 50
	}
	cursor, err := r.coll.Find(ctx,
		bson.M{"recipientEmail": strings.ToLower(recipientEmail)},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("find in-app notifications: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []inAppNotificationDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode in-app notifications: %w", err)
	}
	items := make([]notification.InApp, 0, len(docs))
	for _, doc := range docs {
		items = append(items, doc.toDomain())
	}
	return items, nil
}

// UnreadCount reports how many of recipientEmail's notifications are still
// unread.
func (r *InAppNotificationRepository) UnreadCount(ctx context.Context, recipientEmail string) (int, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{
		"recipientEmail": strings.ToLower(recipientEmail),
		"read":           false,
	})
	if err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return int(count), nil
}

// MarkRead marks one notification read, scoped to recipientEmail so one
// person can never mark another person's notification read.
func (r *InAppNotificationRepository) MarkRead(ctx context.Context, id, recipientEmail string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return notification.ErrInAppNotFound
	}
	res, err := r.coll.UpdateOne(ctx,
		bson.M{"_id": oid, "recipientEmail": strings.ToLower(recipientEmail)},
		bson.M{"$set": bson.M{"read": true}},
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return notification.ErrInAppNotFound
		}
		return fmt.Errorf("mark notification read: %w", err)
	}
	if res.MatchedCount == 0 {
		return notification.ErrInAppNotFound
	}
	return nil
}

// MarkAllRead marks every unread notification for recipientEmail read.
func (r *InAppNotificationRepository) MarkAllRead(ctx context.Context, recipientEmail string) error {
	_, err := r.coll.UpdateMany(ctx,
		bson.M{"recipientEmail": strings.ToLower(recipientEmail), "read": false},
		bson.M{"$set": bson.M{"read": true}},
	)
	if err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}
