package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	cmsapp "github.com/xcreativs/terios/api/internal/app/cms"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// contentTestRig bundles a server with both CMS surfaces mounted — the
// public one and the practitioner one — over in-memory fakes.
type contentTestRig struct {
	srv               *Server
	practitionerToken string
	clientToken       string
}

func newContentTestRig(t *testing.T) contentTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	svc := cmsapp.NewService(
		portstest.NewFakePageRepository(),
		portstest.NewFakePostRepository(),
		portstest.NewFakeFAQRepository(),
		portstest.NewFakeTestimonialRepository(),
	)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return contentTestRig{
		srv:               NewServer(WithAuth(authSvc), WithContent(svc, authSvc)),
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
	}
}

type postTestBody struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Excerpt     string   `json:"excerpt"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	PublishedAt string   `json:"publishedAt"`
}

// createPost adds a draft article and returns it.
func createPost(t *testing.T, rig contentTestRig, slug, title string) postTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/posts", map[string]any{
		"slug": slug, "title": title, "body": "The body of " + title,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create post status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Post postTestBody `json:"post"`
	}
	decodeBody(t, rec, &res)
	return res.Post
}

func publishPost(t *testing.T, rig contentTestRig, id string) {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/posts/"+id+"/publish", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body %s", rec.Code, rec.Body.String())
	}
}

// TestPublicRoutesNeverServeDrafts is the contract the public site depends
// on: an unpublished page or post is a 404, not a 403, and never appears in
// a listing.
func TestPublicRoutesNeverServeDrafts(t *testing.T) {
	rig := newContentTestRig(t)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/pages", map[string]any{
		"slug": "about", "title": "About", "body": "Our story",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create page status = %d, body %s", rec.Code, rec.Body.String())
	}
	draft := createPost(t, rig, "draft-post", "Draft post")

	for path, wantCode := range map[string]string{
		"/v1/content/pages/about":      "page_not_found",
		"/v1/content/posts/draft-post": "post_not_found",
	} {
		rec := doJSON(t, rig.srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("GET %s = %d, want 404 (body %s)", path, rec.Code, rec.Body.String())
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != wantCode {
			t.Errorf("GET %s code = %q, want %q", path, errRes.Error.Code, wantCode)
		}
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/posts", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public feed status = %d", rec.Code)
	}
	var feed struct {
		Items []postTestBody `json:"items"`
	}
	decodeBody(t, rec, &feed)
	if len(feed.Items) != 0 {
		t.Errorf("public feed = %+v, want it empty while everything is a draft", feed.Items)
	}

	// Publishing flips both.
	publishPost(t, rig, draft.ID)
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/posts/draft-post", nil, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("published post status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
}

// TestPublicFeedOmitsBodies: the listing carries summaries, the article
// route carries the article.
func TestPublicFeedOmitsBodies(t *testing.T) {
	rig := newContentTestRig(t)
	post := createPost(t, rig, "first", "First")
	publishPost(t, rig, post.ID)

	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/content/posts", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("feed status = %d", rec.Code)
	}
	var feed struct {
		Items []postTestBody `json:"items"`
	}
	decodeBody(t, rec, &feed)
	if len(feed.Items) != 1 {
		t.Fatalf("feed = %d items, want 1", len(feed.Items))
	}
	if feed.Items[0].Body != "" {
		t.Errorf("feed item carries the full body (%q), want the summary only", feed.Items[0].Body)
	}
	if feed.Items[0].Title != "First" || feed.Items[0].Slug != "first" {
		t.Errorf("feed item = %+v, want the summary fields", feed.Items[0])
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/posts/first", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("article status = %d", rec.Code)
	}
	var article struct {
		Post postTestBody `json:"post"`
	}
	decodeBody(t, rec, &article)
	if article.Post.Body == "" {
		t.Error("article route returned no body")
	}
}

// TestTestimonialModerationGate: nothing reaches the public list until a
// practitioner approves it, and the public shape does not leak the queue.
func TestTestimonialModerationGate(t *testing.T) {
	rig := newContentTestRig(t)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/testimonials", map[string]any{
		"authorName": "Ama Serwaa", "authorRole": "Client", "quote": "A calm, careful practice.",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create testimonial status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Testimonial struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"testimonial"`
	}
	decodeBody(t, rec, &created)
	if created.Testimonial.Status != "pending" {
		t.Errorf("status = %q on creation, want pending", created.Testimonial.Status)
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/testimonials", nil, nil)
	var public struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rec, &public)
	if len(public.Items) != 0 {
		t.Fatalf("public testimonials = %d, want none before approval", len(public.Items))
	}

	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/testimonials/"+created.Testimonial.ID+"/approve", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/testimonials", nil, nil)
	decodeBody(t, rec, &public)
	if len(public.Items) != 1 {
		t.Fatalf("public testimonials = %d, want the approved one", len(public.Items))
	}
	if public.Items[0]["quote"] != "A calm, careful practice." {
		t.Errorf("item = %+v, want the quote", public.Items[0])
	}
	for _, internal := range []string{"status", "submittedAt", "approvedAt", "sortOrder"} {
		if _, present := public.Items[0][internal]; present {
			t.Errorf("%q leaked into the public testimonial shape: %+v", internal, public.Items[0])
		}
	}

	// Rejection takes it back off the site.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/testimonials/"+created.Testimonial.ID+"/reject", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("reject status = %d", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/testimonials", nil, nil)
	decodeBody(t, rec, &public)
	if len(public.Items) != 0 {
		t.Errorf("public testimonials = %d after rejection, want none", len(public.Items))
	}
}

// TestAdminRoutesArePractitionerOnly: a client — or an anonymous caller —
// cannot read drafts, publish, or moderate.
func TestAdminRoutesArePractitionerOnly(t *testing.T) {
	rig := newContentTestRig(t)
	post := createPost(t, rig, "draft", "Draft")

	cases := []struct{ method, path string }{
		{http.MethodGet, "/v1/admin/content/pages"},
		{http.MethodPost, "/v1/admin/content/pages"},
		{http.MethodGet, "/v1/admin/content/posts"},
		{http.MethodGet, "/v1/admin/content/posts/" + post.ID},
		{http.MethodPost, "/v1/admin/content/posts/" + post.ID + "/publish"},
		{http.MethodDelete, "/v1/admin/content/posts/" + post.ID},
		{http.MethodGet, "/v1/admin/content/faqs"},
		{http.MethodGet, "/v1/admin/content/testimonials"},
	}
	for _, tc := range cases {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(rig.clientToken))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as client = %d, want 403", tc.method, tc.path, rec.Code)
		}
		rec = doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}

	// The draft is still a draft: no unauthorized call published it.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/content/posts/draft", nil, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("draft is publicly readable after the unauthorized attempts (status %d)", rec.Code)
	}
}

// TestPublicRoutesNeedNoAuth: the site is read by strangers.
func TestPublicRoutesNeedNoAuth(t *testing.T) {
	rig := newContentTestRig(t)
	for _, path := range []string{
		"/v1/content/posts",
		"/v1/content/faqs",
		"/v1/content/testimonials",
	} {
		rec := doJSON(t, rig.srv, http.MethodGet, path, nil, nil)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s anonymous = %d, want 200 (body %s)", path, rec.Code, rec.Body.String())
		}
	}
}

// TestSlugConflictIs409.
func TestSlugConflictIs409(t *testing.T) {
	rig := newContentTestRig(t)
	createPost(t, rig, "first", "First")

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/posts", map[string]any{
		"slug": "first", "title": "Duplicate", "body": "body",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "slug_taken" {
		t.Errorf("code = %q, want slug_taken", errRes.Error.Code)
	}
}

// TestValidationIs400.
func TestContentValidationIs400(t *testing.T) {
	rig := newContentTestRig(t)
	post := createPost(t, rig, "first", "First")

	cases := map[string]struct {
		method, path string
		body         map[string]any
	}{
		"blank title": {http.MethodPost, "/v1/admin/content/posts", map[string]any{"slug": "x", "title": "", "body": "b"}},
		"blank slug":  {http.MethodPost, "/v1/admin/content/pages", map[string]any{"slug": "", "title": "T", "body": "b"}},
		"blank answer": {http.MethodPost, "/v1/admin/content/faqs",
			map[string]any{"question": "Q?", "answer": ""}},
		"script cover image": {http.MethodPatch, "/v1/admin/content/posts/" + post.ID,
			map[string]any{"coverImage": "javascript:alert(1)"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, tc.method, tc.path, tc.body, bearer(rig.practitionerToken))
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

// TestFAQLifecycle: created live, ordered, deactivated, deleted.
func TestFAQLifecycle(t *testing.T) {
	rig := newContentTestRig(t)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/content/faqs", map[string]any{
		"question": "Do you offer prenatal massage?", "answer": "Yes.", "sortOrder": 1,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create faq status = %d, body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		FAQ struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"faq"`
	}
	decodeBody(t, rec, &created)
	if !created.FAQ.Active {
		t.Error("a new FAQ is inactive, want it live")
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/faqs", nil, nil)
	var public struct {
		Items []faqBody `json:"items"`
	}
	decodeBody(t, rec, &public)
	if len(public.Items) != 1 {
		t.Fatalf("public faqs = %d, want 1", len(public.Items))
	}

	rec = doJSON(t, rig.srv, http.MethodPatch, "/v1/admin/content/faqs/"+created.FAQ.ID, map[string]any{
		"active": false,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch faq status = %d, body %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/content/faqs", nil, nil)
	decodeBody(t, rec, &public)
	if len(public.Items) != 0 {
		t.Errorf("public faqs = %d after deactivation, want none", len(public.Items))
	}

	rec = doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/content/faqs/"+created.FAQ.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete faq status = %d", rec.Code)
	}
	rec = doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/content/faqs/"+created.FAQ.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", rec.Code)
	}
}

// TestInvalidModerationFilterIs400.
func TestInvalidModerationFilterIs400(t *testing.T) {
	rig := newContentTestRig(t)
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/content/testimonials?status=live", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
	}
}

// TestContentUnavailableWithoutDatabase.
func TestContentUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithContent(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/content/posts"},
		{http.MethodGet, "/v1/content/pages/about"},
		{http.MethodGet, "/v1/content/faqs"},
		{http.MethodGet, "/v1/admin/content/posts"},
		{http.MethodPost, "/v1/admin/content/posts"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
