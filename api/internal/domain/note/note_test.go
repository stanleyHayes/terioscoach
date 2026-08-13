package note

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)

func TestReplaceContentValidation(t *testing.T) {
	n := New("booking-1", "client-1", "prac-1", testNow)

	if err := n.ReplaceContent(strings.Repeat("x", MaxPrivateNotesLen+1), "", nil, testNow); err != ErrPrivateNotesTooLong {
		t.Errorf("err = %v, want ErrPrivateNotesTooLong", err)
	}
	if err := n.ReplaceContent("", strings.Repeat("x", MaxSharedFeedbackLen+1), nil, testNow); err != ErrSharedFeedbackTooLong {
		t.Errorf("err = %v, want ErrSharedFeedbackTooLong", err)
	}
	if err := n.ReplaceContent("", "", make([]string, MaxSharedResources+1), testNow); err != ErrTooManyResources {
		t.Errorf("err = %v, want ErrTooManyResources", err)
	}
	if err := n.ReplaceContent("", "", []string{strings.Repeat("x", MaxResourceLen+1)}, testNow); err != ErrResourceTooLong {
		t.Errorf("err = %v, want ErrResourceTooLong", err)
	}
	if err := n.ReplaceContent("private", "feedback", []string{"https://example.com/aftercare"}, testNow); err != nil {
		t.Fatalf("valid ReplaceContent: %v", err)
	}
	if n.PrivateNotes != "private" || n.SharedFeedback != "feedback" || len(n.SharedResources) != 1 {
		t.Errorf("note = %+v, want content replaced", n)
	}
}

// TestVisibilityRuleUnshared: a client sees nothing until the practitioner
// shares — ClientView reports not-visible.
func TestVisibilityRuleUnshared(t *testing.T) {
	n := New("booking-1", "client-1", "prac-1", testNow)
	if err := n.ReplaceContent("secret diagnosis", "well done", nil, testNow); err != nil {
		t.Fatalf("ReplaceContent: %v", err)
	}
	if n.IsShared() {
		t.Fatal("new note must not be shared")
	}
	if _, visible := n.ClientView(); visible {
		t.Fatal("unshared note must not be visible to the client")
	}
}

// TestVisibilityRuleShared: once shared, the client view carries the shared
// fields only — private notes are absent by construction.
func TestVisibilityRuleShared(t *testing.T) {
	n := New("booking-1", "client-1", "prac-1", testNow)
	if err := n.ReplaceContent("secret diagnosis", "well done", []string{"https://example.com/aftercare"}, testNow); err != nil {
		t.Fatalf("ReplaceContent: %v", err)
	}
	n.Share(testNow.Add(time.Hour))

	view, visible := n.ClientView()
	if !visible {
		t.Fatal("shared note must be visible to the client")
	}
	if view.Feedback != "well done" || len(view.Resources) != 1 || view.BookingID != "booking-1" {
		t.Errorf("view = %+v, want shared content", view)
	}
	if !view.SharedAt.Equal(testNow.Add(time.Hour)) {
		t.Errorf("sharedAt = %v, want the share stamp", view.SharedAt)
	}
	// The projection has no private-notes field at all — the compiler
	// enforces it; nothing to assert at runtime.
}

// TestShareIdempotent: the first share stamps; repeats keep the stamp.
func TestShareIdempotent(t *testing.T) {
	n := New("booking-1", "client-1", "prac-1", testNow)
	first := testNow.Add(time.Hour)
	n.Share(first)
	n.Share(first.Add(48 * time.Hour))
	if !n.SharedAt.Equal(first) {
		t.Errorf("sharedAt = %v, want the first stamp kept", n.SharedAt)
	}
}

// TestReplaceContentKeepsSharedAt: editing after sharing neither unshares
// nor re-stamps.
func TestReplaceContentKeepsSharedAt(t *testing.T) {
	n := New("booking-1", "client-1", "prac-1", testNow)
	n.Share(testNow.Add(time.Hour))
	stamp := *n.SharedAt
	if err := n.ReplaceContent("new private", "new feedback", nil, testNow.Add(2*time.Hour)); err != nil {
		t.Fatalf("ReplaceContent: %v", err)
	}
	if n.SharedAt == nil || !n.SharedAt.Equal(stamp) {
		t.Errorf("sharedAt = %v, want untouched %v", n.SharedAt, stamp)
	}
}
