package httpapi

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	reportsapp "github.com/xcreativs/terios/api/internal/app/reports"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// reportTestRig bundles a server with the reporting routes over in-memory
// repositories shared with the slices that own them.
type reportTestRig struct {
	srv                *Server
	bookings           *portstest.FakeBookingRepository
	payments           *portstest.FakePaymentRepository
	services           *portstest.FakeServiceRepository
	practitionerToken  string
	practitioner2Token string
	clientToken        string
}

func newReportTestRig(t *testing.T) reportTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	rig := reportTestRig{
		bookings: portstest.NewFakeBookingRepository(),
		payments: portstest.NewFakePaymentRepository(),
		services: portstest.NewFakeServiceRepository(),
	}
	svc := reportsapp.NewService(rig.bookings, rig.payments, rig.services, portstest.NewFakeReviewRepository())
	rig.srv = NewServer(WithAuth(authSvc), WithReports(svc, authSvc))

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	rig.practitionerToken = issue("prac-1", identity.RolePractitioner)
	rig.practitioner2Token = issue("prac-2", identity.RolePractitioner)
	rig.clientToken = issue("client-1", identity.RoleClient)
	return rig
}

// seedPracticeMonth fills one practitioner's August with a representative
// month and returns the service id.
func seedPracticeMonth(t *testing.T, rig reportTestRig, practitionerID string) string {
	t.Helper()
	created := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)
	svc, err := catalog.NewService(practitionerID, "Deep Tissue Massage", "", 60, 25000, "GHS", 1, created)
	if err != nil {
		t.Fatalf("catalog.NewService: %v", err)
	}
	svc, err = rig.services.Create(t.Context(), svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}

	seed := func(clientID string, day int, status domainbooking.Status, paid bool) {
		start := time.Date(2026, 8, day, 9, 0, 0, 0, time.UTC)
		b, err := domainbooking.New(clientID, practitionerID, svc.ID, start, 60, created)
		if err != nil {
			t.Fatalf("booking.New: %v", err)
		}
		b.Status = status
		b, err = rig.bookings.Create(t.Context(), b)
		if err != nil {
			t.Fatalf("seed booking: %v", err)
		}
		if !paid {
			return
		}
		p, err := payment.New(b.ID, clientID, 25000, "GHS", "ref-"+b.ID, start)
		if err != nil {
			t.Fatalf("payment.New: %v", err)
		}
		p.Status = payment.StatusSuccess
		paidAt := start
		p.PaidAt = &paidAt
		if _, err := rig.payments.Create(t.Context(), p); err != nil {
			t.Fatalf("seed payment: %v", err)
		}
	}

	seed("client-1", 3, domainbooking.StatusCompleted, true)
	seed("client-2", 4, domainbooking.StatusCompleted, true)
	seed("client-1", 5, domainbooking.StatusCancelled, false)
	seed("client-3", 6, domainbooking.StatusNoShow, false)
	return svc.ID
}

type practiceReportBody struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Granularity string `json:"granularity"`
	Summary     struct {
		SessionsCompleted int    `json:"sessionsCompleted"`
		Cancellations     int    `json:"cancellations"`
		NoShows           int    `json:"noShows"`
		NewClients        int    `json:"newClients"`
		IncomeKobo        int64  `json:"incomeKobo"`
		RefundedKobo      int64  `json:"refundedKobo"`
		NetKobo           int64  `json:"netKobo"`
		Currency          string `json:"currency"`
	} `json:"summary"`
	ByService []struct {
		ServiceID  string `json:"serviceId"`
		Name       string `json:"name"`
		Sessions   int    `json:"sessions"`
		IncomeKobo int64  `json:"incomeKobo"`
	} `json:"byService"`
	Series []struct {
		Start      string `json:"start"`
		Sessions   int    `json:"sessions"`
		IncomeKobo int64  `json:"incomeKobo"`
	} `json:"series"`
	Reviews struct {
		Count   int     `json:"count"`
		Average float64 `json:"average"`
	} `json:"reviews"`
}

func fetchReport(t *testing.T, rig reportTestRig, query, token string) practiceReportBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reports/practice"+query, nil, bearer(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("report status = %d, body %s", rec.Code, rec.Body.String())
	}
	var body practiceReportBody
	decodeBody(t, rec, &body)
	return body
}

// TestPracticeReportNumbers.
func TestPracticeReportNumbers(t *testing.T) {
	rig := newReportTestRig(t)
	serviceID := seedPracticeMonth(t, rig, "prac-1")

	report := fetchReport(t, rig, "?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z", rig.practitionerToken)

	if report.Summary.SessionsCompleted != 2 {
		t.Errorf("completed = %d, want 2", report.Summary.SessionsCompleted)
	}
	if report.Summary.Cancellations != 1 || report.Summary.NoShows != 1 {
		t.Errorf("summary = %+v, want cancellations and no-shows counted apart", report.Summary)
	}
	if report.Summary.IncomeKobo != 50000 || report.Summary.NetKobo != 50000 {
		t.Errorf("income = %d, net = %d, want 50000 each", report.Summary.IncomeKobo, report.Summary.NetKobo)
	}
	if report.Summary.Currency != "GHS" {
		t.Errorf("currency = %q, want GHS", report.Summary.Currency)
	}
	if report.Summary.NewClients != 3 {
		t.Errorf("newClients = %d, want the 3 clients whose first booking is in August", report.Summary.NewClients)
	}

	if len(report.ByService) != 1 {
		t.Fatalf("byService = %+v, want one row", report.ByService)
	}
	if report.ByService[0].ServiceID != serviceID || report.ByService[0].Name != "Deep Tissue Massage" {
		t.Errorf("byService[0] = %+v, want the named service", report.ByService[0])
	}
	if report.ByService[0].IncomeKobo != 50000 || report.ByService[0].Sessions != 2 {
		t.Errorf("byService[0] = %+v, want 50000 from 2 sessions", report.ByService[0])
	}

	if len(report.Series) != 31 {
		t.Errorf("series = %d buckets, want one per day in August", len(report.Series))
	}
}

// TestReportIsScopedToTheCallersPractice: there is no cross-practice view.
func TestReportIsScopedToTheCallersPractice(t *testing.T) {
	rig := newReportTestRig(t)
	seedPracticeMonth(t, rig, "prac-1")

	other := fetchReport(t, rig, "?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z", rig.practitioner2Token)
	if other.Summary.SessionsCompleted != 0 || other.Summary.IncomeKobo != 0 {
		t.Errorf("another practice's report = %+v, want zeros", other.Summary)
	}
	if len(other.ByService) != 0 {
		t.Errorf("byService = %+v, want it empty for another practice", other.ByService)
	}
}

// TestGranularityChangesTheBuckets.
func TestGranularityChangesTheBuckets(t *testing.T) {
	rig := newReportTestRig(t)
	seedPracticeMonth(t, rig, "prac-1")

	// August 2026 opens on a Saturday, so the first Monday-anchored week
	// starts on 27 July: six weekly buckets cover the month, the first of
	// them partial. That is what a weekly chart should show — clamping the
	// label to the 1st would mislabel a two-day bucket as a full week.
	for granularity, want := range map[string]int{"week": 6, "month": 1} {
		report := fetchReport(t, rig,
			"?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z&granularity="+granularity,
			rig.practitionerToken)
		if report.Granularity != granularity {
			t.Errorf("granularity = %q, want %q", report.Granularity, granularity)
		}
		if len(report.Series) != want {
			t.Errorf("%s series = %d buckets, want %d", granularity, len(report.Series), want)
		}
	}
}

// TestDefaultWindowIsTheCurrentMonth.
func TestDefaultWindowIsTheCurrentMonth(t *testing.T) {
	rig := newReportTestRig(t)
	report := fetchReport(t, rig, "", rig.practitionerToken)

	now := time.Now().UTC()
	wantFrom := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	from, err := time.Parse(time.RFC3339, report.From)
	if err != nil {
		t.Fatalf("parse from: %v", err)
	}
	if !from.Equal(wantFrom) {
		t.Errorf("from = %v, want the start of the current month %v", from, wantFrom)
	}
}

// TestReportRequestValidation.
func TestReportRequestValidation(t *testing.T) {
	rig := newReportTestRig(t)

	for name, query := range map[string]string{
		"inverted range":      "?from=2026-09-01T00:00:00Z&to=2026-08-01T00:00:00Z",
		"equal range":         "?from=2026-08-01T00:00:00Z&to=2026-08-01T00:00:00Z",
		"unparseable from":    "?from=yesterday&to=2026-09-01T00:00:00Z",
		"unknown granularity": "?from=2026-08-01T00:00:00Z&to=2026-09-01T00:00:00Z&granularity=fortnight",
		"range too long":      "?from=2020-01-01T00:00:00Z&to=2026-09-01T00:00:00Z",
		"only from":           "?from=2026-08-01T00:00:00Z",
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reports/practice"+query, nil, bearer(rig.practitionerToken))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			var errRes errorBody
			decodeBody(t, rec, &errRes)
			if errRes.Error.Code != "validation_error" {
				t.Errorf("code = %q, want validation_error", errRes.Error.Code)
			}
		})
	}
}

// TestUpcomingLoad.
func TestUpcomingLoad(t *testing.T) {
	rig := newReportTestRig(t)
	created := time.Now().UTC()

	svc, err := catalog.NewService("prac-1", "Massage", "", 60, 25000, "GHS", 1, created)
	if err != nil {
		t.Fatalf("catalog.NewService: %v", err)
	}
	svc, err = rig.services.Create(t.Context(), svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	for i, offset := range []time.Duration{2 * time.Hour, 26 * time.Hour, 27 * time.Hour} {
		b, err := domainbooking.New(fmt.Sprintf("client-%d", i), "prac-1", svc.ID, created.Add(offset), 60, created)
		if err != nil {
			t.Fatalf("booking.New: %v", err)
		}
		if _, err := rig.bookings.Create(t.Context(), b); err != nil {
			t.Fatalf("seed booking: %v", err)
		}
	}

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reports/upcoming-load?days=3", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Items []struct {
			Date     string `json:"date"`
			Sessions int    `json:"sessions"`
		} `json:"items"`
	}
	decodeBody(t, rec, &res)

	if len(res.Items) != 3 {
		t.Fatalf("days = %d, want 3", len(res.Items))
	}
	total := 0
	for _, day := range res.Items {
		total += day.Sessions
		if len(day.Date) != len("2006-01-02") {
			t.Errorf("date = %q, want a calendar date", day.Date)
		}
	}
	if total != 3 {
		t.Errorf("sessions across the horizon = %d, want 3", total)
	}

	for _, days := range []string{"0", "-1", "91", "many"} {
		rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reports/upcoming-load?days="+days, nil, bearer(rig.practitionerToken))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("days=%s status = %d, want 400", days, rec.Code)
		}
	}
}

// TestReportRoleGuards.
func TestReportRoleGuards(t *testing.T) {
	rig := newReportTestRig(t)
	for _, path := range []string{"/v1/admin/reports/practice", "/v1/admin/reports/upcoming-load"} {
		rec := doJSON(t, rig.srv, http.MethodGet, path, nil, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("GET %s as client = %d, want 403", path, rec.Code)
		}
		rec = doJSON(t, rig.srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s anonymous = %d, want 401", path, rec.Code)
		}
	}
}

// TestReportsUnavailableWithoutDatabase.
func TestReportsUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithReports(nil, nil))
	for _, path := range []string{"/v1/admin/reports/practice", "/v1/admin/reports/upcoming-load"} {
		rec := doJSON(t, srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("GET %s = %d, want 503 (body %s)", path, rec.Code, rec.Body.String())
		}
	}
}
