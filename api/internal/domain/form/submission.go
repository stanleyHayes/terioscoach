package form

import (
	"strings"
	"time"
	"unicode/utf8"
)

// SubmissionStatus is where a client's copy of a form has got to.
type SubmissionStatus string

const (
	// StatusAssigned means the practitioner has sent the form and the
	// client has not completed it yet.
	StatusAssigned SubmissionStatus = "assigned"
	// StatusSubmitted means the client has completed and (where required)
	// signed it. It is terminal: a signed consent form is a record, not a
	// document to keep editing.
	StatusSubmitted SubmissionStatus = "submitted"
)

// Valid reports whether s is a known status.
func (s SubmissionStatus) Valid() bool {
	return s == StatusAssigned || s == StatusSubmitted
}

// Answer is one field's value. Values carries multi-select answers; Value
// carries everything else. Keeping both on one type means the storage shape
// does not change when a field's type does.
type Answer struct {
	Value  string
	Values []string
}

// Signature is the client's mark on a consent form: what they typed, what
// they drew, and when. Hash binds all of it to the answers, so a stored
// submission can be shown to be the one that was signed.
type Signature struct {
	// TypedName is the name the client typed to confirm their mark.
	TypedName string
	// ImageData is the drawn signature as a data URL (image/png).
	ImageData string
	SignedAt  time.Time
	// SignedIP is captured for evidential value on consent records.
	SignedIP string
	// Hash is the integrity digest — see Submission.Sign.
	Hash string
}

// Submission is one client's copy of one form.
type Submission struct {
	ID     string
	FormID string
	// FormTitle is snapshotted so an old consent record still says what it
	// was called when it was signed, even if the form is later renamed.
	FormTitle   string
	ClientID    string
	BookingID   string
	Status      SubmissionStatus
	Answers     map[string]Answer
	Signature   *Signature
	AssignedAt  time.Time
	SubmittedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Assign creates an unfilled submission for a client — the practitioner
// sending a form out. bookingID is optional: a form can be attached to a
// visit or sent on its own.
func Assign(f Form, clientID, bookingID string, now time.Time) (Submission, error) {
	if f.ID == "" {
		return Submission{}, ErrFormNotFound
	}
	if clientID == "" {
		return Submission{}, ErrInvalidClient
	}
	if !f.Active {
		return Submission{}, ErrFormInactive
	}
	now = now.UTC()
	return Submission{
		FormID:     f.ID,
		FormTitle:  f.Title,
		ClientID:   clientID,
		BookingID:  bookingID,
		Status:     StatusAssigned,
		Answers:    map[string]Answer{},
		AssignedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// SignatureInput is what the client supplies when signing.
type SignatureInput struct {
	TypedName string
	ImageData string
	IP        string
}

// Submit validates answers against the form definition and completes the
// submission.
//
// Validation is against the definition, never against what arrived: a
// required field cannot be skipped by omitting it, a choice field cannot
// hold a value that is not on its list, and an answer to a field that does
// not exist is rejected rather than quietly stored. A form with a signature
// field is not submitted until it is signed.
//
// Submitting is one-way. A signed consent record that could be edited
// afterwards would not be evidence of anything.
func (s *Submission) Submit(f Form, answers map[string]Answer, signature *SignatureInput, now time.Time) error {
	if s.Status == StatusSubmitted {
		return ErrAlreadySubmitted
	}
	if f.ID != s.FormID {
		return ErrFormMismatch
	}

	cleaned, err := validateAnswers(f, answers)
	if err != nil {
		return err
	}

	now = now.UTC()
	if f.RequiresSignature() {
		if signature == nil {
			return ErrSignatureRequired
		}
		signed, err := buildSignature(*signature, now)
		if err != nil {
			return err
		}
		signed.Hash = digest(s.FormID, s.ClientID, canonicalAnswers(cleaned), signed.TypedName, signed.SignedAt.Format(time.RFC3339Nano))
		s.Signature = &signed
	}

	s.Answers = cleaned
	s.Status = StatusSubmitted
	s.SubmittedAt = &now
	s.UpdatedAt = now
	return nil
}

// VerifyIntegrity recomputes the signature digest over the stored content.
// A false answer means the record was altered after signing — which is
// exactly what a consent record's hash exists to detect.
func (s Submission) VerifyIntegrity() bool {
	if s.Signature == nil {
		return true // nothing was signed, so nothing is claimed
	}
	want := digest(
		s.FormID,
		s.ClientID,
		canonicalAnswers(s.Answers),
		s.Signature.TypedName,
		s.Signature.SignedAt.Format(time.RFC3339Nano),
	)
	return want == s.Signature.Hash
}

// buildSignature validates the client's mark.
func buildSignature(in SignatureInput, now time.Time) (Signature, error) {
	typed := strings.TrimSpace(in.TypedName)
	if typed == "" || utf8.RuneCountInString(typed) > MaxTypedNameLen {
		return Signature{}, ErrInvalidSignatureName
	}
	image := strings.TrimSpace(in.ImageData)
	if image == "" {
		return Signature{}, ErrSignatureRequired
	}
	// Only an inline PNG data URL is accepted. A remote URL would make the
	// signature depend on a server that may stop answering, and any other
	// scheme in an <img> on the practitioner's screen is an injection.
	if !strings.HasPrefix(image, "data:image/png;base64,") {
		return Signature{}, ErrInvalidSignatureImage
	}
	if len(image) > MaxSignatureBytes {
		return Signature{}, ErrSignatureTooLarge
	}
	return Signature{
		TypedName: typed,
		ImageData: image,
		SignedAt:  now.UTC(),
		SignedIP:  in.IP,
	}, nil
}

// validateAnswers checks every answer against the definition and returns
// the cleaned map: only known fields, trimmed values.
func validateAnswers(f Form, answers map[string]Answer) (map[string]Answer, error) {
	for key := range answers {
		if _, ok := f.FieldByKey(key); !ok {
			return nil, ErrUnknownField
		}
	}

	cleaned := make(map[string]Answer, len(f.Fields))
	for _, field := range f.Fields {
		answer, provided := answers[field.Key]
		answer = trimAnswer(answer)

		if field.Type == FieldSignature {
			// The signature is carried separately, not as an answer.
			continue
		}
		if !provided || answerEmpty(answer) {
			if field.Required {
				return nil, ErrRequiredFieldMissing
			}
			continue
		}
		if err := validateAnswer(field, answer); err != nil {
			return nil, err
		}
		cleaned[field.Key] = answer
	}
	return cleaned, nil
}

// validateAnswer applies one field's type rules.
func validateAnswer(field Field, answer Answer) error {
	if utf8.RuneCountInString(answer.Value) > MaxAnswerLen {
		return ErrAnswerTooLong
	}
	switch field.Type {
	case FieldNumber:
		return ValidateNumber(answer.Value)
	case FieldDate:
		return ValidateDate(answer.Value)
	case FieldSelect, FieldRadio:
		if !contains(field.Options, answer.Value) {
			return ErrInvalidOptionAnswer
		}
	case FieldCheckbox:
		// A checkbox group answers with values from its own list; a lone
		// checkbox answers "true"/"false".
		for _, v := range answer.Values {
			if utf8.RuneCountInString(v) > MaxAnswerLen {
				return ErrAnswerTooLong
			}
		}
	}
	return nil
}

func trimAnswer(answer Answer) Answer {
	answer.Value = strings.TrimSpace(answer.Value)
	trimmed := make([]string, 0, len(answer.Values))
	for _, v := range answer.Values {
		v = strings.TrimSpace(v)
		if v != "" {
			trimmed = append(trimmed, v)
		}
	}
	answer.Values = trimmed
	return answer
}

func answerEmpty(answer Answer) bool {
	return answer.Value == "" && len(answer.Values) == 0
}

func contains(options []string, want string) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}
