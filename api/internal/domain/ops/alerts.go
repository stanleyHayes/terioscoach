// Package ops holds the operational health rules: what counts as a
// condition worth waking someone for, and when a condition has cleared.
//
// It is pure. It knows nothing about MongoDB, Resend or HTTP — it is given
// counts and a clock, and it returns judgements. That is what makes the
// thresholds testable without a running system, which is the only way a
// threshold ever gets checked at all.
package ops

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Severity is how loudly a condition should be reported.
type Severity string

const (
	// SeverityWarning is a condition worth looking at today.
	SeverityWarning Severity = "warning"
	// SeverityCritical is a condition that is already costing the practice
	// something: sessions unconfirmed, clients locked out, money unrecorded.
	SeverityCritical Severity = "critical"
)

// Kind identifies what has gone wrong. It is a closed set, because an
// alert nobody has decided how to respond to is noise.
type Kind string

const (
	// KindNotificationBacklog means messages are queued and not going out.
	// Every one is a client who was not told something.
	KindNotificationBacklog Kind = "notification_backlog"
	// KindNotificationFailures means delivery is being attempted and
	// failing — a different problem from a backlog, and usually the
	// provider or a bad sender domain.
	KindNotificationFailures Kind = "notification_failures"
	// KindLockoutSpike means many accounts are hitting the brute-force
	// lockout at once, which is what credential stuffing looks like from
	// the inside.
	KindLockoutSpike Kind = "lockout_spike"
	// KindPaymentVerificationFailures means the gateway is answering but
	// not agreeing, so payments are not being recorded.
	KindPaymentVerificationFailures Kind = "payment_verification_failures"
)

// Alert is one active condition.
type Alert struct {
	Kind     Kind
	Severity Severity
	// Observed is the measured value that tripped the threshold.
	Observed int
	// Threshold is the value it had to exceed.
	Threshold int
	// Since is when the condition first tripped, so a responder can tell
	// a spike from something that has been broken all week.
	Since time.Time
	// Summary is one sentence, written for whoever is woken up.
	Summary string
}

// Thresholds are the numbers a condition has to cross. They are values,
// not constants, so a deployment can tune them without a code change and a
// test can drive them without waiting.
type Thresholds struct {
	// NotificationBacklog is how many pending-and-overdue jobs are too
	// many. A handful is a slow minute; a hundred is a broken worker.
	NotificationBacklogWarning  int
	NotificationBacklogCritical int
	// NotificationFailures counts jobs that have exhausted every retry
	// within Window.
	NotificationFailuresWarning  int
	NotificationFailuresCritical int
	// Lockouts counts distinct accounts locked within Window.
	LockoutWarning  int
	LockoutCritical int
	// PaymentVerificationFailures counts gateway verifications that
	// errored within Window.
	PaymentFailuresWarning  int
	PaymentFailuresCritical int
	// Window is how far back the rate-based counts look.
	Window time.Duration
}

// DefaultThresholds are tuned for one practitioner's practice.
//
// The numbers are deliberately low. This is not a system with background
// noise to filter out: on a normal day the failure counts are zero, so a
// threshold of five is already "something is wrong", not "traffic is up".
func DefaultThresholds() Thresholds {
	return Thresholds{
		NotificationBacklogWarning:   20,
		NotificationBacklogCritical:  100,
		NotificationFailuresWarning:  3,
		NotificationFailuresCritical: 10,
		// One person locking themselves out is a Monday. Five different
		// accounts in fifteen minutes is not.
		LockoutWarning:          5,
		LockoutCritical:         20,
		PaymentFailuresWarning:  3,
		PaymentFailuresCritical: 10,
		Window:                  15 * time.Minute,
	}
}

// Snapshot is what the system currently measures.
type Snapshot struct {
	// NotificationBacklog is pending jobs whose run time has passed.
	NotificationBacklog int
	// NotificationFailures is jobs that gave up within the window.
	NotificationFailures int
	// LockedAccounts is distinct accounts locked within the window.
	LockedAccounts int
	// PaymentVerificationFailures is gateway errors within the window.
	PaymentVerificationFailures int
}

// Evaluate turns a snapshot into the conditions that are active.
//
// Returned in severity order, worst first: whoever reads this list acts on
// the top of it, so the top must be the thing that matters most.
func Evaluate(snapshot Snapshot, t Thresholds, since map[Kind]time.Time, now time.Time) []Alert {
	checks := []struct {
		kind     Kind
		observed int
		warning  int
		critical int
		summary  func(int) string
	}{
		{
			KindNotificationBacklog, snapshot.NotificationBacklog,
			t.NotificationBacklogWarning, t.NotificationBacklogCritical,
			func(n int) string {
				return fmt.Sprintf(
					"%d notifications are queued past their send time — clients are not being told about their sessions", n)
			},
		},
		{
			KindNotificationFailures, snapshot.NotificationFailures,
			t.NotificationFailuresWarning, t.NotificationFailuresCritical,
			func(n int) string {
				return fmt.Sprintf(
					"%d notifications gave up after every retry in the last %s — check the mail provider and the sender domain",
					n, t.Window)
			},
		},
		{
			KindLockoutSpike, snapshot.LockedAccounts,
			t.LockoutWarning, t.LockoutCritical,
			func(n int) string {
				return fmt.Sprintf(
					"%d accounts hit the brute-force lockout in the last %s — this is what credential stuffing looks like",
					n, t.Window)
			},
		},
		{
			KindPaymentVerificationFailures, snapshot.PaymentVerificationFailures,
			t.PaymentFailuresWarning, t.PaymentFailuresCritical,
			func(n int) string {
				return fmt.Sprintf(
					"%d payment verifications failed in the last %s — payments may be taken and not recorded", n, t.Window)
			},
		},
	}

	var alerts []Alert
	for _, check := range checks {
		severity, ok := classify(check.observed, check.warning, check.critical)
		if !ok {
			continue
		}
		threshold := check.warning
		if severity == SeverityCritical {
			threshold = check.critical
		}
		start, seen := since[check.kind]
		if !seen {
			start = now
		}
		alerts = append(alerts, Alert{
			Kind:      check.kind,
			Severity:  severity,
			Observed:  check.observed,
			Threshold: threshold,
			Since:     start,
			Summary:   check.summary(check.observed),
		})
	}

	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity == SeverityCritical
		}
		return alerts[i].Kind < alerts[j].Kind
	})
	return alerts
}

// classify reports which band a value falls in, and whether it is in one
// at all. A zero or negative threshold disables that band, so a deployment
// can turn off a check it cannot act on rather than learning to ignore it.
func classify(observed, warning, critical int) (Severity, bool) {
	if critical > 0 && observed >= critical {
		return SeverityCritical, true
	}
	if warning > 0 && observed >= warning {
		return SeverityWarning, true
	}
	return "", false
}

// Tracker holds the rate counters the snapshot is built from.
//
// It keeps timestamps rather than a running total, because the question is
// always "how many in the last N minutes" and a running total cannot
// answer that without also knowing when to forget.
type Tracker struct {
	mu     sync.Mutex
	window time.Duration
	events map[Kind][]time.Time
	since  map[Kind]time.Time
	now    func() time.Time
}

// NewTracker builds a tracker over the given window.
func NewTracker(window time.Duration) *Tracker {
	if window <= 0 {
		window = 15 * time.Minute
	}
	return &Tracker{
		window: window,
		events: make(map[Kind][]time.Time),
		since:  make(map[Kind]time.Time),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// Record notes one occurrence of a kind.
func (t *Tracker) Record(kind Kind) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.events[kind] = append(prune(t.events[kind], now.Add(-t.window)), now)
}

// Count returns how many of a kind fall inside the window.
func (t *Tracker) Count(kind Kind) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	kept := prune(t.events[kind], now.Add(-t.window))
	t.events[kind] = kept
	return len(kept)
}

// Since returns when each currently-active condition first tripped.
//
// Read before Evaluate and written after it, so an alert reports when the
// problem started rather than when it was last looked at.
func (t *Tracker) Since() map[Kind]time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[Kind]time.Time, len(t.since))
	for kind, at := range t.since {
		out[kind] = at
	}
	return out
}

// NoteActive records when a condition first became active and clears the
// record when it stops, so `Since` on an alert means "since it started"
// rather than "since it was last observed".
//
// It takes `now` rather than reading the clock itself so that the stored
// start is the *same instant* the alert was evaluated at. Two clock reads
// microseconds apart would make the first response's Since disagree with
// every later one — which reads as the incident restarting on every poll.
func (t *Tracker) NoteActive(active []Alert, now time.Time) map[Kind]time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()

	current := make(map[Kind]bool, len(active))
	for _, alert := range active {
		current[alert.Kind] = true
		if _, seen := t.since[alert.Kind]; !seen {
			t.since[alert.Kind] = now
		}
	}
	for kind := range t.since {
		if !current[kind] {
			delete(t.since, kind)
		}
	}

	out := make(map[Kind]time.Time, len(t.since))
	for kind, at := range t.since {
		out[kind] = at
	}
	return out
}

// prune drops timestamps at or before the cutoff. The slice is sorted by
// construction — appends only ever add `now` — so the first kept index
// ends the scan.
func prune(events []time.Time, cutoff time.Time) []time.Time {
	for i, at := range events {
		if at.After(cutoff) {
			if i == 0 {
				return events
			}
			return events[i:]
		}
	}
	return nil
}
