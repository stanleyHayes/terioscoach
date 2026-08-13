package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	reviewsapp "github.com/xcreativs/terios/api/internal/app/reviews"
	domainbooking "github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

const reviewPracID = "prac-1"

// reviewTestRig bundles a server with all three review surfaces mounted.
type reviewTestRig struct {
	srv                *Server
	bookings           *portstest.FakeBookingRepository
	services           *portstest.FakeServiceRepository
	users              *portstest.FakeUserRepository
	practitionerToken  string
	practitioner2Token string
	clientToken        string
	otherClientToken   string
	clientID           string
	otherClientID      string
}

func newReviewTestRig(t *testing.T) reviewTestRig {
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
	services := portstest.NewFakeServiceRepository()
	svc := reviewsapp.NewService(portstest.NewFakeReviewRepository(), bookings, users, services)

	rig := reviewTestRig{
		srv:      NewServer(WithAuth(authSvc), WithReviews(svc, authSvc)),
		bookings: bookings,
		services: services,
		users:    users,
	}

	seedUser := func(email, name string) string {
		user, err := identity.NewUser(email, name, "hash", identity.RoleClient, time.Now().UTC())
		if err != nil {
			t.Fatalf("identity.NewUser: %v", err)
		}
		user, err = users.Create(t.Context(), user)
		if err != nil {
			t.Fatalf("seed user: %v", err)
		}
		return user.ID
	}
	rig.clientID = seedUser("ama@example.com", "Ama Serwaa")
	rig.otherClientID = seedUser("koffi@example.com", "Koffi Mensah")

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	rig.practitionerToken = issue(reviewPracID, identity.RolePractitioner)
	rig.practitioner2Token = issue("prac-2", identity.RolePractitioner)
	rig.clientToken = issue(rig.clientID, identity.RoleClient)
	rig.otherClientToken = issue(rig.otherClientID, identity.RoleClient)
	return rig
}

// seedReviewableBooking stores a completed booking for the given client and
// the service it was for.
func seedReviewableBooking(t *testing.T, rig reviewTestRig, clientID string, status domainbooking.Status) string {
	t.Helper()
	svc, err := catalog.NewService(reviewPracID, "Deep Tissue Massage", "", 60, 25000, "GHS", 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("catalog.NewService: %v", err)
	}
	svc, err = rig.services.Create(t.Context(), svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}

	start := time.Now().UTC().Add(-48 * time.Hour)
	b, err := domainbooking.New(clientID, reviewPracID, svc.ID, start, 60, start.Add(-72*time.Hour))
	if err != nil {
		t.Fatalf("booking.New: %v", err)
	}
	b.Status = status
	b, err = rig.bookings.Create(t.Context(), b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return b.ID
}

type reviewTestBody struct {
	ID        string `json:"id"`
	BookingID string `json:"bookingId"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment"`
	Status    string `json:"status"`
}

func submitReview(t *testing.T, rig reviewTestRig, token, bookingID string, rating int, comment string) (int, reviewTestBody) {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
		"bookingId": bookingID, "rating": rating, "comment": comment,
	}, bearer(token))
	var res struct {
		Review reviewTestBody `json:"review"`
	}
	if rec.Code == http.StatusCreated {
		decodeBody(t, rec, &res)
	}
	return rec.Code, res.Review
}

// TestSubmitRequiresACompletedOwnSession is the pair of guards that make a
// public rating mean something: you reviewed a session, and it was yours.
func TestSubmitRequiresACompletedOwnSession(t *testing.T) {
	rig := newReviewTestRig(t)

	upcoming := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusConfirmed)
	code, _ := submitReview(t, rig, rig.clientToken, upcoming, 5, "Looking forward to it.")
	if code != http.StatusUnprocessableEntity {
		t.Errorf("review of an upcoming session = %d, want 422", code)
	}

	// Another client's completed session is not-found, never forbidden:
	// "forbidden" would confirm the booking exists.
	someoneElses := seedReviewableBooking(t, rig, rig.otherClientID, domainbooking.StatusCompleted)
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
		"bookingId": someoneElses, "rating": 5,
	}, bearer(rig.clientToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("review of another client's session = %d, want 404", rec.Code)
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "booking_not_found" {
		t.Errorf("code = %q, want booking_not_found", errRes.Error.Code)
	}

	completed := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	code, review := submitReview(t, rig, rig.clientToken, completed, 5, "Wonderful session.")
	if code != http.StatusCreated {
		t.Fatalf("review of an own completed session = %d, want 201", code)
	}
	if review.Status != "pending" {
		t.Errorf("status = %q on submission, want pending", review.Status)
	}
}

// TestOneReviewPerSession.
func TestOneReviewPerSession(t *testing.T) {
	rig := newReviewTestRig(t)
	booking := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)

	if code, _ := submitReview(t, rig, rig.clientToken, booking, 5, "First."); code != http.StatusCreated {
		t.Fatalf("first review = %d, want 201", code)
	}
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
		"bookingId": booking, "rating": 1, "comment": "Second.",
	}, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second review = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "review_exists" {
		t.Errorf("code = %q, want review_exists", errRes.Error.Code)
	}
}

// TestModerationGatesThePublicList: nothing appears until the practitioner
// approves, and the public shape publishes the minimum.
func TestModerationGatesThePublicList(t *testing.T) {
	rig := newReviewTestRig(t)
	booking := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	_, review := submitReview(t, rig, rig.clientToken, booking, 5, "Wonderful session.")

	publicItems := func() []map[string]any {
		t.Helper()
		rec := doJSON(t, rig.srv, http.MethodGet, "/v1/content/reviews", nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("public list status = %d, body %s", rec.Code, rec.Body.String())
		}
		var res struct {
			Items []map[string]any `json:"items"`
		}
		decodeBody(t, rec, &res)
		return res.Items
	}

	if got := publicItems(); len(got) != 0 {
		t.Fatalf("public reviews = %d, want none before approval", len(got))
	}

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/reviews/"+review.ID+"/approve", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, body %s", rec.Code, rec.Body.String())
	}

	items := publicItems()
	if len(items) != 1 {
		t.Fatalf("public reviews = %d, want the approved one", len(items))
	}
	if items[0]["authorName"] != "Ama" {
		t.Errorf("authorName = %v, want the first name only", items[0]["authorName"])
	}
	if items[0]["serviceName"] != "Deep Tissue Massage" {
		t.Errorf("serviceName = %v, want the reviewed service", items[0]["serviceName"])
	}
	for _, leak := range []string{"clientId", "bookingId", "status", "email"} {
		if _, present := items[0][leak]; present {
			t.Errorf("%q leaked into the public review shape: %+v", leak, items[0])
		}
	}

	// Rejection takes it back off.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/admin/reviews/"+review.ID+"/reject", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("reject status = %d", rec.Code)
	}
	if got := publicItems(); len(got) != 0 {
		t.Errorf("public reviews = %d after rejection, want none", len(got))
	}
}

// TestEditingIsPendingOnlyOverHTTP.
func TestEditingIsPendingOnlyOverHTTP(t *testing.T) {
	rig := newReviewTestRig(t)
	booking := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	_, review := submitReview(t, rig, rig.clientToken, booking, 5, "Wonderful.")

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/reviews/"+review.ID, map[string]any{
		"rating": 4, "comment": "On reflection, four stars.",
	}, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("edit while pending = %d, body %s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/reviews/"+review.ID+"/approve", nil, bearer(rig.practitionerToken)); rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d", rec.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodPatch, "/v1/reviews/"+review.ID, map[string]any{
		"rating": 1, "comment": "Actually, terrible.",
	}, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("edit after approval = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "already_moderated" {
		t.Errorf("code = %q, want already_moderated", errRes.Error.Code)
	}

	// The published text is untouched.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/reviews", nil, nil)
	var public struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rec, &public)
	if len(public.Items) != 1 || public.Items[0]["comment"] != "On reflection, four stars." {
		t.Errorf("public review = %+v, want the approved text intact", public.Items)
	}
}

// TestReviewIsolation: a client cannot touch another client's review, and a
// practitioner cannot moderate another practice's.
func TestReviewIsolation(t *testing.T) {
	rig := newReviewTestRig(t)
	booking := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	_, review := submitReview(t, rig, rig.clientToken, booking, 5, "Wonderful.")

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/reviews/"+review.ID, map[string]any{
		"rating": 1,
	}, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another client's edit = %d, want 404", rec.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/admin/reviews/"+review.ID+"/approve", nil, bearer(rig.practitioner2Token))
	if rec.Code != http.StatusNotFound {
		t.Errorf("another practitioner's approval = %d, want 404", rec.Code)
	}

	// The queue is scoped too.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reviews", nil, bearer(rig.practitioner2Token))
	var queue struct {
		Items []reviewTestBody `json:"items"`
	}
	decodeBody(t, rec, &queue)
	if len(queue.Items) != 0 {
		t.Errorf("another practitioner's queue = %+v, want it empty", queue.Items)
	}

	// A client sees only their own.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/reviews/mine", nil, bearer(rig.otherClientToken))
	var mine struct {
		Items []reviewTestBody `json:"items"`
	}
	decodeBody(t, rec, &mine)
	if len(mine.Items) != 0 {
		t.Errorf("another client's list = %+v, want it empty", mine.Items)
	}
}

// TestRoleGuards.
func TestReviewRoleGuards(t *testing.T) {
	rig := newReviewTestRig(t)

	for _, tc := range []struct{ method, path, token string }{
		{http.MethodPost, "/v1/reviews", rig.practitionerToken},
		{http.MethodGet, "/v1/reviews/mine", rig.practitionerToken},
		{http.MethodGet, "/v1/admin/reviews", rig.clientToken},
		{http.MethodPost, "/v1/admin/reviews/review-1/approve", rig.clientToken},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(tc.token))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with the wrong role = %d, want 403", tc.method, tc.path, rec.Code)
		}
	}

	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/v1/reviews"},
		{http.MethodGet, "/v1/reviews/mine"},
		{http.MethodGet, "/v1/admin/reviews"},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}

	// The public list needs no auth at all.
	if rec := doJSON(t, rig.srv, http.MethodGet, "/v1/content/reviews", nil, nil); rec.Code != http.StatusOK {
		t.Errorf("public list anonymous = %d, want 200", rec.Code)
	}
}

// TestPublicSummaryAggregatesApprovedOnly.
func TestPublicSummaryAggregatesApprovedOnly(t *testing.T) {
	rig := newReviewTestRig(t)

	approve := func(id string) {
		t.Helper()
		if rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/reviews/"+id+"/approve", nil, bearer(rig.practitionerToken)); rec.Code != http.StatusOK {
			t.Fatalf("approve status = %d", rec.Code)
		}
	}

	first := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	_, r1 := submitReview(t, rig, rig.clientToken, first, 5, "Five.")
	approve(r1.ID)

	second := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	_, r2 := submitReview(t, rig, rig.clientToken, second, 4, "Four.")
	approve(r2.ID)

	// A third, left pending, must not move the average.
	third := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)
	submitReview(t, rig, rig.clientToken, third, 1, "One.")

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/content/reviews/summary", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status = %d, body %s", rec.Code, rec.Body.String())
	}
	var summary struct {
		Count        int            `json:"count"`
		Average      float64        `json:"average"`
		Distribution map[string]int `json:"distribution"`
	}
	decodeBody(t, rec, &summary)

	if summary.Count != 2 {
		t.Errorf("count = %d, want only the approved reviews", summary.Count)
	}
	if summary.Average != 4.5 {
		t.Errorf("average = %v, want 4.5", summary.Average)
	}
	if summary.Distribution["5"] != 1 || summary.Distribution["4"] != 1 || summary.Distribution["1"] != 0 {
		t.Errorf("distribution = %v, want the pending review excluded", summary.Distribution)
	}
}

// TestReviewValidation.
func TestReviewValidation(t *testing.T) {
	rig := newReviewTestRig(t)
	booking := seedReviewableBooking(t, rig, rig.clientID, domainbooking.StatusCompleted)

	for name, rating := range map[string]int{"zero": 0, "six": 6, "negative": -1} {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/reviews", map[string]any{
				"bookingId": booking, "rating": rating,
			}, bearer(rig.clientToken))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
		})
	}

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/reviews?status=live", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown status filter = %d, want 400", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/reviews?limit=0", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("limit=0 = %d, want 400", rec.Code)
	}
}

// TestReviewsUnavailableWithoutDatabase.
func TestReviewsUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithReviews(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/content/reviews"},
		{http.MethodGet, "/v1/content/reviews/summary"},
		{http.MethodPost, "/v1/reviews"},
		{http.MethodGet, "/v1/reviews/mine"},
		{http.MethodGet, "/v1/admin/reviews"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
