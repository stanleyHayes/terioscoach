// Package reports is the application service for practice reporting. It
// implements the inbound ports.ReportService port purely against outbound
// ports — no framework, driver, or transport imports.
//
// It fetches; the reporting domain decides what the numbers mean. Keeping
// the split that way is what makes the definitions ("a cancelled booking
// is not a session", "income is dated by when it was collected") testable
// without a database.
package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/reporting"
	"github.com/xcreativs/terios/api/internal/domain/review"
	"github.com/xcreativs/terios/api/internal/ports"
)

// maxWindow bounds a report request. A practice's whole history is a
// perfectly reasonable thing to want, but not in one unbounded query
// triggered by a URL.
const maxWindow = 731 * 24 * time.Hour // two years

// Service assembles practice reports over outbound ports.
type Service struct {
	bookings ports.BookingRepository
	payments ports.PaymentRepository
	services ports.ServiceRepository
	reviews  ports.ReviewRepository
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.ReportService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. Every repository
// is shared with the slice that owns it: a report is a read over the same
// records the rest of the API writes, never a parallel set of numbers.
func NewService(
	bookings ports.BookingRepository,
	payments ports.PaymentRepository,
	services ports.ServiceRepository,
	reviews ports.ReviewRepository,
) *Service {
	return &Service{
		bookings: bookings,
		payments: payments,
		services: services,
		reviews:  reviews,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Practice assembles the dashboard report for one window.
func (s *Service) Practice(ctx context.Context, practitionerID string, req ports.ReportRequest) (ports.PracticeReport, error) {
	window, granularity, err := normalize(req, s.now())
	if err != nil {
		return ports.PracticeReport{}, err
	}

	// The practitioner's whole booking history is fetched, not just the
	// window: "new client" means first-ever, which cannot be answered from
	// a window alone. A single practice's history is small enough that
	// this is the honest, simple answer.
	all, err := s.bookings.ListByPractitioner(ctx, practitionerID, ports.BookingFilter{})
	if err != nil {
		return ports.PracticeReport{}, fmt.Errorf("list bookings: %w", err)
	}

	bookingIDs := make([]string, 0, len(all))
	for _, b := range all {
		bookingIDs = append(bookingIDs, b.ID)
	}
	payments, err := s.payments.ListByBookingIDs(ctx, bookingIDs, ports.PaymentFilter{})
	if err != nil {
		return ports.PracticeReport{}, fmt.Errorf("list payments: %w", err)
	}

	serviceNames, err := s.serviceNames(ctx, practitionerID)
	if err != nil {
		return ports.PracticeReport{}, err
	}

	inputs := reporting.Inputs{
		Bookings:             all,
		Payments:             payments,
		ServiceNames:         serviceNames,
		FirstBookingByClient: firstBookingByClient(all),
	}

	report := ports.PracticeReport{
		From:        window.From,
		To:          window.To,
		Summary:     reporting.Summarize(inputs, window, s.now()),
		ByService:   reporting.IncomeByService(inputs, window),
		Series:      reporting.IncomeByPeriod(inputs, window, granularity),
		Granularity: granularity,
	}

	// A degraded review store must not take the whole dashboard down: the
	// booking and money numbers are the ones a practitioner is here for.
	if approved, err := s.reviews.ListByPractitioner(ctx, practitionerID, ports.ReviewFilter{ApprovedOnly: true}); err == nil {
		report.Reviews = review.Summarize(approved)
	}
	return report, nil
}

// UpcomingLoad is the next `days` of confirmed sessions per day.
func (s *Service) UpcomingLoad(ctx context.Context, practitionerID string, days int) ([]reporting.DayLoad, error) {
	if days < 1 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	now := s.now()
	from := now.Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, days)

	bookings, err := s.bookings.ListByPractitioner(ctx, practitionerID, ports.BookingFilter{
		From: &from,
		To:   &to,
	})
	if err != nil {
		return nil, fmt.Errorf("list bookings: %w", err)
	}
	return reporting.UpcomingLoad(bookings, now, days), nil
}

// serviceNames resolves the practitioner's catalog for the per-service
// breakdown, including retired services — a report of last quarter must
// still be able to name what was sold.
func (s *Service) serviceNames(ctx context.Context, practitionerID string) (map[string]string, error) {
	catalog, err := s.services.ListByPractitioner(ctx, practitionerID, false)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	names := make(map[string]string, len(catalog))
	for _, svc := range catalog {
		names[svc.ID] = svc.Name
	}
	return names, nil
}

// firstBookingByClient finds each client's earliest booking, which is what
// makes "new client" mean new rather than merely active.
func firstBookingByClient(bookings []booking.Booking) map[string]time.Time {
	first := make(map[string]time.Time, len(bookings))
	for _, b := range bookings {
		at := b.StartAt.UTC()
		if existing, ok := first[b.ClientID]; !ok || at.Before(existing) {
			first[b.ClientID] = at
		}
	}
	return first
}

// normalize validates the request and fills in the defaults: the current
// calendar month, bucketed by day.
func normalize(req ports.ReportRequest, now time.Time) (reporting.Range, reporting.Granularity, error) {
	granularity := req.Granularity
	if granularity == "" {
		granularity = reporting.Daily
	}
	if !granularity.Valid() {
		return reporting.Range{}, "", ErrInvalidGranularity
	}

	from, to := req.From.UTC(), req.To.UTC()
	if from.IsZero() && to.IsZero() {
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = from.AddDate(0, 1, 0)
	}
	if from.IsZero() || to.IsZero() || !from.Before(to) {
		return reporting.Range{}, "", ErrInvalidRange
	}
	if to.Sub(from) > maxWindow {
		return reporting.Range{}, "", ErrRangeTooLong
	}
	return reporting.Range{From: from, To: to}, granularity, nil
}
