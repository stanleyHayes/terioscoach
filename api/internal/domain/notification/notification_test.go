package notification

import (
	"errors"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func newTestJob(t *testing.T) Job {
	t.Helper()
	j, err := New(KindBookingConfirmation, "ama@example.com", map[string]string{"clientName": "Ama"}, fixedNow, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return j
}

// TestNewValidates: a job needs a known kind and somewhere to go.
func TestNewValidates(t *testing.T) {
	if _, err := New("not_a_kind", "ama@example.com", nil, fixedNow, fixedNow); !errors.Is(err, ErrInvalidKind) {
		t.Errorf("unknown kind err = %v, want ErrInvalidKind", err)
	}
	if _, err := New(KindSessionReminder, "", nil, fixedNow, fixedNow); !errors.Is(err, ErrRecipientRequired) {
		t.Errorf("empty recipient err = %v, want ErrRecipientRequired", err)
	}
}

// TestNewCopiesData: the job owns its payload, so a caller mutating the map
// afterwards cannot rewrite a queued message.
func TestNewCopiesData(t *testing.T) {
	data := map[string]string{"clientName": "Ama"}
	j, err := New(KindBookingConfirmation, "ama@example.com", data, fixedNow, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	data["clientName"] = "Someone Else"
	if j.Data["clientName"] != "Ama" {
		t.Errorf("clientName = %q, want the value captured at queue time", j.Data["clientName"])
	}
}

// TestDue: only pending jobs whose time has come are due.
func TestDue(t *testing.T) {
	j := newTestJob(t)
	j.DueAt = fixedNow.Add(time.Hour)

	if j.Due(fixedNow) {
		t.Error("job due an hour early")
	}
	if !j.Due(fixedNow.Add(time.Hour)) {
		t.Error("job not due at its due time")
	}
	if !j.Due(fixedNow.Add(2 * time.Hour)) {
		t.Error("overdue job reported not due")
	}

	j.Status = StatusCancelled
	if j.Due(fixedNow.Add(2 * time.Hour)) {
		t.Error("cancelled job reported due — the dispatcher would resurrect it")
	}
}

// TestMarkSentIsOneWay: a delivered job cannot be re-delivered or recalled.
func TestMarkSentIsOneWay(t *testing.T) {
	j := newTestJob(t)
	if err := j.MarkSent(fixedNow); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	if j.Status != StatusSent || j.SentAt == nil || !j.SentAt.Equal(fixedNow) {
		t.Errorf("job = %+v, want sent and stamped", j)
	}
	if err := j.MarkSent(fixedNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("second MarkSent = %v, want ErrInvalidTransition", err)
	}
	if err := j.Cancel(fixedNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("cancel after send = %v, want ErrInvalidTransition — a sent email cannot be recalled", err)
	}
}

// TestRetryBacksOffThenFails: failures reschedule with growing backoff until
// the budget is spent, then terminate.
func TestRetryBacksOffThenFails(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 3, Backoffs: []time.Duration{time.Minute, 10 * time.Minute}}
	j := newTestJob(t)

	if err := j.RecordFailure("smtp timeout", policy, fixedNow); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	if j.Status != StatusPending || j.Attempts != 1 {
		t.Fatalf("job = %+v, want still pending after one failure", j)
	}
	if !j.DueAt.Equal(fixedNow.Add(time.Minute)) {
		t.Errorf("dueAt = %v, want the first backoff applied", j.DueAt)
	}
	if j.LastError != "smtp timeout" {
		t.Errorf("lastError = %q, want the provider's reason kept", j.LastError)
	}

	if err := j.RecordFailure("smtp timeout", policy, fixedNow); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	if !j.DueAt.Equal(fixedNow.Add(10 * time.Minute)) {
		t.Errorf("dueAt = %v, want the second, longer backoff", j.DueAt)
	}

	if err := j.RecordFailure("smtp timeout", policy, fixedNow); err != nil {
		t.Fatalf("RecordFailure: %v", err)
	}
	if j.Status != StatusFailed {
		t.Errorf("status = %q after %d attempts, want failed", j.Status, policy.MaxAttempts)
	}
	if j.Due(fixedNow.Add(24 * time.Hour)) {
		t.Error("a failed job is still due — it would retry forever")
	}
}

// TestRetryBackoffRepeatsLastEntry: more attempts than backoffs keeps the
// longest wait rather than panicking or resetting to the shortest.
func TestRetryBackoffRepeatsLastEntry(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 6, Backoffs: []time.Duration{time.Minute, time.Hour}}
	j := newTestJob(t)

	for i := 0; i < 4; i++ {
		if err := j.RecordFailure("down", policy, fixedNow); err != nil {
			t.Fatalf("RecordFailure %d: %v", i+1, err)
		}
	}
	if !j.DueAt.Equal(fixedNow.Add(time.Hour)) {
		t.Errorf("dueAt = %v, want the last backoff repeated", j.DueAt)
	}
}

// TestZeroRetryPolicyStillTerminates: a misconfigured policy must not turn
// into an infinite retry loop.
func TestZeroRetryPolicyStillTerminates(t *testing.T) {
	var zero RetryPolicy
	j := newTestJob(t)

	for i := 0; i < DefaultRetryPolicy().MaxAttempts; i++ {
		if err := j.RecordFailure("down", zero, fixedNow); err != nil {
			t.Fatalf("RecordFailure %d: %v", i+1, err)
		}
	}
	if j.Status != StatusFailed {
		t.Errorf("status = %q, want the default attempt cap applied", j.Status)
	}
}

// TestCancelStopsDelivery: cancelling a booking's reminder takes it out of
// the dispatcher's reach.
func TestCancelStopsDelivery(t *testing.T) {
	j := newTestJob(t)
	if err := j.Cancel(fixedNow); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if j.Status != StatusCancelled {
		t.Errorf("status = %q, want cancelled", j.Status)
	}
	if err := j.RecordFailure("x", DefaultRetryPolicy(), fixedNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("RecordFailure on cancelled = %v, want ErrInvalidTransition", err)
	}
}

// TestReminderDueAt: the reminder lands one lead time before the session,
// and is skipped when the booking is made inside that window.
func TestReminderDueAt(t *testing.T) {
	start := fixedNow.Add(72 * time.Hour)

	due, ok := ReminderDueAt(start, DefaultReminderLead, fixedNow)
	if !ok {
		t.Fatal("no reminder for a session three days out, want one")
	}
	if !due.Equal(start.Add(-DefaultReminderLead)) {
		t.Errorf("due = %v, want one lead time before %v", due, start)
	}

	if _, ok := ReminderDueAt(fixedNow.Add(2*time.Hour), DefaultReminderLead, fixedNow); ok {
		t.Error("scheduled a reminder for a session inside the lead time — it would arrive with the confirmation")
	}
	if _, ok := ReminderDueAt(fixedNow.Add(DefaultReminderLead), DefaultReminderLead, fixedNow); ok {
		t.Error("scheduled a reminder due exactly now, want it skipped")
	}

	// A zero lead falls back to the default rather than firing immediately.
	due, ok = ReminderDueAt(start, 0, fixedNow)
	if !ok || !due.Equal(start.Add(-DefaultReminderLead)) {
		t.Errorf("zero lead gave (%v, %v), want the default lead applied", due, ok)
	}
}

// TestSortByDue: a backlog is worked oldest-first.
func TestSortByDue(t *testing.T) {
	jobs := []Job{
		{ID: "c", DueAt: fixedNow.Add(time.Hour)},
		{ID: "a", DueAt: fixedNow.Add(-time.Hour)},
		{ID: "b", DueAt: fixedNow},
	}
	SortByDue(jobs)
	for i, want := range []string{"a", "b", "c"} {
		if jobs[i].ID != want {
			t.Fatalf("order = %s at %d, want %s (oldest due first)", jobs[i].ID, i, want)
		}
	}
}
