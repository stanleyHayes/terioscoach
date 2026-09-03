// Package notification is the domain core for outbound messages: what kinds
// exist, the delivery job's lifecycle, and the retry and reminder-timing
// rules. It imports nothing outside the standard library — no frameworks,
// no drivers, no mail provider.
//
// Every message is a durable job rather than an inline send. A booking must
// never fail because a mail provider is briefly unreachable, and a reminder
// must survive a restart of the process that scheduled it — both fall out of
// writing the job down first and delivering it separately.
package notification

import (
	"sort"
	"time"
)

// Kind identifies which message a job carries. It selects the template and
// the subject line; the payload travels in Job.Data.
type Kind string

const (
	KindBookingPaymentRequired Kind = "booking_payment_required"
	KindBookingConfirmation    Kind = "booking_confirmation"
	KindSessionReminder        Kind = "session_reminder"
	KindBookingRescheduled     Kind = "booking_rescheduled"
	KindBookingCancelled       Kind = "booking_cancelled"
	KindFeedbackShared         Kind = "feedback_shared"
	KindEnquiryReceived        Kind = "enquiry_received"
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	switch k {
	case KindBookingPaymentRequired, KindBookingConfirmation, KindSessionReminder, KindBookingRescheduled,
		KindBookingCancelled, KindFeedbackShared, KindEnquiryReceived:
		return true
	}
	return false
}

// Status is the delivery state of a job. Pending is the only state the
// dispatcher picks up; Sent and Cancelled are terminal, and Failed is
// terminal too — it means the retry budget is spent and a person should
// look at it.
type Status string

const (
	StatusPending   Status = "pending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusSent, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// DefaultReminderLead is how far ahead of an appointment the reminder goes
// out. It is a per-practice setting; 24 hours is the platform default and
// deliberately matches the reschedule cutoff, so the reminder is the last
// moment a client can still move the session themselves.
const DefaultReminderLead = 24 * time.Hour

// Job is one message waiting to be delivered. Data carries the already
// resolved, presentation-ready template values (names, formatted times,
// URLs) rather than ids: a reminder that goes out tomorrow should read the
// way it did when it was scheduled, and the dispatcher must not need to
// re-query half the database to send an email.
type Job struct {
	ID        string
	Kind      Kind
	BookingID string
	Recipient string
	Data      map[string]string
	DueAt     time.Time
	Status    Status
	Attempts  int
	LastError string
	SentAt    *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// New builds a pending job due at dueAt. A due time in the past is not an
// error — an immediate message is simply due now, and the dispatcher will
// pick it up on its next pass.
func New(kind Kind, recipient string, data map[string]string, dueAt, now time.Time) (Job, error) {
	if !kind.Valid() {
		return Job{}, ErrInvalidKind
	}
	if recipient == "" {
		return Job{}, ErrRecipientRequired
	}
	now = now.UTC()
	copied := make(map[string]string, len(data))
	for k, v := range data {
		copied[k] = v
	}
	return Job{
		Kind:      kind,
		Recipient: recipient,
		Data:      copied,
		DueAt:     dueAt.UTC(),
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Due reports whether the job is ready to be delivered.
func (j Job) Due(now time.Time) bool {
	return j.Status == StatusPending && !j.DueAt.After(now.UTC())
}

// MarkSent closes a job as delivered.
func (j *Job) MarkSent(now time.Time) error {
	if j.Status != StatusPending {
		return ErrInvalidTransition
	}
	now = now.UTC()
	j.Status = StatusSent
	j.SentAt = &now
	j.LastError = ""
	j.UpdatedAt = now
	return nil
}

// Cancel abandons a job that is no longer wanted — the reminder for a
// booking that was cancelled or moved. Already-sent jobs cannot be recalled;
// cancelling one is a transition error, not a silent no-op.
func (j *Job) Cancel(now time.Time) error {
	if j.Status != StatusPending {
		return ErrInvalidTransition
	}
	now = now.UTC()
	j.Status = StatusCancelled
	j.UpdatedAt = now
	return nil
}

// RetryPolicy bounds redelivery. Backoff grows with each attempt so a
// provider outage is not hammered, and the attempt cap stops a permanently
// undeliverable address (a typo, a closed mailbox) from retrying forever.
type RetryPolicy struct {
	MaxAttempts int
	// Backoffs are the waits after attempt 1, 2, 3, … The last entry
	// repeats once the slice is exhausted.
	Backoffs []time.Duration
}

// DefaultRetryPolicy is the platform default: five attempts spread over
// roughly two hours.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 5,
		Backoffs:    []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour},
	}
}

// normalized fills zero fields with the defaults so a partly configured
// policy still retries and still terminates.
func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = DefaultRetryPolicy().MaxAttempts
	}
	if len(p.Backoffs) == 0 {
		p.Backoffs = DefaultRetryPolicy().Backoffs
	}
	return p
}

// backoffFor returns the wait after the given attempt number (1-based).
func (p RetryPolicy) backoffFor(attempt int) time.Duration {
	p = p.normalized()
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(p.Backoffs) {
		index = len(p.Backoffs) - 1
	}
	return p.Backoffs[index]
}

// RecordFailure folds one failed delivery attempt into the job: either it
// is rescheduled after the backoff, or the retry budget is spent and the
// job is failed for a person to look at. The reason is kept so that person
// can see what the provider said.
func (j *Job) RecordFailure(reason string, policy RetryPolicy, now time.Time) error {
	if j.Status != StatusPending {
		return ErrInvalidTransition
	}
	policy = policy.normalized()
	now = now.UTC()
	j.Attempts++
	j.LastError = reason
	j.UpdatedAt = now
	if j.Attempts >= policy.MaxAttempts {
		j.Status = StatusFailed
		return nil
	}
	j.DueAt = now.Add(policy.backoffFor(j.Attempts))
	return nil
}

// ReminderDueAt is when the reminder for an appointment should go out, and
// whether it is worth scheduling at all. A session booked inside the lead
// time (someone booking for this afternoon) gets no reminder: it would fire
// immediately and arrive alongside the confirmation, which is noise rather
// than a reminder.
func ReminderDueAt(startAt time.Time, lead time.Duration, now time.Time) (time.Time, bool) {
	if lead <= 0 {
		lead = DefaultReminderLead
	}
	due := startAt.UTC().Add(-lead)
	if !due.After(now.UTC()) {
		return time.Time{}, false
	}
	return due, true
}

// SortByDue orders jobs oldest-due first — the order a dispatcher should
// work through a backlog, so the most overdue message goes out first.
func SortByDue(jobs []Job) {
	sort.SliceStable(jobs, func(i, j int) bool { return jobs[i].DueAt.Before(jobs[j].DueAt) })
}
