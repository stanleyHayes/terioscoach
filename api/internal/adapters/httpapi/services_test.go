package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	"github.com/xcreativs/terios/api/internal/app/catalog"
	schedulingapp "github.com/xcreativs/terios/api/internal/app/scheduling"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// catalogTestRig bundles a fully wired server over in-memory fakes plus
// ready-made client and practitioner bearer tokens.
type catalogTestRig struct {
	srv               *Server
	services          *portstest.FakeServiceRepository
	avail             *portstest.FakeAvailabilityRepository
	busy              *portstest.FakeBusyIntervalReader
	practitionerToken string
	clientToken       string
}

const testPractitionerID = "prac-1"

func newCatalogTestRig(t *testing.T) catalogTestRig {
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
	busy := &portstest.FakeBusyIntervalReader{}
	catalogSvc := catalog.NewService(services)
	schedulingSvc := schedulingapp.NewService(services, avail, busy)

	srv := NewServer(
		WithAuth(authSvc),
		WithCatalog(catalogSvc, authSvc),
		WithScheduling(schedulingSvc, authSvc),
	)

	practitionerToken, _, err := issuer.IssueAccessToken(identity.Identity{
		UserID: testPractitionerID,
		Role:   identity.RolePractitioner,
	})
	if err != nil {
		t.Fatalf("issue practitioner token: %v", err)
	}
	clientToken, _, err := issuer.IssueAccessToken(identity.Identity{
		UserID: "client-1",
		Role:   identity.RoleClient,
	})
	if err != nil {
		t.Fatalf("issue client token: %v", err)
	}

	return catalogTestRig{
		srv:               srv,
		services:          services,
		avail:             avail,
		busy:              busy,
		practitionerToken: practitionerToken,
		clientToken:       clientToken,
	}
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// serviceTestBody is the contract service shape for assertions.
type serviceTestBody struct {
	ID              string `json:"id"`
	PractitionerID  string `json:"practitionerId"`
	Name            string `json:"name"`
	DurationMinutes int    `json:"durationMinutes"`
	PriceKobo       int64  `json:"priceKobo"`
	Currency        string `json:"currency"`
	Active          bool   `json:"active"`
	SortOrder       int    `json:"sortOrder"`
}

type serviceListTestBody struct {
	Items []serviceTestBody `json:"items"`
}

func createServiceViaHTTP(t *testing.T, rig catalogTestRig, name string, sortOrder int) serviceTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/services", map[string]any{
		"name":            name,
		"description":     "desc",
		"durationMinutes": 60,
		"priceKobo":       25000,
		"sortOrder":       sortOrder,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %q status = %d, body %s", name, rec.Code, rec.Body.String())
	}
	var res struct {
		Service serviceTestBody `json:"service"`
	}
	decodeBody(t, rec, &res)
	return res.Service
}

func TestPublicServicesListHidesInactive(t *testing.T) {
	rig := newCatalogTestRig(t)
	second := createServiceViaHTTP(t, rig, "Second", 2)
	first := createServiceViaHTTP(t, rig, "First", 1)
	inactive := createServiceViaHTTP(t, rig, "Inactive", 3)

	// Deactivate one via PATCH.
	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/services/"+inactive.ID, map[string]any{
		"active": false,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate status = %d, body %s", rec.Code, rec.Body.String())
	}

	// Public route, no auth header at all.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/services", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public list status = %d, body %s", rec.Code, rec.Body.String())
	}
	var list serviceListTestBody
	decodeBody(t, rec, &list)
	if len(list.Items) != 2 || list.Items[0].ID != first.ID || list.Items[1].ID != second.ID {
		t.Fatalf("public items = %+v, want First, Second ordered by sortOrder, inactive hidden", list.Items)
	}

	// Practitioner sees all three.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/services/all", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list all status = %d", rec.Code)
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 3 {
		t.Fatalf("practitioner items = %+v, want all 3 incl. inactive", list.Items)
	}
}

func TestServiceCreateShape(t *testing.T) {
	rig := newCatalogTestRig(t)
	created := createServiceViaHTTP(t, rig, "Swedish Massage", 1)
	if created.ID == "" || created.PractitionerID != testPractitionerID {
		t.Errorf("created = %+v, want id assigned and practitioner stamped", created)
	}
	if created.Currency != "GHS" || !created.Active || created.DurationMinutes != 60 || created.PriceKobo != 25000 {
		t.Errorf("created = %+v, want contract defaults (GHS, active)", created)
	}
}

func TestServiceCreateValidation(t *testing.T) {
	rig := newCatalogTestRig(t)
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/services", map[string]any{
		"name":            "Too Short",
		"durationMinutes": 3,
		"priceKobo":       100,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "validation_error" {
		t.Errorf("error code = %q, want validation_error", errRes.Error.Code)
	}
}

func TestServiceUpdateAndDelete(t *testing.T) {
	rig := newCatalogTestRig(t)
	created := createServiceViaHTTP(t, rig, "Massage", 1)

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/services/"+created.ID, map[string]any{
		"priceKobo": 30000,
		"sortOrder": 5,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Service serviceTestBody `json:"service"`
	}
	decodeBody(t, rec, &res)
	if res.Service.PriceKobo != 30000 || res.Service.SortOrder != 5 || res.Service.Name != "Massage" {
		t.Errorf("patched = %+v, want price 30000, order 5, name untouched", res.Service)
	}

	// Unknown id -> 404 service_not_found.
	rec = doJSON(t, rig.srv, http.MethodPatch, "/v1/services/unknown", map[string]any{
		"priceKobo": 1,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("patch unknown status = %d, want 404", rec.Code)
	}
	var errRes errorTestResponse
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "service_not_found" {
		t.Errorf("error code = %q, want service_not_found", errRes.Error.Code)
	}

	// Delete -> 204, gone from the practitioner list.
	rec = doJSON(t, rig.srv, http.MethodDelete, "/v1/services/"+created.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/services/all", nil, bearer(rig.practitionerToken))
	var list serviceListTestBody
	decodeBody(t, rec, &list)
	if len(list.Items) != 0 {
		t.Errorf("items after delete = %+v, want empty", list.Items)
	}
}

func TestServiceSoftDeleteWithBookings(t *testing.T) {
	rig := newCatalogTestRig(t)
	created := createServiceViaHTTP(t, rig, "Booked Massage", 1)
	rig.services.BookedServiceIDs[created.ID] = true

	rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/services/"+created.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	// Retained for history but hidden everywhere.
	raw, ok := rig.services.Raw(created.ID)
	if !ok || raw.DeletedAt == nil {
		t.Errorf("soft-deleted record missing or unmarked: %+v", raw)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/services", nil, nil)
	var list serviceListTestBody
	decodeBody(t, rec, &list)
	if len(list.Items) != 0 {
		t.Errorf("public items after soft delete = %+v, want empty", list.Items)
	}
}

func TestServiceMutationsRequirePractitioner(t *testing.T) {
	rig := newCatalogTestRig(t)
	created := createServiceViaHTTP(t, rig, "Massage", 1)

	mutations := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/v1/services", map[string]any{"name": "X", "durationMinutes": 60}},
		{http.MethodPatch, "/v1/services/" + created.ID, map[string]any{"active": false}},
		{http.MethodDelete, "/v1/services/" + created.ID, nil},
		{http.MethodGet, "/v1/services/all", nil},
	}
	for _, m := range mutations {
		// Client role -> 403 forbidden.
		rec := doJSON(t, rig.srv, m.method, m.path, m.body, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as client status = %d, want 403", m.method, m.path, rec.Code)
		}
		// No token -> 401.
		rec = doJSON(t, rig.srv, m.method, m.path, m.body, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s unauthenticated status = %d, want 401", m.method, m.path, rec.Code)
		}
	}
}

func TestCatalogUnavailableWithoutService(t *testing.T) {
	srv := NewServer(WithCatalog(nil, nil), WithScheduling(nil, nil))
	for _, path := range []string{"/v1/services", "/v1/services/all", "/v1/availability/rules", "/v1/availability/slots"} {
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

// seedSlotSchedule gives the practitioner a 60-minute service and opens
// 09:00-12:00 on the weekday seven days out, returning (serviceID, day).
func seedSlotSchedule(t *testing.T, rig catalogTestRig) (string, time.Time) {
	t.Helper()
	created := createServiceViaHTTP(t, rig, "Massage", 1)

	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	rules := map[string]any{
		"rules": []map[string]any{{
			"weekday":       int(day.Weekday()),
			"windows":       []map[string]int{{"startMin": 540, "endMin": 720}},
			"bufferMinutes": 0,
		}},
	}
	rec := doJSON(t, rig.srv, http.MethodPut, "/v1/availability/rules", rules, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("put rules status = %d, body %s", rec.Code, rec.Body.String())
	}
	return created.ID, day
}

func TestSlotsEndpoint(t *testing.T) {
	rig := newCatalogTestRig(t)
	serviceID, day := seedSlotSchedule(t, rig)
	date := day.Format("2006-01-02")

	// Public: no auth header.
	rec := doJSON(t, rig.srv, http.MethodGet,
		"/v1/availability/slots?serviceId="+serviceID+"&from="+date+"&to="+date+"&tz=UTC", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("slots status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		ServiceID       string `json:"serviceId"`
		DurationMinutes int    `json:"durationMinutes"`
		Timezone        string `json:"timezone"`
		Slots           []struct {
			StartAt time.Time `json:"startAt"`
			EndAt   time.Time `json:"endAt"`
		} `json:"slots"`
	}
	decodeBody(t, rec, &res)
	if res.ServiceID != serviceID || res.DurationMinutes != 60 || res.Timezone != "UTC" {
		t.Errorf("envelope = %+v, want serviceId/duration/timezone echoed", res)
	}
	if len(res.Slots) != 3 {
		t.Fatalf("slots = %+v, want 3 hourly starts", res.Slots)
	}
	for _, s := range res.Slots {
		if s.EndAt.Sub(s.StartAt) != time.Hour || s.StartAt.Location() != time.UTC {
			t.Errorf("slot %+v, want 1h in UTC", s)
		}
	}
}

func TestSlotsEndpointTimeOffBlocks(t *testing.T) {
	rig := newCatalogTestRig(t)
	serviceID, day := seedSlotSchedule(t, rig)
	date := day.Format("2006-01-02")

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/availability/time-off", map[string]any{
		"startAt": day,
		"endAt":   day.Add(24 * time.Hour),
		"reason":  "holiday",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("time-off status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet,
		"/v1/availability/slots?serviceId="+serviceID+"&from="+date+"&to="+date+"&tz=UTC", nil, nil)
	var res struct {
		Slots []any `json:"slots"`
	}
	decodeBody(t, rec, &res)
	if len(res.Slots) != 0 {
		t.Errorf("slots during time-off = %v, want none", res.Slots)
	}
}

func TestSlotsEndpointBadInput(t *testing.T) {
	rig := newCatalogTestRig(t)
	serviceID, day := seedSlotSchedule(t, rig)
	date := day.Format("2006-01-02")

	cases := []struct {
		name     string
		path     string
		wantCode int
		wantErr  string
	}{
		{"missing serviceId", "/v1/availability/slots?from=" + date + "&to=" + date, 400, "validation_error"},
		{"bad from", "/v1/availability/slots?serviceId=" + serviceID + "&from=nextweek&to=" + date, 400, "validation_error"},
		{"bad tz", "/v1/availability/slots?serviceId=" + serviceID + "&from=" + date + "&to=" + date + "&tz=Mars/Olympus", 400, "invalid_timezone"},
		{"unknown service", "/v1/availability/slots?serviceId=unknown&from=" + date + "&to=" + date, 404, "service_not_found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodGet, tc.path, nil, nil)
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

func TestAvailabilityRulesRoundTrip(t *testing.T) {
	rig := newCatalogTestRig(t)
	rules := map[string]any{
		"rules": []map[string]any{
			{"weekday": 1, "windows": []map[string]int{{"startMin": 540, "endMin": 720}}, "bufferMinutes": 15},
			{"weekday": 3, "windows": []map[string]int{{"startMin": 600, "endMin": 900}}, "bufferMinutes": 0},
		},
	}
	rec := doJSON(t, rig.srv, http.MethodPut, "/v1/availability/rules", rules, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("put rules status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/availability/rules", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("get rules status = %d", rec.Code)
	}
	var body struct {
		Rules []struct {
			Weekday int `json:"weekday"`
			Windows []struct {
				StartMin int `json:"startMin"`
				EndMin   int `json:"endMin"`
			} `json:"windows"`
			BufferMinutes int `json:"bufferMinutes"`
		} `json:"rules"`
	}
	decodeBody(t, rec, &body)
	if len(body.Rules) != 2 || body.Rules[0].Weekday != 1 || body.Rules[1].Weekday != 3 {
		t.Fatalf("rules = %+v, want weekday 1 and 3 in order", body.Rules)
	}
	if body.Rules[0].BufferMinutes != 15 || body.Rules[0].Windows[0].StartMin != 540 {
		t.Errorf("rule = %+v, want buffer 15 and window start 540", body.Rules[0])
	}

	// Client role -> 403 on both mutations and reads.
	for _, m := range []struct{ method, path string }{
		{http.MethodGet, "/v1/availability/rules"},
		{http.MethodPut, "/v1/availability/rules"},
		{http.MethodPost, "/v1/availability/time-off"},
	} {
		rec := doJSON(t, rig.srv, m.method, m.path, map[string]any{}, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as client status = %d, want 403", m.method, m.path, rec.Code)
		}
	}

	// Invalid window -> 400 validation_error.
	rec = doJSON(t, rig.srv, http.MethodPut, "/v1/availability/rules", map[string]any{
		"rules": []map[string]any{
			{"weekday": 2, "windows": []map[string]int{{"startMin": 1200, "endMin": 600}}},
		},
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("overnight rules status = %d, want 400", rec.Code)
	}
}
