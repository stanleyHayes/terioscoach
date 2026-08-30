// Package cms is the application service for the site-content slice. It
// implements the inbound ports.ContentService port purely against outbound
// ports — no framework, driver, or transport imports.
//
// The slice's one job beyond CRUD is keeping unpublished work off the
// public site. That is enforced twice: the Public* use cases ask their
// repository for published records only, and the domain's Public() is the
// final say on any single record. A draft is reported as not-found rather
// than forbidden — an unreleased article's existence is itself private.
package cms

import (
	"context"
	"time"

	domain "github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the CMS use cases over outbound ports.
type Service struct {
	pages        ports.PageRepository
	posts        ports.PostRepository
	faqs         ports.FAQRepository
	testimonials ports.TestimonialRepository
	now          func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.ContentService = (*Service)(nil)

// NewService wires the use cases to their outbound ports.
func NewService(
	pages ports.PageRepository,
	posts ports.PostRepository,
	faqs ports.FAQRepository,
	testimonials ports.TestimonialRepository,
) *Service {
	return &Service{
		pages:        pages,
		posts:        posts,
		faqs:         faqs,
		testimonials: testimonials,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// ---- Pages ----

// PublicPage serves an anonymous visitor. A draft answers not-found.
func (s *Service) PublicPage(ctx context.Context, slug string) (domain.Page, error) {
	page, err := s.pages.FindBySlug(ctx, domain.NormalizeSlug(slug), true)
	if err != nil {
		return domain.Page{}, err
	}
	if !page.Public() {
		return domain.Page{}, domain.ErrPageNotFound
	}
	return page, nil
}

// ListPages returns every page, drafts included — practitioner-only.
func (s *Service) ListPages(ctx context.Context) ([]domain.Page, error) {
	return s.pages.List(ctx, ports.ContentFilter{})
}

// GetPage returns one page by id, draft or not — practitioner-only.
func (s *Service) GetPage(ctx context.Context, id string) (domain.Page, error) {
	return s.pages.FindByID(ctx, id)
}

// CreatePage adds a draft page.
func (s *Service) CreatePage(ctx context.Context, in ports.PageInput) (domain.Page, error) {
	page, err := domain.NewPage(in.Slug, in.Title, in.Body, s.now())
	if err != nil {
		return domain.Page{}, err
	}
	if err := page.Apply(domain.PagePatch{CoverImage: &in.CoverImage}, s.now()); err != nil {
		return domain.Page{}, err
	}
	return s.pages.Create(ctx, page)
}

// UpdatePage applies an edit. It cannot publish: PagePatch has no status.
func (s *Service) UpdatePage(ctx context.Context, id string, patch domain.PagePatch) (domain.Page, error) {
	page, err := s.pages.FindByID(ctx, id)
	if err != nil {
		return domain.Page{}, err
	}
	if err := page.Apply(patch, s.now()); err != nil {
		return domain.Page{}, err
	}
	return s.pages.Update(ctx, page)
}

// SetPagePublished is the explicit publish/unpublish transition.
func (s *Service) SetPagePublished(ctx context.Context, id string, published bool) (domain.Page, error) {
	page, err := s.pages.FindByID(ctx, id)
	if err != nil {
		return domain.Page{}, err
	}
	if published {
		page.Publish(s.now())
	} else {
		page.Unpublish(s.now())
	}
	return s.pages.Update(ctx, page)
}

// DeletePage removes a page outright.
func (s *Service) DeletePage(ctx context.Context, id string) error {
	if _, err := s.pages.FindByID(ctx, id); err != nil {
		return err
	}
	return s.pages.Delete(ctx, id)
}

// ---- Posts ----

// PublicPosts serves the blog feed: published posts only, newest first.
func (s *Service) PublicPosts(ctx context.Context, filter ports.ContentFilter) ([]domain.Post, error) {
	filter.PublishedOnly = true
	posts, err := s.posts.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	return keepPublic(posts, func(p domain.Post) bool { return p.Public() }), nil
}

// PublicPost serves one article. A draft answers not-found.
func (s *Service) PublicPost(ctx context.Context, slug string) (domain.Post, error) {
	post, err := s.posts.FindBySlug(ctx, domain.NormalizeSlug(slug), true)
	if err != nil {
		return domain.Post{}, err
	}
	if !post.Public() {
		return domain.Post{}, domain.ErrPostNotFound
	}
	return post, nil
}

// ListPosts returns every post, drafts included — practitioner-only.
func (s *Service) ListPosts(ctx context.Context) ([]domain.Post, error) {
	return s.posts.List(ctx, ports.ContentFilter{})
}

// GetPost returns one post by id, draft or not — practitioner-only.
func (s *Service) GetPost(ctx context.Context, id string) (domain.Post, error) {
	return s.posts.FindByID(ctx, id)
}

// CreatePost adds a draft article.
func (s *Service) CreatePost(ctx context.Context, in ports.PostInput) (domain.Post, error) {
	post, err := domain.NewPost(in.Slug, in.Title, in.Body, s.now())
	if err != nil {
		return domain.Post{}, err
	}
	return s.posts.Create(ctx, post)
}

// UpdatePost applies an edit.
func (s *Service) UpdatePost(ctx context.Context, id string, patch domain.PostPatch) (domain.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return domain.Post{}, err
	}
	if err := post.Apply(patch, s.now()); err != nil {
		return domain.Post{}, err
	}
	return s.posts.Update(ctx, post)
}

// SetPostPublished is the explicit publish/unpublish transition.
func (s *Service) SetPostPublished(ctx context.Context, id string, published bool) (domain.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return domain.Post{}, err
	}
	if published {
		post.Publish(s.now())
	} else {
		post.Unpublish(s.now())
	}
	return s.posts.Update(ctx, post)
}

// DeletePost removes an article outright.
func (s *Service) DeletePost(ctx context.Context, id string) error {
	if _, err := s.posts.FindByID(ctx, id); err != nil {
		return err
	}
	return s.posts.Delete(ctx, id)
}

// ---- FAQs ----

// PublicFAQs serves active entries in their admin-controlled order.
func (s *Service) PublicFAQs(ctx context.Context, category string) ([]domain.FAQ, error) {
	faqs, err := s.faqs.List(ctx, ports.ContentFilter{PublishedOnly: true, Category: category})
	if err != nil {
		return nil, err
	}
	return keepPublic(faqs, func(f domain.FAQ) bool { return f.Public() }), nil
}

// ListFAQs returns every entry, inactive included — practitioner-only.
func (s *Service) ListFAQs(ctx context.Context) ([]domain.FAQ, error) {
	return s.faqs.List(ctx, ports.ContentFilter{})
}

// CreateFAQ adds an entry.
func (s *Service) CreateFAQ(ctx context.Context, in ports.FAQInput) (domain.FAQ, error) {
	faq, err := domain.NewFAQ(in.Question, in.Answer, in.Category, in.SortOrder, s.now())
	if err != nil {
		return domain.FAQ{}, err
	}
	return s.faqs.Create(ctx, faq)
}

// UpdateFAQ applies an edit, including activation.
func (s *Service) UpdateFAQ(ctx context.Context, id string, patch domain.FAQPatch) (domain.FAQ, error) {
	faq, err := s.faqs.FindByID(ctx, id)
	if err != nil {
		return domain.FAQ{}, err
	}
	if err := faq.Apply(patch, s.now()); err != nil {
		return domain.FAQ{}, err
	}
	return s.faqs.Update(ctx, faq)
}

// DeleteFAQ removes an entry outright.
func (s *Service) DeleteFAQ(ctx context.Context, id string) error {
	if _, err := s.faqs.FindByID(ctx, id); err != nil {
		return err
	}
	return s.faqs.Delete(ctx, id)
}

// ---- Testimonials ----

// PublicTestimonials serves approved quotes only.
func (s *Service) PublicTestimonials(ctx context.Context) ([]domain.Testimonial, error) {
	items, err := s.testimonials.List(ctx, ports.ContentFilter{PublishedOnly: true}, domain.ModerationApproved)
	if err != nil {
		return nil, err
	}
	return keepPublic(items, func(t domain.Testimonial) bool { return t.Public() }), nil
}

// ListTestimonials returns the moderation queue. An empty status returns
// every testimonial regardless of state.
func (s *Service) ListTestimonials(ctx context.Context, status domain.Moderation) ([]domain.Testimonial, error) {
	return s.testimonials.List(ctx, ports.ContentFilter{}, status)
}

// CreateTestimonial adds a pending quote.
func (s *Service) CreateTestimonial(ctx context.Context, in ports.TestimonialInput) (domain.Testimonial, error) {
	testimonial, err := domain.NewTestimonial(in.AuthorName, in.AuthorRole, in.Quote, s.now())
	if err != nil {
		return domain.Testimonial{}, err
	}
	return s.testimonials.Create(ctx, testimonial)
}

// UpdateTestimonial applies an edit. It cannot approve.
func (s *Service) UpdateTestimonial(ctx context.Context, id string, patch domain.TestimonialPatch) (domain.Testimonial, error) {
	testimonial, err := s.testimonials.FindByID(ctx, id)
	if err != nil {
		return domain.Testimonial{}, err
	}
	if err := testimonial.Apply(patch, s.now()); err != nil {
		return domain.Testimonial{}, err
	}
	return s.testimonials.Update(ctx, testimonial)
}

// ModerateTestimonial approves or rejects a quote — the approve-before-
// publish gate.
func (s *Service) ModerateTestimonial(ctx context.Context, id string, approve bool) (domain.Testimonial, error) {
	testimonial, err := s.testimonials.FindByID(ctx, id)
	if err != nil {
		return domain.Testimonial{}, err
	}
	if approve {
		testimonial.Approve(s.now())
	} else {
		testimonial.Reject(s.now())
	}
	return s.testimonials.Update(ctx, testimonial)
}

// DeleteTestimonial removes a quote outright.
func (s *Service) DeleteTestimonial(ctx context.Context, id string) error {
	if _, err := s.testimonials.FindByID(ctx, id); err != nil {
		return err
	}
	return s.testimonials.Delete(ctx, id)
}

// keepPublic is the belt-and-braces filter on a public listing. The
// repository has already narrowed the query; this makes a repository bug
// or a stale record fail closed — nothing unpublished leaves the service,
// whatever storage returned.
func keepPublic[T any](items []T, public func(T) bool) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if public(item) {
			out = append(out, item)
		}
	}
	return out
}
