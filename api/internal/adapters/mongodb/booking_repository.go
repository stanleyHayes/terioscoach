package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// BookingRepository persists bookings in the bookings collection. The
// document shape ({clientId, practitionerId, serviceId, startAt, endAt,
// status}) is fixed by BE-03/BE-04: BusyIntervalReader reads exactly these
// fields, and the unique_confirmed_slot partial index guards
// {practitionerId, startAt} for status=confirmed.
type BookingRepository struct {
	coll *mongo.Collection
}

var _ ports.BookingRepository = (*BookingRepository)(nil)

// NewBookingRepository binds the repository to the bookings collection.
func NewBookingRepository(db *mongo.Database) *BookingRepository {
	return &BookingRepository{coll: db.Collection("bookings")}
}

// bookingDoc is the storage shape; kept separate from the domain entity.
type bookingDoc struct {
	ID             bson.ObjectID  `bson:"_id,omitempty"`
	ClientID       bson.ObjectID  `bson:"clientId"`
	PractitionerID bson.ObjectID  `bson:"practitionerId"`
	ServiceID      bson.ObjectID  `bson:"serviceId"`
	StartAt        bson.DateTime  `bson:"startAt"`
	EndAt          bson.DateTime  `bson:"endAt"`
	Status         string         `bson:"status"`
	CreatedAt      bson.DateTime  `bson:"createdAt"`
	UpdatedAt      bson.DateTime  `bson:"updatedAt"`
	CancelledAt    *bson.DateTime `bson:"cancelledAt,omitempty"`
	CompletedAt    *bson.DateTime `bson:"completedAt,omitempty"`
	PaymentStatus  string         `bson:"paymentStatus,omitempty"`
	PaidAt         *bson.DateTime `bson:"paidAt,omitempty"`
}

// Create inserts a new booking, assigning its ID. A duplicate-key error
// from the unique_confirmed_slot index means a concurrent booking claimed
// the slot first — surfaced as the domain conflict.
func (r *BookingRepository) Create(ctx context.Context, b booking.Booking) (booking.Booking, error) {
	doc, err := newBookingDoc(b)
	if err != nil {
		return booking.Booking{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return booking.Booking{}, booking.ErrSlotUnavailable
		}
		return booking.Booking{}, fmt.Errorf("insert booking: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		b.ID = oid.Hex()
	}
	return b, nil
}

// FindByID looks up a booking by hex ObjectID; misses return
// booking.ErrBookingNotFound.
func (r *BookingRepository) FindByID(ctx context.Context, id string) (booking.Booking, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	var doc bookingDoc
	err = r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return booking.Booking{}, booking.ErrBookingNotFound
		}
		return booking.Booking{}, fmt.Errorf("find booking: %w", err)
	}
	return bookingFromDoc(doc), nil
}

// Update replaces a booking's mutable state. Misses return
// booking.ErrBookingNotFound; a confirmed-slot collision on reschedule
// returns booking.ErrSlotUnavailable.
func (r *BookingRepository) Update(ctx context.Context, b booking.Booking) (booking.Booking, error) {
	oid, err := bson.ObjectIDFromHex(b.ID)
	if err != nil {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	doc, err := newBookingDoc(b)
	if err != nil {
		return booking.Booking{}, err
	}
	doc.ID = oid

	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return booking.Booking{}, booking.ErrSlotUnavailable
		}
		return booking.Booking{}, fmt.Errorf("update booking: %w", err)
	}
	if res.MatchedCount == 0 {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

// ListByClient returns the client's bookings ordered by startAt ascending.
// Client data isolation leads every client-scoped query with clientId.
func (r *BookingRepository) ListByClient(ctx context.Context, clientID string) ([]booking.Booking, error) {
	oid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return nil, nil
	}
	return r.list(ctx, bson.M{"clientId": oid})
}

// ListByPractitioner returns the practitioner's bookings matching the
// filter, ordered by startAt ascending.
func (r *BookingRepository) ListByPractitioner(ctx context.Context, practitionerID string, filter ports.BookingFilter) ([]booking.Booking, error) {
	oid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return nil, nil
	}
	m := bson.M{"practitionerId": oid}
	if filter.From != nil || filter.To != nil {
		start := bson.M{}
		if filter.From != nil {
			start["$gte"] = bson.NewDateTimeFromTime(*filter.From)
		}
		if filter.To != nil {
			start["$lt"] = bson.NewDateTimeFromTime(*filter.To)
		}
		m["startAt"] = start
	}
	if filter.Status != "" {
		m["status"] = string(filter.Status)
	}
	return r.list(ctx, m)
}

func (r *BookingRepository) list(ctx context.Context, filter bson.M) ([]booking.Booking, error) {
	opts := options.Find().SetSort(bson.D{{Key: "startAt", Value: 1}})
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []booking.Booking
	for cursor.Next(ctx) {
		var doc bookingDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode booking: %w", err)
		}
		out = append(out, bookingFromDoc(doc))
	}
	return out, cursor.Err()
}

// newBookingDoc maps the domain entity to storage. IDs must be hex
// ObjectIDs — they always originate from authenticated identities and
// stored services.
func newBookingDoc(b booking.Booking) (bookingDoc, error) {
	clientOID, err := bson.ObjectIDFromHex(b.ClientID)
	if err != nil {
		return bookingDoc{}, fmt.Errorf("booking clientId %q is not an ObjectID: %w", b.ClientID, err)
	}
	practitionerOID, err := bson.ObjectIDFromHex(b.PractitionerID)
	if err != nil {
		return bookingDoc{}, fmt.Errorf("booking practitionerId %q is not an ObjectID: %w", b.PractitionerID, err)
	}
	serviceOID, err := bson.ObjectIDFromHex(b.ServiceID)
	if err != nil {
		return bookingDoc{}, fmt.Errorf("booking serviceId %q is not an ObjectID: %w", b.ServiceID, err)
	}
	doc := bookingDoc{
		ClientID:       clientOID,
		PractitionerID: practitionerOID,
		ServiceID:      serviceOID,
		StartAt:        bson.NewDateTimeFromTime(b.StartAt),
		EndAt:          bson.NewDateTimeFromTime(b.EndAt),
		Status:         string(b.Status),
		CreatedAt:      bson.NewDateTimeFromTime(b.CreatedAt),
		UpdatedAt:      bson.NewDateTimeFromTime(b.UpdatedAt),
	}
	if b.ID != "" {
		oid, err := bson.ObjectIDFromHex(b.ID)
		if err != nil {
			return bookingDoc{}, fmt.Errorf("booking id %q is not an ObjectID: %w", b.ID, err)
		}
		doc.ID = oid
	}
	if b.CancelledAt != nil {
		cancelled := bson.NewDateTimeFromTime(*b.CancelledAt)
		doc.CancelledAt = &cancelled
	}
	if b.CompletedAt != nil {
		completed := bson.NewDateTimeFromTime(*b.CompletedAt)
		doc.CompletedAt = &completed
	}
	doc.PaymentStatus = string(b.PaymentStatus)
	if b.PaidAt != nil {
		paid := bson.NewDateTimeFromTime(*b.PaidAt)
		doc.PaidAt = &paid
	}
	return doc, nil
}

func bookingFromDoc(doc bookingDoc) booking.Booking {
	b := booking.Booking{
		ID:             doc.ID.Hex(),
		ClientID:       doc.ClientID.Hex(),
		PractitionerID: doc.PractitionerID.Hex(),
		ServiceID:      doc.ServiceID.Hex(),
		StartAt:        doc.StartAt.Time(),
		EndAt:          doc.EndAt.Time(),
		Status:         booking.Status(doc.Status),
		CreatedAt:      doc.CreatedAt.Time(),
		UpdatedAt:      doc.UpdatedAt.Time(),
	}
	if doc.CancelledAt != nil {
		cancelled := doc.CancelledAt.Time()
		b.CancelledAt = &cancelled
	}
	if doc.CompletedAt != nil {
		completed := doc.CompletedAt.Time()
		b.CompletedAt = &completed
	}
	b.PaymentStatus = booking.PaymentStatus(doc.PaymentStatus)
	if doc.PaidAt != nil {
		paid := doc.PaidAt.Time()
		b.PaidAt = &paid
	}
	return b
}
