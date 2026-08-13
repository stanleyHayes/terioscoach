package reporting

import (
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/payment"
)

// The window is August 2026; "now" sits mid-month so past and future are
// both representable inside it.
var (
	windowFrom = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	windowTo   = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	fixedNow   = time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	window     = Range{From: windowFrom, To: windowTo}
)

func day(d int, hour int) time.Time {
	return time.Date(2026, 8, d, hour, 0, 0, 0, time.UTC)
}

func mkBooking(id, clientID, serviceID string, start time.Time, status booking.Status) booking.Booking {
	return booking.Booking{
		ID: id, ClientID: clientID, PractitionerID: "prac-1", ServiceID: serviceID,
		StartAt: start, EndAt: start.Add(time.Hour), Status: status,
	}
}

func mkPayment(bookingID string, amount int64, status payment.Status, at time.Time) payment.Payment {
	p := payment.Payment{
		BookingID: bookingID, ClientID: "client-1", AmountKobo: amount,
		Currency: "GHS", Status: status, CreatedAt: at,
	}
	stamp := at
	switch status {
	case payment.StatusSuccess:
		p.PaidAt = &stamp
	case payment.StatusRefunded:
		p.RefundedAt = &stamp
	}
	return p
}

// TestSummarizeCountsEachOutcomeSeparately.
func TestSummarizeCountsEachOutcomeSeparately(t *testing.T) {
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-1", day(3, 9), booking.StatusCompleted),
			mkBooking("b2", "client-1", "svc-1", day(4, 9), booking.StatusCancelled),
			mkBooking("b3", "client-2", "svc-1", day(5, 9), booking.StatusNoShow),
			mkBooking("b4", "client-2", "svc-1", day(20, 9), booking.StatusConfirmed), // ahead
			// Confirmed but already past: the practitioner has not closed it
			// off yet, and it is still a session that happened.
			mkBooking("b5", "client-1", "svc-1", day(10, 9), booking.StatusConfirmed),
			// Outside the window entirely.
			mkBooking("b6", "client-1", "svc-1", time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), booking.StatusCompleted),
		},
	}

	summary := Summarize(in, window, fixedNow)

	if summary.SessionsCompleted != 2 {
		t.Errorf("completed = %d, want 2 (one closed, one past-confirmed)", summary.SessionsCompleted)
	}
	if summary.SessionsUpcoming != 1 {
		t.Errorf("upcoming = %d, want 1", summary.SessionsUpcoming)
	}
	if summary.Cancellations != 1 || summary.NoShows != 1 {
		t.Errorf("summary = %+v, want cancellations and no-shows counted apart", summary)
	}
}

// TestIncomeIsDatedByWhenItWasCollected: a session paid for in advance
// belongs to next month's diary and this month's income.
func TestIncomeIsDatedByWhenItWasCollected(t *testing.T) {
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-1", time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), booking.StatusConfirmed),
		},
		Payments: []payment.Payment{
			mkPayment("b1", 25000, payment.StatusSuccess, day(12, 10)), // paid in August
		},
	}

	summary := Summarize(in, window, fixedNow)
	if summary.IncomeKobo != 25000 {
		t.Errorf("income = %d, want the August payment counted", summary.IncomeKobo)
	}
	if summary.SessionsCompleted != 0 || summary.SessionsUpcoming != 0 {
		t.Errorf("summary = %+v, want the September session outside this window", summary)
	}
}

// TestRefundsAreReportedBesideIncome: netting them would hide a
// refund-heavy month.
func TestRefundsAreReportedBesideIncome(t *testing.T) {
	in := Inputs{
		Payments: []payment.Payment{
			mkPayment("b1", 25000, payment.StatusSuccess, day(3, 10)),
			mkPayment("b2", 10000, payment.StatusRefunded, day(9, 10)),
			mkPayment("b3", 5000, payment.StatusPending, day(9, 10)),
			mkPayment("b4", 7000, payment.StatusFailed, day(9, 10)),
		},
	}

	summary := Summarize(in, window, fixedNow)
	if summary.IncomeKobo != 25000 {
		t.Errorf("income = %d, want only the successful payment", summary.IncomeKobo)
	}
	if summary.RefundedKobo != 10000 {
		t.Errorf("refunded = %d, want the refund reported separately", summary.RefundedKobo)
	}
	if summary.NetKobo() != 15000 {
		t.Errorf("net = %d, want 15000", summary.NetKobo())
	}
	if summary.Currency != "GHS" {
		t.Errorf("currency = %q, want GHS", summary.Currency)
	}
}

// TestRefundIsDatedToTheRefund: dating it to the original sale would put it
// in the wrong month.
func TestRefundIsDatedToTheRefund(t *testing.T) {
	paidAt := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	refundedAt := day(5, 10)
	refund := payment.Payment{
		BookingID: "b1", AmountKobo: 10000, Currency: "GHS",
		Status: payment.StatusRefunded, PaidAt: &paidAt, RefundedAt: &refundedAt,
	}

	summary := Summarize(Inputs{Payments: []payment.Payment{refund}}, window, fixedNow)
	if summary.RefundedKobo != 10000 {
		t.Errorf("refunded = %d, want the August refund counted in August", summary.RefundedKobo)
	}
}

// TestNewClientsAreNewNotMerelyActive.
func TestNewClientsAreNewNotMerelyActive(t *testing.T) {
	in := Inputs{
		FirstBookingByClient: map[string]time.Time{
			"client-1": day(2, 9),                                   // first ever, in window
			"client-2": time.Date(2025, 3, 1, 9, 0, 0, 0, time.UTC), // long-standing
			"client-3": day(28, 9),                                  // first ever, in window
			"client-4": time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC), // after the window
		},
	}

	summary := Summarize(in, window, fixedNow)
	if summary.NewClients != 2 {
		t.Errorf("newClients = %d, want the 2 whose first booking is in the window", summary.NewClients)
	}
}

// TestWindowIsHalfOpen: adjacent periods must add up, with midnight in
// exactly one of them.
func TestWindowIsHalfOpen(t *testing.T) {
	if !window.Contains(windowFrom) {
		t.Error("the window excludes its own start")
	}
	if window.Contains(windowTo) {
		t.Error("the window includes its end — adjacent periods would double-count")
	}
	if window.Contains(windowTo.Add(-time.Nanosecond)) != true {
		t.Error("the window excludes its last instant")
	}
}

// TestIncomeByServiceRanksAndIncludesUnpaidWork.
func TestIncomeByServiceRanksAndIncludesUnpaidWork(t *testing.T) {
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-massage", day(3, 9), booking.StatusCompleted),
			mkBooking("b2", "client-2", "svc-massage", day(4, 9), booking.StatusCompleted),
			mkBooking("b3", "client-1", "svc-reiki", day(5, 9), booking.StatusCompleted),
			mkBooking("b4", "client-1", "svc-consult", day(6, 9), booking.StatusCompleted), // never paid
			mkBooking("b5", "client-1", "svc-massage", day(7, 9), booking.StatusCancelled), // freed slot
		},
		Payments: []payment.Payment{
			mkPayment("b1", 25000, payment.StatusSuccess, day(3, 10)),
			mkPayment("b2", 25000, payment.StatusSuccess, day(4, 10)),
			mkPayment("b3", 40000, payment.StatusSuccess, day(5, 10)),
		},
		ServiceNames: map[string]string{
			"svc-massage": "Massage", "svc-reiki": "Reiki", "svc-consult": "Consultation",
		},
	}

	rows := IncomeByService(in, window)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want one per service with activity", len(rows))
	}
	if rows[0].ServiceID != "svc-massage" || rows[0].IncomeKobo != 50000 || rows[0].Sessions != 2 {
		t.Errorf("top row = %+v, want massage at 50000 from 2 sessions (the cancellation excluded)", rows[0])
	}
	if rows[1].ServiceID != "svc-reiki" || rows[1].IncomeKobo != 40000 {
		t.Errorf("second row = %+v, want reiki at 40000", rows[1])
	}
	if rows[2].ServiceID != "svc-consult" || rows[2].IncomeKobo != 0 || rows[2].Sessions != 1 {
		t.Errorf("third row = %+v, want the unpaid session visible", rows[2])
	}
	if rows[0].Name != "Massage" {
		t.Errorf("name = %q, want it resolved", rows[0].Name)
	}
}

// TestIncomeByPeriodEmitsEmptyBuckets: a chart with the quiet weeks missing
// tells a rosier story than the truth.
func TestIncomeByPeriodEmitsEmptyBuckets(t *testing.T) {
	shortWindow := Range{From: day(3, 0), To: day(8, 0)} // Mon 3rd – Fri 7th
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-1", day(3, 9), booking.StatusCompleted),
			mkBooking("b2", "client-1", "svc-1", day(5, 9), booking.StatusCompleted),
		},
		Payments: []payment.Payment{
			mkPayment("b1", 25000, payment.StatusSuccess, day(3, 10)),
		},
	}

	series := IncomeByPeriod(in, shortWindow, Daily)
	if len(series) != 5 {
		t.Fatalf("buckets = %d, want one per day in the window", len(series))
	}
	if !series[0].Start.Equal(day(3, 0)) || series[0].Sessions != 1 || series[0].IncomeKobo != 25000 {
		t.Errorf("first bucket = %+v, want the 3rd with its session and income", series[0])
	}
	if series[1].Sessions != 0 || series[1].IncomeKobo != 0 {
		t.Errorf("second bucket = %+v, want an empty day reported as empty", series[1])
	}
	if series[2].Sessions != 1 {
		t.Errorf("third bucket = %+v, want the 5th's session", series[2])
	}
}

// TestWeeklyBucketsStartOnMonday.
func TestWeeklyBucketsStartOnMonday(t *testing.T) {
	// 3 Aug 2026 is a Monday; 9 Aug is the Sunday that closes that week.
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-1", day(5, 9), booking.StatusCompleted),
			mkBooking("b2", "client-1", "svc-1", day(9, 9), booking.StatusCompleted),
			mkBooking("b3", "client-1", "svc-1", day(10, 9), booking.StatusCompleted),
		},
	}

	series := IncomeByPeriod(in, Range{From: day(3, 0), To: day(17, 0)}, Weekly)
	if len(series) != 2 {
		t.Fatalf("buckets = %d, want 2 weeks", len(series))
	}
	if !series[0].Start.Equal(day(3, 0)) {
		t.Errorf("first week starts %v, want Monday the 3rd", series[0].Start)
	}
	if series[0].Sessions != 2 {
		t.Errorf("first week = %d sessions, want the Wednesday and the Sunday", series[0].Sessions)
	}
	if series[1].Sessions != 1 {
		t.Errorf("second week = %d sessions, want the following Monday", series[1].Sessions)
	}
}

// TestMonthlyBuckets.
func TestMonthlyBuckets(t *testing.T) {
	in := Inputs{
		Bookings: []booking.Booking{
			mkBooking("b1", "client-1", "svc-1", day(5, 9), booking.StatusCompleted),
			mkBooking("b2", "client-1", "svc-1", time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC), booking.StatusCompleted),
		},
	}
	series := IncomeByPeriod(in, Range{
		From: windowFrom,
		To:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}, Monthly)

	if len(series) != 2 {
		t.Fatalf("buckets = %d, want August and September", len(series))
	}
	if !series[0].Start.Equal(windowFrom) || series[0].Sessions != 1 {
		t.Errorf("august = %+v, want one session", series[0])
	}
	if series[1].Sessions != 1 {
		t.Errorf("september = %+v, want one session", series[1])
	}
}

// TestUnknownGranularityFallsBackToDaily.
func TestUnknownGranularityFallsBackToDaily(t *testing.T) {
	series := IncomeByPeriod(Inputs{}, Range{From: day(3, 0), To: day(5, 0)}, "fortnight")
	if len(series) != 2 {
		t.Errorf("buckets = %d, want daily buckets as the fallback", len(series))
	}
	if !Daily.Valid() || !Weekly.Valid() || !Monthly.Valid() || Granularity("fortnight").Valid() {
		t.Error("Granularity.Valid does not match the known set")
	}
}

// TestUpcomingLoadShowsTheShapeOfTheWeek.
func TestUpcomingLoadShowsTheShapeOfTheWeek(t *testing.T) {
	bookings := []booking.Booking{
		mkBooking("b1", "client-1", "svc-1", day(15, 14), booking.StatusConfirmed), // today, later
		mkBooking("b2", "client-2", "svc-1", day(15, 16), booking.StatusConfirmed),
		mkBooking("b3", "client-1", "svc-1", day(17, 9), booking.StatusConfirmed),
		mkBooking("b4", "client-1", "svc-1", day(17, 11), booking.StatusCancelled), // freed
		mkBooking("b5", "client-1", "svc-1", day(15, 9), booking.StatusConfirmed),  // already past
		mkBooking("b6", "client-1", "svc-1", day(25, 9), booking.StatusConfirmed),  // beyond the horizon
	}

	load := UpcomingLoad(bookings, fixedNow, 7)
	if len(load) != 7 {
		t.Fatalf("days = %d, want 7", len(load))
	}
	if !load[0].Date.Equal(day(15, 0)) || load[0].Sessions != 2 {
		t.Errorf("today = %+v, want the two sessions still ahead", load[0])
	}
	if load[1].Sessions != 0 {
		t.Errorf("tomorrow = %+v, want an empty day shown", load[1])
	}
	if load[2].Sessions != 1 {
		t.Errorf("day 3 = %+v, want one session (the cancellation excluded)", load[2])
	}
	if load[6].Sessions != 0 {
		t.Errorf("last day = %+v, want the far booking outside the horizon", load[6])
	}
}

// TestUpcomingLoadDefaultsTheHorizon.
func TestUpcomingLoadDefaultsTheHorizon(t *testing.T) {
	if got := len(UpcomingLoad(nil, fixedNow, 0)); got != 7 {
		t.Errorf("days = %d, want a 7-day default", got)
	}
}

// TestEmptyInputsProduceZeros: a new practice reads zeros, not nonsense.
func TestEmptyInputsProduceZeros(t *testing.T) {
	summary := Summarize(Inputs{}, window, fixedNow)
	if summary != (Summary{}) {
		t.Errorf("summary = %+v, want a zero summary", summary)
	}
	if rows := IncomeByService(Inputs{}, window); len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}
