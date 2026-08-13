package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	notesapp "github.com/xcreativs/terios/api/internal/app/notes"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// noteTestRig bundles a server with the session-notes slice mounted over
// in-memory fakes. Bookings are seeded straight into the repository: the
// booking routes are a different slice, and notes only need the record's
// ownership fields.
type noteTestRig struct {
	srv                *Server
	bookings           *portstest.FakeBookingRepository
	practitionerToken  string
	practitioner2Token string
	clientToken        string
	otherClientToken   string
}

func newNoteTestRig(t *testing.T) noteTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	bookings := portstest.NewFakeBookingRepository()
	svc := notesapp.NewService(portstest.NewFakeSessionNoteRepository(), bookings)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return noteTestRig{
		srv:                NewServer(WithAuth(authSvc), WithNotes(svc, authSvc)),
		bookings:           bookings,
		practitionerToken:  issue("prac-1", identity.RolePractitioner),
		practitioner2Token: issue("prac-2", identity.RolePractitioner),
		clientToken:        issue("client-1", identity.RoleClient),
		otherClientToken:   issue("client-2", identity.RoleClient),
	}
}

// seedNoteBooking inserts a confirmed booking for client-1 with prac-1.
func seedNoteBooking(t *testing.T, rig noteTestRig) booking.Booking {
	t.Helper()
	start := time.Now().UTC().Add(24 * time.Hour)
	b, err := booking.New("client-1", "prac-1", "svc-1", start, 60, time.Now().UTC())
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b, err = rig.bookings.Create(t.Context(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b
}

func notesPath(bookingID string) string { return "/v1/bookings/" + bookingID + "/notes" }
func sharePath(bookingID string) string { return notesPath(bookingID) + "/share" }

// putNote writes note content as the practitioner and returns the raw
// response for status assertions.
func putNote(t *testing.T, rig noteTestRig, bookingID string, body map[string]any, token string) map[string]any {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPut, notesPath(bookingID), body, bearer(token))
	if rec.Code != http.StatusOK {
		t.Fatalf("put notes status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Note map[string]any `json:"note"`
	}
	decodeBody(t, rec, &res)
	return res.Note
}

// TestNotesUpsertAndPractitionerRead: PUT creates then replaces wholesale,
// and the practitioner always reads the full shape.
func TestNotesUpsertAndPractitionerRead(t *testing.T) {
	rig := newNoteTestRig(t)
	b := seedNoteBooking(t, rig)

	created := putNote(t, rig, b.ID, map[string]any{
		"privateNotes":    "tension in left shoulder",
		"sharedFeedback":  "great progress",
		"sharedResources": []string{"https://example.com/stretch"},
	}, rig.practitionerToken)

	if created["bookingId"] != b.ID || created["clientId"] != "client-1" || created["practitionerId"] != "prac-1" {
		t.Errorf("note = %+v, want the parties stamped from the booking", created)
	}
	if created["id"] == "" || created["id"] == nil {
		t.Error("note id missing")
	}
	if _, present := created["sharedAt"]; present {
		t.Errorf("sharedAt present before sharing: %+v", created)
	}

	replaced := putNote(t, rig, b.ID, map[string]any{
		"privateNotes":   "revised private",
		"sharedFeedback": "revised feedback",
	}, rig.practitionerToken)
	if replaced["id"] != created["id"] {
		t.Errorf("second PUT id = %v, want the same note %v", replaced["id"], created["id"])
	}
	if resources, ok := replaced["sharedResources"].([]any); !ok || len(resources) != 0 {
		t.Errorf("sharedResources = %v, want cleared by the wholesale replace", replaced["sharedResources"])
	}

	rec := doJSON(t, rig.srv, http.MethodGet, notesPath(b.ID), nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("practitioner get status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Note map[string]any `json:"note"`
	}
	decodeBody(t, rec, &res)
	if res.Note["privateNotes"] != "revised private" {
		t.Errorf("practitioner note = %+v, want the private notes included", res.Note)
	}
}

// TestNotesSharingVisibility: the client sees nothing until the note is
// shared, then sees the shared projection only — never the private notes.
func TestNotesSharingVisibility(t *testing.T) {
	rig := newNoteTestRig(t)
	b := seedNoteBooking(t, rig)
	putNote(t, rig, b.ID, map[string]any{
		"privateNotes":    "secret diagnosis",
		"sharedFeedback":  "great progress",
		"sharedResources": []string{"https://example.com/stretch"},
	}, rig.practitionerToken)

	// Before sharing: indistinguishable from "no note exists".
	rec := doJSON(t, rig.srv, http.MethodGet, notesPath(b.ID), nil, bearer(rig.clientToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("client get before share = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "note_not_found" {
		t.Errorf("code = %q, want note_not_found", errRes.Error.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodPost, sharePath(b.ID), nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("share status = %d, body %s", rec.Code, rec.Body.String())
	}
	var shareRes struct {
		Note map[string]any `json:"note"`
	}
	decodeBody(t, rec, &shareRes)
	firstStamp, ok := shareRes.Note["sharedAt"].(string)
	if !ok || firstStamp == "" {
		t.Fatalf("sharedAt = %v, want a timestamp after sharing", shareRes.Note["sharedAt"])
	}

	// Share is idempotent: a repeat answers 200 with the original stamp.
	rec = doJSON(t, rig.srv, http.MethodPost, sharePath(b.ID), nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("repeat share status = %d, body %s", rec.Code, rec.Body.String())
	}
	decodeBody(t, rec, &shareRes)
	if shareRes.Note["sharedAt"] != firstStamp {
		t.Errorf("sharedAt = %v, want the original stamp %q kept", shareRes.Note["sharedAt"], firstStamp)
	}

	// After sharing the client gets the shared projection only.
	rec = doJSON(t, rig.srv, http.MethodGet, notesPath(b.ID), nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("client get after share = %d, body %s", rec.Code, rec.Body.String())
	}
	var clientRes struct {
		Note map[string]any `json:"note"`
	}
	decodeBody(t, rec, &clientRes)
	if clientRes.Note["sharedFeedback"] != "great progress" || clientRes.Note["bookingId"] != b.ID {
		t.Errorf("client note = %+v, want the shared content", clientRes.Note)
	}
	for _, forbidden := range []string{"privateNotes", "id", "practitionerId", "clientId"} {
		if _, present := clientRes.Note[forbidden]; present {
			t.Errorf("%q leaked into the client note view: %+v", forbidden, clientRes.Note)
		}
	}

	// Editing after sharing neither unshares nor re-stamps.
	edited := putNote(t, rig, b.ID, map[string]any{
		"privateNotes":   "updated private",
		"sharedFeedback": "updated feedback",
	}, rig.practitionerToken)
	if edited["sharedAt"] != firstStamp {
		t.Errorf("sharedAt after edit = %v, want %q untouched", edited["sharedAt"], firstStamp)
	}
}

// TestNotesShareWithoutNote: sharing before any PUT is note_not_found.
func TestNotesShareWithoutNote(t *testing.T) {
	rig := newNoteTestRig(t)
	b := seedNoteBooking(t, rig)

	rec := doJSON(t, rig.srv, http.MethodPost, sharePath(b.ID), nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("share status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "note_not_found" {
		t.Errorf("code = %q, want note_not_found", errRes.Error.Code)
	}

	// Reading a booking that has no note is the same answer for both roles.
	rec = doJSON(t, rig.srv, http.MethodGet, notesPath(b.ID), nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("practitioner get with no note = %d, want 404", rec.Code)
	}
}

// TestNotesValidation: content limits answer 400 validation_error.
func TestNotesValidation(t *testing.T) {
	rig := newNoteTestRig(t)
	b := seedNoteBooking(t, rig)

	for name, body := range map[string]map[string]any{
		"private notes too long":   {"privateNotes": strings.Repeat("x", 10001)},
		"shared feedback too long": {"sharedFeedback": strings.Repeat("x", 5001)},
		"too many resources":       {"sharedResources": make([]string, 21)},
		"resource too long":        {"sharedResources": []string{strings.Repeat("u", 501)}},
	} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPut, notesPath(b.ID), body, bearer(rig.practitionerToken))
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

// TestNotesIsolationAndRoles: cross-owner access is booking_not_found (no
// existence leak) and writes are practitioner-only.
func TestNotesIsolationAndRoles(t *testing.T) {
	rig := newNoteTestRig(t)
	b := seedNoteBooking(t, rig)
	putNote(t, rig, b.ID, map[string]any{"sharedFeedback": "shared"}, rig.practitionerToken)
	if rec := doJSON(t, rig.srv, http.MethodPost, sharePath(b.ID), nil, bearer(rig.practitionerToken)); rec.Code != http.StatusOK {
		t.Fatalf("share status = %d", rec.Code)
	}

	// Another client and another practitioner both get booking_not_found.
	for name, token := range map[string]string{
		"other client":       rig.otherClientToken,
		"other practitioner": rig.practitioner2Token,
	} {
		rec := doJSON(t, rig.srv, http.MethodGet, notesPath(b.ID), nil, bearer(token))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s get = %d, want 404 (body %s)", name, rec.Code, rec.Body.String())
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "booking_not_found" {
			t.Errorf("%s code = %q, want booking_not_found", name, errRes.Error.Code)
		}
	}

	// An unknown booking id is the same answer.
	rec := doJSON(t, rig.srv, http.MethodGet, notesPath("no-such-booking"), nil, bearer(rig.clientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown booking = %d, want 404", rec.Code)
	}

	// Writes are practitioner-only; a client attempt never reaches the service.
	for _, tc := range []struct{ method, path string }{
		{http.MethodPut, notesPath(b.ID)},
		{http.MethodPost, sharePath(b.ID)},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s as client = %d, want 403", tc.method, rec.Code)
		}
	}

	// Unauthenticated reads and writes are 401.
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, notesPath(b.ID)},
		{http.MethodPut, notesPath(b.ID)},
		{http.MethodPost, sharePath(b.ID)},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s unauthenticated = %d, want 401", tc.method, rec.Code)
		}
	}
}

// TestNotesUnavailableWithoutDatabase: a nil service keeps the routes
// mounted and answers 503.
func TestNotesUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithNotes(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/bookings/abc/notes"},
		{http.MethodPut, "/v1/bookings/abc/notes"},
		{http.MethodPost, "/v1/bookings/abc/notes/share"},
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
