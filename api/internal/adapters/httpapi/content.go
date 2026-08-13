package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithContent mounts the CMS routes backed by the content port (BE-12).
//
// The surface splits in two on purpose. /v1/content/* is public and reaches
// only the Public* use cases, which cannot return a draft or an unapproved
// quote. /v1/admin/content/* is practitioner-only and sees everything. The
// separation is structural: no flag on a shared handler decides whether a
// visitor may see unpublished work.
//
// A nil service keeps the routes mounted but answering 503.
func WithContent(svc ports.ContentService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/content", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleContentUnavailable)
				return
			}
			h := &contentHandler{svc: svc}
			r.Get("/pages/{slug}", h.publicPage)
			r.Get("/posts", h.publicPosts)
			r.Get("/posts/{slug}", h.publicPost)
			r.Get("/faqs", h.publicFAQs)
			r.Get("/testimonials", h.publicTestimonials)
		})

		s.Router.Route("/v1/admin/content", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleContentUnavailable)
				return
			}
			h := &contentHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))

			r.Get("/pages", h.listPages)
			r.Post("/pages", h.createPage)
			r.Get("/pages/{id}", h.getPage)
			r.Patch("/pages/{id}", h.updatePage)
			r.Post("/pages/{id}/publish", h.publishPage)
			r.Post("/pages/{id}/unpublish", h.unpublishPage)
			r.Delete("/pages/{id}", h.deletePage)

			r.Get("/posts", h.listPosts)
			r.Post("/posts", h.createPost)
			r.Get("/posts/{id}", h.getPost)
			r.Patch("/posts/{id}", h.updatePost)
			r.Post("/posts/{id}/publish", h.publishPost)
			r.Post("/posts/{id}/unpublish", h.unpublishPost)
			r.Delete("/posts/{id}", h.deletePost)

			r.Get("/faqs", h.listFAQs)
			r.Post("/faqs", h.createFAQ)
			r.Patch("/faqs/{id}", h.updateFAQ)
			r.Delete("/faqs/{id}", h.deleteFAQ)

			r.Get("/testimonials", h.listTestimonials)
			r.Post("/testimonials", h.createTestimonial)
			r.Patch("/testimonials/{id}", h.updateTestimonial)
			r.Post("/testimonials/{id}/approve", h.approveTestimonial)
			r.Post("/testimonials/{id}/reject", h.rejectTestimonial)
			r.Delete("/testimonials/{id}", h.deleteTestimonial)
		})
	}
}

// handleContentUnavailable answers every CMS route when the database — and
// therefore the content service — is not configured.
func handleContentUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "site content is unavailable: database not connected")
}

type contentHandler struct {
	svc ports.ContentService
}

// ---- Response shapes ----

type pageBody struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	MetaTitle   string     `json:"metaTitle,omitempty"`
	MetaDesc    string     `json:"metaDescription,omitempty"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func newPageBody(p cms.Page) pageBody {
	return pageBody{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Body:        p.Body,
		MetaTitle:   p.MetaTitle,
		MetaDesc:    p.MetaDesc,
		Status:      string(p.Status),
		PublishedAt: p.PublishedAt,
		CreatedAt:   p.CreatedAt.UTC(),
		UpdatedAt:   p.UpdatedAt.UTC(),
	}
}

type postBody struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt,omitempty"`
	Body        string     `json:"body"`
	CoverImage  string     `json:"coverImage,omitempty"`
	Category    string     `json:"category,omitempty"`
	Tags        []string   `json:"tags"`
	MetaTitle   string     `json:"metaTitle,omitempty"`
	MetaDesc    string     `json:"metaDescription,omitempty"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func newPostBody(p cms.Post) postBody {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return postBody{
		ID:          p.ID,
		Slug:        p.Slug,
		Title:       p.Title,
		Excerpt:     p.Excerpt,
		Body:        p.Body,
		CoverImage:  p.CoverImage,
		Category:    p.Category,
		Tags:        tags,
		MetaTitle:   p.MetaTitle,
		MetaDesc:    p.MetaDesc,
		Status:      string(p.Status),
		PublishedAt: p.PublishedAt,
		CreatedAt:   p.CreatedAt.UTC(),
		UpdatedAt:   p.UpdatedAt.UTC(),
	}
}

type faqBody struct {
	ID        string    `json:"id"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Category  string    `json:"category,omitempty"`
	SortOrder int       `json:"sortOrder"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func newFAQBody(f cms.FAQ) faqBody {
	return faqBody{
		ID:        f.ID,
		Question:  f.Question,
		Answer:    f.Answer,
		Category:  f.Category,
		SortOrder: f.SortOrder,
		Active:    f.Active,
		CreatedAt: f.CreatedAt.UTC(),
		UpdatedAt: f.UpdatedAt.UTC(),
	}
}

type testimonialBody struct {
	ID          string     `json:"id"`
	AuthorName  string     `json:"authorName"`
	AuthorRole  string     `json:"authorRole,omitempty"`
	Quote       string     `json:"quote"`
	Status      string     `json:"status"`
	SortOrder   int        `json:"sortOrder"`
	SubmittedAt time.Time  `json:"submittedAt"`
	ApprovedAt  *time.Time `json:"approvedAt,omitempty"`
}

func newTestimonialBody(t cms.Testimonial) testimonialBody {
	return testimonialBody{
		ID:          t.ID,
		AuthorName:  t.AuthorName,
		AuthorRole:  t.AuthorRole,
		Quote:       t.Quote,
		Status:      string(t.Status),
		SortOrder:   t.SortOrder,
		SubmittedAt: t.SubmittedAt.UTC(),
		ApprovedAt:  t.ApprovedAt,
	}
}

// publicTestimonialBody is what an anonymous visitor sees: the quote and
// who said it. The moderation state is deliberately absent — the public
// list contains only approved quotes, so publishing the field would say
// nothing while inviting a client to depend on it.
type publicTestimonialBody struct {
	ID         string `json:"id"`
	AuthorName string `json:"authorName"`
	AuthorRole string `json:"authorRole,omitempty"`
	Quote      string `json:"quote"`
}

// ---- Public routes ----

func (h *contentHandler) publicPage(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.PublicPage(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]pageBody{"page": newPageBody(page)})
}

func (h *contentHandler) publicPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := h.svc.PublicPosts(r.Context(), ports.ContentFilter{
		Category: r.URL.Query().Get("category"),
		Tag:      r.URL.Query().Get("tag"),
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]postBody, 0, len(posts))
	for _, post := range posts {
		// The feed carries the summary, not the article: a listing that
		// ships every body would grow without bound as the blog does.
		body := newPostBody(post)
		body.Body = ""
		items = append(items, body)
	}
	writeJSON(w, http.StatusOK, map[string][]postBody{"items": items})
}

func (h *contentHandler) publicPost(w http.ResponseWriter, r *http.Request) {
	post, err := h.svc.PublicPost(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]postBody{"post": newPostBody(post)})
}

func (h *contentHandler) publicFAQs(w http.ResponseWriter, r *http.Request) {
	faqs, err := h.svc.PublicFAQs(r.Context(), r.URL.Query().Get("category"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]faqBody, 0, len(faqs))
	for _, faq := range faqs {
		items = append(items, newFAQBody(faq))
	}
	writeJSON(w, http.StatusOK, map[string][]faqBody{"items": items})
}

func (h *contentHandler) publicTestimonials(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.PublicTestimonials(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]publicTestimonialBody, 0, len(items))
	for _, t := range items {
		out = append(out, publicTestimonialBody{
			ID:         t.ID,
			AuthorName: t.AuthorName,
			AuthorRole: t.AuthorRole,
			Quote:      t.Quote,
		})
	}
	writeJSON(w, http.StatusOK, map[string][]publicTestimonialBody{"items": out})
}

// ---- Admin: pages ----

func (h *contentHandler) listPages(w http.ResponseWriter, r *http.Request) {
	pages, err := h.svc.ListPages(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]pageBody, 0, len(pages))
	for _, page := range pages {
		items = append(items, newPageBody(page))
	}
	writeJSON(w, http.StatusOK, map[string][]pageBody{"items": items})
}

func (h *contentHandler) getPage(w http.ResponseWriter, r *http.Request) {
	page, err := h.svc.GetPage(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]pageBody{"page": newPageBody(page)})
}

func (h *contentHandler) createPage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	page, err := h.svc.CreatePage(r.Context(), ports.PageInput{Slug: req.Slug, Title: req.Title, Body: req.Body})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]pageBody{"page": newPageBody(page)})
}

func (h *contentHandler) updatePage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug      *string `json:"slug"`
		Title     *string `json:"title"`
		Body      *string `json:"body"`
		MetaTitle *string `json:"metaTitle"`
		MetaDesc  *string `json:"metaDescription"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	page, err := h.svc.UpdatePage(r.Context(), chi.URLParam(r, "id"), cms.PagePatch{
		Slug:      req.Slug,
		Title:     req.Title,
		Body:      req.Body,
		MetaTitle: req.MetaTitle,
		MetaDesc:  req.MetaDesc,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]pageBody{"page": newPageBody(page)})
}

func (h *contentHandler) publishPage(w http.ResponseWriter, r *http.Request) {
	h.setPagePublished(w, r, true)
}

func (h *contentHandler) unpublishPage(w http.ResponseWriter, r *http.Request) {
	h.setPagePublished(w, r, false)
}

func (h *contentHandler) setPagePublished(w http.ResponseWriter, r *http.Request, published bool) {
	page, err := h.svc.SetPagePublished(r.Context(), chi.URLParam(r, "id"), published)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]pageBody{"page": newPageBody(page)})
}

func (h *contentHandler) deletePage(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeletePage(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: posts ----

func (h *contentHandler) listPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := h.svc.ListPosts(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]postBody, 0, len(posts))
	for _, post := range posts {
		items = append(items, newPostBody(post))
	}
	writeJSON(w, http.StatusOK, map[string][]postBody{"items": items})
}

func (h *contentHandler) getPost(w http.ResponseWriter, r *http.Request) {
	post, err := h.svc.GetPost(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]postBody{"post": newPostBody(post)})
}

func (h *contentHandler) createPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	post, err := h.svc.CreatePost(r.Context(), ports.PostInput{Slug: req.Slug, Title: req.Title, Body: req.Body})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]postBody{"post": newPostBody(post)})
}

func (h *contentHandler) updatePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug       *string   `json:"slug"`
		Title      *string   `json:"title"`
		Excerpt    *string   `json:"excerpt"`
		Body       *string   `json:"body"`
		CoverImage *string   `json:"coverImage"`
		Category   *string   `json:"category"`
		Tags       *[]string `json:"tags"`
		MetaTitle  *string   `json:"metaTitle"`
		MetaDesc   *string   `json:"metaDescription"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	post, err := h.svc.UpdatePost(r.Context(), chi.URLParam(r, "id"), cms.PostPatch{
		Slug:       req.Slug,
		Title:      req.Title,
		Excerpt:    req.Excerpt,
		Body:       req.Body,
		CoverImage: req.CoverImage,
		Category:   req.Category,
		Tags:       req.Tags,
		MetaTitle:  req.MetaTitle,
		MetaDesc:   req.MetaDesc,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]postBody{"post": newPostBody(post)})
}

func (h *contentHandler) publishPost(w http.ResponseWriter, r *http.Request) {
	h.setPostPublished(w, r, true)
}

func (h *contentHandler) unpublishPost(w http.ResponseWriter, r *http.Request) {
	h.setPostPublished(w, r, false)
}

func (h *contentHandler) setPostPublished(w http.ResponseWriter, r *http.Request, published bool) {
	post, err := h.svc.SetPostPublished(r.Context(), chi.URLParam(r, "id"), published)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]postBody{"post": newPostBody(post)})
}

func (h *contentHandler) deletePost(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeletePost(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: FAQs ----

func (h *contentHandler) listFAQs(w http.ResponseWriter, r *http.Request) {
	faqs, err := h.svc.ListFAQs(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]faqBody, 0, len(faqs))
	for _, faq := range faqs {
		items = append(items, newFAQBody(faq))
	}
	writeJSON(w, http.StatusOK, map[string][]faqBody{"items": items})
}

func (h *contentHandler) createFAQ(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question  string `json:"question"`
		Answer    string `json:"answer"`
		Category  string `json:"category"`
		SortOrder int    `json:"sortOrder"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	faq, err := h.svc.CreateFAQ(r.Context(), ports.FAQInput{
		Question:  req.Question,
		Answer:    req.Answer,
		Category:  req.Category,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]faqBody{"faq": newFAQBody(faq)})
}

func (h *contentHandler) updateFAQ(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question  *string `json:"question"`
		Answer    *string `json:"answer"`
		Category  *string `json:"category"`
		SortOrder *int    `json:"sortOrder"`
		Active    *bool   `json:"active"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	faq, err := h.svc.UpdateFAQ(r.Context(), chi.URLParam(r, "id"), cms.FAQPatch{
		Question:  req.Question,
		Answer:    req.Answer,
		Category:  req.Category,
		SortOrder: req.SortOrder,
		Active:    req.Active,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]faqBody{"faq": newFAQBody(faq)})
}

func (h *contentHandler) deleteFAQ(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteFAQ(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Admin: testimonials ----

func (h *contentHandler) listTestimonials(w http.ResponseWriter, r *http.Request) {
	status := cms.Moderation(r.URL.Query().Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "validation_error", "status must be pending, approved, or rejected")
		return
	}
	items, err := h.svc.ListTestimonials(r.Context(), status)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]testimonialBody, 0, len(items))
	for _, t := range items {
		out = append(out, newTestimonialBody(t))
	}
	writeJSON(w, http.StatusOK, map[string][]testimonialBody{"items": out})
}

func (h *contentHandler) createTestimonial(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorName string `json:"authorName"`
		AuthorRole string `json:"authorRole"`
		Quote      string `json:"quote"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	t, err := h.svc.CreateTestimonial(r.Context(), ports.TestimonialInput{
		AuthorName: req.AuthorName,
		AuthorRole: req.AuthorRole,
		Quote:      req.Quote,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]testimonialBody{"testimonial": newTestimonialBody(t)})
}

func (h *contentHandler) updateTestimonial(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorName *string `json:"authorName"`
		AuthorRole *string `json:"authorRole"`
		Quote      *string `json:"quote"`
		SortOrder  *int    `json:"sortOrder"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	t, err := h.svc.UpdateTestimonial(r.Context(), chi.URLParam(r, "id"), cms.TestimonialPatch{
		AuthorName: req.AuthorName,
		AuthorRole: req.AuthorRole,
		Quote:      req.Quote,
		SortOrder:  req.SortOrder,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]testimonialBody{"testimonial": newTestimonialBody(t)})
}

func (h *contentHandler) approveTestimonial(w http.ResponseWriter, r *http.Request) {
	h.moderateTestimonial(w, r, true)
}

func (h *contentHandler) rejectTestimonial(w http.ResponseWriter, r *http.Request) {
	h.moderateTestimonial(w, r, false)
}

func (h *contentHandler) moderateTestimonial(w http.ResponseWriter, r *http.Request, approve bool) {
	t, err := h.svc.ModerateTestimonial(r.Context(), chi.URLParam(r, "id"), approve)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]testimonialBody{"testimonial": newTestimonialBody(t)})
}

func (h *contentHandler) deleteTestimonial(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteTestimonial(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
