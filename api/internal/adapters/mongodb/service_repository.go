package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ServiceRepository persists services in the services collection. Storage
// field names match the seed tool (durationMin, priceKobo, ...).
type ServiceRepository struct {
	coll     *mongo.Collection
	bookings *mongo.Collection
}

var _ ports.ServiceRepository = (*ServiceRepository)(nil)

// NewServiceRepository binds the repository to the services collection;
// HasBookings reads the bookings collection (schema lands in BE-05).
func NewServiceRepository(db *mongo.Database) *ServiceRepository {
	return &ServiceRepository{
		coll:     db.Collection("services"),
		bookings: db.Collection("bookings"),
	}
}

// serviceDoc is the storage shape; kept separate from the domain entity.
type serviceDoc struct {
	ID             bson.ObjectID  `bson:"_id,omitempty"`
	PractitionerID bson.ObjectID  `bson:"practitionerId"`
	Name           string         `bson:"name"`
	Description    string         `bson:"description"`
	DurationMin    int            `bson:"durationMin"`
	PriceKobo      int64          `bson:"priceKobo"`
	Currency       string         `bson:"currency"`
	Active         bool           `bson:"active"`
	SortOrder      int            `bson:"sortOrder"`
	CreatedAt      bson.DateTime  `bson:"createdAt"`
	UpdatedAt      bson.DateTime  `bson:"updatedAt"`
	DeletedAt      *bson.DateTime `bson:"deletedAt,omitempty"`
}

// Create inserts a new service, assigning its ID.
func (r *ServiceRepository) Create(ctx context.Context, svc catalog.Service) (catalog.Service, error) {
	doc, err := newServiceDoc(svc)
	if err != nil {
		return catalog.Service{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return catalog.Service{}, fmt.Errorf("insert service: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		svc.ID = oid.Hex()
	}
	return svc, nil
}

// FindByID looks up a non-deleted service by hex ObjectID.
func (r *ServiceRepository) FindByID(ctx context.Context, id string) (catalog.Service, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return catalog.Service{}, catalog.ErrServiceNotFound
	}
	return r.findOne(ctx, bson.M{"_id": oid, "deletedAt": bson.M{"$exists": false}})
}

// ListByPractitioner returns non-deleted services ordered by sortOrder
// then createdAt. An empty practitionerID lists across practitioners —
// the platform has a single one, and the public catalog route is
// unauthenticated so it cannot scope by identity.
func (r *ServiceRepository) ListByPractitioner(ctx context.Context, practitionerID string, activeOnly bool) ([]catalog.Service, error) {
	filter := bson.M{"deletedAt": bson.M{"$exists": false}}
	if practitionerID != "" {
		oid, err := bson.ObjectIDFromHex(practitionerID)
		if err != nil {
			return nil, nil
		}
		filter["practitionerId"] = oid
	}
	if activeOnly {
		filter["active"] = true
	}

	opts := options.Find().SetSort(bson.D{{Key: "sortOrder", Value: 1}, {Key: "createdAt", Value: 1}})
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []catalog.Service
	for cursor.Next(ctx) {
		var doc serviceDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode service: %w", err)
		}
		out = append(out, serviceFromDoc(doc))
	}
	return out, cursor.Err()
}

// Update replaces a service's mutable fields, including the soft-delete
// marker (DeletedAt) the app layer sets when bookings exist. Misses return
// catalog.ErrServiceNotFound.
func (r *ServiceRepository) Update(ctx context.Context, svc catalog.Service) (catalog.Service, error) {
	oid, err := bson.ObjectIDFromHex(svc.ID)
	if err != nil {
		return catalog.Service{}, catalog.ErrServiceNotFound
	}
	doc, err := newServiceDoc(svc)
	if err != nil {
		return catalog.Service{}, err
	}
	doc.ID = oid

	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		return catalog.Service{}, fmt.Errorf("update service: %w", err)
	}
	if res.MatchedCount == 0 {
		return catalog.Service{}, catalog.ErrServiceNotFound
	}
	return svc, nil
}

// Delete hard-deletes a service. Misses are not an error.
func (r *ServiceRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// HasBookings reports whether any booking references the service — the
// soft-delete trigger. Reads the bookings collection written by BE-05.
func (r *ServiceRepository) HasBookings(ctx context.Context, serviceID string) (bool, error) {
	oid, err := bson.ObjectIDFromHex(serviceID)
	if err != nil {
		return false, nil
	}
	count, err := r.bookings.CountDocuments(ctx, bson.M{"serviceId": oid}, options.Count().SetLimit(1))
	if err != nil {
		return false, fmt.Errorf("count bookings for service: %w", err)
	}
	return count > 0, nil
}

func (r *ServiceRepository) findOne(ctx context.Context, filter bson.M) (catalog.Service, error) {
	var doc serviceDoc
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return catalog.Service{}, catalog.ErrServiceNotFound
		}
		return catalog.Service{}, fmt.Errorf("find service: %w", err)
	}
	return serviceFromDoc(doc), nil
}

// newServiceDoc maps the domain entity to storage. practitionerId must be
// a hex ObjectID — it always originates from the authenticated identity.
func newServiceDoc(svc catalog.Service) (serviceDoc, error) {
	practitionerOID, err := bson.ObjectIDFromHex(svc.PractitionerID)
	if err != nil {
		return serviceDoc{}, fmt.Errorf("service practitionerId %q is not an ObjectID: %w", svc.PractitionerID, err)
	}
	doc := serviceDoc{
		PractitionerID: practitionerOID,
		Name:           svc.Name,
		Description:    svc.Description,
		DurationMin:    svc.DurationMinutes,
		PriceKobo:      svc.PriceKobo,
		Currency:       svc.Currency,
		Active:         svc.Active,
		SortOrder:      svc.SortOrder,
		CreatedAt:      bson.NewDateTimeFromTime(svc.CreatedAt),
		UpdatedAt:      bson.NewDateTimeFromTime(svc.UpdatedAt),
	}
	if svc.ID != "" {
		oid, err := bson.ObjectIDFromHex(svc.ID)
		if err != nil {
			return serviceDoc{}, fmt.Errorf("service id %q is not an ObjectID: %w", svc.ID, err)
		}
		doc.ID = oid
	}
	if svc.DeletedAt != nil {
		deleted := bson.NewDateTimeFromTime(*svc.DeletedAt)
		doc.DeletedAt = &deleted
	}
	return doc, nil
}

func serviceFromDoc(doc serviceDoc) catalog.Service {
	svc := catalog.Service{
		ID:              doc.ID.Hex(),
		PractitionerID:  doc.PractitionerID.Hex(),
		Name:            doc.Name,
		Description:     doc.Description,
		DurationMinutes: doc.DurationMin,
		PriceKobo:       doc.PriceKobo,
		Currency:        doc.Currency,
		Active:          doc.Active,
		SortOrder:       doc.SortOrder,
		CreatedAt:       doc.CreatedAt.Time(),
		UpdatedAt:       doc.UpdatedAt.Time(),
	}
	if svc.Currency == "" {
		// Rows written before the currency field existed (early seeds).
		svc.Currency = catalog.DefaultCurrency
	}
	if doc.DeletedAt != nil {
		deleted := doc.DeletedAt.Time()
		svc.DeletedAt = &deleted
	}
	return svc
}
