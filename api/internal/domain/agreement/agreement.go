// Package agreement is the domain core for the practice's service
// agreements: the contract a client accepts before their first session
// under a line of work, and the signature that records that acceptance. It
// imports nothing outside the standard library — no frameworks, no drivers.
//
// The rule that shapes the package: a signature is for an *agreement*, not
// for a booking. The practice offers several services under one agreement
// (a Holistic Coaching session and an Initial Holistic Coaching Session are
// both covered by the Holistic Coaching Agreement), and a client signs each
// agreement once. Every later booking of any service under that agreement —
// including the same service again — is already covered.
//
// A signature is also pinned to the version of the text it was given
// against. Editing an agreement bumps its version, and clients who signed
// the previous wording are recorded as having signed *that* wording; what
// they agreed to is never rewritten underneath them.
package agreement

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits.
const (
	MaxTitleLen      = 200
	MaxBodyLen       = 120_000
	MaxSignedNameLen = 120
	MinSignedNameLen = 2
)

// Agreement is one contract the practice asks clients to accept, with the
// full text it is accepted against.
//
// Body is the agreement in Markdown-ish plain text: paragraphs separated by
// blank lines, "## " for headings, and "1. " / "- " for lists. It is
// rendered by the client, never as HTML from the server, so the text can
// never carry markup into a page.
type Agreement struct {
	ID             string
	PractitionerID string
	/** Stable machine name, e.g. "holistic_coaching". Services point at the
	 * ID, but the key is what seeds and fixtures refer to. */
	Key     string
	Title   string
	Body    string
	Version int
	Active  bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// New builds an active agreement at version 1.
func New(practitionerID, key, title, body string, now time.Time) (Agreement, error) {
	key = strings.TrimSpace(key)
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	if practitionerID == "" || key == "" {
		return Agreement{}, ErrInvalidAgreement
	}
	if err := ValidateTitle(title); err != nil {
		return Agreement{}, err
	}
	if err := ValidateBody(body); err != nil {
		return Agreement{}, err
	}
	now = now.UTC()
	return Agreement{
		PractitionerID: practitionerID,
		Key:            key,
		Title:          title,
		Body:           body,
		Version:        1,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Patch is a partial edit of an agreement. A change to the text bumps the
// version; a change to the title or the active flag alone does not, because
// neither alters what a past signatory agreed to.
type Patch struct {
	Title  *string
	Body   *string
	Active *bool
}

// Apply validates and applies p, returning the updated agreement.
func (a Agreement) Apply(p Patch, now time.Time) (Agreement, error) {
	if p.Title != nil {
		title := strings.TrimSpace(*p.Title)
		if err := ValidateTitle(title); err != nil {
			return Agreement{}, err
		}
		a.Title = title
	}
	if p.Body != nil {
		body := strings.TrimSpace(*p.Body)
		if err := ValidateBody(body); err != nil {
			return Agreement{}, err
		}
		// Only a real change counts: re-saving the same text must not
		// invalidate the coverage every existing signature provides.
		if body != a.Body {
			a.Body = body
			a.Version++
		}
	}
	if p.Active != nil {
		a.Active = *p.Active
	}
	a.UpdatedAt = now.UTC()
	return a, nil
}

// Signature records one client accepting one agreement, at one version.
//
// SignedName is what the client typed. It is kept verbatim — it is the
// signature — and is never normalised or corrected to the name on the
// account, which is stored alongside it so the two can be compared later.
type Signature struct {
	ID               string
	AgreementID      string
	AgreementKey     string
	AgreementTitle   string
	AgreementVersion int
	/** The wording as it stood when this was signed. Snapshotted, not
	 * looked up: an agreement edited later must not silently change what a
	 * past signatory is recorded as having accepted. */
	AgreementBody string
	ClientID      string
	/** The name on the account when the signature was given. */
	ClientName  string
	ClientEmail string
	SignedName  string
	/** The booking being made when the agreement was presented, when there
	 * was one. Kept for the audit trail, never for coverage: coverage is by
	 * agreement, so this booking is simply the first one. */
	BookingID string
	SignedAt  time.Time
}

// Sign builds a signature of a against the wording a currently carries.
func (a Agreement) Sign(clientID, clientName, clientEmail, signedName, bookingID string, now time.Time) (Signature, error) {
	if clientID == "" {
		return Signature{}, ErrInvalidClient
	}
	signedName = strings.TrimSpace(signedName)
	if err := ValidateSignedName(signedName); err != nil {
		return Signature{}, err
	}
	if !a.Active {
		return Signature{}, ErrAgreementInactive
	}
	return Signature{
		AgreementID:      a.ID,
		AgreementKey:     a.Key,
		AgreementTitle:   a.Title,
		AgreementVersion: a.Version,
		AgreementBody:    a.Body,
		ClientID:         clientID,
		ClientName:       strings.TrimSpace(clientName),
		ClientEmail:      strings.TrimSpace(clientEmail),
		SignedName:       signedName,
		BookingID:        strings.TrimSpace(bookingID),
		SignedAt:         now.UTC(),
	}, nil
}

// ValidateTitle reports whether a title is usable.
func ValidateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return ErrInvalidTitle
	}
	if utf8.RuneCountInString(title) > MaxTitleLen {
		return ErrTitleTooLong
	}
	return nil
}

// ValidateBody reports whether agreement text is usable.
func ValidateBody(body string) error {
	body = strings.TrimSpace(body)
	if body == "" {
		return ErrInvalidBody
	}
	if utf8.RuneCountInString(body) > MaxBodyLen {
		return ErrBodyTooLong
	}
	return nil
}

// ValidateSignedName reports whether a typed signature is usable.
//
// The bar is deliberately low: a legal signature is whatever mark the
// signatory chooses to make, and a practice cannot reject one for failing
// to match the name on file. What it must not be is blank, a single
// character, or long enough to be a paste of something else.
func ValidateSignedName(name string) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < MinSignedNameLen {
		return ErrInvalidSignedName
	}
	if utf8.RuneCountInString(name) > MaxSignedNameLen {
		return ErrSignedNameTooLong
	}
	return nil
}
