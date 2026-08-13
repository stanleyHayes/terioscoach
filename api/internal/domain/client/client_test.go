package client

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)

func strPtr(s string) *string { return &s }

func TestNewDefaults(t *testing.T) {
	p := New("user-1", testNow)
	if p.UserID != "user-1" || p.Tags == nil || len(p.Tags) != 0 || p.Phone != "" || p.PracticeNotes != "" {
		t.Errorf("profile = %+v, want empty profile linked to user", p)
	}
	if !p.CreatedAt.Equal(testNow) {
		t.Errorf("createdAt = %v, want %v", p.CreatedAt, testNow)
	}
}

func TestApplyPracticePatchPartial(t *testing.T) {
	p := New("user-1", testNow)
	phone := "+233 24 000 0000"
	tags := []string{" vip ", "regular", "vip", ""}
	if err := p.ApplyPracticePatch(PracticePatch{Phone: &phone, Tags: &tags}, testNow); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if p.Phone != phone {
		t.Errorf("phone = %q, want %q", p.Phone, phone)
	}
	// Trimmed, deduped, empties dropped.
	if len(p.Tags) != 2 || p.Tags[0] != "vip" || p.Tags[1] != "regular" {
		t.Errorf("tags = %v, want [vip regular]", p.Tags)
	}

	// A nil field keeps its value; a non-nil empty clears it.
	notes := "prefers quiet room"
	if err := p.ApplyPracticePatch(PracticePatch{PracticeNotes: &notes}, testNow); err != nil {
		t.Fatalf("patch notes: %v", err)
	}
	if p.Phone != phone {
		t.Errorf("phone = %q, want untouched %q", p.Phone, phone)
	}
	empty := ""
	if err := p.ApplyPracticePatch(PracticePatch{Phone: &empty}, testNow); err != nil {
		t.Fatalf("clear phone: %v", err)
	}
	if p.Phone != "" {
		t.Errorf("phone = %q, want cleared", p.Phone)
	}
}

func TestApplyPracticePatchValidation(t *testing.T) {
	p := New("user-1", testNow)
	if err := p.ApplyPracticePatch(PracticePatch{Phone: strPtr(strings.Repeat("1", MaxPhoneLen+1))}, testNow); err != ErrPhoneTooLong {
		t.Errorf("err = %v, want ErrPhoneTooLong", err)
	}
	if err := p.ApplyPracticePatch(PracticePatch{PracticeNotes: strPtr(strings.Repeat("x", MaxPracticeNotesLen+1))}, testNow); err != ErrPracticeNotesTooLong {
		t.Errorf("err = %v, want ErrPracticeNotesTooLong", err)
	}
	tags := make([]string, MaxTags+1)
	for i := range tags {
		tags[i] = strings.Repeat("t", 1) + strings.Repeat("0", i%3) + string(rune('a'+i))
	}
	if err := p.ApplyPracticePatch(PracticePatch{Tags: &tags}, testNow); err != ErrTooManyTags {
		t.Errorf("err = %v, want ErrTooManyTags", err)
	}
	longTag := []string{strings.Repeat("x", MaxTagLen+1)}
	if err := p.ApplyPracticePatch(PracticePatch{Tags: &longTag}, testNow); err != ErrTagTooLong {
		t.Errorf("err = %v, want ErrTagTooLong", err)
	}
}
