package mongodb

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/ports"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// This file holds the four CMS collections (BE-12). They share one shape of
// problem — a public read must never see unpublished work — so the
// published filter is applied in the query itself, not left to the caller.

// ---- Pages ----

// PageRepository persists editable site pages in cms_pages.
type PageRepository struct {
	coll *mongo.Collection
}

var _ ports.PageRepository = (*PageRepository)(nil)

// NewPageRepository binds the repository to cms_pages.
func NewPageRepository(db *mongo.Database) *PageRepository {
	return &PageRepository{coll: db.Collection("cms_pages")}
}

type pageDoc struct {
	ID          bson.ObjectID  `bson:"_id,omitempty"`
	Slug        string         `bson:"slug"`
	Title       string         `bson:"title"`
	Body        string         `bson:"body"`
	MetaTitle   string         `bson:"metaTitle,omitempty"`
	MetaDesc    string         `bson:"metaDescription,omitempty"`
	Status      string         `bson:"status"`
	PublishedAt *bson.DateTime `bson:"publishedAt,omitempty"`
	CreatedAt   bson.DateTime  `bson:"createdAt"`
	UpdatedAt   bson.DateTime  `bson:"updatedAt"`
}

func newPageDoc(p cms.Page) pageDoc {
	doc := pageDoc{
		Slug:      p.Slug,
		Title:     p.Title,
		Body:      p.Body,
		MetaTitle: p.MetaTitle,
		MetaDesc:  p.MetaDesc,
		Status:    string(p.Status),
		CreatedAt: bson.NewDateTimeFromTime(p.CreatedAt),
		UpdatedAt: bson.NewDateTimeFromTime(p.UpdatedAt),
	}
	if p.PublishedAt != nil {
		stamp := bson.NewDateTimeFromTime(*p.PublishedAt)
		doc.PublishedAt = &stamp
	}
	return doc
}

func (d pageDoc) toDomain() cms.Page {
	page := cms.Page{
		ID:        d.ID.Hex(),
		Slug:      d.Slug,
		Title:     d.Title,
		Body:      d.Body,
		MetaTitle: d.MetaTitle,
		MetaDesc:  d.MetaDesc,
		Status:    cms.Status(d.Status),
		CreatedAt: d.CreatedAt.Time().UTC(),
		UpdatedAt: d.UpdatedAt.Time().UTC(),
	}
	if d.PublishedAt != nil {
		at := d.PublishedAt.Time().UTC()
		page.PublishedAt = &at
	}
	return page
}

func (r *PageRepository) Create(ctx context.Context, page cms.Page) (cms.Page, error) {
	res, err := r.coll.InsertOne(ctx, newPageDoc(page))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return cms.Page{}, cms.ErrSlugTaken
		}
		return cms.Page{}, fmt.Errorf("insert page: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		page.ID = oid.Hex()
	}
	return page, nil
}

func (r *PageRepository) Update(ctx context.Context, page cms.Page) (cms.Page, error) {
	oid, err := bson.ObjectIDFromHex(page.ID)
	if err != nil {
		return cms.Page{}, cms.ErrPageNotFound
	}
	doc := newPageDoc(page)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"slug":            doc.Slug,
		"title":           doc.Title,
		"body":            doc.Body,
		"metaTitle":       doc.MetaTitle,
		"metaDescription": doc.MetaDesc,
		"status":          doc.Status,
		"publishedAt":     doc.PublishedAt,
		"updatedAt":       doc.UpdatedAt,
	}})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return cms.Page{}, cms.ErrSlugTaken
		}
		return cms.Page{}, fmt.Errorf("update page: %w", err)
	}
	if res.MatchedCount == 0 {
		return cms.Page{}, cms.ErrPageNotFound
	}
	return page, nil
}

func (r *PageRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.ErrPageNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete page: %w", err)
	}
	return nil
}

func (r *PageRepository) FindByID(ctx context.Context, id string) (cms.Page, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.Page{}, cms.ErrPageNotFound
	}
	var doc pageDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.Page{}, cms.ErrPageNotFound
		}
		return cms.Page{}, fmt.Errorf("find page: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *PageRepository) FindBySlug(ctx context.Context, slug string, publishedOnly bool) (cms.Page, error) {
	filter := bson.M{"slug": slug}
	if publishedOnly {
		filter["status"] = string(cms.StatusPublished)
	}
	var doc pageDoc
	if err := r.coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.Page{}, cms.ErrPageNotFound
		}
		return cms.Page{}, fmt.Errorf("find page by slug: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *PageRepository) List(ctx context.Context, filter ports.ContentFilter) ([]cms.Page, error) {
	query := bson.M{}
	if filter.PublishedOnly {
		query["status"] = string(cms.StatusPublished)
	}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "title", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("list pages: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []pageDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode pages: %w", err)
	}
	pages := make([]cms.Page, 0, len(docs))
	for _, doc := range docs {
		pages = append(pages, doc.toDomain())
	}
	return pages, nil
}

// ---- Posts ----

// PostRepository persists blog posts in blog_posts.
type PostRepository struct {
	coll *mongo.Collection
}

var _ ports.PostRepository = (*PostRepository)(nil)

// NewPostRepository binds the repository to blog_posts.
func NewPostRepository(db *mongo.Database) *PostRepository {
	return &PostRepository{coll: db.Collection("blog_posts")}
}

type postDoc struct {
	ID          bson.ObjectID  `bson:"_id,omitempty"`
	Slug        string         `bson:"slug"`
	Title       string         `bson:"title"`
	Excerpt     string         `bson:"excerpt,omitempty"`
	Body        string         `bson:"body"`
	CoverImage  string         `bson:"coverImage,omitempty"`
	Category    string         `bson:"category,omitempty"`
	Tags        []string       `bson:"tags"`
	MetaTitle   string         `bson:"metaTitle,omitempty"`
	MetaDesc    string         `bson:"metaDescription,omitempty"`
	Status      string         `bson:"status"`
	PublishedAt *bson.DateTime `bson:"publishedAt,omitempty"`
	CreatedAt   bson.DateTime  `bson:"createdAt"`
	UpdatedAt   bson.DateTime  `bson:"updatedAt"`
}

func newPostDoc(p cms.Post) postDoc {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	doc := postDoc{
		Slug:       p.Slug,
		Title:      p.Title,
		Excerpt:    p.Excerpt,
		Body:       p.Body,
		CoverImage: p.CoverImage,
		Category:   p.Category,
		Tags:       tags,
		MetaTitle:  p.MetaTitle,
		MetaDesc:   p.MetaDesc,
		Status:     string(p.Status),
		CreatedAt:  bson.NewDateTimeFromTime(p.CreatedAt),
		UpdatedAt:  bson.NewDateTimeFromTime(p.UpdatedAt),
	}
	if p.PublishedAt != nil {
		stamp := bson.NewDateTimeFromTime(*p.PublishedAt)
		doc.PublishedAt = &stamp
	}
	return doc
}

func (d postDoc) toDomain() cms.Post {
	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}
	post := cms.Post{
		ID:         d.ID.Hex(),
		Slug:       d.Slug,
		Title:      d.Title,
		Excerpt:    d.Excerpt,
		Body:       d.Body,
		CoverImage: d.CoverImage,
		Category:   d.Category,
		Tags:       tags,
		MetaTitle:  d.MetaTitle,
		MetaDesc:   d.MetaDesc,
		Status:     cms.Status(d.Status),
		CreatedAt:  d.CreatedAt.Time().UTC(),
		UpdatedAt:  d.UpdatedAt.Time().UTC(),
	}
	if d.PublishedAt != nil {
		at := d.PublishedAt.Time().UTC()
		post.PublishedAt = &at
	}
	return post
}

func (r *PostRepository) Create(ctx context.Context, post cms.Post) (cms.Post, error) {
	res, err := r.coll.InsertOne(ctx, newPostDoc(post))
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return cms.Post{}, cms.ErrSlugTaken
		}
		return cms.Post{}, fmt.Errorf("insert post: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		post.ID = oid.Hex()
	}
	return post, nil
}

func (r *PostRepository) Update(ctx context.Context, post cms.Post) (cms.Post, error) {
	oid, err := bson.ObjectIDFromHex(post.ID)
	if err != nil {
		return cms.Post{}, cms.ErrPostNotFound
	}
	doc := newPostDoc(post)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"slug":            doc.Slug,
		"title":           doc.Title,
		"excerpt":         doc.Excerpt,
		"body":            doc.Body,
		"coverImage":      doc.CoverImage,
		"category":        doc.Category,
		"tags":            doc.Tags,
		"metaTitle":       doc.MetaTitle,
		"metaDescription": doc.MetaDesc,
		"status":          doc.Status,
		"publishedAt":     doc.PublishedAt,
		"updatedAt":       doc.UpdatedAt,
	}})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return cms.Post{}, cms.ErrSlugTaken
		}
		return cms.Post{}, fmt.Errorf("update post: %w", err)
	}
	if res.MatchedCount == 0 {
		return cms.Post{}, cms.ErrPostNotFound
	}
	return post, nil
}

func (r *PostRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.ErrPostNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}

func (r *PostRepository) FindByID(ctx context.Context, id string) (cms.Post, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.Post{}, cms.ErrPostNotFound
	}
	var doc postDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.Post{}, cms.ErrPostNotFound
		}
		return cms.Post{}, fmt.Errorf("find post: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *PostRepository) FindBySlug(ctx context.Context, slug string, publishedOnly bool) (cms.Post, error) {
	filter := bson.M{"slug": slug}
	if publishedOnly {
		filter["status"] = string(cms.StatusPublished)
	}
	var doc postDoc
	if err := r.coll.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.Post{}, cms.ErrPostNotFound
		}
		return cms.Post{}, fmt.Errorf("find post by slug: %w", err)
	}
	return doc.toDomain(), nil
}

// List orders newest-published first, falling back to creation order for
// drafts (which have no publication date yet).
func (r *PostRepository) List(ctx context.Context, filter ports.ContentFilter) ([]cms.Post, error) {
	query := bson.M{}
	if filter.PublishedOnly {
		query["status"] = string(cms.StatusPublished)
	}
	if filter.Category != "" {
		query["category"] = filter.Category
	}
	if filter.Tag != "" {
		query["tags"] = filter.Tag
	}
	sort := bson.D{{Key: "publishedAt", Value: -1}, {Key: "createdAt", Value: -1}}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(sort))
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []postDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode posts: %w", err)
	}
	posts := make([]cms.Post, 0, len(docs))
	for _, doc := range docs {
		posts = append(posts, doc.toDomain())
	}
	return posts, nil
}

// ---- FAQs ----

// FAQRepository persists FAQ entries in faqs.
type FAQRepository struct {
	coll *mongo.Collection
}

var _ ports.FAQRepository = (*FAQRepository)(nil)

// NewFAQRepository binds the repository to faqs.
func NewFAQRepository(db *mongo.Database) *FAQRepository {
	return &FAQRepository{coll: db.Collection("faqs")}
}

type faqDoc struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Question  string        `bson:"question"`
	Answer    string        `bson:"answer"`
	Category  string        `bson:"category,omitempty"`
	SortOrder int           `bson:"sortOrder"`
	Active    bool          `bson:"active"`
	CreatedAt bson.DateTime `bson:"createdAt"`
	UpdatedAt bson.DateTime `bson:"updatedAt"`
}

func newFAQDoc(f cms.FAQ) faqDoc {
	return faqDoc{
		Question:  f.Question,
		Answer:    f.Answer,
		Category:  f.Category,
		SortOrder: f.SortOrder,
		Active:    f.Active,
		CreatedAt: bson.NewDateTimeFromTime(f.CreatedAt),
		UpdatedAt: bson.NewDateTimeFromTime(f.UpdatedAt),
	}
}

func (d faqDoc) toDomain() cms.FAQ {
	return cms.FAQ{
		ID:        d.ID.Hex(),
		Question:  d.Question,
		Answer:    d.Answer,
		Category:  d.Category,
		SortOrder: d.SortOrder,
		Active:    d.Active,
		CreatedAt: d.CreatedAt.Time().UTC(),
		UpdatedAt: d.UpdatedAt.Time().UTC(),
	}
}

func (r *FAQRepository) Create(ctx context.Context, faq cms.FAQ) (cms.FAQ, error) {
	res, err := r.coll.InsertOne(ctx, newFAQDoc(faq))
	if err != nil {
		return cms.FAQ{}, fmt.Errorf("insert faq: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		faq.ID = oid.Hex()
	}
	return faq, nil
}

func (r *FAQRepository) Update(ctx context.Context, faq cms.FAQ) (cms.FAQ, error) {
	oid, err := bson.ObjectIDFromHex(faq.ID)
	if err != nil {
		return cms.FAQ{}, cms.ErrFAQNotFound
	}
	doc := newFAQDoc(faq)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"question":  doc.Question,
		"answer":    doc.Answer,
		"category":  doc.Category,
		"sortOrder": doc.SortOrder,
		"active":    doc.Active,
		"updatedAt": doc.UpdatedAt,
	}})
	if err != nil {
		return cms.FAQ{}, fmt.Errorf("update faq: %w", err)
	}
	if res.MatchedCount == 0 {
		return cms.FAQ{}, cms.ErrFAQNotFound
	}
	return faq, nil
}

func (r *FAQRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.ErrFAQNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete faq: %w", err)
	}
	return nil
}

func (r *FAQRepository) FindByID(ctx context.Context, id string) (cms.FAQ, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.FAQ{}, cms.ErrFAQNotFound
	}
	var doc faqDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.FAQ{}, cms.ErrFAQNotFound
		}
		return cms.FAQ{}, fmt.Errorf("find faq: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *FAQRepository) List(ctx context.Context, filter ports.ContentFilter) ([]cms.FAQ, error) {
	query := bson.M{}
	if filter.PublishedOnly {
		query["active"] = true
	}
	if filter.Category != "" {
		query["category"] = filter.Category
	}
	sort := bson.D{{Key: "sortOrder", Value: 1}, {Key: "createdAt", Value: 1}}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(sort))
	if err != nil {
		return nil, fmt.Errorf("list faqs: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []faqDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode faqs: %w", err)
	}
	faqs := make([]cms.FAQ, 0, len(docs))
	for _, doc := range docs {
		faqs = append(faqs, doc.toDomain())
	}
	return faqs, nil
}

// ---- Testimonials ----

// TestimonialRepository persists testimonials in testimonials.
type TestimonialRepository struct {
	coll *mongo.Collection
}

var _ ports.TestimonialRepository = (*TestimonialRepository)(nil)

// NewTestimonialRepository binds the repository to testimonials.
func NewTestimonialRepository(db *mongo.Database) *TestimonialRepository {
	return &TestimonialRepository{coll: db.Collection("testimonials")}
}

type testimonialDoc struct {
	ID          bson.ObjectID  `bson:"_id,omitempty"`
	AuthorName  string         `bson:"authorName"`
	AuthorRole  string         `bson:"authorRole,omitempty"`
	Quote       string         `bson:"quote"`
	Status      string         `bson:"status"`
	SortOrder   int            `bson:"sortOrder"`
	SubmittedAt bson.DateTime  `bson:"submittedAt"`
	ApprovedAt  *bson.DateTime `bson:"approvedAt,omitempty"`
	CreatedAt   bson.DateTime  `bson:"createdAt"`
	UpdatedAt   bson.DateTime  `bson:"updatedAt"`
}

func newTestimonialDoc(t cms.Testimonial) testimonialDoc {
	doc := testimonialDoc{
		AuthorName:  t.AuthorName,
		AuthorRole:  t.AuthorRole,
		Quote:       t.Quote,
		Status:      string(t.Status),
		SortOrder:   t.SortOrder,
		SubmittedAt: bson.NewDateTimeFromTime(t.SubmittedAt),
		CreatedAt:   bson.NewDateTimeFromTime(t.CreatedAt),
		UpdatedAt:   bson.NewDateTimeFromTime(t.UpdatedAt),
	}
	if t.ApprovedAt != nil {
		stamp := bson.NewDateTimeFromTime(*t.ApprovedAt)
		doc.ApprovedAt = &stamp
	}
	return doc
}

func (d testimonialDoc) toDomain() cms.Testimonial {
	t := cms.Testimonial{
		ID:          d.ID.Hex(),
		AuthorName:  d.AuthorName,
		AuthorRole:  d.AuthorRole,
		Quote:       d.Quote,
		Status:      cms.Moderation(d.Status),
		SortOrder:   d.SortOrder,
		SubmittedAt: d.SubmittedAt.Time().UTC(),
		CreatedAt:   d.CreatedAt.Time().UTC(),
		UpdatedAt:   d.UpdatedAt.Time().UTC(),
	}
	if d.ApprovedAt != nil {
		at := d.ApprovedAt.Time().UTC()
		t.ApprovedAt = &at
	}
	return t
}

func (r *TestimonialRepository) Create(ctx context.Context, t cms.Testimonial) (cms.Testimonial, error) {
	res, err := r.coll.InsertOne(ctx, newTestimonialDoc(t))
	if err != nil {
		return cms.Testimonial{}, fmt.Errorf("insert testimonial: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		t.ID = oid.Hex()
	}
	return t, nil
}

func (r *TestimonialRepository) Update(ctx context.Context, t cms.Testimonial) (cms.Testimonial, error) {
	oid, err := bson.ObjectIDFromHex(t.ID)
	if err != nil {
		return cms.Testimonial{}, cms.ErrTestimonialNotFound
	}
	doc := newTestimonialDoc(t)
	res, err := r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"authorName": doc.AuthorName,
		"authorRole": doc.AuthorRole,
		"quote":      doc.Quote,
		"status":     doc.Status,
		"sortOrder":  doc.SortOrder,
		"approvedAt": doc.ApprovedAt,
		"updatedAt":  doc.UpdatedAt,
	}})
	if err != nil {
		return cms.Testimonial{}, fmt.Errorf("update testimonial: %w", err)
	}
	if res.MatchedCount == 0 {
		return cms.Testimonial{}, cms.ErrTestimonialNotFound
	}
	return t, nil
}

func (r *TestimonialRepository) Delete(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.ErrTestimonialNotFound
	}
	if _, err := r.coll.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete testimonial: %w", err)
	}
	return nil
}

func (r *TestimonialRepository) FindByID(ctx context.Context, id string) (cms.Testimonial, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return cms.Testimonial{}, cms.ErrTestimonialNotFound
	}
	var doc testimonialDoc
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return cms.Testimonial{}, cms.ErrTestimonialNotFound
		}
		return cms.Testimonial{}, fmt.Errorf("find testimonial: %w", err)
	}
	return doc.toDomain(), nil
}

func (r *TestimonialRepository) List(ctx context.Context, filter ports.ContentFilter, status cms.Moderation) ([]cms.Testimonial, error) {
	query := bson.M{}
	if status != "" {
		query["status"] = string(status)
	}
	if filter.PublishedOnly {
		query["status"] = string(cms.ModerationApproved)
	}
	sort := bson.D{{Key: "sortOrder", Value: 1}, {Key: "createdAt", Value: -1}}
	cursor, err := r.coll.Find(ctx, query, options.Find().SetSort(sort))
	if err != nil {
		return nil, fmt.Errorf("list testimonials: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	var docs []testimonialDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode testimonials: %w", err)
	}
	items := make([]cms.Testimonial, 0, len(docs))
	for _, doc := range docs {
		items = append(items, doc.toDomain())
	}
	return items, nil
}
