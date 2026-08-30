// Package cms is the domain core for site content: editable pages, blog
// posts, FAQ entries, and testimonials, plus the publish and moderation
// rules that decide what the public site is allowed to show. It imports
// nothing outside the standard library — no frameworks, no drivers.
//
// One rule runs through the whole package: nothing reaches the public site
// by default. Pages and posts are drafts until published; testimonials are
// pending until a person approves them. Every public read is filtered by
// that state in the domain, not by a caller remembering to ask for it.
package cms

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits. They exist to keep a single field from becoming a way to
// break the page it renders into, and to bound what a database write costs.
const (
	MaxSlugLen     = 120
	MaxTitleLen    = 200
	MaxExcerptLen  = 400
	MaxBodyLen     = 100000
	MaxCategoryLen = 60
	MaxTags        = 10
	MaxTagLen      = 40
	MaxQuestionLen = 300
	MaxAnswerLen   = 5000
	MaxAuthorLen   = 120
	MaxQuoteLen    = 1000
	MaxRoleLen     = 120
	MaxURLLen      = 500
)

// Status is the publication state of a page or post. Draft is invisible to
// the public site; Published is live.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	return s == StatusDraft || s == StatusPublished
}

// Moderation is the review state of a testimonial. Pending is the state
// everything starts in, so nothing a stranger submitted can appear before
// a person has looked at it.
type Moderation string

const (
	ModerationPending  Moderation = "pending"
	ModerationApproved Moderation = "approved"
	ModerationRejected Moderation = "rejected"
)

// Valid reports whether m is a known moderation state.
func (m Moderation) Valid() bool {
	switch m {
	case ModerationPending, ModerationApproved, ModerationRejected:
		return true
	}
	return false
}

// Page is an editable site page addressed by slug (about, policies, …).
type Page struct {
	ID          string
	Slug        string
	Title       string
	Body        string
	CoverImage  string
	MetaTitle   string
	MetaDesc    string
	Status      Status
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewPage builds a draft page. Nothing is published on creation: a page
// becomes visible only through an explicit Publish.
func NewPage(slug, title, body string, now time.Time) (Page, error) {
	slug = NormalizeSlug(slug)
	if err := ValidateSlug(slug); err != nil {
		return Page{}, err
	}
	if err := validateTitle(title); err != nil {
		return Page{}, err
	}
	if err := validateBody(body); err != nil {
		return Page{}, err
	}
	now = now.UTC()
	return Page{
		Slug:      slug,
		Title:     strings.TrimSpace(title),
		Body:      body,
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// PagePatch is the set of editable page fields. Nil fields are untouched;
// Status is deliberately absent — publishing is its own transition.
type PagePatch struct {
	Slug       *string
	Title      *string
	Body       *string
	MetaTitle  *string
	MetaDesc   *string
	CoverImage *string
}

// Apply validates and applies a patch.
func (p *Page) Apply(patch PagePatch, now time.Time) error {
	if patch.Slug != nil {
		slug := NormalizeSlug(*patch.Slug)
		if err := ValidateSlug(slug); err != nil {
			return err
		}
		p.Slug = slug
	}
	if patch.Title != nil {
		if err := validateTitle(*patch.Title); err != nil {
			return err
		}
		p.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Body != nil {
		if err := validateBody(*patch.Body); err != nil {
			return err
		}
		p.Body = *patch.Body
	}
	if patch.MetaTitle != nil {
		if utf8.RuneCountInString(*patch.MetaTitle) > MaxTitleLen {
			return ErrTitleTooLong
		}
		p.MetaTitle = strings.TrimSpace(*patch.MetaTitle)
	}
	if patch.MetaDesc != nil {
		if utf8.RuneCountInString(*patch.MetaDesc) > MaxExcerptLen {
			return ErrExcerptTooLong
		}
		p.MetaDesc = strings.TrimSpace(*patch.MetaDesc)
	}
	if patch.CoverImage != nil {
		if err := validateURL(*patch.CoverImage); err != nil {
			return err
		}
		p.CoverImage = strings.TrimSpace(*patch.CoverImage)
	}
	p.UpdatedAt = now.UTC()
	return nil
}

// Publish makes the page live. It is idempotent: re-publishing keeps the
// original PublishedAt, so "published on" does not drift with every edit.
func (p *Page) Publish(now time.Time) {
	now = now.UTC()
	if p.Status != StatusPublished {
		p.Status = StatusPublished
		if p.PublishedAt == nil {
			p.PublishedAt = &now
		}
	}
	p.UpdatedAt = now
}

// Unpublish returns the page to draft, removing it from the public site.
// PublishedAt is kept: it records when the page first went live, which is
// still true after it is taken down.
func (p *Page) Unpublish(now time.Time) {
	p.Status = StatusDraft
	p.UpdatedAt = now.UTC()
}

// Public reports whether the page may be served to an anonymous visitor.
func (p Page) Public() bool { return p.Status == StatusPublished }

// Post is a blog article.
type Post struct {
	ID          string
	Slug        string
	Title       string
	Excerpt     string
	Body        string
	CoverImage  string
	Category    string
	Tags        []string
	MetaTitle   string
	MetaDesc    string
	Status      Status
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewPost builds a draft post.
func NewPost(slug, title, body string, now time.Time) (Post, error) {
	slug = NormalizeSlug(slug)
	if err := ValidateSlug(slug); err != nil {
		return Post{}, err
	}
	if err := validateTitle(title); err != nil {
		return Post{}, err
	}
	if err := validateBody(body); err != nil {
		return Post{}, err
	}
	now = now.UTC()
	return Post{
		Slug:      slug,
		Title:     strings.TrimSpace(title),
		Body:      body,
		Tags:      []string{},
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// PostPatch is the set of editable post fields.
type PostPatch struct {
	Slug       *string
	Title      *string
	Excerpt    *string
	Body       *string
	CoverImage *string
	Category   *string
	Tags       *[]string
	MetaTitle  *string
	MetaDesc   *string
}

// Apply validates and applies a patch.
func (p *Post) Apply(patch PostPatch, now time.Time) error {
	if patch.Slug != nil {
		slug := NormalizeSlug(*patch.Slug)
		if err := ValidateSlug(slug); err != nil {
			return err
		}
		p.Slug = slug
	}
	if patch.Title != nil {
		if err := validateTitle(*patch.Title); err != nil {
			return err
		}
		p.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Excerpt != nil {
		if utf8.RuneCountInString(*patch.Excerpt) > MaxExcerptLen {
			return ErrExcerptTooLong
		}
		p.Excerpt = strings.TrimSpace(*patch.Excerpt)
	}
	if patch.Body != nil {
		if err := validateBody(*patch.Body); err != nil {
			return err
		}
		p.Body = *patch.Body
	}
	if patch.CoverImage != nil {
		if err := validateURL(*patch.CoverImage); err != nil {
			return err
		}
		p.CoverImage = strings.TrimSpace(*patch.CoverImage)
	}
	if patch.Category != nil {
		if utf8.RuneCountInString(*patch.Category) > MaxCategoryLen {
			return ErrCategoryTooLong
		}
		p.Category = strings.TrimSpace(*patch.Category)
	}
	if patch.Tags != nil {
		tags, err := NormalizeTags(*patch.Tags)
		if err != nil {
			return err
		}
		p.Tags = tags
	}
	if patch.MetaTitle != nil {
		if utf8.RuneCountInString(*patch.MetaTitle) > MaxTitleLen {
			return ErrTitleTooLong
		}
		p.MetaTitle = strings.TrimSpace(*patch.MetaTitle)
	}
	if patch.MetaDesc != nil {
		if utf8.RuneCountInString(*patch.MetaDesc) > MaxExcerptLen {
			return ErrExcerptTooLong
		}
		p.MetaDesc = strings.TrimSpace(*patch.MetaDesc)
	}
	p.UpdatedAt = now.UTC()
	return nil
}

// Publish makes the post live, stamping the publication date once.
func (p *Post) Publish(now time.Time) {
	now = now.UTC()
	if p.Status != StatusPublished {
		p.Status = StatusPublished
		if p.PublishedAt == nil {
			p.PublishedAt = &now
		}
	}
	p.UpdatedAt = now
}

// Unpublish returns the post to draft.
func (p *Post) Unpublish(now time.Time) {
	p.Status = StatusDraft
	p.UpdatedAt = now.UTC()
}

// Public reports whether the post may be served to an anonymous visitor.
func (p Post) Public() bool { return p.Status == StatusPublished }

// FAQ is one question-and-answer entry, shown in an admin-controlled order.
type FAQ struct {
	ID        string
	Question  string
	Answer    string
	Category  string
	SortOrder int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewFAQ builds an active entry. Unlike pages and posts, an FAQ is live as
// soon as it exists: it is practice-authored, not submitted, and an entry
// with no answer is caught by validation rather than by a review step.
func NewFAQ(question, answer, category string, sortOrder int, now time.Time) (FAQ, error) {
	question = strings.TrimSpace(question)
	answer = strings.TrimSpace(answer)
	if question == "" || utf8.RuneCountInString(question) > MaxQuestionLen {
		return FAQ{}, ErrInvalidQuestion
	}
	if answer == "" || utf8.RuneCountInString(answer) > MaxAnswerLen {
		return FAQ{}, ErrInvalidAnswer
	}
	if utf8.RuneCountInString(category) > MaxCategoryLen {
		return FAQ{}, ErrCategoryTooLong
	}
	now = now.UTC()
	return FAQ{
		Question:  question,
		Answer:    answer,
		Category:  strings.TrimSpace(category),
		SortOrder: sortOrder,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// FAQPatch is the set of editable FAQ fields.
type FAQPatch struct {
	Question  *string
	Answer    *string
	Category  *string
	SortOrder *int
	Active    *bool
}

// Apply validates and applies a patch.
func (f *FAQ) Apply(patch FAQPatch, now time.Time) error {
	if patch.Question != nil {
		question := strings.TrimSpace(*patch.Question)
		if question == "" || utf8.RuneCountInString(question) > MaxQuestionLen {
			return ErrInvalidQuestion
		}
		f.Question = question
	}
	if patch.Answer != nil {
		answer := strings.TrimSpace(*patch.Answer)
		if answer == "" || utf8.RuneCountInString(answer) > MaxAnswerLen {
			return ErrInvalidAnswer
		}
		f.Answer = answer
	}
	if patch.Category != nil {
		if utf8.RuneCountInString(*patch.Category) > MaxCategoryLen {
			return ErrCategoryTooLong
		}
		f.Category = strings.TrimSpace(*patch.Category)
	}
	if patch.SortOrder != nil {
		f.SortOrder = *patch.SortOrder
	}
	if patch.Active != nil {
		f.Active = *patch.Active
	}
	f.UpdatedAt = now.UTC()
	return nil
}

// Public reports whether the entry may be served to an anonymous visitor.
func (f FAQ) Public() bool { return f.Active }

// Testimonial is a quote shown on the public site. It always starts
// pending: approve-before-publish is the whole point of the type.
type Testimonial struct {
	ID          string
	AuthorName  string
	AuthorRole  string
	Quote       string
	Status      Moderation
	SortOrder   int
	SubmittedAt time.Time
	ApprovedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewTestimonial builds a pending testimonial.
func NewTestimonial(authorName, authorRole, quote string, now time.Time) (Testimonial, error) {
	authorName = strings.TrimSpace(authorName)
	quote = strings.TrimSpace(quote)
	if authorName == "" || utf8.RuneCountInString(authorName) > MaxAuthorLen {
		return Testimonial{}, ErrInvalidAuthor
	}
	if quote == "" || utf8.RuneCountInString(quote) > MaxQuoteLen {
		return Testimonial{}, ErrInvalidQuote
	}
	if utf8.RuneCountInString(authorRole) > MaxRoleLen {
		return Testimonial{}, ErrInvalidAuthor
	}
	now = now.UTC()
	return Testimonial{
		AuthorName:  authorName,
		AuthorRole:  strings.TrimSpace(authorRole),
		Quote:       quote,
		Status:      ModerationPending,
		SubmittedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// TestimonialPatch is the set of editable testimonial fields. Status is
// absent: moderation is its own transition, so an edit can never approve.
type TestimonialPatch struct {
	AuthorName *string
	AuthorRole *string
	Quote      *string
	SortOrder  *int
}

// Apply validates and applies a patch.
func (t *Testimonial) Apply(patch TestimonialPatch, now time.Time) error {
	if patch.AuthorName != nil {
		name := strings.TrimSpace(*patch.AuthorName)
		if name == "" || utf8.RuneCountInString(name) > MaxAuthorLen {
			return ErrInvalidAuthor
		}
		t.AuthorName = name
	}
	if patch.AuthorRole != nil {
		if utf8.RuneCountInString(*patch.AuthorRole) > MaxRoleLen {
			return ErrInvalidAuthor
		}
		t.AuthorRole = strings.TrimSpace(*patch.AuthorRole)
	}
	if patch.Quote != nil {
		quote := strings.TrimSpace(*patch.Quote)
		if quote == "" || utf8.RuneCountInString(quote) > MaxQuoteLen {
			return ErrInvalidQuote
		}
		t.Quote = quote
	}
	if patch.SortOrder != nil {
		t.SortOrder = *patch.SortOrder
	}
	t.UpdatedAt = now.UTC()
	return nil
}

// Approve publishes the testimonial to the public site. It is idempotent
// and keeps the original approval stamp.
func (t *Testimonial) Approve(now time.Time) {
	now = now.UTC()
	if t.Status != ModerationApproved {
		t.Status = ModerationApproved
		if t.ApprovedAt == nil {
			t.ApprovedAt = &now
		}
	}
	t.UpdatedAt = now
}

// Reject takes the testimonial off the site (or keeps it off). Rejection is
// reversible — a later Approve puts it back — because moderation is a
// judgement, not a lifecycle.
func (t *Testimonial) Reject(now time.Time) {
	t.Status = ModerationRejected
	t.UpdatedAt = now.UTC()
}

// Public reports whether the testimonial may be served to an anonymous
// visitor. Only an approved one may.
func (t Testimonial) Public() bool { return t.Status == ModerationApproved }

// transliterations maps accented Latin letters to their ASCII base, so a
// title like "Séance à deux" becomes "seance-a-deux" rather than
// "sance-deux". Dropping the letter outright would quietly mangle names
// and French/Portuguese service titles into something unreadable.
var transliterations = map[rune]string{
	'á': "a", 'à': "a", 'â': "a", 'ä': "a", 'ã': "a", 'å': "a", 'æ': "ae",
	'ç': "c",
	'é': "e", 'è': "e", 'ê': "e", 'ë': "e",
	'í': "i", 'ì': "i", 'î': "i", 'ï': "i",
	'ñ': "n",
	'ó': "o", 'ò': "o", 'ô': "o", 'ö': "o", 'õ': "o", 'ø': "o", 'œ': "oe",
	'ú': "u", 'ù': "u", 'û': "u", 'ü': "u",
	'ý': "y", 'ÿ': "y",
	'ß': "ss",
	'ð': "d", 'þ': "th",
}

// NormalizeSlug lowercases, trims, transliterates accents, and collapses a
// slug candidate into the URL-safe form: an editor may type "About Our
// Approach" and get "about-our-approach".
func NormalizeSlug(raw string) string {
	var b strings.Builder
	lastHyphen := true // leading hyphens are dropped
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if ascii, ok := transliterations[r]; ok {
			b.WriteString(ascii)
			lastHyphen = false
			continue
		}
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastHyphen = false
		case r == '-' || r == ' ' || r == '_' || r == '/':
			if !lastHyphen {
				b.WriteRune('-')
				lastHyphen = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// ValidateSlug rejects a slug that would not address a page.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrInvalidSlug
	}
	if utf8.RuneCountInString(slug) > MaxSlugLen {
		return ErrSlugTooLong
	}
	return nil
}

// NormalizeTags trims, drops empties, dedupes (first wins), and enforces
// the tag limits.
func NormalizeTags(raw []string) ([]string, error) {
	seen := make(map[string]bool, len(raw))
	tags := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if utf8.RuneCountInString(t) > MaxTagLen {
			return nil, ErrTagTooLong
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		tags = append(tags, t)
	}
	if len(tags) > MaxTags {
		return nil, ErrTooManyTags
	}
	return tags, nil
}

func validateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrInvalidTitle
	}
	if utf8.RuneCountInString(title) > MaxTitleLen {
		return ErrTitleTooLong
	}
	return nil
}

func validateBody(body string) error {
	if utf8.RuneCountInString(body) > MaxBodyLen {
		return ErrBodyTooLong
	}
	return nil
}

// validateURL bounds a link field and rejects schemes that are not links at
// all. A javascript: or data: URL in a cover image is a stored-XSS payload
// waiting for a renderer that trusts it; the domain refuses to hold one.
func validateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if utf8.RuneCountInString(raw) > MaxURLLen {
		return ErrURLTooLong
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "/") {
		return nil
	}
	return ErrInvalidURL
}
