package cms

import "errors"

// Domain errors for the CMS slice. Adapters and the HTTP layer map these
// to storage results and status codes via errors.Is.
var (
	// ErrPageNotFound / ErrPostNotFound / ErrFAQNotFound /
	// ErrTestimonialNotFound mean no record matches the lookup — including
	// a draft or unapproved record read through a public route, which must
	// be indistinguishable from one that does not exist.
	ErrPageNotFound        = errors.New("page not found")
	ErrPostNotFound        = errors.New("post not found")
	ErrFAQNotFound         = errors.New("faq not found")
	ErrTestimonialNotFound = errors.New("testimonial not found")

	// ErrSlugTaken signals a unique-slug conflict.
	ErrSlugTaken = errors.New("slug is already in use")

	// Validation errors.
	ErrInvalidSlug     = errors.New("slug must contain at least one letter or digit")
	ErrSlugTooLong     = errors.New("slug is too long")
	ErrInvalidTitle    = errors.New("title is required")
	ErrTitleTooLong    = errors.New("title is too long")
	ErrExcerptTooLong  = errors.New("excerpt is too long")
	ErrBodyTooLong     = errors.New("body is too long")
	ErrCategoryTooLong = errors.New("category is too long")
	ErrTooManyTags     = errors.New("too many tags")
	ErrTagTooLong      = errors.New("tag is too long")
	ErrInvalidQuestion = errors.New("question is required")
	ErrInvalidAnswer   = errors.New("answer is required")
	ErrInvalidAuthor   = errors.New("author name is required")
	ErrInvalidQuote    = errors.New("quote is required")
	ErrInvalidURL      = errors.New("url must be http(s) or a site-relative path")
	ErrURLTooLong      = errors.New("url is too long")
)
