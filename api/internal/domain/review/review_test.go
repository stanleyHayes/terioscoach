package review

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func newValid(t *testing.T) Review {
	t.Helper()
	r, err := New("booking-1", "client-1", "prac-1", "svc-1", 5, "  Wonderful session.  ", fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return r
}

// TestNewStartsPending: a review is client-authored content published under
// the practice's name, so it waits for approval.
func TestNewStartsPending(t *testing.T) {
	r := newValid(t)
	if r.Status != StatusPending || r.Public() {
		t.Errorf("review = %+v, want pending and not public", r)
	}
	if r.Comment != "Wonderful session." {
		t.Errorf("comment = %q, want it trimmed", r.Comment)
	}
	if r.ModeratedAt != nil {
		t.Error("moderatedAt set before moderation")
	}
	if r.BookingID != "booking-1" || r.PractitionerID != "prac-1" || r.ServiceID != "svc-1" {
		t.Errorf("review = %+v, want the parties denormalized from the booking", r)
	}
}

// TestRatingBounds: the scale is 1–5 and nothing else.
func TestRatingBounds(t *testing.T) {
	for _, rating := range []int{0, -1, 6, 100} {
		if _, err := New("booking-1", "client-1", "prac-1", "svc-1", rating, "", fixedNow); !errors.Is(err, ErrInvalidRating) {
			t.Errorf("rating %d err = %v, want ErrInvalidRating", rating, err)
		}
	}
	for rating := MinRating; rating <= MaxRating; rating++ {
		if _, err := New("booking-1", "client-1", "prac-1", "svc-1", rating, "", fixedNow); err != nil {
			t.Errorf("rating %d err = %v, want it accepted", rating, err)
		}
	}
}

// TestNewValidation covers the remaining content rules.
func TestNewValidation(t *testing.T) {
	if _, err := New("", "client-1", "prac-1", "svc-1", 5, "", fixedNow); !errors.Is(err, ErrInvalidBooking) {
		t.Errorf("missing booking err = %v, want ErrInvalidBooking", err)
	}
	if _, err := New("booking-1", "", "prac-1", "svc-1", 5, "", fixedNow); !errors.Is(err, ErrInvalidBooking) {
		t.Errorf("missing client err = %v, want ErrInvalidBooking", err)
	}
	long := strings.Repeat("x", MaxCommentLen+1)
	if _, err := New("booking-1", "client-1", "prac-1", "svc-1", 5, long, fixedNow); !errors.Is(err, ErrCommentTooLong) {
		t.Errorf("long comment err = %v, want ErrCommentTooLong", err)
	}
}

// TestEditingIsPendingOnly is the rule that keeps the public site honest: a
// client cannot swap the text of a review the practitioner already approved.
func TestEditingIsPendingOnly(t *testing.T) {
	r := newValid(t)

	rating := 4
	comment := "On reflection, four stars."
	if err := r.Apply(Patch{Rating: &rating, Comment: &comment}, fixedNow); err != nil {
		t.Fatalf("Apply while pending: %v", err)
	}
	if r.Rating != 4 || r.Comment != "On reflection, four stars." {
		t.Errorf("review = %+v, want the edit applied", r)
	}

	r.Approve(fixedNow)
	one := 1
	abusive := "Actually, terrible."
	if err := r.Apply(Patch{Rating: &one, Comment: &abusive}, fixedNow); !errors.Is(err, ErrAlreadyModerated) {
		t.Fatalf("Apply after approval = %v, want ErrAlreadyModerated", err)
	}
	if r.Rating != 4 || r.Comment != "On reflection, four stars." {
		t.Errorf("review = %+v, want the approved content untouched", r)
	}

	// A rejected review is equally closed to edits — otherwise a client
	// could rewrite it and wait for a second look at different text.
	r.Reject(fixedNow)
	if err := r.Apply(Patch{Comment: &abusive}, fixedNow); !errors.Is(err, ErrAlreadyModerated) {
		t.Errorf("Apply after rejection = %v, want ErrAlreadyModerated", err)
	}
}

// TestEditValidation: an edit is held to the same content rules.
func TestEditValidation(t *testing.T) {
	r := newValid(t)
	bad := 9
	if err := r.Apply(Patch{Rating: &bad}, fixedNow); !errors.Is(err, ErrInvalidRating) {
		t.Errorf("err = %v, want ErrInvalidRating", err)
	}
	long := strings.Repeat("x", MaxCommentLen+1)
	if err := r.Apply(Patch{Comment: &long}, fixedNow); !errors.Is(err, ErrCommentTooLong) {
		t.Errorf("err = %v, want ErrCommentTooLong", err)
	}
	if r.Rating != 5 {
		t.Errorf("rating = %d, want the review untouched by a rejected edit", r.Rating)
	}
}

// TestModerationIsIdempotentAndReversible.
func TestModerationIsIdempotentAndReversible(t *testing.T) {
	r := newValid(t)

	r.Approve(fixedNow)
	if !r.Public() || r.ModeratedAt == nil {
		t.Fatalf("review = %+v, want approved and stamped", r)
	}
	first := *r.ModeratedAt

	r.Approve(fixedNow.Add(time.Hour))
	if !r.ModeratedAt.Equal(first) {
		t.Errorf("moderatedAt = %v, want the first stamp %v kept", r.ModeratedAt, first)
	}

	r.Reject(fixedNow.Add(2 * time.Hour))
	if r.Public() {
		t.Error("a rejected review is still public")
	}
	r.Approve(fixedNow.Add(3 * time.Hour))
	if !r.Public() {
		t.Error("a re-approved review is not public")
	}
}

// TestSummarizeCountsApprovedOnly: the average beside a list of reviews
// must be the average *of that list*.
func TestSummarizeCountsApprovedOnly(t *testing.T) {
	build := func(rating int, status Status) Review {
		r, err := New("booking-1", "client-1", "prac-1", "svc-1", rating, "", fixedNow)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		r.Status = status
		return r
	}

	summary := Summarize([]Review{
		build(5, StatusApproved),
		build(4, StatusApproved),
		build(1, StatusPending),  // not yet moderated
		build(1, StatusRejected), // never going live
	})

	if summary.Count != 2 {
		t.Errorf("count = %d, want only the approved reviews", summary.Count)
	}
	if summary.Average != 4.5 {
		t.Errorf("average = %v, want 4.5", summary.Average)
	}
	if summary.Distribution[5] != 1 || summary.Distribution[4] != 1 || summary.Distribution[1] != 0 {
		t.Errorf("distribution = %v, want only approved ratings counted", summary.Distribution)
	}
}

// TestSummarizeRoundsAndHandlesEmpty.
func TestSummarizeRoundsAndHandlesEmpty(t *testing.T) {
	empty := Summarize(nil)
	if empty.Count != 0 || empty.Average != 0 {
		t.Errorf("summary = %+v, want a zero summary for no reviews", empty)
	}

	build := func(rating int) Review {
		r, err := New("booking-1", "client-1", "prac-1", "svc-1", rating, "", fixedNow)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		r.Status = StatusApproved
		return r
	}
	// 5, 4, 4 → 4.333… → 4.3
	summary := Summarize([]Review{build(5), build(4), build(4)})
	if summary.Average != 4.3 {
		t.Errorf("average = %v, want 4.3", summary.Average)
	}
	// 5, 4 → 4.5 exactly; 4, 4, 5, 5 → 4.5
	if got := Summarize([]Review{build(4), build(4), build(5), build(5)}).Average; got != 4.5 {
		t.Errorf("average = %v, want 4.5", got)
	}
}

// TestStatusValidity guards the enum against values entering from storage.
func TestStatusValidity(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusApproved, StatusRejected} {
		if !status.Valid() {
			t.Errorf("%q is not valid, want it in the known set", status)
		}
	}
	if Status("").Valid() || Status("published").Valid() {
		t.Error("Status.Valid accepted a value outside the known set")
	}
}
