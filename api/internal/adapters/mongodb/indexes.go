package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates the full index design for every collection. It is
// idempotent and safe to run on boot or from the seed tool.
//
// Client data isolation: every client-owned collection carries clientId, and
// client-scoped queries lead with it so a client can only ever read its own
// slice of the data.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	indexes := map[string][]mongo.IndexModel{
		// Accounts. Email is the login identifier — globally unique.
		"users": {
			{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "passwordResetTokenHash", Value: 1}}, Options: options.Index().SetUnique(true).SetSparse(true)},
			// Login/role lookups: admin lists staff, auth filters by role.
			{Keys: bson.D{{Key: "role", Value: 1}}},
		},

		// Refresh sessions. tokenHash is unique; the TTL index lets MongoDB
		// reap expired sessions; userId+revoked backs "list/revoke my sessions".
		"refresh_tokens": {
			{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "revoked", Value: 1}}},
		},

		// Brute-force accounting (BE-02). One record per login identifier
		// (the submitted email, real or not); the TTL index on expiresAt
		// reaps a record once its window and cooldown have passed.
		"login_attempts": {
			{Keys: bson.D{{Key: "identifier", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
		},

		// Services a practitioner offers; client app lists active ones ordered.
		"services": {
			{Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "active", Value: 1}, {Key: "sortOrder", Value: 1}}},
		},

		// Weekly opening hours per practitioner. One rule per weekday.
		"availability_rules": {
			{Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "weekday", Value: 1}}, Options: options.Index().SetUnique(true)},
		},

		// Holidays / blocked periods; checked when computing free slots.
		"time_off": {
			{Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "startAt", Value: 1}}},
		},

		// Bookings. Client portal reads by clientId (isolation leads); the
		// calendar reads by practitioner; admin dashboards filter by status.
		// The partial unique index makes a confirmed double-booking physically
		// impossible — the second insert fails even under a race.
		"bookings": {
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "startAt", Value: -1}}},
			{Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "startAt", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "startAt", Value: 1}}},
			{
				Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "startAt", Value: 1}},
				Options: options.Index().
					SetUnique(true).
					SetName("unique_confirmed_slot").
					SetPartialFilterExpression(bson.D{{Key: "status", Value: "confirmed"}}),
			},
		},

		// Payments. One payment record per booking; client history leads with
		// clientId; the Paystack reference is unique for webhook idempotency
		// (partial — pending records may not have a reference yet).
		"payments": {
			{Keys: bson.D{{Key: "bookingId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "createdAt", Value: -1}}},
			{
				Keys: bson.D{{Key: "paystackReference", Value: 1}},
				Options: options.Index().
					SetUnique(true).
					SetPartialFilterExpression(bson.D{{Key: "paystackReference", Value: bson.D{{Key: "$type", Value: "string"}}}}),
			},
			// The other half of the webhook join. A re-initialized payment
			// keeps the references it used before, because the checkout
			// pages behind them are still live and can still be paid; a
			// delivery quoting one has to find this record. Not unique: the
			// field is an array and only exists on records that were
			// re-initialized, which is why it is sparse.
			{
				Keys:    bson.D{{Key: "previousReferences", Value: 1}},
				Options: options.Index().SetSparse(true),
			},
		},

		// Practitioner treatment notes. One note per booking; a client's
		// history is client-scoped (isolation leads).
		"session_notes": {
			{Keys: bson.D{{Key: "bookingId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},

		// Practice-side client profiles (contact detail, tags, private
		// summary). One profile per client account — upserted by userId.
		"client_profiles": {
			{Keys: bson.D{{Key: "userId", Value: 1}}, Options: options.Index().SetUnique(true)},
		},

		// Intake/consent form definitions. Templates are the reusable library
		// the admin clones from; list views order by sortOrder.
		"forms": {
			{Keys: bson.D{{Key: "template", Value: 1}, {Key: "sortOrder", Value: 1}}},
		},

		// Filled-in forms. Client portal lists own submissions by status
		// (isolation leads); booking lookup attaches the form to a visit.
		"form_submissions": {
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "bookingId", Value: 1}}},
		},

		// Uploaded files (medical history, etc.) — strictly client-scoped.
		"documents": {
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "createdAt", Value: -1}}},
		},

		// Editable site pages (about, policies, ...). Slug is the URL key.
		"cms_pages": {
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
		},

		// Blog. Slug is the URL key; the public feed lists published posts.
		"blog_posts": {
			{Keys: bson.D{{Key: "slug", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "publishedAt", Value: -1}}},
		},

		// FAQ entries shown in a fixed, admin-controlled order.
		"faqs": {
			{Keys: bson.D{{Key: "sortOrder", Value: 1}}},
		},

		// Testimonials pending/approved for the public site.
		"testimonials": {
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
		},

		// Contact-form enquiries triaged by status in the admin app.
		"enquiries": {
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
		},

		// Post-visit reviews (BE-14). One review per booking, enforced so a
		// double submit loses the race rather than creating a second row.
		// Client portal reads own reviews (isolation leads); the moderation
		// queue and the public list both filter by status.
		"reviews": {
			{Keys: bson.D{{Key: "bookingId", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "clientId", Value: 1}, {Key: "createdAt", Value: -1}}},
			{Keys: bson.D{{Key: "practitionerId", Value: 1}, {Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
		},

		// Outbound message outbox (BE-09). The dispatcher claims due,
		// pending jobs oldest-first — that query runs on a timer and must
		// stay indexed. The booking+kind index backs "cancel this
		// booking's reminder" when a session moves or is called off.
		"notification_jobs": {
			{Keys: bson.D{{Key: "status", Value: 1}, {Key: "dueAt", Value: 1}}},
			{Keys: bson.D{{Key: "bookingId", Value: 1}, {Key: "kind", Value: 1}, {Key: "status", Value: 1}}},
		},
	}

	for collection, models := range indexes {
		if _, err := db.Collection(collection).Indexes().CreateMany(ctx, models); err != nil {
			return fmt.Errorf("ensure indexes for %s: %w", collection, err)
		}
	}
	return nil
}
