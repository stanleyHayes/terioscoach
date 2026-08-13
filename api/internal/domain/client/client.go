// Package client is the domain core for practice-side client records: the
// client profile a practitioner keeps about a client (contact detail, tags,
// private summary) and the practice-membership rule. It imports nothing
// outside the standard library — no frameworks, no drivers.
package client

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits for the practice-side profile. They bound what a
// practitioner can store, keeping profiles small enough for list views.
const (
	MaxPhoneLen         = 40
	MaxPracticeNotesLen = 5000
	MaxTags             = 20
	MaxTagLen           = 40
)

// Profile is the practice-side record about one client. It is keyed on the
// client's user id (UserID) — the profile links to the identity account and
// never duplicates it (name/email live on the user). Every field here is
// practitioner-owned: clients can read their phone but can write nothing,
// and PracticeNotes/Tags never appear in any client-facing view.
type Profile struct {
	ID            string
	UserID        string
	Phone         string
	Tags          []string
	PracticeNotes string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// New builds an empty profile for a client account.
func New(userID string, now time.Time) Profile {
	now = now.UTC()
	return Profile{
		UserID:    userID,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// PracticePatch carries the writable practice-side fields. Nil means
// "leave unchanged" — PATCH semantics; a non-nil zero value clears.
type PracticePatch struct {
	Phone         *string
	PracticeNotes *string
	Tags          *[]string
}

// ApplyPracticePatch applies the practitioner's edits, validating each
// touched field. Tags are trimmed and deduplicated (first occurrence wins,
// order preserved).
func (p *Profile) ApplyPracticePatch(patch PracticePatch, now time.Time) error {
	if patch.Phone != nil {
		phone := strings.TrimSpace(*patch.Phone)
		if utf8.RuneCountInString(phone) > MaxPhoneLen {
			return ErrPhoneTooLong
		}
		p.Phone = phone
	}
	if patch.PracticeNotes != nil {
		if utf8.RuneCountInString(*patch.PracticeNotes) > MaxPracticeNotesLen {
			return ErrPracticeNotesTooLong
		}
		p.PracticeNotes = *patch.PracticeNotes
	}
	if patch.Tags != nil {
		tags, err := normalizeTags(*patch.Tags)
		if err != nil {
			return err
		}
		p.Tags = tags
	}
	p.UpdatedAt = now.UTC()
	return nil
}

// normalizeTags trims, drops empties, dedupes, and enforces the tag limits.
func normalizeTags(raw []string) ([]string, error) {
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

// Summary is the practitioner list row: the profile joined with the
// account's name/email and the booking rollup. Name/Email are copied in by
// the app layer from the identity account — the profile itself stays a
// pure link.
type Summary struct {
	Profile       Profile
	Name          string
	Email         string
	TotalSessions int
	LastSessionAt *time.Time
}
