package form

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

const validSignatureImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="

// consentForm is an intake form with every interesting field kind on it.
func consentForm(t *testing.T) Form {
	t.Helper()
	f, err := New("Intake and consent", "Please complete before your first session.", []Field{
		{Key: "full_name", Label: "Full name", Type: FieldText, Required: true},
		{Key: "notes", Label: "Anything we should know?", Type: FieldTextarea},
		{Key: "age", Label: "Age", Type: FieldNumber},
		{Key: "last_treatment", Label: "Date of last treatment", Type: FieldDate},
		{Key: "pressure", Label: "Preferred pressure", Type: FieldSelect, Required: true,
			Options: []string{"Light", "Medium", "Firm"}},
		{Key: "areas", Label: "Areas to focus on", Type: FieldCheckbox},
		{Key: "consent", Label: "Signature", Type: FieldSignature, Required: true},
	}, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.ID = "form-1"
	return f
}

func assigned(t *testing.T, f Form) Submission {
	t.Helper()
	s, err := Assign(f, "client-1", "booking-1", fixedNow)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	s.ID = "submission-1"
	return s
}

func completeAnswers() map[string]Answer {
	return map[string]Answer{
		"full_name": {Value: "  Ama Serwaa  "},
		"pressure":  {Value: "Medium"},
		"areas":     {Values: []string{"Shoulders", "Lower back"}},
	}
}

func validSignature() *SignatureInput {
	return &SignatureInput{TypedName: "Ama Serwaa", ImageData: validSignatureImage, IP: "203.0.113.7"}
}

// TestNewValidatesTheDefinition.
func TestNewValidatesTheDefinition(t *testing.T) {
	if _, err := New("", "", []Field{{Key: "a", Label: "A", Type: FieldText}}, fixedNow); !errors.Is(err, ErrInvalidTitle) {
		t.Errorf("blank title err = %v, want ErrInvalidTitle", err)
	}
	if _, err := New("T", "", nil, fixedNow); !errors.Is(err, ErrNoFields) {
		t.Errorf("no fields err = %v, want ErrNoFields", err)
	}
}

// TestFieldValidation covers the builder's rules.
func TestFieldValidation(t *testing.T) {
	cases := map[string]struct {
		fields []Field
		want   error
	}{
		"duplicate keys": {[]Field{
			{Key: "name", Label: "A", Type: FieldText},
			{Key: "name", Label: "B", Type: FieldText},
		}, ErrDuplicateFieldKey},
		"blank key":              {[]Field{{Key: "!!!", Label: "A", Type: FieldText}}, ErrInvalidFieldKey},
		"blank label":            {[]Field{{Key: "a", Label: "  ", Type: FieldText}}, ErrInvalidFieldLabel},
		"unknown type":           {[]Field{{Key: "a", Label: "A", Type: "colour_picker"}}, ErrInvalidFieldType},
		"select with no options": {[]Field{{Key: "a", Label: "A", Type: FieldSelect}}, ErrOptionsRequired},
		"text with options": {[]Field{
			{Key: "a", Label: "A", Type: FieldText, Options: []string{"x"}},
		}, ErrOptionsNotAllowed},
		"duplicate options": {[]Field{
			{Key: "a", Label: "A", Type: FieldRadio, Options: []string{"x", "x"}},
		}, ErrDuplicateOption},
		"blank option": {[]Field{
			{Key: "a", Label: "A", Type: FieldRadio, Options: []string{"x", "  "}},
		}, ErrInvalidOption},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New("Form", "", tc.fields, fixedNow); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestKeyNormalization: an editor types a label, the builder derives a key.
func TestKeyNormalization(t *testing.T) {
	for raw, want := range map[string]string{
		"Full Name":       "full_name",
		"  spaced  key  ": "spaced_key",
		"kebab-case":      "kebab_case",
		"__leading__":     "leading",
		"Mixed CASE 2026": "mixed_case_2026",
	} {
		if got := NormalizeKey(raw); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", raw, got, want)
		}
	}
}

// TestAssignStartsUnfilled.
func TestAssignStartsUnfilled(t *testing.T) {
	s := assigned(t, consentForm(t))
	if s.Status != StatusAssigned || s.SubmittedAt != nil || s.Signature != nil {
		t.Errorf("submission = %+v, want an unfilled assignment", s)
	}
	if s.FormTitle != "Intake and consent" {
		t.Errorf("formTitle = %q, want the title snapshotted at assignment", s.FormTitle)
	}
}

// TestAssignRejectsInactiveForms: a retired form must not be sent out.
func TestAssignRejectsInactiveForms(t *testing.T) {
	f := consentForm(t)
	inactive := false
	if err := f.Apply(Patch{Active: &inactive}, fixedNow); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := Assign(f, "client-1", "", fixedNow); !errors.Is(err, ErrFormInactive) {
		t.Errorf("err = %v, want ErrFormInactive", err)
	}
	if _, err := Assign(f, "", "", fixedNow); err == nil {
		t.Error("assigned a form to nobody")
	}
}

// TestSubmitValidatesAgainstTheDefinition is the slice's central rule.
func TestSubmitValidatesAgainstTheDefinition(t *testing.T) {
	f := consentForm(t)

	t.Run("required field missing", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		delete(answers, "full_name")
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrRequiredFieldMissing) {
			t.Errorf("err = %v, want ErrRequiredFieldMissing", err)
		}
	})

	t.Run("required field blank", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["full_name"] = Answer{Value: "   "}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrRequiredFieldMissing) {
			t.Errorf("err = %v, want ErrRequiredFieldMissing — whitespace is not an answer", err)
		}
	})

	t.Run("answer to a field that does not exist", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["smuggled"] = Answer{Value: "payload"}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrUnknownField) {
			t.Errorf("err = %v, want ErrUnknownField", err)
		}
	})

	t.Run("choice outside the option list", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["pressure"] = Answer{Value: "Bone-crushing"}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrInvalidOptionAnswer) {
			t.Errorf("err = %v, want ErrInvalidOptionAnswer", err)
		}
	})

	t.Run("non-numeric number", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["age"] = Answer{Value: "thirty"}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrInvalidNumber) {
			t.Errorf("err = %v, want ErrInvalidNumber", err)
		}
	})

	t.Run("malformed date", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["last_treatment"] = Answer{Value: "11/08/2026"}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("err = %v, want ErrInvalidDate", err)
		}
	})

	t.Run("over-long answer", func(t *testing.T) {
		s := assigned(t, f)
		answers := completeAnswers()
		answers["notes"] = Answer{Value: strings.Repeat("x", MaxAnswerLen+1)}
		if err := s.Submit(f, answers, validSignature(), fixedNow); !errors.Is(err, ErrAnswerTooLong) {
			t.Errorf("err = %v, want ErrAnswerTooLong", err)
		}
	})
}

// TestSubmitHappyPath.
func TestSubmitHappyPath(t *testing.T) {
	f := consentForm(t)
	s := assigned(t, f)

	if err := s.Submit(f, completeAnswers(), validSignature(), fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if s.Status != StatusSubmitted || s.SubmittedAt == nil {
		t.Fatalf("submission = %+v, want it completed and stamped", s)
	}
	if s.Answers["full_name"].Value != "Ama Serwaa" {
		t.Errorf("full_name = %q, want it trimmed", s.Answers["full_name"].Value)
	}
	if len(s.Answers["areas"].Values) != 2 {
		t.Errorf("areas = %v, want both values kept", s.Answers["areas"].Values)
	}
	// Optional fields left blank simply are not there.
	if _, present := s.Answers["notes"]; present {
		t.Errorf("answers = %v, want blank optional fields omitted", s.Answers)
	}
	// The signature field never becomes an answer.
	if _, present := s.Answers["consent"]; present {
		t.Error("the signature field was stored as an answer")
	}
	if s.Signature == nil || s.Signature.TypedName != "Ama Serwaa" || s.Signature.SignedIP != "203.0.113.7" {
		t.Errorf("signature = %+v, want the client's mark recorded", s.Signature)
	}
}

// TestSignatureIsRequiredWhenTheFormHasOne.
func TestSignatureIsRequiredWhenTheFormHasOne(t *testing.T) {
	f := consentForm(t)
	s := assigned(t, f)

	if err := s.Submit(f, completeAnswers(), nil, fixedNow); !errors.Is(err, ErrSignatureRequired) {
		t.Fatalf("err = %v, want ErrSignatureRequired", err)
	}
	if s.Status != StatusAssigned {
		t.Errorf("status = %q, want the submission untouched by a rejected submit", s.Status)
	}
}

// TestSignatureValidation: a mark has to be a real one.
func TestSignatureValidation(t *testing.T) {
	f := consentForm(t)

	cases := map[string]struct {
		signature SignatureInput
		want      error
	}{
		"no typed name":  {SignatureInput{TypedName: "  ", ImageData: validSignatureImage}, ErrInvalidSignatureName},
		"no drawn mark":  {SignatureInput{TypedName: "Ama"}, ErrSignatureRequired},
		"remote url":     {SignatureInput{TypedName: "Ama", ImageData: "https://example.com/sig.png"}, ErrInvalidSignatureImage},
		"script url":     {SignatureInput{TypedName: "Ama", ImageData: "javascript:alert(1)"}, ErrInvalidSignatureImage},
		"svg data url":   {SignatureInput{TypedName: "Ama", ImageData: "data:image/svg+xml;base64,PHN2Zz4="}, ErrInvalidSignatureImage},
		"oversized mark": {SignatureInput{TypedName: "Ama", ImageData: "data:image/png;base64," + strings.Repeat("A", MaxSignatureBytes)}, ErrSignatureTooLarge},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := assigned(t, f)
			sig := tc.signature
			if err := s.Submit(f, completeAnswers(), &sig, fixedNow); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestFormWithoutSignatureNeedsNone.
func TestFormWithoutSignatureNeedsNone(t *testing.T) {
	f, err := New("Feedback", "", []Field{
		{Key: "comments", Label: "Comments", Type: FieldTextarea},
	}, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	f.ID = "form-2"

	s, err := Assign(f, "client-1", "", fixedNow)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if err := s.Submit(f, map[string]Answer{"comments": {Value: "Lovely."}}, nil, fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if s.Signature != nil {
		t.Error("a signature was recorded on a form that has no signature field")
	}
	if !s.VerifyIntegrity() {
		t.Error("an unsigned submission failed its integrity check")
	}
}

// TestSubmitIsOneWay: a signed consent record is not an editable document.
func TestSubmitIsOneWay(t *testing.T) {
	f := consentForm(t)
	s := assigned(t, f)
	if err := s.Submit(f, completeAnswers(), validSignature(), fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	answers := completeAnswers()
	answers["full_name"] = Answer{Value: "Someone Else"}
	if err := s.Submit(f, answers, validSignature(), fixedNow.Add(time.Hour)); !errors.Is(err, ErrAlreadySubmitted) {
		t.Fatalf("second submit = %v, want ErrAlreadySubmitted", err)
	}
	if s.Answers["full_name"].Value != "Ama Serwaa" {
		t.Errorf("full_name = %q, want the signed record unchanged", s.Answers["full_name"].Value)
	}
}

// TestSubmitRejectsTheWrongForm: answers must be validated against the form
// the submission is actually for.
func TestSubmitRejectsTheWrongForm(t *testing.T) {
	f := consentForm(t)
	s := assigned(t, f)

	other := consentForm(t)
	other.ID = "form-99"
	if err := s.Submit(other, completeAnswers(), validSignature(), fixedNow); !errors.Is(err, ErrFormMismatch) {
		t.Errorf("err = %v, want ErrFormMismatch", err)
	}
}

// TestIntegrityHashDetectsTampering is what makes a stored consent record
// evidence: altering the answers after signing breaks the digest.
func TestIntegrityHashDetectsTampering(t *testing.T) {
	f := consentForm(t)
	s := assigned(t, f)
	if err := s.Submit(f, completeAnswers(), validSignature(), fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !s.VerifyIntegrity() {
		t.Fatal("a freshly signed submission failed its own integrity check")
	}
	if s.Signature.Hash == "" {
		t.Fatal("no integrity hash was recorded")
	}

	// Someone edits the stored answers directly.
	tampered := s
	tampered.Answers = map[string]Answer{
		"full_name": {Value: "Someone Else"},
		"pressure":  {Value: "Medium"},
		"areas":     {Values: []string{"Shoulders", "Lower back"}},
	}
	if tampered.VerifyIntegrity() {
		t.Error("tampered answers passed the integrity check")
	}

	// So does swapping the name on the signature.
	renamed := s
	sig := *s.Signature
	sig.TypedName = "Someone Else"
	renamed.Signature = &sig
	if renamed.VerifyIntegrity() {
		t.Error("a swapped signature name passed the integrity check")
	}
}

// TestHashIsOrderIndependent: the same content must hash the same however
// the answer map was built, or every verification would be a coin toss.
func TestHashIsOrderIndependent(t *testing.T) {
	f := consentForm(t)

	first := assigned(t, f)
	if err := first.Submit(f, map[string]Answer{
		"full_name": {Value: "Ama Serwaa"},
		"pressure":  {Value: "Medium"},
	}, validSignature(), fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	second := assigned(t, f)
	if err := second.Submit(f, map[string]Answer{
		"pressure":  {Value: "Medium"},
		"full_name": {Value: "Ama Serwaa"},
	}, validSignature(), fixedNow); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	if first.Signature.Hash != second.Signature.Hash {
		t.Error("identical submissions produced different digests — verification would be unreliable")
	}
}

// TestStatusValidity guards the enum.
func TestStatusValidity(t *testing.T) {
	if !StatusAssigned.Valid() || !StatusSubmitted.Valid() || SubmissionStatus("draft").Valid() {
		t.Error("SubmissionStatus.Valid does not match the known set")
	}
	if !FieldText.Valid() || FieldType("colour").Valid() {
		t.Error("FieldType.Valid does not match the known set")
	}
}
