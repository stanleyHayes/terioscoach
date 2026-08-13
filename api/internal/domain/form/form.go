// Package form is the domain core for intake and consent forms: the form
// definition a practitioner builds, the submission a client fills in, and
// the signature that makes a consent form binding. It imports nothing
// outside the standard library — no frameworks, no drivers.
//
// The rule that shapes the package: a submission is validated against its
// form definition, not against whatever the client happened to send. A
// consent form with a required signature cannot be submitted without one,
// and an answer to a field that does not exist is not stored.
package form

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Field and form limits.
const (
	MaxTitleLen       = 200
	MaxDescriptionLen = 2000
	MaxFields         = 60
	MaxLabelLen       = 300
	MaxKeyLen         = 60
	MaxHelpTextLen    = 500
	MaxOptions        = 40
	MaxOptionLen      = 200
	MaxAnswerLen      = 5000
	MaxSignatureBytes = 256 * 1024 // a drawn signature is a small PNG
	MaxTypedNameLen   = 120
)

// FieldType is the kind of input a field renders as. The set is closed: the
// design system builds a custom control for each one, and a type nothing
// can render is a broken form.
type FieldType string

const (
	FieldText      FieldType = "text"
	FieldTextarea  FieldType = "textarea"
	FieldNumber    FieldType = "number"
	FieldDate      FieldType = "date"
	FieldSelect    FieldType = "select"
	FieldRadio     FieldType = "radio"
	FieldCheckbox  FieldType = "checkbox"
	FieldSignature FieldType = "signature"
)

// Valid reports whether t is a known field type.
func (t FieldType) Valid() bool {
	switch t {
	case FieldText, FieldTextarea, FieldNumber, FieldDate,
		FieldSelect, FieldRadio, FieldCheckbox, FieldSignature:
		return true
	}
	return false
}

// needsOptions reports whether the type is meaningless without a choice list.
func (t FieldType) needsOptions() bool {
	return t == FieldSelect || t == FieldRadio
}

// Field is one question on a form. Key is the stable identifier answers are
// stored under, so renaming a label never orphans historical answers.
type Field struct {
	Key      string
	Label    string
	Type     FieldType
	Required bool
	HelpText string
	Options  []string
}

// Form is a reusable intake or consent form.
type Form struct {
	ID          string
	Title       string
	Description string
	Fields      []Field
	// Template marks the reusable library a practitioner clones from.
	Template  bool
	SortOrder int
	Active    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New validates and builds an active form.
func New(title, description string, fields []Field, now time.Time) (Form, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > MaxTitleLen {
		return Form{}, ErrInvalidTitle
	}
	if utf8.RuneCountInString(description) > MaxDescriptionLen {
		return Form{}, ErrDescriptionTooLong
	}
	normalized, err := NormalizeFields(fields)
	if err != nil {
		return Form{}, err
	}
	now = now.UTC()
	return Form{
		Title:       title,
		Description: strings.TrimSpace(description),
		Fields:      normalized,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Patch is the set of editable form fields.
type Patch struct {
	Title       *string
	Description *string
	Fields      *[]Field
	Template    *bool
	SortOrder   *int
	Active      *bool
}

// Apply validates and applies a patch.
func (f *Form) Apply(patch Patch, now time.Time) error {
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" || utf8.RuneCountInString(title) > MaxTitleLen {
			return ErrInvalidTitle
		}
		f.Title = title
	}
	if patch.Description != nil {
		if utf8.RuneCountInString(*patch.Description) > MaxDescriptionLen {
			return ErrDescriptionTooLong
		}
		f.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Fields != nil {
		normalized, err := NormalizeFields(*patch.Fields)
		if err != nil {
			return err
		}
		f.Fields = normalized
	}
	if patch.Template != nil {
		f.Template = *patch.Template
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

// FieldByKey finds a field by its key.
func (f Form) FieldByKey(key string) (Field, bool) {
	for _, field := range f.Fields {
		if field.Key == key {
			return field, true
		}
	}
	return Field{}, false
}

// RequiresSignature reports whether the form has a signature field.
func (f Form) RequiresSignature() bool {
	for _, field := range f.Fields {
		if field.Type == FieldSignature {
			return true
		}
	}
	return false
}

// NormalizeFields validates a field list and returns it trimmed, with keys
// normalized and duplicates rejected.
func NormalizeFields(fields []Field) ([]Field, error) {
	if len(fields) == 0 {
		return nil, ErrNoFields
	}
	if len(fields) > MaxFields {
		return nil, ErrTooManyFields
	}

	seen := make(map[string]bool, len(fields))
	out := make([]Field, 0, len(fields))
	for _, field := range fields {
		key := NormalizeKey(field.Key)
		if key == "" || utf8.RuneCountInString(key) > MaxKeyLen {
			return nil, ErrInvalidFieldKey
		}
		if seen[key] {
			return nil, ErrDuplicateFieldKey
		}
		seen[key] = true

		label := strings.TrimSpace(field.Label)
		if label == "" || utf8.RuneCountInString(label) > MaxLabelLen {
			return nil, ErrInvalidFieldLabel
		}
		if !field.Type.Valid() {
			return nil, ErrInvalidFieldType
		}
		if utf8.RuneCountInString(field.HelpText) > MaxHelpTextLen {
			return nil, ErrHelpTextTooLong
		}

		options, err := normalizeOptions(field)
		if err != nil {
			return nil, err
		}

		out = append(out, Field{
			Key:      key,
			Label:    label,
			Type:     field.Type,
			Required: field.Required,
			HelpText: strings.TrimSpace(field.HelpText),
			Options:  options,
		})
	}
	return out, nil
}

// normalizeOptions validates a field's choice list. Only choice fields may
// carry one: options on a free-text field are a builder mistake that would
// silently render nothing.
func normalizeOptions(field Field) ([]string, error) {
	if !field.Type.needsOptions() {
		if len(field.Options) > 0 {
			return nil, ErrOptionsNotAllowed
		}
		return []string{}, nil
	}
	if len(field.Options) == 0 {
		return nil, ErrOptionsRequired
	}
	if len(field.Options) > MaxOptions {
		return nil, ErrTooManyOptions
	}

	seen := make(map[string]bool, len(field.Options))
	options := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		option = strings.TrimSpace(option)
		if option == "" || utf8.RuneCountInString(option) > MaxOptionLen {
			return nil, ErrInvalidOption
		}
		if seen[option] {
			return nil, ErrDuplicateOption
		}
		seen[option] = true
		options = append(options, option)
	}
	return options, nil
}

// NormalizeKey lowercases a field key and reduces it to letters, digits,
// and underscores, so a key is safe as a map key and stable across edits.
func NormalizeKey(raw string) string {
	var b strings.Builder
	lastUnderscore := true
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		case r == '_' || r == '-' || r == ' ':
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

// ValidateNumber and ValidateDate are exported so the transport can give
// the same answer the domain would, without duplicating the rule.
func ValidateNumber(value string) error {
	if _, err := strconv.ParseFloat(value, 64); err != nil {
		return ErrInvalidNumber
	}
	return nil
}

// ValidateDate accepts a calendar date, which is what a date control emits.
func ValidateDate(value string) error {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return ErrInvalidDate
	}
	return nil
}

// canonicalAnswers renders answers deterministically for hashing: keys
// sorted, values escaped. Two identical submissions must hash identically,
// and no reordering may change the digest.
func canonicalAnswers(answers map[string]Answer) string {
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		answer := answers[key]
		fmt.Fprintf(&b, "%s=%s", key, strconv.Quote(answer.Value))
		for _, v := range answer.Values {
			fmt.Fprintf(&b, ",%s", strconv.Quote(v))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// digest is the integrity hash over a signed submission's content.
func digest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
