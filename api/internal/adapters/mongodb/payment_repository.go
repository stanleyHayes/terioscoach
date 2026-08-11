package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// PaymentRepository persists payments in the payments collection. The
// document shape is guarded by the fixed index design: bookingId unique
// (one payment per booking), paystackReference unique when present (the
// webhook join key), and clientId+createdAt for the client history. The
// collection deliberately carries no card or mobile-money data — checkout
// is hosted by Paystack.
type PaymentRepository struct {
	coll *mongo.Collection
}

var _ ports.PaymentRepository = (*PaymentRepository)(nil)

// NewPaymentRepository binds the repository to the payments collection.
func NewPaymentRepository(db *mongo.Database) *PaymentRepository {
	return &PaymentRepository{coll: db.Collection("payments")}
}

// paymentDoc is the storage shape; kept separate from the domain entity.
type paymentDoc struct {
	ID                bson.ObjectID  `bson:"_id,omitempty"`
	BookingID         bson.ObjectID  `bson:"bookingId"`
	ClientID          bson.ObjectID  `bson:"clientId"`
	AmountKobo        int64          `bson:"amountKobo"`
	Currency          string         `bson:"currency"`
	Status            string         `bson:"status"`
	PaystackReference string         `bson:"paystackReference"`
	Channel           string         `bson:"channel,omitempty"`
	PaidAt            *bson.DateTime `bson:"paidAt,omitempty"`
	RefundedAt        *bson.DateTime `bson:"refundedAt,omitempty"`
	CreatedAt         bson.DateTime  `bson:"createdAt"`
	UpdatedAt         bson.DateTime  `bson:"updatedAt"`
}

// Create inserts a new payment, assigning its ID. A duplicate-key error
// from the bookingId unique index means a payment already exists for the
// booking — surfaced as the domain conflict.
func (r *PaymentRepository) Create(ctx context.Context, p payment.Payment) (payment.Payment, error) {
	doc, err := newPaymentDoc(p)
	if err != nil {
		return payment.Payment{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return payment.Payment{}, payment.ErrAlreadyPaid
		}
		return payment.Payment{}, fmt.Errorf("insert payment: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		p.ID = oid.Hex()
	}
	return p, nil
}

// FindByID looks up a payment by hex ObjectID; misses return
// payment.ErrPaymentNotFound.
func (r *PaymentRepository) FindByID(ctx context.Context, id string) (payment.Payment, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

// FindByBookingID looks up the booking's single payment (unique index);
// misses return payment.ErrPaymentNotFound.
func (r *PaymentRepository) FindByBookingID(ctx context.Context, bookingID string) (payment.Payment, error) {
	oid, err := bson.ObjectIDFromHex(bookingID)
	if err != nil {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	return r.findOne(ctx, bson.M{"bookingId": oid})
}

// FindByReference looks up a payment by its Paystack reference — the
// webhook join key (unique partial index); misses return
// payment.ErrPaymentNotFound.
func (r *PaymentRepository) FindByReference(ctx context.Context, reference string) (payment.Payment, error) {
	return r.findOne(ctx, bson.M{"paystackReference": reference})
}

func (r *PaymentRepository) findOne(ctx context.Context, filter bson.M) (payment.Payment, error) {
	var doc paymentDoc
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return payment.Payment{}, payment.ErrPaymentNotFound
		}
		return payment.Payment{}, fmt.Errorf("find payment: %w", err)
	}
	return paymentFromDoc(doc), nil
}

// Update replaces a payment's mutable state. Misses return
// payment.ErrPaymentNotFound.
func (r *PaymentRepository) Update(ctx context.Context, p payment.Payment) (payment.Payment, error) {
	oid, err := bson.ObjectIDFromHex(p.ID)
	if err != nil {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	doc, err := newPaymentDoc(p)
	if err != nil {
		return payment.Payment{}, err
	}
	doc.ID = oid

	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return payment.Payment{}, payment.ErrAlreadyPaid
		}
		return payment.Payment{}, fmt.Errorf("update payment: %w", err)
	}
	if res.MatchedCount == 0 {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	return p, nil
}

// ListByClient returns the client's payments ordered by createdAt
// descending. Client data isolation leads every client-scoped query with
// clientId.
func (r *PaymentRepository) ListByClient(ctx context.Context, clientID string) ([]payment.Payment, error) {
	oid, err := bson.ObjectIDFromHex(clientID)
	if err != nil {
		return nil, nil
	}
	return r.list(ctx, bson.M{"clientId": oid})
}

// ListByBookingIDs returns payments for the given bookings matching the
// creation-time filter, ordered by createdAt descending. The practitioner
// view joins through booking ids — the collection carries no
// practitionerId by design.
func (r *PaymentRepository) ListByBookingIDs(ctx context.Context, bookingIDs []string, filter ports.PaymentFilter) ([]payment.Payment, error) {
	if len(bookingIDs) == 0 {
		return nil, nil
	}
	oids := make([]bson.ObjectID, 0, len(bookingIDs))
	for _, id := range bookingIDs {
		oid, err := bson.ObjectIDFromHex(id)
		if err != nil {
			continue // foreign ids are simply skipped
		}
		oids = append(oids, oid)
	}
	if len(oids) == 0 {
		return nil, nil
	}
	m := bson.M{"bookingId": bson.M{"$in": oids}}
	if filter.From != nil || filter.To != nil {
		created := bson.M{}
		if filter.From != nil {
			created["$gte"] = bson.NewDateTimeFromTime(*filter.From)
		}
		if filter.To != nil {
			created["$lt"] = bson.NewDateTimeFromTime(*filter.To)
		}
		m["createdAt"] = created
	}
	return r.list(ctx, m)
}

func (r *PaymentRepository) list(ctx context.Context, filter bson.M) ([]payment.Payment, error) {
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []payment.Payment
	for cursor.Next(ctx) {
		var doc paymentDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode payment: %w", err)
		}
		out = append(out, paymentFromDoc(doc))
	}
	return out, cursor.Err()
}

// newPaymentDoc maps the domain entity to storage. IDs must be hex
// ObjectIDs — they always originate from stored bookings and authenticated
// identities.
func newPaymentDoc(p payment.Payment) (paymentDoc, error) {
	bookingOID, err := bson.ObjectIDFromHex(p.BookingID)
	if err != nil {
		return paymentDoc{}, fmt.Errorf("payment bookingId %q is not an ObjectID: %w", p.BookingID, err)
	}
	clientOID, err := bson.ObjectIDFromHex(p.ClientID)
	if err != nil {
		return paymentDoc{}, fmt.Errorf("payment clientId %q is not an ObjectID: %w", p.ClientID, err)
	}
	doc := paymentDoc{
		BookingID:         bookingOID,
		ClientID:          clientOID,
		AmountKobo:        p.AmountKobo,
		Currency:          p.Currency,
		Status:            string(p.Status),
		PaystackReference: p.PaystackReference,
		Channel:           p.Channel,
		CreatedAt:         bson.NewDateTimeFromTime(p.CreatedAt),
		UpdatedAt:         bson.NewDateTimeFromTime(p.UpdatedAt),
	}
	if p.ID != "" {
		oid, err := bson.ObjectIDFromHex(p.ID)
		if err != nil {
			return paymentDoc{}, fmt.Errorf("payment id %q is not an ObjectID: %w", p.ID, err)
		}
		doc.ID = oid
	}
	if p.PaidAt != nil {
		paid := bson.NewDateTimeFromTime(*p.PaidAt)
		doc.PaidAt = &paid
	}
	if p.RefundedAt != nil {
		refunded := bson.NewDateTimeFromTime(*p.RefundedAt)
		doc.RefundedAt = &refunded
	}
	return doc, nil
}

func paymentFromDoc(doc paymentDoc) payment.Payment {
	p := payment.Payment{
		ID:                doc.ID.Hex(),
		BookingID:         doc.BookingID.Hex(),
		ClientID:          doc.ClientID.Hex(),
		AmountKobo:        doc.AmountKobo,
		Currency:          doc.Currency,
		Status:            payment.Status(doc.Status),
		PaystackReference: doc.PaystackReference,
		Channel:           doc.Channel,
		CreatedAt:         doc.CreatedAt.Time(),
		UpdatedAt:         doc.UpdatedAt.Time(),
	}
	if doc.PaidAt != nil {
		paid := doc.PaidAt.Time()
		p.PaidAt = &paid
	}
	if doc.RefundedAt != nil {
		refunded := doc.RefundedAt.Time()
		p.RefundedAt = &refunded
	}
	return p
}
