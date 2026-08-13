// Package reporting is the domain core for practice reporting: the rules
// that turn bookings, payments, and reviews into the numbers a practitioner
// reads. It imports nothing outside the standard library — no frameworks,
// no drivers, and no repositories.
//
// Everything here is a pure function over slices the caller has already
// fetched. That is deliberate: the interesting part of a report is not the
// query, it is the definitions — what counts as a session, which money is
// income, where a week starts — and those are worth stating once, in one
// place, with tests.
package reporting

import (
	"sort"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/payment"
)

// Granularity is the bucket size for a time series.
type Granularity string

const (
	Daily   Granularity = "day"
	Weekly  Granularity = "week"
	Monthly Granularity = "month"
)

// Valid reports whether g is a known granularity.
func (g Granularity) Valid() bool {
	switch g {
	case Daily, Weekly, Monthly:
		return true
	}
	return false
}

// Range is the reporting window: from inclusive, to exclusive. A
// half-open window is what makes adjacent periods add up — a session at
// midnight belongs to exactly one of them.
type Range struct {
	From time.Time
	To   time.Time
}

// Contains reports whether an instant falls inside the window.
func (r Range) Contains(at time.Time) bool {
	at = at.UTC()
	return !at.Before(r.From.UTC()) && at.Before(r.To.UTC())
}

// Summary is the headline set of numbers for one window.
type Summary struct {
	// SessionsCompleted counts bookings that were seen through.
	SessionsCompleted int
	// SessionsUpcoming counts confirmed bookings still ahead of `now`.
	SessionsUpcoming int
	// Cancellations and NoShows are counted separately: one is a client
	// changing plans, the other is unrecovered time, and a practitioner
	// reads them differently.
	Cancellations int
	NoShows       int
	// NewClients counts clients whose first-ever booking falls in the
	// window — the growth number, not the activity number.
	NewClients int
	// IncomeKobo is money actually collected: successful payments only.
	IncomeKobo int64
	// RefundedKobo is money given back. It is reported beside income
	// rather than subtracted from it, because a practitioner needs to see
	// both — netting them hides a refund-heavy month.
	RefundedKobo int64
	Currency     string
}

// NetKobo is income after refunds, for the callers that want one number.
func (s Summary) NetKobo() int64 { return s.IncomeKobo - s.RefundedKobo }

// ServiceIncome is one service's contribution over the window.
type ServiceIncome struct {
	ServiceID  string
	Name       string
	Sessions   int
	IncomeKobo int64
}

// PeriodIncome is one bucket of a time series.
type PeriodIncome struct {
	// Start is the bucket's first instant, UTC.
	Start      time.Time
	Sessions   int
	IncomeKobo int64
}

// DayLoad is how many confirmed sessions fall on one upcoming day.
type DayLoad struct {
	Date     time.Time
	Sessions int
}

// Inputs is everything a report is computed from. The caller fetches; this
// package decides what the numbers mean.
type Inputs struct {
	// Bookings are the practitioner's bookings overlapping the window,
	// plus enough history to identify first-time clients.
	Bookings []booking.Booking
	// Payments are the payment records on those bookings.
	Payments []payment.Payment
	// ServiceNames resolves service ids for the per-service breakdown.
	ServiceNames map[string]string
	// FirstBookingByClient is each client's earliest booking time across
	// all history — what makes "new client" mean new, not just active.
	FirstBookingByClient map[string]time.Time
}

// Summarize computes the headline numbers for the window.
//
// Sessions are counted by when they happened (StartAt); money is counted by
// when it was collected (PaidAt). Those are genuinely different questions —
// a session paid for in advance belongs to next month's diary and this
// month's income — and conflating them is the classic way a practice report
// stops reconciling.
func Summarize(in Inputs, window Range, now time.Time) Summary {
	var summary Summary
	now = now.UTC()

	for _, b := range in.Bookings {
		if !window.Contains(b.StartAt) {
			continue
		}
		switch b.Status {
		case booking.StatusCompleted:
			summary.SessionsCompleted++
		case booking.StatusCancelled:
			summary.Cancellations++
		case booking.StatusNoShow:
			summary.NoShows++
		case booking.StatusConfirmed:
			if b.StartAt.UTC().After(now) {
				summary.SessionsUpcoming++
			} else {
				// A confirmed booking whose time has passed but which the
				// practitioner has not closed off yet is still a session
				// that happened; counting it as neither would make the
				// diary and the report disagree.
				summary.SessionsCompleted++
			}
		}
	}

	for _, p := range in.Payments {
		at, ok := collectedAt(p)
		if !ok || !window.Contains(at) {
			continue
		}
		switch p.Status {
		case payment.StatusSuccess:
			summary.IncomeKobo += p.AmountKobo
		case payment.StatusRefunded:
			summary.RefundedKobo += p.AmountKobo
		}
		if summary.Currency == "" {
			summary.Currency = p.Currency
		}
	}

	for clientID, first := range in.FirstBookingByClient {
		if clientID != "" && window.Contains(first) {
			summary.NewClients++
		}
	}
	return summary
}

// IncomeByService breaks the window down per service, biggest earner first.
// A service with sessions but no collected money still appears — unpaid
// work is exactly what a practitioner wants to see.
func IncomeByService(in Inputs, window Range) []ServiceIncome {
	byService := map[string]*ServiceIncome{}
	ensure := func(serviceID string) *ServiceIncome {
		row := byService[serviceID]
		if row == nil {
			row = &ServiceIncome{ServiceID: serviceID, Name: in.ServiceNames[serviceID]}
			byService[serviceID] = row
		}
		return row
	}

	serviceOfBooking := make(map[string]string, len(in.Bookings))
	for _, b := range in.Bookings {
		serviceOfBooking[b.ID] = b.ServiceID
		if window.Contains(b.StartAt) && countsAsSession(b) {
			ensure(b.ServiceID).Sessions++
		}
	}

	for _, p := range in.Payments {
		if p.Status != payment.StatusSuccess {
			continue
		}
		at, ok := collectedAt(p)
		if !ok || !window.Contains(at) {
			continue
		}
		ensure(serviceOfBooking[p.BookingID]).IncomeKobo += p.AmountKobo
	}

	out := make([]ServiceIncome, 0, len(byService))
	for _, row := range byService {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IncomeKobo != out[j].IncomeKobo {
			return out[i].IncomeKobo > out[j].IncomeKobo
		}
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// IncomeByPeriod builds a time series across the window.
//
// Every bucket in the range is emitted, including empty ones: a chart with
// the quiet weeks missing tells a much rosier story than the truth.
func IncomeByPeriod(in Inputs, window Range, granularity Granularity) []PeriodIncome {
	if !granularity.Valid() {
		granularity = Daily
	}

	buckets := map[time.Time]*PeriodIncome{}
	for start := bucketStart(window.From, granularity); start.Before(window.To.UTC()); start = nextBucket(start, granularity) {
		buckets[start] = &PeriodIncome{Start: start}
	}
	at := func(t time.Time) *PeriodIncome {
		return buckets[bucketStart(t, granularity)]
	}

	for _, b := range in.Bookings {
		if !window.Contains(b.StartAt) || !countsAsSession(b) {
			continue
		}
		if bucket := at(b.StartAt); bucket != nil {
			bucket.Sessions++
		}
	}
	for _, p := range in.Payments {
		if p.Status != payment.StatusSuccess {
			continue
		}
		collected, ok := collectedAt(p)
		if !ok || !window.Contains(collected) {
			continue
		}
		if bucket := at(collected); bucket != nil {
			bucket.IncomeKobo += p.AmountKobo
		}
	}

	out := make([]PeriodIncome, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, *bucket)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// UpcomingLoad counts confirmed sessions per day over the next `days`,
// starting today. Empty days are included so the shape of the week is
// visible.
func UpcomingLoad(bookings []booking.Booking, now time.Time, days int) []DayLoad {
	if days < 1 {
		days = 7
	}
	start := now.UTC().Truncate(24 * time.Hour)

	load := make([]DayLoad, 0, days)
	index := make(map[time.Time]int, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		index[day] = i
		load = append(load, DayLoad{Date: day})
	}

	for _, b := range bookings {
		if b.Status != booking.StatusConfirmed || b.StartAt.UTC().Before(now.UTC()) {
			continue
		}
		if i, ok := index[b.StartAt.UTC().Truncate(24*time.Hour)]; ok {
			load[i].Sessions++
		}
	}
	return load
}

// countsAsSession reports whether a booking represents time actually
// spent: completed, or confirmed. A cancellation freed the slot and a
// no-show is counted on its own.
func countsAsSession(b booking.Booking) bool {
	return b.Status == booking.StatusCompleted || b.Status == booking.StatusConfirmed
}

// collectedAt is when money moved. A successful payment uses PaidAt; a
// refund uses RefundedAt. Falling back to CreatedAt would date a refund to
// the original sale, which is the wrong month.
func collectedAt(p payment.Payment) (time.Time, bool) {
	switch p.Status {
	case payment.StatusSuccess:
		if p.PaidAt != nil {
			return p.PaidAt.UTC(), true
		}
	case payment.StatusRefunded:
		if p.RefundedAt != nil {
			return p.RefundedAt.UTC(), true
		}
		if p.PaidAt != nil {
			return p.PaidAt.UTC(), true
		}
	}
	return time.Time{}, false
}

// bucketStart truncates an instant to the start of its bucket. Weeks start
// on Monday, which is how a practice diary reads.
func bucketStart(at time.Time, granularity Granularity) time.Time {
	at = at.UTC()
	day := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC)
	switch granularity {
	case Weekly:
		offset := (int(day.Weekday()) + 6) % 7 // Monday = 0
		return day.AddDate(0, 0, -offset)
	case Monthly:
		return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return day
	}
}

// nextBucket steps one bucket forward.
func nextBucket(start time.Time, granularity Granularity) time.Time {
	switch granularity {
	case Weekly:
		return start.AddDate(0, 0, 7)
	case Monthly:
		return start.AddDate(0, 1, 0)
	default:
		return start.AddDate(0, 0, 1)
	}
}
