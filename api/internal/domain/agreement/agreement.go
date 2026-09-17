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
	Key                      string
	Title                    string
	Body                     string
	RequiresCountersignature bool
	CollectionID             string
	Version                  int
	Active                   bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// AgreementCollection groups multiple agreements into a bundle (e.g. Holistic Coaching Agreement Collection).
type AgreementCollection struct {
	ID             string
	PractitionerID string
	Key            string
	Title          string
	Description    string
	AgreementIDs   []string
	Active         bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RequiresPractitionerSignature is the execution policy for the practice's
// documents. Derive it from the stable key so legacy stored flags cannot make
// a client-only document require a practitioner signature.
func RequiresPractitionerSignature(key string) bool {
	return key == "holistic_coaching" || key == "nurse_coaching"
}

// RequiresClientSignature returns whether a document is a legal agreement the
// client must sign. Statement-of-work documents are fill-in forms only and do
// not stand as a signature gate.
func RequiresClientSignature(key string) bool {
	return key != "holistic_sow" && key != "nurse_sow"
}

// New builds an active agreement at version 1. The legacy boolean argument is
// retained for caller compatibility; the document key determines signing roles.
func New(practitionerID, key, title, body string, _ bool, now time.Time) (Agreement, error) {
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
		PractitionerID:           practitionerID,
		Key:                      key,
		Title:                    title,
		Body:                     body,
		RequiresCountersignature: RequiresPractitionerSignature(key),
		Version:                  1,
		Active:                   true,
		CreatedAt:                now,
		UpdatedAt:                now,
	}, nil
}

// Patch is a partial edit of an agreement. A change to the text bumps the
// version; a change to the title or the active flag alone does not, because
// neither alters what a past signatory agreed to.
type Patch struct {
	Title                    *string
	Body                     *string
	RequiresCountersignature *bool
	CollectionID             *string
	Active                   *bool
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
	a.RequiresCountersignature = RequiresPractitionerSignature(a.Key)
	if p.CollectionID != nil {
		a.CollectionID = strings.TrimSpace(*p.CollectionID)
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
	StatementOfWork  *StatementOfWork
	SubmittedAt      *time.Time
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
	ClientName               string
	ClientEmail              string
	SignedName               string
	RequiresCountersignature bool
	PractitionerSignedName   string
	PractitionerSignedAt     *time.Time
	SharedWithClient         bool
	/** The booking being made when the agreement was presented, when there
	 * was one. Kept for the audit trail, never for coverage: coverage is by
	 * agreement, so this booking is simply the first one. */
	BookingID string
	SignedAt  time.Time
}

// NewCollection builds an active collection.
func NewCollection(practitionerID, key, title, description string, agreementIDs []string, now time.Time) (AgreementCollection, error) {
	key = strings.TrimSpace(key)
	title = strings.TrimSpace(title)
	if practitionerID == "" || key == "" || title == "" {
		return AgreementCollection{}, ErrInvalidCollection
	}
	now = now.UTC()
	return AgreementCollection{
		PractitionerID: practitionerID,
		Key:            key,
		Title:          title,
		Description:    strings.TrimSpace(description),
		AgreementIDs:   agreementIDs,
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Countersign records the practitioner countersigning the agreement.
func (s Signature) Countersign(practitionerName string, now time.Time) (Signature, error) {
	if !RequiresPractitionerSignature(s.AgreementKey) {
		return s, ErrCountersignatureNotRequired
	}
	if s.PractitionerSignedAt != nil {
		return s, ErrAlreadyCountersigned
	}
	practitionerName = strings.TrimSpace(practitionerName)
	if err := ValidateSignedName(practitionerName); err != nil {
		return s, err
	}
	t := now.UTC()
	s.PractitionerSignedName = practitionerName
	s.PractitionerSignedAt = &t
	return s, nil
}

// Sign builds a signature of a against the wording a currently carries.
func (a Agreement) Sign(clientID, clientName, clientEmail, signedName, bookingID string, now time.Time) (Signature, error) {
	if !RequiresClientSignature(a.Key) || IsStatementOfWork(a.Key) {
		return Signature{}, ErrSignatureNotRequired
	}
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
		AgreementID:              a.ID,
		AgreementKey:             a.Key,
		AgreementTitle:           a.Title,
		AgreementVersion:         a.Version,
		AgreementBody:            a.Body,
		ClientID:                 clientID,
		ClientName:               strings.TrimSpace(clientName),
		ClientEmail:              strings.TrimSpace(clientEmail),
		SignedName:               signedName,
		RequiresCountersignature: RequiresPractitionerSignature(a.Key),
		BookingID:                strings.TrimSpace(bookingID),
		SignedAt:                 now.UTC(),
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
