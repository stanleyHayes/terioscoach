package agreement

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)

func newFixture(t *testing.T) Agreement {
	t.Helper()
	a, err := New("prac-1", "holistic_coaching", "Holistic Coaching Agreement", "## Terms\n\nBe punctual.", fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a.ID = "agr-1"
	return a
}

func TestNewStartsActiveAtVersionOne(t *testing.T) {
	a := newFixture(t)
	if a.Version != 1 {
		t.Errorf("Version = %d, want 1", a.Version)
	}
	if !a.Active {
		t.Error("a new agreement should be active")
	}
	if !a.CreatedAt.Equal(fixedNow) || !a.UpdatedAt.Equal(fixedNow) {
		t.Error("timestamps should be the supplied clock, in UTC")
	}
}

func TestNewRejectsEmptyInput(t *testing.T) {
	for name, tc := range map[string]struct {
		practitioner, key, title, body string
		want                           error
	}{
		"no practitioner": {"", "k", "T", "body", ErrInvalidAgreement},
		"no key":          {"p", "", "T", "body", ErrInvalidAgreement},
		"no title":        {"p", "k", "  ", "body", ErrInvalidTitle},
		"no body":         {"p", "k", "T", "   ", ErrInvalidBody},
		"title too long":  {"p", "k", strings.Repeat("a", MaxTitleLen+1), "body", ErrTitleTooLong},
		"body too long":   {"p", "k", "T", strings.Repeat("a", MaxBodyLen+1), ErrBodyTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(tc.practitioner, tc.key, tc.title, tc.body, fixedNow); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// A signature is evidence of what was agreed, so changing the text has to
// move the version on — otherwise an old signature would silently come to
// cover wording its signatory never saw.
func TestEditingTheTextBumpsTheVersion(t *testing.T) {
	a := newFixture(t)
	body := "## Terms\n\nBe punctual, and bring water."
	updated, err := a.Apply(Patch{Body: &body}, fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("Version = %d, want 2", updated.Version)
	}
}

func TestResavingTheSameTextKeepsTheVersion(t *testing.T) {
	a := newFixture(t)
	same := a.Body
	title := "Holistic Coaching Agreement (2026)"
	updated, err := a.Apply(Patch{Body: &same, Title: &title}, fixedNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if updated.Version != 1 {
		t.Errorf("Version = %d, want 1 — the wording did not change", updated.Version)
	}
	if updated.Title != title {
		t.Errorf("Title = %q, want the new one", updated.Title)
	}
}

func TestDeactivatingDoesNotBumpTheVersion(t *testing.T) {
	a := newFixture(t)
	no := false
	updated, err := a.Apply(Patch{Active: &no}, fixedNow)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if updated.Active || updated.Version != 1 {
		t.Errorf("Active = %v, Version = %d; want false, 1", updated.Active, updated.Version)
	}
}

func TestSignPinsTheVersionAndKeepsTheNameVerbatim(t *testing.T) {
	a := newFixture(t)
	body := "## Terms\n\nRevised."
	a, _ = a.Apply(Patch{Body: &body}, fixedNow)

	sig, err := a.Sign("client-1", "Daniel Baah", "daniel@example.com", "  Daniel K. Baah  ", "booking-9", fixedNow)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if sig.AgreementVersion != 2 {
		t.Errorf("AgreementVersion = %d, want 2", sig.AgreementVersion)
	}
	// Only the surrounding whitespace goes; the mark itself is untouched,
	// including a middle initial the account name does not carry.
	if sig.SignedName != "Daniel K. Baah" {
		t.Errorf("SignedName = %q, want it kept verbatim", sig.SignedName)
	}
	if sig.ClientName != "Daniel Baah" {
		t.Errorf("ClientName = %q, want the name on the account alongside it", sig.ClientName)
	}
	if sig.AgreementKey != a.Key || sig.AgreementTitle != a.Title {
		t.Error("the signature should carry what was signed, not just its id")
	}
}

func TestSignRejectsUnusableNames(t *testing.T) {
	a := newFixture(t)
	for name, tc := range map[string]struct {
		signed string
		want   error
	}{
		"blank":      {"   ", ErrInvalidSignedName},
		"one letter": {"D", ErrInvalidSignedName},
		"a paste":    {strings.Repeat("a", MaxSignedNameLen+1), ErrSignedNameTooLong},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := a.Sign("client-1", "Daniel", "d@e.com", tc.signed, "", fixedNow); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSignRequiresAClient(t *testing.T) {
	a := newFixture(t)
	if _, err := a.Sign("", "Daniel", "d@e.com", "Daniel Baah", "", fixedNow); !errors.Is(err, ErrInvalidClient) {
		t.Errorf("err = %v, want ErrInvalidClient", err)
	}
}

// A retired agreement must not collect new signatures: whatever replaced it
// is the wording the practice now stands behind.
func TestInactiveAgreementCannotBeSigned(t *testing.T) {
	a := newFixture(t)
	no := false
	a, _ = a.Apply(Patch{Active: &no}, fixedNow)
	if _, err := a.Sign("client-1", "Daniel", "d@e.com", "Daniel Baah", "", fixedNow); !errors.Is(err, ErrAgreementInactive) {
		t.Errorf("err = %v, want ErrAgreementInactive", err)
	}
}
