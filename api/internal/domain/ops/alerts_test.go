package ops

import (
	"strings"
	"testing"
	"time"
)

func TestQuietSystemRaisesNothing(t *testing.T) {
	alerts := Evaluate(Snapshot{}, DefaultThresholds(), nil, time.Now())
	if len(alerts) != 0 {
		t.Fatalf("a healthy system produced %d alerts: %+v", len(alerts), alerts)
	}
}

// TestThresholdsAreInclusive: an alert set at 5 must fire at 5. Firing at
// 6 means the number in the runbook is not the number in the code.
func TestThresholdsAreInclusive(t *testing.T) {
	thresholds := DefaultThresholds()

	below := Evaluate(Snapshot{LockedAccounts: thresholds.LockoutWarning - 1}, thresholds, nil, time.Now())
	if len(below) != 0 {
		t.Errorf("fired below the threshold: %+v", below)
	}

	at := Evaluate(Snapshot{LockedAccounts: thresholds.LockoutWarning}, thresholds, nil, time.Now())
	if len(at) != 1 || at[0].Severity != SeverityWarning {
		t.Errorf("at the threshold got %+v, want one warning", at)
	}
}

func TestCriticalOutranksWarning(t *testing.T) {
	thresholds := DefaultThresholds()
	alerts := Evaluate(
		Snapshot{LockedAccounts: thresholds.LockoutCritical},
		thresholds, nil, time.Now(),
	)

	if len(alerts) != 1 {
		t.Fatalf("got %d alerts, want 1", len(alerts))
	}
	// A value past both bands is one critical alert, not one of each —
	// two alerts for one condition is how a pager gets ignored.
	if alerts[0].Severity != SeverityCritical {
		t.Errorf("severity = %q, want critical", alerts[0].Severity)
	}
	if alerts[0].Threshold != thresholds.LockoutCritical {
		t.Errorf("threshold = %d, want the critical one", alerts[0].Threshold)
	}
}

// TestWorstFirst: whoever reads this list acts on the top of it.
func TestWorstFirst(t *testing.T) {
	thresholds := DefaultThresholds()
	alerts := Evaluate(Snapshot{
		NotificationBacklog:         thresholds.NotificationBacklogWarning,
		LockedAccounts:              thresholds.LockoutCritical,
		PaymentVerificationFailures: thresholds.PaymentFailuresWarning,
	}, thresholds, nil, time.Now())

	if len(alerts) != 3 {
		t.Fatalf("got %d alerts, want 3", len(alerts))
	}
	if alerts[0].Severity != SeverityCritical {
		t.Errorf("first alert is %q, want the critical one first", alerts[0].Severity)
	}
}

// TestSummariesSayWhatItCostsSomeone. An alert that says
// "notification_backlog: 42" tells whoever is woken nothing they can act on.
func TestSummariesSayWhatItCostsSomeone(t *testing.T) {
	thresholds := DefaultThresholds()
	alerts := Evaluate(Snapshot{
		NotificationBacklog:         thresholds.NotificationBacklogCritical,
		NotificationFailures:        thresholds.NotificationFailuresCritical,
		LockedAccounts:              thresholds.LockoutCritical,
		PaymentVerificationFailures: thresholds.PaymentFailuresCritical,
	}, thresholds, nil, time.Now())

	for _, alert := range alerts {
		if len(alert.Summary) < 40 {
			t.Errorf("%s summary is too terse to act on: %q", alert.Kind, alert.Summary)
		}
		if !strings.Contains(alert.Summary, "—") {
			t.Errorf("%s summary states a number but not a consequence: %q", alert.Kind, alert.Summary)
		}
	}
}

// TestSinceSurvivesRepeatedEvaluation: a responder has to be able to tell
// a spike from something that has been broken all week.
func TestSinceSurvivesRepeatedEvaluation(t *testing.T) {
	thresholds := DefaultThresholds()
	tracker := NewTracker(thresholds.Window)
	start := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return start }

	snapshot := Snapshot{LockedAccounts: thresholds.LockoutWarning}
	first := Evaluate(snapshot, thresholds, nil, start)
	since := tracker.NoteActive(first, start)

	later := start.Add(30 * time.Minute)
	tracker.now = func() time.Time { return later }
	second := Evaluate(snapshot, thresholds, since, later)
	tracker.NoteActive(second, later)

	if len(second) != 1 {
		t.Fatalf("got %d alerts, want 1", len(second))
	}
	if !second[0].Since.Equal(start) {
		t.Errorf("Since = %v, want the first sighting %v", second[0].Since, start)
	}
}

// TestSinceResetsAfterRecovery: a condition that cleared and came back is
// a new incident, not a continuing one.
func TestSinceResetsAfterRecovery(t *testing.T) {
	thresholds := DefaultThresholds()
	tracker := NewTracker(thresholds.Window)
	start := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return start }

	tripped := Snapshot{LockedAccounts: thresholds.LockoutWarning}
	since := tracker.NoteActive(Evaluate(tripped, thresholds, nil, start), start)

	recovered := start.Add(10 * time.Minute)
	tracker.now = func() time.Time { return recovered }
	since = tracker.NoteActive(Evaluate(Snapshot{}, thresholds, since, recovered), recovered)
	if len(since) != 0 {
		t.Fatalf("a cleared condition is still being tracked: %+v", since)
	}

	again := start.Add(20 * time.Minute)
	tracker.now = func() time.Time { return again }
	alerts := Evaluate(tripped, thresholds, since, again)
	tracker.NoteActive(alerts, again)

	if !alerts[0].Since.Equal(again) {
		t.Errorf("Since = %v, want the new incident's start %v", alerts[0].Since, again)
	}
}

func TestZeroThresholdDisablesACheck(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.LockoutWarning = 0
	thresholds.LockoutCritical = 0

	alerts := Evaluate(Snapshot{LockedAccounts: 1000}, thresholds, nil, time.Now())

	// Turning a check off is better than leaving one nobody acts on.
	if len(alerts) != 0 {
		t.Errorf("a disabled check still fired: %+v", alerts)
	}
}

func TestTrackerCountsOnlyInsideItsWindow(t *testing.T) {
	tracker := NewTracker(15 * time.Minute)
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return now }

	for range 3 {
		tracker.Record(KindLockoutSpike)
	}
	if got := tracker.Count(KindLockoutSpike); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}

	// Two more, twenty minutes later. The first three have aged out.
	now = now.Add(20 * time.Minute)
	tracker.Record(KindLockoutSpike)
	tracker.Record(KindLockoutSpike)

	if got := tracker.Count(KindLockoutSpike); got != 2 {
		t.Errorf("count = %d, want 2 — events outside the window must not accumulate forever", got)
	}
}

func TestTrackerKeepsKindsApart(t *testing.T) {
	tracker := NewTracker(time.Minute)
	tracker.Record(KindLockoutSpike)
	tracker.Record(KindNotificationFailures)
	tracker.Record(KindNotificationFailures)

	if got := tracker.Count(KindLockoutSpike); got != 1 {
		t.Errorf("lockouts = %d, want 1", got)
	}
	if got := tracker.Count(KindNotificationFailures); got != 2 {
		t.Errorf("failures = %d, want 2", got)
	}
}

func TestTrackerIsSafeUnderConcurrentUse(t *testing.T) {
	tracker := NewTracker(time.Hour)
	done := make(chan struct{})

	for range 8 {
		go func() {
			for range 50 {
				tracker.Record(KindNotificationFailures)
				tracker.Count(KindNotificationFailures)
			}
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}

	// Every event is recorded exactly once — a dropped one understates the
	// problem, which is the failure mode that matters here.
	if got := tracker.Count(KindNotificationFailures); got != 400 {
		t.Errorf("count = %d, want 400", got)
	}
}
