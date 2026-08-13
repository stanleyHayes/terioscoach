package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/client"
)

// PaymentSummary is the rollup of a client's payments on the practitioner's
// bookings. Money is integer minor units; Currency comes from the most
// recent payment and is empty when the client has none.
type PaymentSummary struct {
	TotalPaidKobo     int64
	TotalRefundedKobo int64
	PaymentCount      int
	Currency          string
}

// ClientRecord is the full practice-side record assembled for
// GET /v1/clients/{id}: profile + account identity + recent bookings +
// payment rollup + document/form counts.
type ClientRecord struct {
	Profile             client.Profile
	Name                string
	Email               string
	RecentBookings      []booking.Booking
	Payments            PaymentSummary
	DocumentCount       int
	FormSubmissionCount int
}

// ClientMe is the client's own view of their profile. Practice-side fields
// (practice notes, tags) are absent by construction.
type ClientMe struct {
	ID        string
	Name      string
	Email     string
	Phone     string
	CreatedAt time.Time
}

// ClientService is the inbound port for the client-records slice.
type ClientService interface {
	// ListClients returns every client of the practice (users with at least
	// one booking with the practitioner, any status) with the booking
	// rollup, ordered by last session descending then name ascending.
	ListClients(ctx context.Context, practitionerID string) ([]client.Summary, error)
	// GetClientRecord assembles the full record for one client of the
	// practice. A user with no bookings at this practice returns
	// client.ErrClientNotFound (isolation).
	GetClientRecord(ctx context.Context, practitionerID, clientUserID string) (ClientRecord, error)
	// UpdatePracticeFields applies the practitioner-owned patch, creating
	// the profile on first write (upsert keyed on the user id). Membership
	// is enforced exactly like GetClientRecord.
	UpdatePracticeFields(ctx context.Context, practitionerID, clientUserID string, patch client.PracticePatch) (client.Profile, error)
	// GetMe returns the caller's own profile, without practice-side fields.
	GetMe(ctx context.Context, clientUserID string) (ClientMe, error)
}

// ClientProfileRepository is the outbound port for practice-profile
// persistence (the client_profiles collection).
type ClientProfileRepository interface {
	// Upsert persists a profile keyed on UserID, assigning its ID on
	// insert. The caller stamps CreatedAt; an existing profile keeps its
	// original CreatedAt.
	Upsert(ctx context.Context, profile client.Profile) (client.Profile, error)
	// FindByUserID looks up a profile by the client's user id; misses
	// return client.ErrProfileNotFound.
	FindByUserID(ctx context.Context, userID string) (client.Profile, error)
}

// DocumentCounter counts a client's uploaded documents (the documents
// collection is owned by the documents slice — this is a count only).
type DocumentCounter interface {
	CountByClient(ctx context.Context, clientID string) (int, error)
}

// FormSubmissionCounter counts a client's form submissions (the
// form_submissions collection is owned by the forms slice — count only).
type FormSubmissionCounter interface {
	CountByClient(ctx context.Context, clientID string) (int, error)
}
