// Package clients is the application service for the client-records slice.
// It implements the inbound ports.ClientService port purely against
// outbound ports — no framework, driver, or transport imports. Client
// isolation leads every read with the client id, and practice membership is
// proven through bookings: a user is a client of the practice once they
// have at least one booking with the practitioner.
package clients

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
)

// recentBookingsCap bounds the booking history embedded in the full record.
const recentBookingsCap = 10

// Service orchestrates the client-records use cases over outbound ports.
type Service struct {
	profiles  ports.ClientProfileRepository
	users     ports.UserRepository
	bookings  ports.BookingRepository
	payments  ports.PaymentRepository
	documents ports.DocumentCounter
	forms     ports.FormSubmissionCounter
	now       func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.ClientService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. The user,
// booking, and payment repositories are shared with their own slices: the
// record is assembled from those read models, never re-queried raw. The
// document and form counters are count-only readers of the collections
// owned by the documents and forms slices.
func NewService(
	profiles ports.ClientProfileRepository,
	users ports.UserRepository,
	bookings ports.BookingRepository,
	payments ports.PaymentRepository,
	documents ports.DocumentCounter,
	forms ports.FormSubmissionCounter,
) *Service {
	return &Service{
		profiles:  profiles,
		users:     users,
		bookings:  bookings,
		payments:  payments,
		documents: documents,
		forms:     forms,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// ListClients returns every client of the practice with the booking
// rollup. Membership comes from the practitioner's bookings; the rollup
// counts non-cancelled bookings only (a cancelled booking is not a
// session).
func (s *Service) ListClients(ctx context.Context, practitionerID string) ([]client.Summary, error) {
	bookings, err := s.bookings.ListByPractitioner(ctx, practitionerID, ports.BookingFilter{})
	if err != nil {
		return nil, err
	}

	type rollup struct {
		total    int
		last     *time.Time
		anyAtAll bool
	}
	rollups := make(map[string]*rollup)
	for _, b := range bookings {
		r := rollups[b.ClientID]
		if r == nil {
			r = &rollup{}
			rollups[b.ClientID] = r
		}
		r.anyAtAll = true
		if b.Status == booking.StatusCancelled {
			continue
		}
		r.total++
		if r.last == nil || b.StartAt.After(*r.last) {
			last := b.StartAt
			r.last = &last
		}
	}

	out := make([]client.Summary, 0, len(rollups))
	for clientID, r := range rollups {
		user, err := s.users.FindByID(ctx, clientID)
		if err != nil {
			// A booking whose account is gone is a data-integrity gap —
			// skip the row rather than failing the whole list.
			continue
		}
		profile := s.profileOrEmpty(ctx, clientID)
		out = append(out, client.Summary{
			Profile:       profile,
			Name:          user.Name,
			Email:         user.Email,
			TotalSessions: r.total,
			LastSessionAt: r.last,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].LastSessionAt, out[j].LastSessionAt
		if (a == nil) != (b == nil) {
			return a != nil // clients with sessions first
		}
		if a != nil && !a.Equal(*b) {
			return a.After(*b)
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// GetClientRecord assembles the full practice-side record: profile,
// identity, recent bookings, payment rollup, and document/form counts.
// A user with no bookings at this practice is not a client here — the
// result is ErrClientNotFound, indistinguishable from an unknown id.
func (s *Service) GetClientRecord(ctx context.Context, practitionerID, clientUserID string) (ports.ClientRecord, error) {
	user, bookings, err := s.loadMember(ctx, practitionerID, clientUserID)
	if err != nil {
		return ports.ClientRecord{}, err
	}

	recent := append([]booking.Booking(nil), bookings...)
	sort.Slice(recent, func(i, j int) bool { return recent[i].StartAt.After(recent[j].StartAt) })
	if len(recent) > recentBookingsCap {
		recent = recent[:recentBookingsCap]
	}

	bookingIDs := make([]string, 0, len(bookings))
	for _, b := range bookings {
		bookingIDs = append(bookingIDs, b.ID)
	}
	payments, err := s.payments.ListByBookingIDs(ctx, bookingIDs, ports.PaymentFilter{})
	if err != nil {
		return ports.ClientRecord{}, err
	}

	documentCount, err := s.documents.CountByClient(ctx, clientUserID)
	if err != nil {
		return ports.ClientRecord{}, err
	}
	formCount, err := s.forms.CountByClient(ctx, clientUserID)
	if err != nil {
		return ports.ClientRecord{}, err
	}

	return ports.ClientRecord{
		Profile:             s.profileOrEmpty(ctx, clientUserID),
		Name:                user.Name,
		Email:               user.Email,
		RecentBookings:      recent,
		Payments:            summarizePayments(payments),
		DocumentCount:       documentCount,
		FormSubmissionCount: formCount,
	}, nil
}

// UpdatePracticeFields applies the practitioner-owned patch, creating the
// profile on first write. Membership is enforced exactly like the record
// read — patching a non-client is ErrClientNotFound.
func (s *Service) UpdatePracticeFields(ctx context.Context, practitionerID, clientUserID string, patch client.PracticePatch) (client.Profile, error) {
	if _, _, err := s.loadMember(ctx, practitionerID, clientUserID); err != nil {
		return client.Profile{}, err
	}
	profile, err := s.profiles.FindByUserID(ctx, clientUserID)
	switch {
	case err == nil:
		// existing profile — patched below
	case errors.Is(err, client.ErrProfileNotFound):
		profile = client.New(clientUserID, s.now())
	default:
		return client.Profile{}, err
	}
	if err := profile.ApplyPracticePatch(patch, s.now()); err != nil {
		return client.Profile{}, err
	}
	return s.profiles.Upsert(ctx, profile)
}

// GetMe returns the client's own profile. The account always exists (the
// caller is authenticated); the practice profile may not — phone is simply
// empty until the practitioner records one.
func (s *Service) GetMe(ctx context.Context, clientUserID string) (ports.ClientMe, error) {
	user, err := s.users.FindByID(ctx, clientUserID)
	if err != nil {
		return ports.ClientMe{}, err
	}
	return ports.ClientMe{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		Phone:     s.profileOrEmpty(ctx, clientUserID).Phone,
		CreatedAt: user.CreatedAt,
	}, nil
}

// loadMember proves practice membership and returns the account plus the
// client's bookings with this practitioner. The query leads with the
// client id (isolation) and is then narrowed to this practitioner.
func (s *Service) loadMember(ctx context.Context, practitionerID, clientUserID string) (identity.User, []booking.Booking, error) {
	user, err := s.users.FindByID(ctx, clientUserID)
	if err != nil {
		return identity.User{}, nil, client.ErrClientNotFound
	}
	own, err := s.bookings.ListByClient(ctx, clientUserID)
	if err != nil {
		return identity.User{}, nil, err
	}
	var bookings []booking.Booking
	for _, b := range own {
		if b.PractitionerID == practitionerID {
			bookings = append(bookings, b)
		}
	}
	if len(bookings) == 0 {
		return identity.User{}, nil, client.ErrClientNotFound
	}
	return user, bookings, nil
}

// profileOrEmpty loads the practice profile, substituting an empty one
// (linked to the user, empty tags) when the practitioner has never PATCHed.
func (s *Service) profileOrEmpty(ctx context.Context, userID string) client.Profile {
	profile, err := s.profiles.FindByUserID(ctx, userID)
	if err == nil {
		return profile
	}
	if !errors.Is(err, client.ErrProfileNotFound) {
		// A degraded profile store must not blank the rest of the record;
		// the empty profile is the safe fallback either way.
	}
	return client.Profile{UserID: userID, Tags: []string{}}
}

// summarizePayments rolls a client's payment records into the contract
// summary. The list is newest-first, so the first record's currency is the
// most recent one.
func summarizePayments(payments []payment.Payment) ports.PaymentSummary {
	var sum ports.PaymentSummary
	for _, p := range payments {
		sum.PaymentCount++
		if sum.Currency == "" {
			sum.Currency = p.Currency
		}
		switch p.Status {
		case payment.StatusSuccess:
			sum.TotalPaidKobo += p.AmountKobo
		case payment.StatusRefunded:
			sum.TotalRefundedKobo += p.AmountKobo
		}
	}
	return sum
}
