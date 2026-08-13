package mongodb

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/domain/ops"
)

// OpsSource counts the operational signals the health rules judge (LCH-09).
//
// Every count is a query against data the system already keeps — there is
// no separate metrics store to fall out of step with reality. The cost is
// four small counts per poll, which at one poll a minute is nothing next
// to being able to trust the answer.
type OpsSource struct {
	jobs     *mongo.Collection
	attempts *mongo.Collection
	payments *mongo.Collection
	window   time.Duration
	// maxAttempts mirrors the retry policy: a job is only a *failure* once
	// it has used every attempt. Counting earlier would alert on a job
	// that is about to succeed on its second try.
	maxAttempts int
	now         func() time.Time
}

// NewOpsSource builds the counter over the collections it reads.
func NewOpsSource(db *mongo.Database, window time.Duration, retry notification.RetryPolicy) *OpsSource {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &OpsSource{
		jobs:        db.Collection("notification_jobs"),
		attempts:    db.Collection("login_attempts"),
		payments:    db.Collection("payments"),
		window:      window,
		maxAttempts: retry.MaxAttempts,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// Snapshot reads the current counts.
//
// A failure of any one count fails the whole snapshot rather than
// returning a partial one. A partial snapshot with three zeroes in it is
// indistinguishable from a healthy system, which is the one answer this
// must never give by accident.
func (s *OpsSource) Snapshot(ctx context.Context) (ops.Snapshot, error) {
	now := s.now()
	cutoff := bson.NewDateTimeFromTime(now.Add(-s.window))

	backlog, err := s.jobs.CountDocuments(ctx, bson.M{
		"status": string(notification.StatusPending),
		// Due in the past: a job scheduled for tomorrow is not a backlog,
		// it is a reminder doing exactly what it should.
		"dueAt": bson.M{"$lte": bson.NewDateTimeFromTime(now)},
	})
	if err != nil {
		return ops.Snapshot{}, fmt.Errorf("count notification backlog: %w", err)
	}

	failures, err := s.jobs.CountDocuments(ctx, bson.M{
		"status":    string(notification.StatusFailed),
		"attempts":  bson.M{"$gte": s.maxAttempts},
		"updatedAt": bson.M{"$gte": cutoff},
	})
	if err != nil {
		return ops.Snapshot{}, fmt.Errorf("count notification failures: %w", err)
	}

	// Distinct identifiers, not attempts. Twenty failed logins from one
	// person is a forgotten password; one each from twenty accounts is an
	// attack, and only the second is worth waking up for.
	locked, err := s.attempts.CountDocuments(ctx, bson.M{
		"count":  bson.M{"$gte": s.maxAttempts},
		"lastAt": bson.M{"$gte": cutoff},
	})
	if err != nil {
		return ops.Snapshot{}, fmt.Errorf("count locked accounts: %w", err)
	}

	paymentFailures, err := s.payments.CountDocuments(ctx, bson.M{
		"status":    "failed",
		"updatedAt": bson.M{"$gte": cutoff},
	})
	if err != nil {
		return ops.Snapshot{}, fmt.Errorf("count payment failures: %w", err)
	}

	return ops.Snapshot{
		NotificationBacklog:         int(backlog),
		NotificationFailures:        int(failures),
		LockedAccounts:              int(locked),
		PaymentVerificationFailures: int(paymentFailures),
	}, nil
}
