// Package enquiry is the domain core for website enquiries: the message a
// stranger sends through the contact form and the triage states it moves
// through in the practice inbox. It imports nothing outside the standard
// library — no frameworks, no drivers.
//
// Everything here arrives from an anonymous, unauthenticated caller, so the
// package is written defensively: every field is bounded, trimmed, and
// treated as text rather than markup.
package enquiry

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits. They bound what one anonymous POST can cost, and keep an
// enquiry readable in an inbox row.
const (
	MaxNameLen    = 120
	MaxEmailLen   = 254 // RFC 5321 maximum address length
	MaxPhoneLen   = 40
	MaxSubjectLen = 200
	MaxMessageLen = 5000
)

// Status is the triage state of an enquiry in the practice inbox.
type Status string

const (
	StatusNew      Status = "new"
	StatusRead     Status = "read"
	StatusReplied  Status = "replied"
	StatusArchived Status = "archived"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusRead, StatusReplied, StatusArchived:
		return true
	}
	return false
}

// Enquiry is one message from the public contact form.
type Enquiry struct {
	ID      string
	Name    string
	Email   string
	Phone   string
	Subject string
	Message string
	Status  Status
	// SourceIP is kept for abuse triage only — it is never returned by any
	// route, and the retention policy in the runbook governs how long it
	// stays.
	SourceIP  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New validates and builds an enquiry in the new state.
func New(name, email, phone, subject, message string, now time.Time) (Enquiry, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))
	phone = strings.TrimSpace(phone)
	subject = strings.TrimSpace(subject)
	message = strings.TrimSpace(message)

	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return Enquiry{}, ErrInvalidName
	}
	if err := validateEmail(email); err != nil {
		return Enquiry{}, err
	}
	if utf8.RuneCountInString(phone) > MaxPhoneLen {
		return Enquiry{}, ErrPhoneTooLong
	}
	if utf8.RuneCountInString(subject) > MaxSubjectLen {
		return Enquiry{}, ErrSubjectTooLong
	}
	if message == "" || utf8.RuneCountInString(message) > MaxMessageLen {
		return Enquiry{}, ErrInvalidMessage
	}

	now = now.UTC()
	return Enquiry{
		Name:      name,
		Email:     email,
		Phone:     phone,
		Subject:   subject,
		Message:   message,
		Status:    StatusNew,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SetStatus moves the enquiry through triage. Every state is reachable from
// every other: an archived message that turns out to matter can come back,
// and a "replied" marked in error can be undone. Triage is a note about
// what the practitioner has done, not a lifecycle to be defended.
func (e *Enquiry) SetStatus(status Status, now time.Time) error {
	if !status.Valid() {
		return ErrInvalidStatus
	}
	e.Status = status
	e.UpdatedAt = now.UTC()
	return nil
}

// validateEmail applies the same shape check the identity slice uses: an
// address needs a local part, an @, and a dotted domain. Anything more
// exacting rejects valid addresses; anything less lets a typo through to a
// bounce.
func validateEmail(email string) error {
	if email == "" || utf8.RuneCountInString(email) > MaxEmailLen {
		return ErrInvalidEmail
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return ErrInvalidEmail
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return ErrInvalidEmail
	}
	if strings.ContainsAny(email, " \t\r\n") {
		return ErrInvalidEmail
	}
	return nil
}
