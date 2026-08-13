package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// claimLease is how long a claimed job stays off the queue before another
// dispatcher may pick it up. It bounds the damage of a process that dies
// mid-send: the job is retried after the lease rather than stranded.
const claimLease = 5 * time.Minute

// NotificationJobRepository persists the delivery outbox in the
// notification_jobs collection.
//
// Claiming is a findAndModify per job rather than a plain query, so two
// dispatcher instances (or a restarting one) can never both send the same
// email: the write that stamps claimedAt is the atomic point, and only the
// winner gets the document back.
type NotificationJobRepository struct {
	coll *mongo.Collection
	now  func() time.Time
}

var _ ports.NotificationJobRepository = (*NotificationJobRepository)(nil)

// NewNotificationJobRepository binds the repository to notification_jobs.
func NewNotificationJobRepository(db *mongo.Database) *NotificationJobRepository {
	return &NotificationJobRepository{
		coll: db.Collection("notification_jobs"),
		now:  func() time.Time { return time.Now().UTC() },
	}
}

// notificationJobDoc is the storage shape. bookingId is a plain string:
// it also carries enquiry ids, which are not booking ObjectIDs.
type notificationJobDoc struct {
	ID        bson.ObjectID  `bson:"_id,omitempty"`
	Kind      string         `bson:"kind"`
	BookingID string         `bson:"bookingId,omitempty"`
	Recipient string         `bson:"recipient"`
	Data      map[string]any `bson:"data"`
	DueAt     bson.DateTime  `bson:"dueAt"`
	Status    string         `bson:"status"`
	Attempts  int            `bson:"attempts"`
	LastError string         `bson:"lastError,omitempty"`
	ClaimedAt *bson.DateTime `bson:"claimedAt,omitempty"`
	SentAt    *bson.DateTime `bson:"sentAt,omitempty"`
	CreatedAt bson.DateTime  `bson:"createdAt"`
	UpdatedAt bson.DateTime  `bson:"updatedAt"`
}

func newNotificationJobDoc(j notification.Job) notificationJobDoc {
	data := make(map[string]any, len(j.Data))
	for k, v := range j.Data {
		data[k] = v
	}
	doc := notificationJobDoc{
		Kind:      string(j.Kind),
		BookingID: j.BookingID,
		Recipient: j.Recipient,
		Data:      data,
		DueAt:     bson.NewDateTimeFromTime(j.DueAt),
		Status:    string(j.Status),
		Attempts:  j.Attempts,
		LastError: j.LastError,
		CreatedAt: bson.NewDateTimeFromTime(j.CreatedAt),
		UpdatedAt: bson.NewDateTimeFromTime(j.UpdatedAt),
	}
	if j.SentAt != nil {
		stamp := bson.NewDateTimeFromTime(*j.SentAt)
		doc.SentAt = &stamp
	}
	return doc
}

func (d notificationJobDoc) toDomain() notification.Job {
	data := make(map[string]string, len(d.Data))
	for k, v := range d.Data {
		if s, ok := v.(string); ok {
			data[k] = s
		}
	}
	job := notification.Job{
		ID:        d.ID.Hex(),
		Kind:      notification.Kind(d.Kind),
		BookingID: d.BookingID,
		Recipient: d.Recipient,
		Data:      data,
		DueAt:     d.DueAt.Time().UTC(),
		Status:    notification.Status(d.Status),
		Attempts:  d.Attempts,
		LastError: d.LastError,
		CreatedAt: d.CreatedAt.Time().UTC(),
		UpdatedAt: d.UpdatedAt.Time().UTC(),
	}
	if d.SentAt != nil {
		sent := d.SentAt.Time().UTC()
		job.SentAt = &sent
	}
	return job
}

// Create inserts a new job, assigning its id.
func (r *NotificationJobRepository) Create(ctx context.Context, job notification.Job) (notification.Job, error) {
	doc := newNotificationJobDoc(job)
	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return notification.Job{}, fmt.Errorf("insert notification job: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		job.ID = oid.Hex()
	}
	return job, nil
}

// Update persists a job's lifecycle change and releases any claim on it: a
// job that has been dealt with is no longer in flight.
func (r *NotificationJobRepository) Update(ctx context.Context, job notification.Job) (notification.Job, error) {
	oid, err := bson.ObjectIDFromHex(job.ID)
	if err != nil {
		return notification.Job{}, notification.ErrJobNotFound
	}
	doc := newNotificationJobDoc(job)
	update := bson.M{
		"$set": bson.M{
			"dueAt":     doc.DueAt,
			"status":    doc.Status,
			"attempts":  doc.Attempts,
			"lastError": doc.LastError,
			"updatedAt": doc.UpdatedAt,
			"sentAt":    doc.SentAt,
		},
		"$unset": bson.M{"claimedAt": ""},
	}
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, update)
	if err != nil {
		return notification.Job{}, fmt.Errorf("update notification job: %w", err)
	}
	if res.MatchedCount == 0 {
		return notification.Job{}, notification.ErrJobNotFound
	}
	return job, nil
}

// ClaimDue takes up to limit due jobs for this caller. Each claim is its
// own atomic findAndModify, so a concurrent dispatcher simply gets the next
// job instead of a duplicate.
func (r *NotificationJobRepository) ClaimDue(ctx context.Context, now time.Time, limit int) ([]notification.Job, error) {
	if limit <= 0 {
		return nil, nil
	}
	now = now.UTC()
	leaseCutoff := bson.NewDateTimeFromTime(now.Add(-claimLease))

	filter := bson.M{
		"status": string(notification.StatusPending),
		"dueAt":  bson.M{"$lte": bson.NewDateTimeFromTime(now)},
		// Either never claimed, or the previous claim's lease has lapsed
		// because that dispatcher died mid-send.
		"$or": []bson.M{
			{"claimedAt": bson.M{"$exists": false}},
			{"claimedAt": nil},
			{"claimedAt": bson.M{"$lte": leaseCutoff}},
		},
	}
	update := bson.M{"$set": bson.M{"claimedAt": bson.NewDateTimeFromTime(now)}}
	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "dueAt", Value: 1}}).
		SetReturnDocument(options.After)

	jobs := make([]notification.Job, 0, limit)
	for len(jobs) < limit {
		var doc notificationJobDoc
		err := r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				break
			}
			return jobs, fmt.Errorf("claim notification job: %w", err)
		}
		jobs = append(jobs, doc.toDomain())
	}
	return jobs, nil
}

// PendingByBooking lists a booking's undelivered jobs of one kind.
func (r *NotificationJobRepository) PendingByBooking(ctx context.Context, bookingID string, kind notification.Kind) ([]notification.Job, error) {
	cursor, err := r.coll.Find(ctx, bson.M{
		"bookingId": bookingID,
		"kind":      string(kind),
		"status":    string(notification.StatusPending),
	})
	if err != nil {
		return nil, fmt.Errorf("find pending notifications: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []notificationJobDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode pending notifications: %w", err)
	}
	jobs := make([]notification.Job, 0, len(docs))
	for _, doc := range docs {
		jobs = append(jobs, doc.toDomain())
	}
	return jobs, nil
}
