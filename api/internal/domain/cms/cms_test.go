package cms

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

// TestNothingIsPublicByDefault is the rule the whole package exists to
// enforce: newly created content is invisible until someone says otherwise.
func TestNothingIsPublicByDefault(t *testing.T) {
	page, err := NewPage("about", "About", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if page.Public() || page.Status != StatusDraft {
		t.Errorf("page = %+v, want a draft", page)
	}

	post, err := NewPost("first-post", "First post", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPost: %v", err)
	}
	if post.Public() || post.Status != StatusDraft {
		t.Errorf("post = %+v, want a draft", post)
	}

	testimonial, err := NewTestimonial("Ama", "Client", "Wonderful.", fixedNow)
	if err != nil {
		t.Fatalf("NewTestimonial: %v", err)
	}
	if testimonial.Public() || testimonial.Status != ModerationPending {
		t.Errorf("testimonial = %+v, want pending moderation", testimonial)
	}
}

// TestPublishIsIdempotentAndKeepsTheDate: re-publishing must not make the
// site claim a five-year-old article was posted today.
func TestPublishIsIdempotentAndKeepsTheDate(t *testing.T) {
	post, err := NewPost("first-post", "First post", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPost: %v", err)
	}

	post.Publish(fixedNow)
	if !post.Public() || post.PublishedAt == nil || !post.PublishedAt.Equal(fixedNow) {
		t.Fatalf("post = %+v, want published and stamped", post)
	}
	first := *post.PublishedAt

	later := fixedNow.Add(48 * time.Hour)
	post.Publish(later)
	if !post.PublishedAt.Equal(first) {
		t.Errorf("publishedAt = %v, want the original %v kept", post.PublishedAt, first)
	}

	// Unpublish keeps the historical date but takes the post off the site.
	post.Unpublish(later)
	if post.Public() {
		t.Error("post is still public after unpublishing")
	}
	if post.PublishedAt == nil || !post.PublishedAt.Equal(first) {
		t.Errorf("publishedAt = %v, want the first-live date retained", post.PublishedAt)
	}

	// Re-publishing an old post does not reset the date either.
	post.Publish(later.Add(time.Hour))
	if !post.PublishedAt.Equal(first) {
		t.Errorf("publishedAt = %v, want %v", post.PublishedAt, first)
	}
}

// TestEditingCannotPublish: the patch types have no status field, so an
// edit can never smuggle content onto the site.
func TestEditingCannotPublish(t *testing.T) {
	page, err := NewPage("about", "About", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.Apply(PagePatch{Title: ptr("About us"), Body: ptr("new body")}, fixedNow); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if page.Public() {
		t.Error("editing a draft published it")
	}
	if page.Title != "About us" || page.Body != "new body" {
		t.Errorf("page = %+v, want the patch applied", page)
	}
}

// TestModerationIsReversible: rejecting is a judgement a practitioner can
// change, and approval stamps once.
func TestModerationIsReversible(t *testing.T) {
	testimonial, err := NewTestimonial("Ama", "Client", "Wonderful.", fixedNow)
	if err != nil {
		t.Fatalf("NewTestimonial: %v", err)
	}

	testimonial.Approve(fixedNow)
	if !testimonial.Public() || testimonial.ApprovedAt == nil {
		t.Fatalf("testimonial = %+v, want approved and stamped", testimonial)
	}
	approvedAt := *testimonial.ApprovedAt

	testimonial.Reject(fixedNow.Add(time.Hour))
	if testimonial.Public() {
		t.Error("a rejected testimonial is still public")
	}

	testimonial.Approve(fixedNow.Add(2 * time.Hour))
	if !testimonial.Public() {
		t.Error("a re-approved testimonial is not public")
	}
	if !testimonial.ApprovedAt.Equal(approvedAt) {
		t.Errorf("approvedAt = %v, want the first approval %v kept", testimonial.ApprovedAt, approvedAt)
	}
}

// TestTestimonialEditCannotApprove: the patch has no status field.
func TestTestimonialEditCannotApprove(t *testing.T) {
	testimonial, err := NewTestimonial("Ama", "Client", "Wonderful.", fixedNow)
	if err != nil {
		t.Fatalf("NewTestimonial: %v", err)
	}
	if err := testimonial.Apply(TestimonialPatch{Quote: ptr("Truly wonderful.")}, fixedNow); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if testimonial.Public() {
		t.Error("editing a pending testimonial approved it")
	}
}

// TestSlugNormalization: an editor types a title, the domain produces a URL.
func TestSlugNormalization(t *testing.T) {
	for raw, want := range map[string]string{
		"About Our Approach":     "about-our-approach",
		"  Trailing  Spaces  ":   "trailing-spaces",
		"Mixed_CASE/and-slashes": "mixed-case-and-slashes",
		"--leading-hyphens--":    "leading-hyphens",
		"Séance à deux":          "seance-a-deux",
		"Crème & Öl":             "creme-ol",
		"multiple    spaces":     "multiple-spaces",
		"already-fine-2026":      "already-fine-2026",
	} {
		if got := NormalizeSlug(raw); got != want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", raw, got, want)
		}
	}
}

// TestSlugValidation: a slug that cannot address anything is refused.
func TestSlugValidation(t *testing.T) {
	if _, err := NewPage("!!!", "About", "body", fixedNow); !errors.Is(err, ErrInvalidSlug) {
		t.Errorf("punctuation-only slug err = %v, want ErrInvalidSlug", err)
	}
	if _, err := NewPage(strings.Repeat("a", MaxSlugLen+1), "About", "body", fixedNow); !errors.Is(err, ErrSlugTooLong) {
		t.Errorf("over-long slug err = %v, want ErrSlugTooLong", err)
	}
}

// TestContentValidation covers the length and requiredness rules.
func TestContentValidation(t *testing.T) {
	if _, err := NewPage("about", "   ", "body", fixedNow); !errors.Is(err, ErrInvalidTitle) {
		t.Errorf("blank title err = %v, want ErrInvalidTitle", err)
	}
	if _, err := NewPage("about", "About", strings.Repeat("x", MaxBodyLen+1), fixedNow); !errors.Is(err, ErrBodyTooLong) {
		t.Errorf("over-long body err = %v, want ErrBodyTooLong", err)
	}
	if _, err := NewFAQ("", "answer", "", 0, fixedNow); !errors.Is(err, ErrInvalidQuestion) {
		t.Errorf("blank question err = %v, want ErrInvalidQuestion", err)
	}
	if _, err := NewFAQ("question", "", "", 0, fixedNow); !errors.Is(err, ErrInvalidAnswer) {
		t.Errorf("blank answer err = %v, want ErrInvalidAnswer", err)
	}
	if _, err := NewTestimonial("", "Client", "Wonderful.", fixedNow); !errors.Is(err, ErrInvalidAuthor) {
		t.Errorf("blank author err = %v, want ErrInvalidAuthor", err)
	}
	if _, err := NewTestimonial("Ama", "Client", "", fixedNow); !errors.Is(err, ErrInvalidQuote) {
		t.Errorf("blank quote err = %v, want ErrInvalidQuote", err)
	}
}

// TestCoverImageRejectsScriptURLs: a cover image is rendered into an
// attribute on the public site, so a javascript:/data: URL stored here is
// a stored-XSS payload waiting for a trusting renderer.
func TestCoverImageRejectsScriptURLs(t *testing.T) {
	post, err := NewPost("first-post", "First post", "body", fixedNow)
	if err != nil {
		t.Fatalf("NewPost: %v", err)
	}

	for _, bad := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:text/html;base64,PHNjcmlwdD4=",
		"vbscript:msgbox(1)",
	} {
		if err := post.Apply(PostPatch{CoverImage: ptr(bad)}, fixedNow); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("coverImage %q err = %v, want ErrInvalidURL", bad, err)
		}
	}

	for _, good := range []string{
		"https://res.cloudinary.com/terios/image/upload/v1/cover.jpg",
		"http://example.com/cover.jpg",
		"/images/cover.jpg",
		"",
	} {
		if err := post.Apply(PostPatch{CoverImage: ptr(good)}, fixedNow); err != nil {
			t.Errorf("coverImage %q err = %v, want it accepted", good, err)
		}
	}
}

// TestTagNormalization: trimmed, deduplicated, and bounded.
func TestTagNormalization(t *testing.T) {
	tags, err := NormalizeTags([]string{" massage ", "massage", "", "wellness"})
	if err != nil {
		t.Fatalf("NormalizeTags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "massage" || tags[1] != "wellness" {
		t.Errorf("tags = %v, want [massage wellness]", tags)
	}

	many := make([]string, MaxTags+1)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	if _, err := NormalizeTags(many); !errors.Is(err, ErrTooManyTags) {
		t.Errorf("err = %v, want ErrTooManyTags", err)
	}
	if _, err := NormalizeTags([]string{strings.Repeat("x", MaxTagLen+1)}); !errors.Is(err, ErrTagTooLong) {
		t.Errorf("err = %v, want ErrTagTooLong", err)
	}
}

// TestFAQIsLiveButDeactivatable: practice-authored entries need no review,
// but can be taken down.
func TestFAQIsLiveButDeactivatable(t *testing.T) {
	faq, err := NewFAQ("Do you offer prenatal massage?", "Yes.", "Services", 1, fixedNow)
	if err != nil {
		t.Fatalf("NewFAQ: %v", err)
	}
	if !faq.Public() {
		t.Error("a new FAQ is not public, want it live")
	}
	if err := faq.Apply(FAQPatch{Active: ptr(false)}, fixedNow); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if faq.Public() {
		t.Error("a deactivated FAQ is still public")
	}
}

// TestStatusValidity guards the enum against typos entering from storage.
func TestStatusValidity(t *testing.T) {
	if !StatusDraft.Valid() || !StatusPublished.Valid() || Status("live").Valid() {
		t.Error("Status.Valid does not match the known set")
	}
	if !ModerationPending.Valid() || !ModerationApproved.Valid() || !ModerationRejected.Valid() || Moderation("ok").Valid() {
		t.Error("Moderation.Valid does not match the known set")
	}
}
