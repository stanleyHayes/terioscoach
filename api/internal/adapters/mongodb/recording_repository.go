package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/recording"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type RecordingRepository struct {
	coll *mongo.Collection
}

var _ ports.RecordingRepository = (*RecordingRepository)(nil)

func NewRecordingRepository(db *mongo.Database) *RecordingRepository {
	return &RecordingRepository{coll: db.Collection("session_recordings")}
}

type recordingDoc struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	BookingID      bson.ObjectID `bson:"bookingId"`
	ClientID       bson.ObjectID `bson:"clientId"`
	PractitionerID bson.ObjectID `bson:"practitionerId"`
	ContentType    string        `bson:"contentType"`
	// publicId is the media store reference. dataUrl is the inline copy
	// kept by recordings made before the move; both are optional so the two
	// generations decode from the same document.
	PublicID    string        `bson:"publicId,omitempty"`
	DataURL     string        `bson:"dataUrl,omitempty"`
	Bytes       int64         `bson:"bytes"`
	DurationSec int           `bson:"durationSec"`
	CreatedAt   bson.DateTime `bson:"createdAt"`
	RetainUntil bson.DateTime `bson:"retainUntil"`
}

func (r *RecordingRepository) Create(ctx context.Context, rec recording.SessionRecording) (recording.SessionRecording, error) {
	doc, err := newRecordingDoc(rec)
	if err != nil {
		return recording.SessionRecording{}, err
	}
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return recording.SessionRecording{}, recording.ErrRecordingExists
		}
		return recording.SessionRecording{}, fmt.Errorf("insert session recording: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		rec.ID = oid.Hex()
	}
	return rec, nil
}

func (r *RecordingRepository) ListByBookingID(ctx context.Context, bookingID string) ([]recording.SessionRecording, error) {
	oid, err := bson.ObjectIDFromHex(bookingID)
	if err != nil {
		return []recording.SessionRecording{}, nil
	}
	cursor, err := r.coll.Find(ctx, bson.M{"bookingId": oid}, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list session recordings: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	var out []recording.SessionRecording
	for cursor.Next(ctx) {
		var doc recordingDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode session recording: %w", err)
		}
		out = append(out, recordingFromDoc(doc))
	}
	return out, cursor.Err()
}

// ListExpired returns recordings past their retention date, oldest first,
// so a neglected backlog is worked through in bounded batches rather than
// in one sweep.
func (r *RecordingRepository) ListExpired(ctx context.Context, now time.Time, limit int) ([]recording.SessionRecording, error) {
	cursor, err := r.coll.Find(ctx,
		bson.M{"retainUntil": bson.M{"$lte": bson.NewDateTimeFromTime(now.UTC())}},
		options.Find().SetSort(bson.D{{Key: "retainUntil", Value: 1}}).SetLimit(int64(limit)),
	)
	if err != nil {
		return nil, fmt.Errorf("list expired recordings: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()
	var out []recording.SessionRecording
	for cursor.Next(ctx) {
		var doc recordingDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode session recording: %w", err)
		}
		out = append(out, recordingFromDoc(doc))
	}
	return out, cursor.Err()
}

func (r *RecordingRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete session recording: %w", err)
	}
	return nil
}

func newRecordingDoc(rec recording.SessionRecording) (recordingDoc, error) {
	bookingOID, err := bson.ObjectIDFromHex(rec.BookingID)
	if err != nil {
		return recordingDoc{}, fmt.Errorf("recording bookingId %q is not an ObjectID: %w", rec.BookingID, err)
	}
	clientOID, err := bson.ObjectIDFromHex(rec.ClientID)
	if err != nil {
		return recordingDoc{}, fmt.Errorf("recording clientId %q is not an ObjectID: %w", rec.ClientID, err)
	}
	practitionerOID, err := bson.ObjectIDFromHex(rec.PractitionerID)
	if err != nil {
		return recordingDoc{}, fmt.Errorf("recording practitionerId %q is not an ObjectID: %w", rec.PractitionerID, err)
	}
	doc := recordingDoc{
		BookingID:      bookingOID,
		ClientID:       clientOID,
		PractitionerID: practitionerOID,
		ContentType:    rec.ContentType,
		PublicID:       rec.PublicID,
		DataURL:        rec.DataURL,
		Bytes:          rec.Bytes,
		DurationSec:    rec.DurationSec,
		CreatedAt:      bson.NewDateTimeFromTime(rec.CreatedAt),
		RetainUntil:    bson.NewDateTimeFromTime(rec.RetainUntil),
	}
	if rec.ID != "" {
		oid, err := bson.ObjectIDFromHex(rec.ID)
		if err != nil {
			return recordingDoc{}, fmt.Errorf("recording id %q is not an ObjectID: %w", rec.ID, err)
		}
		doc.ID = oid
	}
	return doc, nil
}

func recordingFromDoc(doc recordingDoc) recording.SessionRecording {
	return recording.SessionRecording{
		ID:             doc.ID.Hex(),
		BookingID:      doc.BookingID.Hex(),
		ClientID:       doc.ClientID.Hex(),
		PractitionerID: doc.PractitionerID.Hex(),
		ContentType:    doc.ContentType,
		PublicID:       doc.PublicID,
		DataURL:        doc.DataURL,
		Bytes:          doc.Bytes,
		DurationSec:    doc.DurationSec,
		CreatedAt:      doc.CreatedAt.Time(),
		RetainUntil:    doc.RetainUntil.Time(),
	}
}
