package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	enquiriesapp "github.com/xcreativs/terios/api/internal/app/enquiries"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// enquiryTestRig bundles a server with both enquiry surfaces mounted: the
// public contact form and the practitioner inbox.
type enquiryTestRig struct {
	srv               *Server
	notifier          *portstest.FakeNotifier
	practitionerToken string
	clientToken       string
}

func newEnquiryTestRig(t *testing.T, rateLimit int) enquiryTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	notifier := portstest.NewFakeNotifier()
	svc := enquiriesapp.NewService(
		portstest.NewFakeEnquiryRepository(),
		enquiriesapp.WithNotifications(notifier),
	)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return enquiryTestRig{
		srv: NewServer(WithAuth(authSvc), WithEnquiries(svc, authSvc, WithEnquiryRateLimit(RateLimitPolicy{
			Limit:  rateLimit,
			Window: time.Hour,
		}))),
		notifier:          notifier,
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
	}
}

type enquiryTestBody struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func submitEnquiry(t *testing.T, rig enquiryTestRig, body map[string]any, ip string) int {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/enquiries", body, map[string]string{"X-Forwarded-For": ip})
	return rec.Code
}

func validEnquiry() map[string]any {
	return map[string]any{
		"name":    "Ama Serwaa",
		"email":   "ama@example.com",
		"subject": "Booking question",
		"message": "Do you offer prenatal massage?",
	}
}

// TestSubmitIsPublicAndAlertsThePractice.
func TestSubmitIsPublicAndAlertsThePractice(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)

	if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
		t.Fatalf("submit status = %d, want 201", code)
	}
	if len(rig.notifier.Enquiries) != 1 {
		t.Fatalf("alerts = %d, want 1", len(rig.notifier.Enquiries))
	}
	notice := rig.notifier.Enquiries[0]
	if notice.SenderEmail != "ama@example.com" || notice.SenderName != "Ama Serwaa" {
		t.Errorf("notice = %+v, want the sender's details", notice)
	}
	if notice.EnquiryID == "" {
		t.Error("alert carries no enquiry id")
	}

	// The inbox has it.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/enquiries", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("inbox status = %d, body %s", rec.Code, rec.Body.String())
	}
	var inbox struct {
		Items []enquiryTestBody `json:"items"`
	}
	decodeBody(t, rec, &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Status != "new" {
		t.Errorf("inbox = %+v, want one new enquiry", inbox.Items)
	}
}

// TestSubmitResponseRevealsNothing: an anonymous caller gets an
// acknowledgement, not a record they have no route to read.
func TestSubmitResponseRevealsNothing(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/enquiries", validEnquiry(), map[string]string{
		"X-Forwarded-For": "203.0.113.7",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	var body map[string]any
	decodeBody(t, rec, &body)
	if body["received"] != true {
		t.Errorf("body = %+v, want an acknowledgement", body)
	}
	for _, leak := range []string{"id", "enquiry", "status", "sourceIp"} {
		if _, present := body[leak]; present {
			t.Errorf("%q leaked into the public submit response: %+v", leak, body)
		}
	}
}

// TestSubmitIsRateLimited: the only unauthenticated write in the API needs
// a cap of its own, or the inbox is a spam target.
func TestSubmitIsRateLimited(t *testing.T) {
	const limit = 3
	rig := newEnquiryTestRig(t, limit)

	for i := 0; i < limit; i++ {
		if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
			t.Fatalf("submission %d status = %d, want 201", i+1, code)
		}
	}
	if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusTooManyRequests {
		t.Errorf("submission %d status = %d, want 429", limit+1, code)
	}

	// Another visitor is unaffected.
	if code := submitEnquiry(t, rig, validEnquiry(), "198.51.100.4"); code != http.StatusCreated {
		t.Errorf("a different visitor was blocked (status %d)", code)
	}
}

// TestSubmitValidation: the form is open to anyone, so bad input is a 400
// rather than a stored row.
func TestSubmitValidation(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)

	cases := map[string]map[string]any{
		"blank name":    {"name": "", "email": "ama@example.com", "message": "Hello"},
		"blank message": {"name": "Ama", "email": "ama@example.com", "message": "   "},
		"bad email":     {"name": "Ama", "email": "not-an-address", "message": "Hello"},
		"header injection in email": {
			"name": "Ama", "email": "ama@example.com\nbcc:victim@example.com", "message": "Hello",
		},
		"over-long message": {"name": "Ama", "email": "ama@example.com", "message": strings.Repeat("x", 5001)},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/enquiries", body, map[string]string{
				"X-Forwarded-For": "203.0.113.7",
			})
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

	if len(rig.notifier.Enquiries) != 0 {
		t.Errorf("alerts = %d, want none for rejected submissions", len(rig.notifier.Enquiries))
	}
}

// TestInboxIsPractitionerOnly.
func TestInboxIsPractitionerOnly(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)
	if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
		t.Fatalf("submit status = %d", code)
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/admin/enquiries"},
		{http.MethodGet, "/v1/admin/enquiries/unread-count"},
		{http.MethodGet, "/v1/admin/enquiries/enquiry-1"},
		{http.MethodPatch, "/v1/admin/enquiries/enquiry-1"},
		{http.MethodDelete, "/v1/admin/enquiries/enquiry-1"},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as client = %d, want 403", tc.method, tc.path, rec.Code)
		}
		rec = doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

// TestTriageAndUnreadCount: the badge follows the inbox.
func TestTriageAndUnreadCount(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)
	for i := 0; i < 2; i++ {
		if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
			t.Fatalf("submit status = %d", code)
		}
	}

	unread := func() int {
		t.Helper()
		rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/enquiries/unread-count", nil, bearer(rig.practitionerToken))
		if rec.Code != http.StatusOK {
			t.Fatalf("unread-count status = %d, body %s", rec.Code, rec.Body.String())
		}
		var res struct {
			Count int `json:"count"`
		}
		decodeBody(t, rec, &res)
		return res.Count
	}

	if got := unread(); got != 2 {
		t.Fatalf("unread = %d, want 2", got)
	}

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/admin/enquiries/enquiry-1", map[string]any{
		"status": "read",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}
	if got := unread(); got != 1 {
		t.Errorf("unread = %d after reading one, want 1", got)
	}

	// Filtering by state.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/enquiries?status=read", nil, bearer(rig.practitionerToken))
	var filtered struct {
		Items []enquiryTestBody `json:"items"`
	}
	decodeBody(t, rec, &filtered)
	if len(filtered.Items) != 1 || filtered.Items[0].ID != "enquiry-1" {
		t.Errorf("filtered inbox = %+v, want the one read enquiry", filtered.Items)
	}

	// An unknown state is a 400, not a silent empty list.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/enquiries?status=spam", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown status filter = %d, want 400", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodPatch, "/v1/admin/enquiries/enquiry-1", map[string]any{
		"status": "spam",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown status patch = %d, want 400", rec.Code)
	}
}

// TestDeleteAndMissingEnquiry.
func TestDeleteAndMissingEnquiry(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)
	if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
		t.Fatalf("submit status = %d", code)
	}

	rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/enquiries/enquiry-1", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/admin/enquiries/enquiry-1"},
		{http.MethodDelete, "/v1/admin/enquiries/enquiry-1"},
		{http.MethodGet, "/v1/admin/enquiries/no-such-id"},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(rig.practitionerToken))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s = %d, want 404", tc.method, tc.path, rec.Code)
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "enquiry_not_found" {
			t.Errorf("%s %s code = %q, want enquiry_not_found", tc.method, tc.path, errRes.Error.Code)
		}
	}
}

// TestEnquiryBodyHidesSourceIP: the address is kept for abuse triage in the
// database, not published to the inbox UI.
func TestEnquiryBodyHidesSourceIP(t *testing.T) {
	rig := newEnquiryTestRig(t, 100)
	if code := submitEnquiry(t, rig, validEnquiry(), "203.0.113.7"); code != http.StatusCreated {
		t.Fatalf("submit status = %d", code)
	}

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/enquiries/enquiry-1", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Enquiry map[string]any `json:"enquiry"`
	}
	decodeBody(t, rec, &res)
	if _, present := res.Enquiry["sourceIp"]; present {
		t.Errorf("sourceIp reached the inbox shape: %+v", res.Enquiry)
	}
	if res.Enquiry["message"] != "Do you offer prenatal massage?" {
		t.Errorf("enquiry = %+v, want the message", res.Enquiry)
	}
}

// TestEnquiriesUnavailableWithoutDatabase.
func TestEnquiriesUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithEnquiries(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/v1/enquiries"},
		{http.MethodGet, "/v1/admin/enquiries"},
		{http.MethodGet, "/v1/admin/enquiries/abc"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
