package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/review"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ReviewRepository persists post-session reviews in the reviews
// collection. One review per booking is enforced by a unique index, so a
// double submit loses the race rather than creating a second row.
type ReviewRepository struct {
	coll *mongo.Collection
}

var _ ports.ReviewRepository = (*ReviewRepository)(nil)

// NewReviewRepository binds the repository to reviews.
func NewReviewRepository(db *mongo.Database) *ReviewRepository {
	return &ReviewRepository{coll: db.Collection("reviews")}
}

type reviewDoc struct {
	ID             bson.ObjectID  `bson:"_id,omitempty"`
	BookingID      string         `bson:"bookingId"`
	ClientID       string         `bson:"clientId"`
	PractitionerID string         `bson:"practitionerId"`
	ServiceID      string         `bson:"serviceId,omitempty"`
	Rating         int            `bson:"rating"`
	Comment        string         `bson:"comment,omitempty"`
	Status         string         `bson:"status"`
	ModeratedAt    *bson.DateTime `bson:"moderatedAt,omitempty"`
	CreatedAt      bson.DateTime  `bson:"createdAt"`
	UpdatedAt      bson.DateTime  `bson:"updatedAt"`
}

func newReviewDoc(r review.Review) reviewDoc {
	doc := reviewDoc{
		BookingID:      r.BookingID,
		ClientID:       r.ClientID,
		PractitionerID: r.PractitionerID,
		ServiceID:      r.ServiceID,
		Rating:         r.Rating,
		Comment:        r.Comment,
		Status:         string(r.Status),
		CreatedAt:      bson.NewDateTimeFromTime(r.CreatedAt),
		UpdatedAt:      bson.NewDateTimeFromTime(r.UpdatedAt),
	}
	if r.ModeratedAt != nil {
		stamp := bson.NewDateTimeFromTime(*r.ModeratedAt)
		doc.ModeratedAt = &stamp
	}
	return doc
}

func (d reviewDoc) toDomain() review.Review {
	r := review.Review{
		ID:             d.ID.Hex(),
		BookingID:      d.BookingID,
		ClientID:       d.ClientID,
		PractitionerID: d.PractitionerID,
		ServiceID:      d.ServiceID,
		Rating:         d.Rating,
		Comment:        d.Comment,
		Status:         review.Status(d.Status),
		CreatedAt:      d.CreatedAt.Time().UTC(),
		UpdatedAt:      d.UpdatedAt.Time().UTC(),
	}
	if d.ModeratedAt != nil {
		at := d.ModeratedAt.Time().UTC()
		r.ModeratedAt = &at
	}
	return r
}

func (r *ReviewRepository) Create(ctx context.Context, rev review.Review) (review.Review, error) {
	res, err := r.coll.InsertOne(ctx, newReviewDoc(rev))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return review.Review{}, review.ErrReviewExists
		}
		return review.Review{}, fmt.Errorf("insert review: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		rev.ID = oid.Hex()
	}
	return rev, nil
}

func (r *ReviewRepository) Update(ctx context.Context, rev review.Review) (review.Review, error) {
	oid, err := bson.ObjectIDFromHex(rev.ID)
	if err != nil {
		return review.Review{}, review.ErrReviewNotFound
	}
	doc := newReviewDoc(rev)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"rating":      doc.Rating,
		"comment":     doc.Comment,
		"status":      doc.Status,
		"moderatedAt": doc.ModeratedAt,
		"updatedAt":   doc.UpdatedAt,
	}})
	if err != nil {
		return review.Review{}, fmt.Errorf("update review: %w", err)
	}
	if res.MatchedCount == 0 {
		return review.Review{}, review.ErrReviewNotFound
	}
	return rev, nil
}

func (r *ReviewRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return review.ErrReviewNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	return nil
}

func (r *ReviewRepository) FindByID(ctx context.Context, id string) (review.Review, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return review.Review{}, review.ErrReviewNotFound
	}
	var doc reviewDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return review.Review{}, review.ErrReviewNotFound
		}
		return review.Review{}, fmt.Errorf("find review: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *ReviewRepository) FindByBookingID(ctx context.Context, bookingID string) (review.Review, error) {
	var doc reviewDoc
	if err := r.coll.FindOne(ctx, bson.M{"bookingId": bookingID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return review.Review{}, review.ErrReviewNotFound
		}
		return review.Review{}, fmt.Errorf("find review by booking: %w", err)
	}
	return doc.toDomain(), nil
}

// ListByClient leads with clientId — the isolation rule every client-scoped
// query in this API follows.
func (r *ReviewRepository) ListByClient(ctx context.Context, clientID string) ([]review.Review, error) {
	return r.find(ctx, bson.M{"clientId": clientID}, 0)
}

func (r *ReviewRepository) ListByPractitioner(ctx context.Context, practitionerID string, filter ports.ReviewFilter) ([]review.Review, error) {
	query := bson.M{"practitionerId": practitionerID}
	if filter.Status != "" {
		query["status"] = string(filter.Status)
	}
	if filter.ApprovedOnly {
		query["status"] = string(review.StatusApproved)
	}
	return r.find(ctx, query, 0)
}

// ListPublic returns approved reviews newest-first. A limit of 0 means all
// of them, which is what the summary aggregate needs.
func (r *ReviewRepository) ListPublic(ctx context.Context, limit int) ([]review.Review, error) {
	return r.find(ctx, bson.M{"status": string(review.StatusApproved)}, limit)
}

func (r *ReviewRepository) find(ctx context.Context, query bson.M, limit int) ([]review.Review, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	if limit > 0 {
		opts = opts.SetLimit(int64(limit))
	}
	cursor, err := r.coll.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []reviewDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode reviews: %w", err)
	}
	out := make([]review.Review, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}
