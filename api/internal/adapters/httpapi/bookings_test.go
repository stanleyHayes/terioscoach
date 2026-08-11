package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	bookingapp "github.com/xcreativs/terios/api/internal/app/booking"
	"github.com/xcreativs/terios/api/internal/app/catalog"
	schedulingapp "github.com/xcreativs/terios/api/internal/app/scheduling"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// bookingTestRig bundles a fully wired server over in-memory fakes plus
// practitioner and two client tokens. The booking repository doubles as the
// busy-interval reader, mirroring the production wiring.
type bookingTestRig struct {
	srv               *Server
	bookings          *portstest.FakeBookingRepository
	practitionerToken string
	clientToken       string
	otherClientToken  string
}

func newBookingTestRig(t *testing.T) bookingTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)

	services := portstest.NewFakeServiceRepository()
	avail := portstest.NewFakeAvailabilityRepository()
	bookings := portstest.NewFakeBookingRepository()

	srv := NewServer(
		WithAuth(authSvc),
		WithCatalog(catalog.NewService(services), authSvc),
		WithScheduling(schedulingapp.NewService(services, avail, bookings), authSvc),
		WithBooking(bookingapp.NewService(bookings, services, avail, bookings, booking.DefaultPolicy()), authSvc),
	)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return bookingTestRig{
		srv:               srv,
		bookings:          bookings,
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
		otherClientToken:  issue("client-2", identity.RoleClient),
	}
}

// bookingTestBody is the contract booking shape for assertions.
type bookingTestBody struct {
	ID             string     `json:"id"`
	ClientID       string     `json:"clientId"`
	PractitionerID string     `json:"practitionerId"`
	ServiceID      string     `json:"serviceId"`
	StartAt        time.Time  `json:"startAt"`
	EndAt          time.Time  `json:"endAt"`
	Status         string     `json:"status"`
	CancelledAt    *time.Time `json:"cancelledAt"`
	CompletedAt    *time.Time `json:"completedAt"`
}

// seedBookableSlot creates a 60-minute service and opens 09:00-12:00 on the
// weekday seven days out (slots 09:00, 10:00, 11:00 UTC). Returns
// (serviceID, day).
func seedBookableSlot(t *testing.T, rig bookingTestRig) (string, time.Time) {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/services", map[string]any{
		"name": "Massage", "durationMinutes": 60, "priceKobo": 25000,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create service status = %d, body %s", rec.Code, rec.Body.String())
	}
	var svcRes struct {
		Service serviceTestBody `json:"service"`
	}
	decodeBody(t, rec, &svcRes)

	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	rec = doJSON(t, rig.srv, http.MethodPut, "/v1/availability/rules", map[string]any{
		"rules": []map[string]any{{
			"weekday":       int(day.Weekday()),
			"windows":       []map[string]int{{"startMin": 540, "endMin": 720}},
			"bufferMinutes": 0,
		}},
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("put rules status = %d, body %s", rec.Code, rec.Body.String())
	}
	return svcRes.Service.ID, day
}

func bookViaHTTP(t *testing.T, rig bookingTestRig, token, serviceID string, startAt time.Time, wantStatus int) bookingTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
		"serviceId": serviceID,
		"startAt":   startAt.Format(time.RFC3339),
		"tz":        "UTC",
	}, bearer(token))
	if rec.Code != wantStatus {
		t.Fatalf("book status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body.String())
	}
	var res struct {
		Booking bookingTestBody `json:"booking"`
	}
	if wantStatus < 300 {
		decodeBody(t, rec, &res)
	}
	return res.Booking
}

func TestBookingCreateShape(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	start := day.Add(9 * time.Hour)

	b := bookViaHTTP(t, rig, rig.clientToken, serviceID, start, http.StatusCreated)
	if b.ID == "" || b.Status != "confirmed" {
		t.Errorf("booking = %+v, want id assigned and status confirmed", b)
	}
	if b.ClientID != "client-1" || b.PractitionerID != "prac-1" || b.ServiceID != serviceID {
		t.Errorf("booking = %+v, want parties stamped", b)
	}
	if !b.StartAt.Equal(start) || !b.EndAt.Equal(start.Add(time.Hour)) {
		t.Errorf("span = [%v, %v), want [09:00, 10:00) UTC", b.StartAt, b.EndAt)
	}
}

func TestBookingCreateSlotValidation(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)

	cases := []struct {
		name     string
		body     map[string]any
		wantCode int
		wantErr  string
	}{
		{"misaligned", map[string]any{"serviceId": serviceID, "startAt": day.Add(9*time.Hour + 45*time.Minute), "tz": "UTC"}, 409, "slot_unavailable"},
		{"closed day", map[string]any{"serviceId": serviceID, "startAt": day.Add(24*time.Hour + 9*time.Hour), "tz": "UTC"}, 409, "slot_unavailable"},
		{"bad tz", map[string]any{"serviceId": serviceID, "startAt": day.Add(9 * time.Hour), "tz": "Mars/Olympus"}, 400, "invalid_timezone"},
		{"unknown service", map[string]any{"serviceId": "unknown", "startAt": day.Add(9 * time.Hour), "tz": "UTC"}, 404, "service_not_found"},
		{"missing startAt", map[string]any{"serviceId": serviceID}, 400, "validation_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", tc.body, bearer(rig.clientToken))
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			var errRes errorTestResponse
			decodeBody(t, rec, &errRes)
			if errRes.Error.Code != tc.wantErr {
				t.Errorf("error code = %q, want %q", errRes.Error.Code, tc.wantErr)
			}
		})
	}
}

func TestBookingDoubleBookConflict(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	start := day.Add(9 * time.Hour)

	bookViaHTTP(t, rig, rig.clientToken, serviceID, start, http.StatusCreated)
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
		"serviceId": serviceID, "startAt": start, "tz": "UTC",
	}, bearer(rig.otherClientToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("double-book status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "slot_unavailable" {
		t.Errorf("error code = %q, want slot_unavailable", errRes.Error.Code)
	}

	// The booked slot also disappears from the public slots route.
	date := day.Format("2006-01-02")
	rec = doJSON(t, rig.srv, http.MethodGet,
		"/v1/availability/slots?serviceId="+serviceID+"&from="+date+"&to="+date+"&tz=UTC", nil, nil)
	var slotsRes struct {
		Slots []struct {
			StartAt time.Time `json:"startAt"`
		} `json:"slots"`
	}
	decodeBody(t, rec, &slotsRes)
	if len(slotsRes.Slots) != 2 {
		t.Errorf("slots after booking = %+v, want the 2 remaining", slotsRes.Slots)
	}
	for _, s := range slotsRes.Slots {
		if s.StartAt.Equal(start) {
			t.Errorf("booked slot %v still offered", start)
		}
	}
}

func TestBookingListsAndIsolation(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	mine := bookViaHTTP(t, rig, rig.clientToken, serviceID, day.Add(9*time.Hour), http.StatusCreated)
	bookViaHTTP(t, rig, rig.otherClientToken, serviceID, day.Add(10*time.Hour), http.StatusCreated)

	// Client sees exactly their own bookings.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/bookings/mine", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("mine status = %d", rec.Code)
	}
	var list struct {
		Items []bookingTestBody `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 || list.Items[0].ID != mine.ID {
		t.Errorf("mine = %+v, want only the client's own booking", list.Items)
	}

	// Practitioner calendar: all bookings, plus filters.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings", nil, bearer(rig.practitionerToken))
	decodeBody(t, rec, &list)
	if len(list.Items) != 2 {
		t.Fatalf("practitioner list = %+v, want both bookings", list.Items)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings?status=cancelled", nil, bearer(rig.practitionerToken))
	decodeBody(t, rec, &list)
	if len(list.Items) != 0 {
		t.Errorf("cancelled filter = %+v, want none yet", list.Items)
	}
	rec = doJSON(t, rig.srv, http.MethodGet,
		"/v1/bookings?from="+day.Add(10*time.Hour).Format(time.RFC3339), nil, bearer(rig.practitionerToken))
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 || !list.Items[0].StartAt.Equal(day.Add(10*time.Hour)) {
		t.Errorf("from filter = %+v, want only the 10:00 booking", list.Items)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings?status=bogus", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bogus status filter status = %d, want 400", rec.Code)
	}

	// Get by id: owner and practitioner 200, other client 404 (isolation).
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings/"+mine.ID, nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Errorf("owner get status = %d, want 200", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings/"+mine.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Errorf("practitioner get status = %d, want 200", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/bookings/"+mine.ID, nil, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other client get status = %d, want 404", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "booking_not_found" {
		t.Errorf("error code = %q, want booking_not_found", errRes.Error.Code)
	}
}

func TestBookingRescheduleAndCancelFlow(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	b := bookViaHTTP(t, rig, rig.clientToken, serviceID, day.Add(9*time.Hour), http.StatusCreated)

	// Other client cannot reschedule or cancel: 404, not 403 — isolation.
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/reschedule",
		map[string]any{"startAt": day.Add(10 * time.Hour), "tz": "UTC"}, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other client reschedule status = %d, want 404", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/cancel", nil, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other client cancel status = %d, want 404", rec.Code)
	}

	// Owner reschedules to a valid slot; misaligned target is 409.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/reschedule",
		map[string]any{"startAt": day.Add(10*time.Hour + 15*time.Minute), "tz": "UTC"}, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("misaligned reschedule status = %d, want 409", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/reschedule",
		map[string]any{"startAt": day.Add(10 * time.Hour), "tz": "UTC"}, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("reschedule status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Booking bookingTestBody `json:"booking"`
	}
	decodeBody(t, rec, &res)
	if !res.Booking.StartAt.Equal(day.Add(10*time.Hour)) || res.Booking.Status != "confirmed" {
		t.Errorf("rescheduled = %+v, want 10:00 still confirmed", res.Booking)
	}

	// Old slot freed: another client can book it.
	bookViaHTTP(t, rig, rig.otherClientToken, serviceID, day.Add(9*time.Hour), http.StatusCreated)

	// Practitioner cancels the client's booking anytime (no cutoff).
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/cancel", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("practitioner cancel status = %d, body %s", rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &res)
	if res.Booking.Status != "cancelled" || res.Booking.CancelledAt == nil {
		t.Errorf("cancelled = %+v, want status cancelled with cancelledAt", res.Booking)
	}

	// Terminal: a second cancel is 409 invalid_status.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/cancel", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("double cancel status = %d, want 409", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "invalid_status" {
		t.Errorf("error code = %q, want invalid_status", errRes.Error.Code)
	}

	// Cancelled slot is bookable again.
	bookViaHTTP(t, rig, rig.otherClientToken, serviceID, day.Add(10*time.Hour), http.StatusCreated)
}

func TestBookingCompleteTooEarlyAndRoleGuard(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	b := bookViaHTTP(t, rig, rig.clientToken, serviceID, day.Add(9*time.Hour), http.StatusCreated)

	// Client role cannot reach practitioner-only transitions.
	for _, path := range []string{"/complete", "/no-show"} {
		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+path, nil, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("client %s status = %d, want 403", path, rec.Code)
		}
	}

	// The appointment is a week out: completing now is too early.
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings/"+b.ID+"/complete", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("early complete status = %d, want 409", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "invalid_status" {
		t.Errorf("error code = %q, want invalid_status", errRes.Error.Code)
	}
}

func TestBookingRoleAndAuthMatrix(t *testing.T) {
	rig := newBookingTestRig(t)
	serviceID, day := seedBookableSlot(t, rig)
	b := bookViaHTTP(t, rig, rig.clientToken, serviceID, day.Add(9*time.Hour), http.StatusCreated)

	routes := []struct {
		method, path string
		body         any
		role         string // "client", "practitioner", or "both"
	}{
		{http.MethodPost, "/v1/bookings", map[string]any{"serviceId": serviceID, "startAt": day.Add(11 * time.Hour)}, "client"},
		{http.MethodGet, "/v1/bookings/mine", nil, "client"},
		{http.MethodGet, "/v1/bookings", nil, "practitioner"},
		{http.MethodGet, "/v1/bookings/" + b.ID, nil, "both"},
		{http.MethodPost, "/v1/bookings/" + b.ID + "/reschedule", map[string]any{"startAt": day.Add(11 * time.Hour)}, "both"},
		{http.MethodPost, "/v1/bookings/" + b.ID + "/cancel", nil, "both"},
		{http.MethodPost, "/v1/bookings/" + b.ID + "/complete", nil, "practitioner"},
		{http.MethodPost, "/v1/bookings/" + b.ID + "/no-show", nil, "practitioner"},
	}
	for _, route := range routes {
		// Unauthenticated -> 401 everywhere.
		rec := doJSON(t, rig.srv, route.method, route.path, route.body, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s unauthenticated status = %d, want 401", route.method, route.path, rec.Code)
		}
		// Wrong role -> 403 on role-guarded routes.
		switch route.role {
		case "client":
			rec = doJSON(t, rig.srv, route.method, route.path, route.body, bearer(rig.practitionerToken))
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as practitioner status = %d, want 403", route.method, route.path, rec.Code)
			}
		case "practitioner":
			rec = doJSON(t, rig.srv, route.method, route.path, route.body, bearer(rig.clientToken))
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as client status = %d, want 403", route.method, route.path, rec.Code)
			}
		}
	}
}

func TestBookingUnavailableWithoutService(t *testing.T) {
	srv := NewServer(WithBooking(nil, nil))
	for _, path := range []string{
		"/v1/bookings",
		"/v1/bookings/mine",
		"/v1/bookings/some-id",
		"/v1/bookings/some-id/cancel",
	} {
		rec := doJSON(t, srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s status = %d, want 503", path, rec.Code)
		}
		var errRes errorTestResponse
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "service_unavailable" {
			t.Errorf("%s error code = %q, want service_unavailable", path, errRes.Error.Code)
		}
	}
}
