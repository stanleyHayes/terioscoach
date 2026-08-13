package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/enquiry"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnquiryRepository persists contact-form enquiries in the enquiries
// collection, listed newest-first for the practice inbox.
type EnquiryRepository struct {
	coll *mongo.Collection
}

var _ ports.EnquiryRepository = (*EnquiryRepository)(nil)

// NewEnquiryRepository binds the repository to enquiries.
func NewEnquiryRepository(db *mongo.Database) *EnquiryRepository {
	return &EnquiryRepository{coll: db.Collection("enquiries")}
}

type enquiryDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Email     string        `bson:"email"`
	Phone     string        `bson:"phone,omitempty"`
	Subject   string        `bson:"subject,omitempty"`
	Message   string        `bson:"message"`
	Status    string        `bson:"status"`
	SourceIP  string        `bson:"sourceIp,omitempty"`
	CreatedAt bson.DateTime `bson:"createdAt"`
	UpdatedAt bson.DateTime `bson:"updatedAt"`
}

func newEnquiryDoc(e enquiry.Enquiry) enquiryDoc {
	return enquiryDoc{
		Name:      e.Name,
		Email:     e.Email,
		Phone:     e.Phone,
		Subject:   e.Subject,
		Message:   e.Message,
		Status:    string(e.Status),
		SourceIP:  e.SourceIP,
		CreatedAt: bson.NewDateTimeFromTime(e.CreatedAt),
		UpdatedAt: bson.NewDateTimeFromTime(e.UpdatedAt),
	}
}

func (d enquiryDoc) toDomain() enquiry.Enquiry {
	return enquiry.Enquiry{
		ID:        d.ID.Hex(),
		Name:      d.Name,
		Email:     d.Email,
		Phone:     d.Phone,
		Subject:   d.Subject,
		Message:   d.Message,
		Status:    enquiry.Status(d.Status),
		SourceIP:  d.SourceIP,
		CreatedAt: d.CreatedAt.Time().UTC(),
		UpdatedAt: d.UpdatedAt.Time().UTC(),
	}
}

func (r *EnquiryRepository) Create(ctx context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error) {
	res, err := r.coll.InsertOne(ctx, newEnquiryDoc(e))
	if err != nil {
		return enquiry.Enquiry{}, fmt.Errorf("insert enquiry: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		e.ID = oid.Hex()
	}
	return e, nil
}

// Update persists a triage change. Only the mutable fields are written —
// the message itself is what a stranger sent and is never rewritten.
func (r *EnquiryRepository) Update(ctx context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error) {
	oid, err := bson.ObjectIDFromHex(e.ID)
	if err != nil {
		return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"status":    string(e.Status),
		"updatedAt": bson.NewDateTimeFromTime(e.UpdatedAt),
	}})
	if err != nil {
		return enquiry.Enquiry{}, fmt.Errorf("update enquiry: %w", err)
	}
	if res.MatchedCount == 0 {
		return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
	}
	return e, nil
}

func (r *EnquiryRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return enquiry.ErrEnquiryNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete enquiry: %w", err)
	}
	return nil
}

func (r *EnquiryRepository) FindByID(ctx context.Context, id string) (enquiry.Enquiry, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
	}
	var doc enquiryDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
		}
		return enquiry.Enquiry{}, fmt.Errorf("find enquiry: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *EnquiryRepository) List(ctx context.Context, filter ports.EnquiryFilter) ([]enquiry.Enquiry, error) {
	query := bson.M{}
	if filter.Status != "" {
		query["status"] = string(filter.Status)
	}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list enquiries: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []enquiryDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode enquiries: %w", err)
	}
	out := make([]enquiry.Enquiry, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.toDomain())
	}
	return out, nil
}

func (r *EnquiryRepository) CountByStatus(ctx context.Context, status enquiry.Status) (int, error) {
	count, err := r.coll.CountDocuments(ctx, bson.M{"status": string(status)})
	if err != nil {
		return 0, fmt.Errorf("count enquiries: %w", err)
	}
	return int(count), nil
}
