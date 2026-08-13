package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// SessionNoteRepository persists session notes in the session_notes
// collection. The document shape carries clientId denormalized from the
// booking so client-scoped history queries lead with it (isolation), and
// the unique index on bookingId enforces one note per booking — both fixed
// by the existing index design.
type SessionNoteRepository struct {
	coll *mongo.Collection
}

var _ ports.SessionNoteRepository = (*SessionNoteRepository)(nil)

// NewSessionNoteRepository binds the repository to the session_notes
// collection.
func NewSessionNoteRepository(db *mongo.Database) *SessionNoteRepository {
	return &SessionNoteRepository{coll: db.Collection("session_notes")}
}

// sessionNoteDoc is the storage shape; kept separate from the domain
// entity.
type sessionNoteDoc struct {
	ID              bson.ObjectID  `bson:"_id,omitempty"`
	BookingID       bson.ObjectID  `bson:"bookingId"`
	ClientID        bson.ObjectID  `bson:"clientId"`
	PractitionerID  bson.ObjectID  `bson:"practitionerId"`
	PrivateNotes    string         `bson:"privateNotes"`
	SharedFeedback  string         `bson:"sharedFeedback"`
	SharedResources []string       `bson:"sharedResources"`
	SharedAt        *bson.DateTime `bson:"sharedAt,omitempty"`
	CreatedAt       bson.DateTime  `bson:"createdAt"`
	UpdatedAt       bson.DateTime  `bson:"updatedAt"`
}

// Create inserts a new note, assigning its ID. A duplicate-key error from
// the bookingId unique index means a note already exists for the booking —
// surfaced as the domain conflict.
func (r *SessionNoteRepository) Create(ctx context.Context, n note.SessionNote) (note.SessionNote, error) {
	doc, err := newSessionNoteDoc(n)
	if err != nil {
		return note.SessionNote{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return note.SessionNote{}, note.ErrNoteExists
		}
		return note.SessionNote{}, fmt.Errorf("insert session note: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		n.ID = oid.Hex()
	}
	return n, nil
}

// FindByBookingID looks up the booking's single note; misses return
// note.ErrNoteNotFound.
func (r *SessionNoteRepository) FindByBookingID(ctx context.Context, bookingID string) (note.SessionNote, error) {
	oid, err := bson.ObjectIDFromHex(bookingID)
	if err != nil {
		return note.SessionNote{}, note.ErrNoteNotFound
	}
	var doc sessionNoteDoc
	err = r.coll.FindOne(ctx, bson.M{"bookingId": oid}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return note.SessionNote{}, note.ErrNoteNotFound
		}
		return note.SessionNote{}, fmt.Errorf("find session note: %w", err)
	}
	return sessionNoteFromDoc(doc), nil
}

// Update replaces a note's mutable state. Misses return
// note.ErrNoteNotFound.
func (r *SessionNoteRepository) Update(ctx context.Context, n note.SessionNote) (note.SessionNote, error) {
	oid, err := bson.ObjectIDFromHex(n.ID)
	if err != nil {
		return note.SessionNote{}, note.ErrNoteNotFound
	}
	doc, err := newSessionNoteDoc(n)
	if err != nil {
		return note.SessionNote{}, err
	}
	doc.ID = oid

	res, err := r.coll.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return note.SessionNote{}, note.ErrNoteExists
		}
		return note.SessionNote{}, fmt.Errorf("update session note: %w", err)
	}
	if res.MatchedCount == 0 {
		return note.SessionNote{}, note.ErrNoteNotFound
	}
	return n, nil
}

// newSessionNoteDoc maps the domain entity to storage. IDs must be hex
// ObjectIDs — they always originate from stored bookings and authenticated
// identities.
func newSessionNoteDoc(n note.SessionNote) (sessionNoteDoc, error) {
	bookingOID, err := bson.ObjectIDFromHex(n.BookingID)
	if err != nil {
		return sessionNoteDoc{}, fmt.Errorf("note bookingId %q is not an ObjectID: %w", n.BookingID, err)
	}
	clientOID, err := bson.ObjectIDFromHex(n.ClientID)
	if err != nil {
		return sessionNoteDoc{}, fmt.Errorf("note clientId %q is not an ObjectID: %w", n.ClientID, err)
	}
	practitionerOID, err := bson.ObjectIDFromHex(n.PractitionerID)
	if err != nil {
		return sessionNoteDoc{}, fmt.Errorf("note practitionerId %q is not an ObjectID: %w", n.PractitionerID, err)
	}
	doc := sessionNoteDoc{
		BookingID:       bookingOID,
		ClientID:        clientOID,
		PractitionerID:  practitionerOID,
		PrivateNotes:    n.PrivateNotes,
		SharedFeedback:  n.SharedFeedback,
		SharedResources: n.SharedResources,
		CreatedAt:       bson.NewDateTimeFromTime(n.CreatedAt),
		UpdatedAt:       bson.NewDateTimeFromTime(n.UpdatedAt),
	}
	if n.ID != "" {
		oid, err := bson.ObjectIDFromHex(n.ID)
		if err != nil {
			return sessionNoteDoc{}, fmt.Errorf("note id %q is not an ObjectID: %w", n.ID, err)
		}
		doc.ID = oid
	}
	if n.SharedAt != nil {
		shared := bson.NewDateTimeFromTime(*n.SharedAt)
		doc.SharedAt = &shared
	}
	return doc, nil
}

func sessionNoteFromDoc(doc sessionNoteDoc) note.SessionNote {
	resources := doc.SharedResources
	if resources == nil {
		resources = []string{}
	}
	n := note.SessionNote{
		ID:              doc.ID.Hex(),
		BookingID:       doc.BookingID.Hex(),
		ClientID:        doc.ClientID.Hex(),
		PractitionerID:  doc.PractitionerID.Hex(),
		PrivateNotes:    doc.PrivateNotes,
		SharedFeedback:  doc.SharedFeedback,
		SharedResources: resources,
		CreatedAt:       doc.CreatedAt.Time(),
		UpdatedAt:       doc.UpdatedAt.Time(),
	}
	if doc.SharedAt != nil {
		shared := doc.SharedAt.Time()
		n.SharedAt = &shared
	}
	return n
}
