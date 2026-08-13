package cms

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

type testRig struct {
	svc          *Service
	pages        *portstest.FakePageRepository
	posts        *portstest.FakePostRepository
	faqs         *portstest.FakeFAQRepository
	testimonials *portstest.FakeTestimonialRepository
}

func newTestRig() testRig {
	rig := testRig{
		pages:        portstest.NewFakePageRepository(),
		posts:        portstest.NewFakePostRepository(),
		faqs:         portstest.NewFakeFAQRepository(),
		testimonials: portstest.NewFakeTestimonialRepository(),
	}
	rig.svc = NewService(rig.pages, rig.posts, rig.faqs, rig.testimonials)
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

func ctx() context.Context { return context.Background() }

// TestDraftsAreInvisibleToThePublic is the slice's central guarantee: an
// unpublished page, post, or testimonial is reported as not existing, not
// as forbidden — its existence is itself unreleased information.
func TestDraftsAreInvisibleToThePublic(t *testing.T) {
	rig := newTestRig()

	page, err := rig.svc.CreatePage(ctx(), ports.PageInput{Slug: "about", Title: "About", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if _, err := rig.svc.PublicPage(ctx(), "about"); !errors.Is(err, domain.ErrPageNotFound) {
		t.Errorf("public read of a draft page = %v, want ErrPageNotFound", err)
	}

	post, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: "first", Title: "First", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if _, err := rig.svc.PublicPost(ctx(), "first"); !errors.Is(err, domain.ErrPostNotFound) {
		t.Errorf("public read of a draft post = %v, want ErrPostNotFound", err)
	}
	posts, err := rig.svc.PublicPosts(ctx(), ports.ContentFilter{})
	if err != nil {
		t.Fatalf("PublicPosts: %v", err)
	}
	if len(posts) != 0 {
		t.Errorf("public feed = %d posts, want none while all are drafts", len(posts))
	}

	if _, err := rig.svc.CreateTestimonial(ctx(), ports.TestimonialInput{
		AuthorName: "Ama", Quote: "Wonderful.",
	}); err != nil {
		t.Fatalf("CreateTestimonial: %v", err)
	}
	approved, err := rig.svc.PublicTestimonials(ctx())
	if err != nil {
		t.Fatalf("PublicTestimonials: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("public testimonials = %d, want none before moderation", len(approved))
	}

	// The practitioner sees all of it.
	allPages, err := rig.svc.ListPages(ctx())
	if err != nil {
		t.Fatalf("ListPages: %v", err)
	}
	if len(allPages) != 1 || allPages[0].ID != page.ID {
		t.Errorf("practitioner page list = %+v, want the draft included", allPages)
	}
	allPosts, err := rig.svc.ListPosts(ctx())
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if len(allPosts) != 1 || allPosts[0].ID != post.ID {
		t.Errorf("practitioner post list = %+v, want the draft included", allPosts)
	}
}

// TestPublishThenUnpublishFlipsVisibility.
func TestPublishThenUnpublishFlipsVisibility(t *testing.T) {
	rig := newTestRig()
	post, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: "first", Title: "First", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	if _, err := rig.svc.SetPostPublished(ctx(), post.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}
	live, err := rig.svc.PublicPost(ctx(), "first")
	if err != nil {
		t.Fatalf("PublicPost after publish: %v", err)
	}
	if live.ID != post.ID || live.PublishedAt == nil {
		t.Errorf("post = %+v, want live and stamped", live)
	}

	if _, err := rig.svc.SetPostPublished(ctx(), post.ID, false); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if _, err := rig.svc.PublicPost(ctx(), "first"); !errors.Is(err, domain.ErrPostNotFound) {
		t.Errorf("public read after unpublish = %v, want ErrPostNotFound", err)
	}
}

// TestPublicListsFailClosed: even if storage hands back an unpublished
// record, the service refuses to pass it on. The repository filter and this
// check are two independent locks on the same door.
func TestPublicListsFailClosed(t *testing.T) {
	rig := newTestRig()

	// Insert a draft straight into storage, bypassing the query filter the
	// public read would normally rely on.
	draft, err := domain.NewPost("leaked", "Leaked draft", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPost: %v", err)
	}
	if _, err := rig.posts.Create(ctx(), draft); err != nil {
		t.Fatalf("seed draft: %v", err)
	}

	// A repository that ignores the filter is exactly the bug this guards.
	posts, err := rig.svc.PublicPosts(ctx(), ports.ContentFilter{})
	if err != nil {
		t.Fatalf("PublicPosts: %v", err)
	}
	for _, p := range posts {
		if p.Status != domain.StatusPublished {
			t.Errorf("unpublished post %q reached a public listing", p.Slug)
		}
	}

	pending, err := domain.NewTestimonial("Ghost", "", "Never approved.", fixedNow)
	if err != nil {
		t.Fatalf("NewTestimonial: %v", err)
	}
	if _, err := rig.testimonials.Create(ctx(), pending); err != nil {
		t.Fatalf("seed testimonial: %v", err)
	}
	items, err := rig.svc.PublicTestimonials(ctx())
	if err != nil {
		t.Fatalf("PublicTestimonials: %v", err)
	}
	for _, item := range items {
		if item.Status != domain.ModerationApproved {
			t.Errorf("unapproved testimonial by %q reached the public list", item.AuthorName)
		}
	}
}

// TestEditingDoesNotPublish: an update on a live post keeps it live, and an
// update on a draft leaves it a draft.
func TestEditingDoesNotPublish(t *testing.T) {
	rig := newTestRig()
	post, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: "first", Title: "First", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	edited, err := rig.svc.UpdatePost(ctx(), post.ID, domain.PostPatch{Title: ptr("First, revised")})
	if err != nil {
		t.Fatalf("UpdatePost: %v", err)
	}
	if edited.Status != domain.StatusDraft {
		t.Errorf("status = %q after editing a draft, want it still draft", edited.Status)
	}

	if _, err := rig.svc.SetPostPublished(ctx(), post.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}
	edited, err = rig.svc.UpdatePost(ctx(), post.ID, domain.PostPatch{Title: ptr("First, revised again")})
	if err != nil {
		t.Fatalf("UpdatePost: %v", err)
	}
	if edited.Status != domain.StatusPublished {
		t.Errorf("status = %q after editing a live post, want it still published", edited.Status)
	}
	if edited.Title != "First, revised again" {
		t.Errorf("title = %q, want the edit applied", edited.Title)
	}
}

// TestSlugsAreUniqueAndNormalized: two pages cannot claim one URL, and the
// URL is derived consistently however the editor typed it.
func TestSlugsAreUniqueAndNormalized(t *testing.T) {
	rig := newTestRig()

	page, err := rig.svc.CreatePage(ctx(), ports.PageInput{Slug: "About Our Approach", Title: "About", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if page.Slug != "about-our-approach" {
		t.Errorf("slug = %q, want the normalized form", page.Slug)
	}

	if _, err := rig.svc.CreatePage(ctx(), ports.PageInput{
		Slug: "about-our-approach", Title: "Duplicate", Body: "body",
	}); !errors.Is(err, domain.ErrSlugTaken) {
		t.Errorf("duplicate slug err = %v, want ErrSlugTaken", err)
	}

	// A public read finds it however the caller cased the URL.
	if _, err := rig.svc.SetPagePublished(ctx(), page.ID, true); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := rig.svc.PublicPage(ctx(), "About Our Approach"); err != nil {
		t.Errorf("public read by un-normalized slug = %v, want it found", err)
	}
}

// TestModerationQueue: the practitioner filters by state, and approval is
// what puts a quote on the site.
func TestModerationQueue(t *testing.T) {
	rig := newTestRig()

	keep, err := rig.svc.CreateTestimonial(ctx(), ports.TestimonialInput{AuthorName: "Ama", Quote: "Wonderful."})
	if err != nil {
		t.Fatalf("CreateTestimonial: %v", err)
	}
	drop, err := rig.svc.CreateTestimonial(ctx(), ports.TestimonialInput{AuthorName: "Spam", Quote: "Buy now."})
	if err != nil {
		t.Fatalf("CreateTestimonial: %v", err)
	}

	pending, err := rig.svc.ListTestimonials(ctx(), domain.ModerationPending)
	if err != nil {
		t.Fatalf("ListTestimonials: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("pending queue = %d, want both", len(pending))
	}

	if _, err := rig.svc.ModerateTestimonial(ctx(), keep.ID, true); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := rig.svc.ModerateTestimonial(ctx(), drop.ID, false); err != nil {
		t.Fatalf("reject: %v", err)
	}

	public, err := rig.svc.PublicTestimonials(ctx())
	if err != nil {
		t.Fatalf("PublicTestimonials: %v", err)
	}
	if len(public) != 1 || public[0].ID != keep.ID {
		t.Errorf("public testimonials = %+v, want only the approved one", public)
	}

	remaining, err := rig.svc.ListTestimonials(ctx(), domain.ModerationPending)
	if err != nil {
		t.Fatalf("ListTestimonials: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("pending queue = %d, want it cleared", len(remaining))
	}

	all, err := rig.svc.ListTestimonials(ctx(), "")
	if err != nil {
		t.Fatalf("ListTestimonials(all): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("full list = %d, want both regardless of state", len(all))
	}
}

// TestFAQVisibilityAndOrder: entries are live on creation, ordered by
// sortOrder, and deactivation takes one off the site.
func TestFAQVisibilityAndOrder(t *testing.T) {
	rig := newTestRig()

	second, err := rig.svc.CreateFAQ(ctx(), ports.FAQInput{Question: "Second?", Answer: "Yes.", SortOrder: 2})
	if err != nil {
		t.Fatalf("CreateFAQ: %v", err)
	}
	if _, err := rig.svc.CreateFAQ(ctx(), ports.FAQInput{Question: "First?", Answer: "Yes.", SortOrder: 1}); err != nil {
		t.Fatalf("CreateFAQ: %v", err)
	}

	public, err := rig.svc.PublicFAQs(ctx(), "")
	if err != nil {
		t.Fatalf("PublicFAQs: %v", err)
	}
	if len(public) != 2 || public[0].Question != "First?" {
		t.Errorf("faqs = %+v, want sortOrder order", public)
	}

	if _, err := rig.svc.UpdateFAQ(ctx(), second.ID, domain.FAQPatch{Active: ptr(false)}); err != nil {
		t.Fatalf("UpdateFAQ: %v", err)
	}
	public, err = rig.svc.PublicFAQs(ctx(), "")
	if err != nil {
		t.Fatalf("PublicFAQs: %v", err)
	}
	if len(public) != 1 {
		t.Errorf("faqs = %d, want the deactivated entry hidden", len(public))
	}

	// The practitioner still sees it.
	all, err := rig.svc.ListFAQs(ctx())
	if err != nil {
		t.Fatalf("ListFAQs: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("practitioner faq list = %d, want the inactive entry included", len(all))
	}
}

// TestPostFeedFiltersAndOrders: newest first, narrowed by category and tag.
func TestPostFeedFiltersAndOrders(t *testing.T) {
	rig := newTestRig()

	seed := func(slug, title, category string, tags []string, publishedAt time.Time) {
		post, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: slug, Title: title, Body: "body"})
		if err != nil {
			t.Fatalf("CreatePost: %v", err)
		}
		if _, err := rig.svc.UpdatePost(ctx(), post.ID, domain.PostPatch{
			Category: ptr(category), Tags: &tags,
		}); err != nil {
			t.Fatalf("UpdatePost: %v", err)
		}
		rig.svc.now = func() time.Time { return publishedAt }
		if _, err := rig.svc.SetPostPublished(ctx(), post.ID, true); err != nil {
			t.Fatalf("publish: %v", err)
		}
		rig.svc.now = func() time.Time { return fixedNow }
	}

	seed("older", "Older", "wellness", []string{"massage"}, fixedNow)
	seed("newer", "Newer", "wellness", []string{"breathing"}, fixedNow.Add(48*time.Hour))
	seed("other", "Other", "studio", []string{"massage"}, fixedNow.Add(24*time.Hour))

	feed, err := rig.svc.PublicPosts(ctx(), ports.ContentFilter{})
	if err != nil {
		t.Fatalf("PublicPosts: %v", err)
	}
	if len(feed) != 3 || feed[0].Slug != "newer" || feed[2].Slug != "older" {
		t.Errorf("feed order = %v, want newest published first", slugs(feed))
	}

	byCategory, err := rig.svc.PublicPosts(ctx(), ports.ContentFilter{Category: "wellness"})
	if err != nil {
		t.Fatalf("PublicPosts(category): %v", err)
	}
	if len(byCategory) != 2 {
		t.Errorf("category feed = %v, want the two wellness posts", slugs(byCategory))
	}

	byTag, err := rig.svc.PublicPosts(ctx(), ports.ContentFilter{Tag: "massage"})
	if err != nil {
		t.Fatalf("PublicPosts(tag): %v", err)
	}
	if len(byTag) != 2 {
		t.Errorf("tag feed = %v, want the two massage posts", slugs(byTag))
	}
}

func slugs(posts []domain.Post) []string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.Slug)
	}
	return out
}

// TestDeleteRequiresExistence: deleting something that is not there is a
// not-found, not a silent success — the admin UI needs to know.
func TestDeleteRequiresExistence(t *testing.T) {
	rig := newTestRig()

	if err := rig.svc.DeletePage(ctx(), "no-such-page"); !errors.Is(err, domain.ErrPageNotFound) {
		t.Errorf("DeletePage = %v, want ErrPageNotFound", err)
	}
	if err := rig.svc.DeletePost(ctx(), "no-such-post"); !errors.Is(err, domain.ErrPostNotFound) {
		t.Errorf("DeletePost = %v, want ErrPostNotFound", err)
	}
	if err := rig.svc.DeleteFAQ(ctx(), "no-such-faq"); !errors.Is(err, domain.ErrFAQNotFound) {
		t.Errorf("DeleteFAQ = %v, want ErrFAQNotFound", err)
	}
	if err := rig.svc.DeleteTestimonial(ctx(), "no-such-testimonial"); !errors.Is(err, domain.ErrTestimonialNotFound) {
		t.Errorf("DeleteTestimonial = %v, want ErrTestimonialNotFound", err)
	}

	page, err := rig.svc.CreatePage(ctx(), ports.PageInput{Slug: "about", Title: "About", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if err := rig.svc.DeletePage(ctx(), page.ID); err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if _, err := rig.svc.GetPage(ctx(), page.ID); !errors.Is(err, domain.ErrPageNotFound) {
		t.Errorf("GetPage after delete = %v, want ErrPageNotFound", err)
	}
}

// TestValidationSurfaces: bad content is rejected by the service, not
// stored and rejected later.
func TestValidationSurfaces(t *testing.T) {
	rig := newTestRig()

	if _, err := rig.svc.CreatePage(ctx(), ports.PageInput{Slug: "", Title: "About", Body: "b"}); !errors.Is(err, domain.ErrInvalidSlug) {
		t.Errorf("empty slug err = %v, want ErrInvalidSlug", err)
	}
	if _, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: "ok", Title: "", Body: "b"}); !errors.Is(err, domain.ErrInvalidTitle) {
		t.Errorf("empty title err = %v, want ErrInvalidTitle", err)
	}
	if _, err := rig.svc.CreateFAQ(ctx(), ports.FAQInput{Question: "q", Answer: ""}); !errors.Is(err, domain.ErrInvalidAnswer) {
		t.Errorf("empty answer err = %v, want ErrInvalidAnswer", err)
	}
	if _, err := rig.svc.CreateTestimonial(ctx(), ports.TestimonialInput{AuthorName: "Ama", Quote: ""}); !errors.Is(err, domain.ErrInvalidQuote) {
		t.Errorf("empty quote err = %v, want ErrInvalidQuote", err)
	}

	post, err := rig.svc.CreatePost(ctx(), ports.PostInput{Slug: "ok", Title: "Ok", Body: "b"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if _, err := rig.svc.UpdatePost(ctx(), post.ID, domain.PostPatch{
		CoverImage: ptr("javascript:alert(1)"),
	}); !errors.Is(err, domain.ErrInvalidURL) {
		t.Errorf("script-scheme cover image err = %v, want ErrInvalidURL", err)
	}
}
