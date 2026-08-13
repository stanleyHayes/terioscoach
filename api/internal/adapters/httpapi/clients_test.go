package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	clientsapp "github.com/xcreativs/terios/api/internal/app/clients"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

const clientsPracID = "prac-1"

// clientTestRig bundles a server with the clients slice mounted over
// in-memory fakes. Client accounts are registered through the real auth
// service (so the user repository the record assembler reads is populated);
// bookings and payments are seeded straight into their repositories because
// the booking and payment routes are not part of this slice's surface.
type clientTestRig struct {
	srv               *Server
	bookings          *portstest.FakeBookingRepository
	payments          *portstest.FakePaymentRepository
	documents         *portstest.FakeDocumentCounter
	forms             *portstest.FakeFormSubmissionCounter
	practitionerToken string
}

func newClientTestRig(t *testing.T) clientTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	users := portstest.NewFakeUserRepository()
	authSvc := auth.NewService(
		users,
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)

	bookings := portstest.NewFakeBookingRepository()
	payments := portstest.NewFakePaymentRepository()
	documents := &portstest.FakeDocumentCounter{Counts: map[string]int{}}
	forms := &portstest.FakeFormSubmissionCounter{Counts: map[string]int{}}

	svc := clientsapp.NewService(
		portstest.NewFakeClientProfileRepository(),
		users,
		bookings,
		payments,
		documents,
		forms,
	)

	token, _, err := issuer.IssueAccessToken(identity.Identity{
		UserID: clientsPracID,
		Role:   identity.RolePractitioner,
	})
	if err != nil {
		t.Fatalf("issue practitioner token: %v", err)
	}

	return clientTestRig{
		srv:               NewServer(WithAuth(authSvc), WithClients(svc, authSvc)),
		bookings:          bookings,
		payments:          payments,
		documents:         documents,
		forms:             forms,
		practitionerToken: token,
	}
}

// registerClient creates a client account through the auth routes and
// returns its user id and access token.
func registerClient(t *testing.T, rig clientTestRig, email, name string) (string, string) {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/auth/register", map[string]any{
		"email":    email,
		"password": "correct-horse-battery",
		"name":     name,
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s status = %d, body %s", email, rec.Code, rec.Body.String())
	}
	var res authTestResponse
	decodeBody(t, rec, &res)
	return res.User.ID, res.AccessToken
}

// seedClientBooking inserts a booking for the practice directly, returning
// the stored booking (the repository assigns the id).
func seedClientBooking(t *testing.T, rig clientTestRig, clientID string, startAt time.Time, status booking.Status) booking.Booking {
	t.Helper()
	b, err := booking.New(clientID, clientsPracID, "svc-1", startAt, 60, startAt.Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b.Status = status
	b, err = rig.bookings.Create(t.Context(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b
}

func seedClientPayment(t *testing.T, rig clientTestRig, b booking.Booking, amountKobo int64, status payment.Status) {
	t.Helper()
	p, err := payment.New(b.ID, b.ClientID, amountKobo, "GHS", "ref-"+b.ID, b.CreatedAt)
	if err != nil {
		t.Fatalf("payment.New: %v", err)
	}
	p.Status = status
	if _, err := rig.payments.Create(t.Context(), p); err != nil {
		t.Fatalf("seed payment: %v", err)
	}
}

type clientSummaryTestBody struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Phone         string     `json:"phone"`
	Tags          []string   `json:"tags"`
	TotalSessions int        `json:"totalSessions"`
	LastSessionAt *time.Time `json:"lastSessionAt"`
}

type clientRecordTestBody struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	Phone          string   `json:"phone"`
	Tags           []string `json:"tags"`
	PracticeNotes  string   `json:"practiceNotes"`
	RecentBookings []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"recentBookings"`
	Payments struct {
		TotalPaidKobo     int64  `json:"totalPaidKobo"`
		TotalRefundedKobo int64  `json:"totalRefundedKobo"`
		PaymentCount      int    `json:"paymentCount"`
		Currency          string `json:"currency"`
	} `json:"payments"`
	DocumentCount       int `json:"documentCount"`
	FormSubmissionCount int `json:"formSubmissionCount"`
}

// TestListClientsRollup: membership comes from bookings, cancelled bookings
// never count as sessions, and the order is last-session-first.
func TestListClientsRollup(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, _ := registerClient(t, rig, "ama@example.com", "Ama Serwaa")
	koffiID, _ := registerClient(t, rig, "koffi@example.com", "Koffi Mensah")
	registerClient(t, rig, "never@example.com", "Never Booked")

	base := time.Now().UTC().Truncate(time.Hour)
	seedClientBooking(t, rig, koffiID, base.Add(-96*time.Hour), booking.StatusCompleted)
	seedClientBooking(t, rig, amaID, base.Add(-48*time.Hour), booking.StatusCompleted)
	seedClientBooking(t, rig, amaID, base.Add(24*time.Hour), booking.StatusConfirmed)
	seedClientBooking(t, rig, amaID, base.Add(48*time.Hour), booking.StatusCancelled)

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/clients", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Items []clientSummaryTestBody `json:"items"`
	}
	decodeBody(t, rec, &res)

	if len(res.Items) != 2 {
		t.Fatalf("items = %d (%+v), want the 2 clients with bookings", len(res.Items), res.Items)
	}
	if res.Items[0].ID != amaID {
		t.Errorf("first row = %s, want %s (most recent session first)", res.Items[0].ID, amaID)
	}
	if res.Items[0].TotalSessions != 2 {
		t.Errorf("totalSessions = %d, want 2 (the cancelled booking is not a session)", res.Items[0].TotalSessions)
	}
	if res.Items[0].LastSessionAt == nil || !res.Items[0].LastSessionAt.Equal(base.Add(24*time.Hour)) {
		t.Errorf("lastSessionAt = %v, want %v", res.Items[0].LastSessionAt, base.Add(24*time.Hour))
	}
	if res.Items[0].Tags == nil {
		t.Error("tags = null, want an empty array before any PATCH")
	}
	if res.Items[1].ID != koffiID {
		t.Errorf("second row = %s, want %s", res.Items[1].ID, koffiID)
	}
}

// TestClientRecordAssembly: the full record joins bookings, the payment
// rollup, and the document/form counters.
func TestClientRecordAssembly(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, _ := registerClient(t, rig, "ama@example.com", "Ama Serwaa")

	base := time.Now().UTC().Truncate(time.Hour)
	paid := seedClientBooking(t, rig, amaID, base.Add(-48*time.Hour), booking.StatusCompleted)
	refunded := seedClientBooking(t, rig, amaID, base.Add(-24*time.Hour), booking.StatusCancelled)
	seedClientPayment(t, rig, paid, 25000, payment.StatusSuccess)
	seedClientPayment(t, rig, refunded, 10000, payment.StatusRefunded)
	rig.documents.Counts[amaID] = 3
	rig.forms.Counts[amaID] = 1

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/clients/"+amaID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("record status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Record clientRecordTestBody `json:"record"`
	}
	decodeBody(t, rec, &res)

	if res.Record.ID != amaID || res.Record.Name != "Ama Serwaa" || res.Record.Email != "ama@example.com" {
		t.Errorf("record identity = %+v, want the registered account", res.Record)
	}
	if len(res.Record.RecentBookings) != 2 {
		t.Fatalf("recentBookings = %d, want 2 (all statuses)", len(res.Record.RecentBookings))
	}
	if res.Record.RecentBookings[0].ID != refunded.ID {
		t.Errorf("recentBookings[0] = %s, want %s (startAt descending)", res.Record.RecentBookings[0].ID, refunded.ID)
	}
	if res.Record.Payments.TotalPaidKobo != 25000 || res.Record.Payments.TotalRefundedKobo != 10000 {
		t.Errorf("payments = %+v, want 25000 paid / 10000 refunded", res.Record.Payments)
	}
	if res.Record.Payments.PaymentCount != 2 || res.Record.Payments.Currency != "GHS" {
		t.Errorf("payments = %+v, want 2 records in GHS", res.Record.Payments)
	}
	if res.Record.DocumentCount != 3 || res.Record.FormSubmissionCount != 1 {
		t.Errorf("counts = %d docs / %d forms, want 3 / 1", res.Record.DocumentCount, res.Record.FormSubmissionCount)
	}
	if res.Record.PracticeNotes != "" || len(res.Record.Tags) != 0 {
		t.Errorf("practice fields = %q / %v, want zero values before the first PATCH", res.Record.PracticeNotes, res.Record.Tags)
	}
}

// TestPatchPracticeFields: the first PATCH creates the profile, omitted
// fields are preserved, and the values surface on the record and list.
func TestPatchPracticeFields(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, _ := registerClient(t, rig, "ama@example.com", "Ama Serwaa")
	seedClientBooking(t, rig, amaID, time.Now().UTC().Add(24*time.Hour), booking.StatusConfirmed)

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/clients/"+amaID, map[string]any{
		"phone":         "+233201234567",
		"practiceNotes": "Prefers late afternoon.",
		"tags":          []string{"regular", "regular", " deep tissue "},
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Profile struct {
			ID            string   `json:"id"`
			Phone         string   `json:"phone"`
			Tags          []string `json:"tags"`
			PracticeNotes string   `json:"practiceNotes"`
		} `json:"profile"`
	}
	decodeBody(t, rec, &res)
	if res.Profile.ID != amaID || res.Profile.Phone != "+233201234567" {
		t.Errorf("profile = %+v, want the patched phone on the client's id", res.Profile)
	}
	if len(res.Profile.Tags) != 2 || res.Profile.Tags[0] != "regular" || res.Profile.Tags[1] != "deep tissue" {
		t.Errorf("tags = %v, want trimmed and de-duplicated", res.Profile.Tags)
	}

	// A second PATCH touching one field leaves the others alone.
	rec = doJSON(t, rig.srv, http.MethodPatch, "/v1/clients/"+amaID, map[string]any{
		"practiceNotes": "Updated summary.",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("second patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &res)
	if res.Profile.PracticeNotes != "Updated summary." || res.Profile.Phone != "+233201234567" {
		t.Errorf("profile = %+v, want the omitted phone preserved", res.Profile)
	}
	if len(res.Profile.Tags) != 2 {
		t.Errorf("tags = %v, want the omitted tags preserved", res.Profile.Tags)
	}
}

// TestPatchValidation: practice-field limits answer 400 validation_error.
func TestPatchValidation(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, _ := registerClient(t, rig, "ama@example.com", "Ama Serwaa")
	seedClientBooking(t, rig, amaID, time.Now().UTC().Add(24*time.Hour), booking.StatusConfirmed)

	tags := make([]string, 21)
	for i := range tags {
		tags[i] = string(rune('a' + i))
	}
	for name, body := range map[string]map[string]any{
		"phone too long": {"phone": strings.Repeat("9", 41)},
		"too many tags":  {"tags": tags},
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/clients/"+amaID, body, bearer(rig.practitionerToken))
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

// TestClientMeExcludesPracticeFields: the client's own view never carries
// the practitioner's private practice fields, only the shared phone.
func TestClientMeExcludesPracticeFields(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, amaToken := registerClient(t, rig, "ama@example.com", "Ama Serwaa")
	seedClientBooking(t, rig, amaID, time.Now().UTC().Add(24*time.Hour), booking.StatusConfirmed)

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/clients/"+amaID, map[string]any{
		"phone":         "+233201234567",
		"practiceNotes": "Private practitioner summary.",
		"tags":          []string{"regular"},
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/clients/me", nil, bearer(amaToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body %s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Profile map[string]any `json:"profile"`
	}
	decodeBody(t, rec, &raw)

	if raw.Profile["id"] != amaID || raw.Profile["name"] != "Ama Serwaa" {
		t.Errorf("profile = %+v, want the caller's own account", raw.Profile)
	}
	if raw.Profile["phone"] != "+233201234567" {
		t.Errorf("phone = %v, want the recorded phone", raw.Profile["phone"])
	}
	if _, ok := raw.Profile["createdAt"]; !ok {
		t.Error("createdAt missing from the client's own profile")
	}
	for _, forbidden := range []string{"practiceNotes", "tags"} {
		if _, present := raw.Profile[forbidden]; present {
			t.Errorf("%q leaked into the client's own view: %+v", forbidden, raw.Profile)
		}
	}
}

// TestClientsIsolationAndRoles: non-members are not found, and every route
// is guarded by the role the contract assigns it.
func TestClientsIsolationAndRoles(t *testing.T) {
	rig := newClientTestRig(t)
	amaID, amaToken := registerClient(t, rig, "ama@example.com", "Ama Serwaa")
	strangerID, _ := registerClient(t, rig, "stranger@example.com", "No Bookings")
	seedClientBooking(t, rig, amaID, time.Now().UTC().Add(24*time.Hour), booking.StatusConfirmed)

	// A real account with no booking at this practice is not a client here.
	for _, path := range []string{"/v1/clients/" + strangerID, "/v1/clients/does-not-exist"} {
		rec := doJSON(t, rig.srv, http.MethodGet, path, nil, bearer(rig.practitionerToken))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want 404 (body %s)", path, rec.Code, rec.Body.String())
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "client_not_found" {
			t.Errorf("GET %s code = %q, want client_not_found", path, errRes.Error.Code)
		}
	}
	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/clients/"+strangerID, map[string]any{"phone": "+233"}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("patch non-member status = %d, want 404", rec.Code)
	}

	// Role guards: clients cannot read the practice list or any record.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/clients"},
		{http.MethodGet, "/v1/clients/" + amaID},
		{http.MethodPatch, "/v1/clients/" + amaID},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(amaToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as client = %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/clients/me", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusForbidden {
		t.Errorf("GET /v1/clients/me as practitioner = %d, want 403", rec.Code)
	}

	// No token at all.
	for _, path := range []string{"/v1/clients", "/v1/clients/me", "/v1/clients/" + amaID} {
		rec := doJSON(t, rig.srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s unauthenticated = %d, want 401", path, rec.Code)
		}
	}
}

// TestClientsUnavailableWithoutDatabase: a nil service keeps the routes
// mounted and answers 503 on all of them.
func TestClientsUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithClients(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/clients"},
		{http.MethodGet, "/v1/clients/me"},
		{http.MethodGet, "/v1/clients/abc"},
		{http.MethodPatch, "/v1/clients/abc"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "service_unavailable" {
			t.Errorf("%s %s code = %q, want service_unavailable", tc.method, tc.path, errRes.Error.Code)
		}
	}
}
