package enquiry

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func newValid(t *testing.T) Enquiry {
	t.Helper()
	e, err := New("Ama Serwaa", "ama@example.com", "", "Booking question", "Do you offer prenatal massage?", fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

// TestNewStartsInTheInbox: an enquiry arrives unread.
func TestNewStartsInTheInbox(t *testing.T) {
	e := newValid(t)
	if e.Status != StatusNew {
		t.Errorf("status = %q, want new", e.Status)
	}
	if !e.CreatedAt.Equal(fixedNow) || !e.UpdatedAt.Equal(fixedNow) {
		t.Errorf("enquiry = %+v, want both stamps set", e)
	}
}

// TestNewTrimsAndNormalizes: whitespace and address casing must not create
// two inbox rows that look identical.
func TestNewTrimsAndNormalizes(t *testing.T) {
	e, err := New("  Ama Serwaa  ", "  AMA@Example.COM ", " +233201234567 ", "  Hello  ", "  A question.  ", fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if e.Name != "Ama Serwaa" || e.Email != "ama@example.com" {
		t.Errorf("enquiry = %+v, want name trimmed and email lowercased", e)
	}
	if e.Phone != "+233201234567" || e.Subject != "Hello" || e.Message != "A question." {
		t.Errorf("enquiry = %+v, want every field trimmed", e)
	}
}

// TestValidationRejectsUnusableInput: the form is open to anyone, so the
// checks are the only thing standing between a stranger and the inbox.
func TestValidationRejectsUnusableInput(t *testing.T) {
	cases := map[string]struct {
		name, email, phone, subject, message string
		want                                 error
	}{
		"blank name":       {"  ", "ama@example.com", "", "", "Hello", ErrInvalidName},
		"long name":        {strings.Repeat("a", MaxNameLen+1), "ama@example.com", "", "", "Hello", ErrInvalidName},
		"blank message":    {"Ama", "ama@example.com", "", "", "   ", ErrInvalidMessage},
		"long message":     {"Ama", "ama@example.com", "", "", strings.Repeat("x", MaxMessageLen+1), ErrInvalidMessage},
		"blank email":      {"Ama", "", "", "", "Hello", ErrInvalidEmail},
		"no at sign":       {"Ama", "ama.example.com", "", "", "Hello", ErrInvalidEmail},
		"no domain dot":    {"Ama", "ama@example", "", "", "Hello", ErrInvalidEmail},
		"trailing dot":     {"Ama", "ama@example.", "", "", "Hello", ErrInvalidEmail},
		"empty local":      {"Ama", "@example.com", "", "", "Hello", ErrInvalidEmail},
		"embedded newline": {"Ama", "ama@example.com\nbcc: victim@example.com", "", "", "Hello", ErrInvalidEmail},
		"long phone":       {"Ama", "ama@example.com", strings.Repeat("9", MaxPhoneLen+1), "", "Hello", ErrPhoneTooLong},
		"long subject":     {"Ama", "ama@example.com", "", strings.Repeat("s", MaxSubjectLen+1), "Hello", ErrSubjectTooLong},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := New(tc.name, tc.email, tc.phone, tc.subject, tc.message, fixedNow); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// TestTriageMovesBothWays: triage records what the practitioner did, so
// every state is reachable — including undoing one set in error.
func TestTriageMovesBothWays(t *testing.T) {
	e := newValid(t)
	later := fixedNow.Add(time.Hour)

	for _, status := range []Status{StatusRead, StatusReplied, StatusArchived, StatusNew, StatusReplied} {
		if err := e.SetStatus(status, later); err != nil {
			t.Fatalf("SetStatus(%q): %v", status, err)
		}
		if e.Status != status {
			t.Errorf("status = %q, want %q", e.Status, status)
		}
		if !e.UpdatedAt.Equal(later) {
			t.Errorf("updatedAt = %v, want the transition stamped", e.UpdatedAt)
		}
	}
}

// TestUnknownStatusIsRejected.
func TestUnknownStatusIsRejected(t *testing.T) {
	e := newValid(t)
	if err := e.SetStatus("spam", fixedNow); !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("err = %v, want ErrInvalidStatus", err)
	}
	if e.Status != StatusNew {
		t.Errorf("status = %q, want the enquiry untouched by a rejected transition", e.Status)
	}
}

// TestStatusValidity guards the enum against values entering from storage.
func TestStatusValidity(t *testing.T) {
	for _, status := range []Status{StatusNew, StatusRead, StatusReplied, StatusArchived} {
		if !status.Valid() {
			t.Errorf("%q is not valid, want it in the known set", status)
		}
	}
	if Status("").Valid() || Status("spam").Valid() {
		t.Error("Status.Valid accepted a value outside the known set")
	}
}
