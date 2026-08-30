package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/cms"
)

// ContentFilter narrows a content listing. PublishedOnly is what separates
// a public read from a practitioner's: the repositories apply it, so a
// caller cannot forget to.
type ContentFilter struct {
	PublishedOnly bool
	// Category, when set, narrows posts or FAQs to one grouping.
	Category string
	// Tag, when set, narrows posts to one tag.
	Tag string
}

// PageRepository is the outbound port for editable site pages.
type PageRepository interface {
	Create(ctx context.Context, page cms.Page) (cms.Page, error)
	Update(ctx context.Context, page cms.Page) (cms.Page, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return cms.ErrPageNotFound.
	FindByID(ctx context.Context, id string) (cms.Page, error)
	// FindBySlug misses return cms.ErrPageNotFound. publishedOnly is the
	// public read: a draft must be indistinguishable from a missing page.
	FindBySlug(ctx context.Context, slug string, publishedOnly bool) (cms.Page, error)
	List(ctx context.Context, filter ContentFilter) ([]cms.Page, error)
}

// PostRepository is the outbound port for blog posts.
type PostRepository interface {
	Create(ctx context.Context, post cms.Post) (cms.Post, error)
	Update(ctx context.Context, post cms.Post) (cms.Post, error)
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (cms.Post, error)
	FindBySlug(ctx context.Context, slug string, publishedOnly bool) (cms.Post, error)
	// List returns posts newest-published first (drafts by creation date).
	List(ctx context.Context, filter ContentFilter) ([]cms.Post, error)
}

// FAQRepository is the outbound port for FAQ entries.
type FAQRepository interface {
	Create(ctx context.Context, faq cms.FAQ) (cms.FAQ, error)
	Update(ctx context.Context, faq cms.FAQ) (cms.FAQ, error)
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (cms.FAQ, error)
	// List returns entries in sortOrder then creation order.
	List(ctx context.Context, filter ContentFilter) ([]cms.FAQ, error)
}

// TestimonialRepository is the outbound port for testimonials.
type TestimonialRepository interface {
	Create(ctx context.Context, testimonial cms.Testimonial) (cms.Testimonial, error)
	Update(ctx context.Context, testimonial cms.Testimonial) (cms.Testimonial, error)
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (cms.Testimonial, error)
	// List returns approved-only for the public site, or the full
	// moderation queue for the practitioner.
	List(ctx context.Context, filter ContentFilter, status cms.Moderation) ([]cms.Testimonial, error)
}

// PageInput is the create payload for a page.
type PageInput struct {
	Slug       string
	Title      string
	Body       string
	CoverImage string
}

// PostInput is the create payload for a post.
type PostInput struct {
	Slug  string
	Title string
	Body  string
}

// FAQInput is the create payload for an FAQ entry.
type FAQInput struct {
	Question  string
	Answer    string
	Category  string
	SortOrder int
}

// TestimonialInput is the create payload for a testimonial.
type TestimonialInput struct {
	AuthorName string
	AuthorRole string
	Quote      string
}

// ContentService is the inbound port for the CMS slice (BE-12).
//
// Read methods come in pairs: the Public* ones serve anonymous visitors and
// can only ever return live content, while the plain ones serve the
// practitioner and see everything. Keeping them apart means a public route
// physically cannot reach a draft, rather than relying on every caller
// passing the right flag.
type ContentService interface {
	// Pages.
	PublicPage(ctx context.Context, slug string) (cms.Page, error)
	ListPages(ctx context.Context) ([]cms.Page, error)
	GetPage(ctx context.Context, id string) (cms.Page, error)
	CreatePage(ctx context.Context, in PageInput) (cms.Page, error)
	UpdatePage(ctx context.Context, id string, patch cms.PagePatch) (cms.Page, error)
	SetPagePublished(ctx context.Context, id string, published bool) (cms.Page, error)
	DeletePage(ctx context.Context, id string) error

	// Posts.
	PublicPosts(ctx context.Context, filter ContentFilter) ([]cms.Post, error)
	PublicPost(ctx context.Context, slug string) (cms.Post, error)
	ListPosts(ctx context.Context) ([]cms.Post, error)
	GetPost(ctx context.Context, id string) (cms.Post, error)
	CreatePost(ctx context.Context, in PostInput) (cms.Post, error)
	UpdatePost(ctx context.Context, id string, patch cms.PostPatch) (cms.Post, error)
	SetPostPublished(ctx context.Context, id string, published bool) (cms.Post, error)
	DeletePost(ctx context.Context, id string) error

	// FAQs.
	PublicFAQs(ctx context.Context, category string) ([]cms.FAQ, error)
	ListFAQs(ctx context.Context) ([]cms.FAQ, error)
	CreateFAQ(ctx context.Context, in FAQInput) (cms.FAQ, error)
	UpdateFAQ(ctx context.Context, id string, patch cms.FAQPatch) (cms.FAQ, error)
	DeleteFAQ(ctx context.Context, id string) error

	// Testimonials.
	PublicTestimonials(ctx context.Context) ([]cms.Testimonial, error)
	ListTestimonials(ctx context.Context, status cms.Moderation) ([]cms.Testimonial, error)
	CreateTestimonial(ctx context.Context, in TestimonialInput) (cms.Testimonial, error)
	UpdateTestimonial(ctx context.Context, id string, patch cms.TestimonialPatch) (cms.Testimonial, error)
	ModerateTestimonial(ctx context.Context, id string, approve bool) (cms.Testimonial, error)
	DeleteTestimonial(ctx context.Context, id string) error
}
