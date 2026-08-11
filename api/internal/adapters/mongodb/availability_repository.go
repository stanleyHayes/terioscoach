package mongodb

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AvailabilityRepository persists weekly rules (availability_rules) and
// blocked periods (time_off).
type AvailabilityRepository struct {
	rules   *mongo.Collection
	timeOff *mongo.Collection
}

var _ ports.AvailabilityRepository = (*AvailabilityRepository)(nil)

// NewAvailabilityRepository binds the repository to its collections.
func NewAvailabilityRepository(db *mongo.Database) *AvailabilityRepository {
	return &AvailabilityRepository{
		rules:   db.Collection("availability_rules"),
		timeOff: db.Collection("time_off"),
	}
}

// windowDoc is the storage shape of a single opening window.
type windowDoc struct {
	StartMin int `bson:"startMin"`
	EndMin   int `bson:"endMin"`
}

// ruleDoc is the storage shape; one document per practitioner+weekday
// (unique index in indexes.go).
type ruleDoc struct {
	PractitionerID bson.ObjectID `bson:"practitionerId"`
	Weekday        int           `bson:"weekday"`
	Windows        []windowDoc   `bson:"windows"`
	BufferMinutes  int           `bson:"bufferMinutes"`
	CreatedAt      bson.DateTime `bson:"createdAt"`
	UpdatedAt      bson.DateTime `bson:"updatedAt"`
}

// timeOffDoc is the storage shape; kept separate from the domain entity.
type timeOffDoc struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	PractitionerID bson.ObjectID `bson:"practitionerId"`
	StartAt        bson.DateTime `bson:"startAt"`
	EndAt          bson.DateTime `bson:"endAt"`
	Reason         string        `bson:"reason"`
	CreatedAt      bson.DateTime `bson:"createdAt"`
}

// GetRules returns the practitioner's rules ordered by weekday.
func (r *AvailabilityRepository) GetRules(ctx context.Context, practitionerID string) ([]scheduling.WeeklyRule, error) {
	oid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return nil, nil
	}
	cursor, err := r.rules.Find(ctx, bson.M{"practitionerId": oid})
	if err != nil {
		return nil, fmt.Errorf("list availability rules: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []scheduling.WeeklyRule
	for cursor.Next(ctx) {
		var doc ruleDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode availability rule: %w", err)
		}
		rule := scheduling.WeeklyRule{
			PractitionerID: practitionerID,
			Weekday:        time.Weekday(doc.Weekday),
			BufferMinutes:  doc.BufferMinutes,
		}
		for _, w := range doc.Windows {
			rule.Windows = append(rule.Windows, scheduling.Window{StartMin: w.StartMin, EndMin: w.EndMin})
		}
		out = append(out, rule)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	// Storage order is not guaranteed; the contract orders by weekday.
	sort.Slice(out, func(i, j int) bool { return out[i].Weekday < out[j].Weekday })
	return out, nil
}

// ReplaceRules swaps the practitioner's full weekly schedule: delete the
// existing set, insert the new one. Single-practitioner platform, so the
// non-transactional delete+insert is acceptable.
func (r *AvailabilityRepository) ReplaceRules(ctx context.Context, practitionerID string, rules []scheduling.WeeklyRule) error {
	oid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return fmt.Errorf("practitionerId %q is not an ObjectID: %w", practitionerID, err)
	}
	if _, err := r.rules.DeleteMany(ctx, bson.M{"practitionerId": oid}); err != nil {
		return fmt.Errorf("clear availability rules: %w", err)
	}
	if len(rules) == 0 {
		return nil
	}

	now := bson.NewDateTimeFromTime(time.Now().UTC())
	docs := make([]any, 0, len(rules))
	for _, rule := range rules {
		windows := make([]windowDoc, 0, len(rule.Windows))
		for _, w := range rule.Windows {
			windows = append(windows, windowDoc{StartMin: w.StartMin, EndMin: w.EndMin})
		}
		docs = append(docs, ruleDoc{
			PractitionerID: oid,
			Weekday:        int(rule.Weekday),
			Windows:        windows,
			BufferMinutes:  rule.BufferMinutes,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}
	if _, err := r.rules.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("insert availability rules: %w", err)
	}
	return nil
}

// CreateTimeOff inserts a blocked period, assigning its ID.
func (r *AvailabilityRepository) CreateTimeOff(ctx context.Context, t scheduling.TimeOff) (scheduling.TimeOff, error) {
	oid, err := bson.ObjectIDFromHex(t.PractitionerID)
	if err != nil {
		return scheduling.TimeOff{}, fmt.Errorf("practitionerId %q is not an ObjectID: %w", t.PractitionerID, err)
	}
	doc := timeOffDoc{
		PractitionerID: oid,
		StartAt:        bson.NewDateTimeFromTime(t.StartAt),
		EndAt:          bson.NewDateTimeFromTime(t.EndAt),
		Reason:         t.Reason,
		CreatedAt:      bson.NewDateTimeFromTime(t.CreatedAt),
	}
	res, err := r.timeOff.InsertOne(ctx, doc)
	if err != nil {
		return scheduling.TimeOff{}, fmt.Errorf("insert time-off: %w", err)
	}
	if inserted, ok := res.InsertedID.(bson.ObjectID); ok {
		t.ID = inserted.Hex()
	}
	return t, nil
}

// ListTimeOff returns the practitioner's blocked periods overlapping
// [from, to) as plain intervals for the slot engine.
func (r *AvailabilityRepository) ListTimeOff(ctx context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error) {
	oid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return nil, nil
	}
	filter := bson.M{
		"practitionerId": oid,
		"startAt":        bson.M{"$lt": bson.NewDateTimeFromTime(to)},
		"endAt":          bson.M{"$gt": bson.NewDateTimeFromTime(from)},
	}
	cursor, err := r.timeOff.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list time-off: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []scheduling.Interval
	for cursor.Next(ctx) {
		var doc timeOffDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode time-off: %w", err)
		}
		out = append(out, scheduling.Interval{Start: doc.StartAt.Time(), End: doc.EndAt.Time()})
	}
	return out, cursor.Err()
}

// BusyIntervalReader feeds existing bookings into slot generation as plain
// intervals. The bookings schema lands in BE-05; this reads the confirmed
// bookings' [startAt, endAt) spans and returns nothing until they exist.
type BusyIntervalReader struct {
	bookings *mongo.Collection
}

var _ ports.BusyIntervalReader = (*BusyIntervalReader)(nil)

// NewBusyIntervalReader binds the reader to the bookings collection.
func NewBusyIntervalReader(db *mongo.Database) *BusyIntervalReader {
	return &BusyIntervalReader{bookings: db.Collection("bookings")}
}

// bookingSpanDoc is the minimal projection the slot engine needs.
type bookingSpanDoc struct {
	StartAt bson.DateTime `bson:"startAt"`
	EndAt   bson.DateTime `bson:"endAt"`
}

// BusyIntervals returns confirmed bookings overlapping [from, to).
// Pending/cancelled bookings do not block — mirroring the partial unique
// index in indexes.go where only confirmed bookings occupy a slot.
func (r *BusyIntervalReader) BusyIntervals(ctx context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error) {
	oid, err := bson.ObjectIDFromHex(practitionerID)
	if err != nil {
		return nil, nil
	}
	filter := bson.M{
		"practitionerId": oid,
		"status":         "confirmed",
		"startAt":        bson.M{"$lt": bson.NewDateTimeFromTime(to)},
		"endAt":          bson.M{"$gt": bson.NewDateTimeFromTime(from)},
	}
	cursor, err := r.bookings.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list busy intervals: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var out []scheduling.Interval
	for cursor.Next(ctx) {
		var doc bookingSpanDoc
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode booking span: %w", err)
		}
		out = append(out, scheduling.Interval{Start: doc.StartAt.Time(), End: doc.EndAt.Time()})
	}
	return out, cursor.Err()
}
