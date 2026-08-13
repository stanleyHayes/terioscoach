// Package notifications is the application service for the messaging
// slice. It implements the inbound ports.Notifier and ports.Dispatcher
// ports purely against outbound ports — no framework, driver, or provider
// imports.
//
// Queueing and delivery are deliberately separate. Notifier writes a job
// and returns; Dispatcher picks jobs up and sends them. That is what lets a
// booking succeed while the mail provider is down, lets a reminder outlive
// the process that scheduled it, and lets a failed send be retried instead
// of lost.
package notifications

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultBatchSize bounds one dispatcher pass.
const defaultBatchSize = 50

// Service queues and delivers notifications.
type Service struct {
	jobs     ports.NotificationJobRepository
	renderer ports.EmailRenderer
	mailer   ports.Mailer
	retry    notification.RetryPolicy
	lead     time.Duration
	timezone string
	// practiceEmail receives practice-facing alerts (new enquiries).
	practiceEmail string
	// report receives failures that cannot be returned to the caller,
	// because the caller is a business action that already succeeded. The
	// composition root points it at the logger.
	report func(error)
	now    func() time.Time
}

// Compile-time checks: Service satisfies both inbound ports.
var (
	_ ports.Notifier   = (*Service)(nil)
	_ ports.Dispatcher = (*Service)(nil)
)

// Options configure a Service. Zero values fall back to platform defaults.
type Options struct {
	// ReminderLead is how far ahead of a session its reminder goes out.
	ReminderLead time.Duration
	// Retry bounds redelivery of a failing job.
	Retry notification.RetryPolicy
	// DefaultTimezone presents times when a notice does not carry one.
	DefaultTimezone string
	// PracticeEmail is where practice-facing alerts (new enquiries) go.
	PracticeEmail string
	// Report receives queueing and delivery failures. Nil discards them,
	// which is only ever right in a test.
	Report func(error)
}

// NewService wires the use cases to their outbound ports.
func NewService(
	jobs ports.NotificationJobRepository,
	renderer ports.EmailRenderer,
	mailer ports.Mailer,
	opts Options,
) *Service {
	lead := opts.ReminderLead
	if lead <= 0 {
		lead = notification.DefaultReminderLead
	}
	timezone := opts.DefaultTimezone
	if timezone == "" {
		timezone = "Africa/Accra"
	}
	report := opts.Report
	if report == nil {
		report = func(error) {}
	}
	return &Service{
		jobs:          jobs,
		renderer:      renderer,
		mailer:        mailer,
		retry:         opts.Retry,
		lead:          lead,
		timezone:      timezone,
		practiceEmail: opts.PracticeEmail,
		report:        report,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// BookingConfirmed queues the confirmation and schedules the reminder.
// A session booked inside the reminder lead gets no reminder — it would
// land alongside the confirmation.
func (s *Service) BookingConfirmed(ctx context.Context, notice ports.BookingNotice) {
	now := s.now()
	data := s.bookingData(notice)
	s.queue(ctx, notification.KindBookingConfirmation, notice.ClientEmail, notice.BookingID, data, now)

	if dueAt, ok := notification.ReminderDueAt(notice.StartAt, s.lead, now); ok {
		reminderData := s.bookingData(notice)
		reminderData["timeUntil"] = humanLead(s.lead)
		s.queue(ctx, notification.KindSessionReminder, notice.ClientEmail, notice.BookingID, reminderData, dueAt)
	}
}

// BookingRescheduled queues the change notice, cancels the reminder for the
// old time, and schedules one for the new time.
func (s *Service) BookingRescheduled(ctx context.Context, notice ports.BookingNotice) {
	now := s.now()
	data := s.bookingData(notice)
	data["oldStartTime"] = s.formatTime(notice.PreviousStartAt, notice.Timezone)
	data["newStartTime"] = data["startTime"]
	s.queue(ctx, notification.KindBookingRescheduled, notice.ClientEmail, notice.BookingID, data, now)

	s.cancelReminders(ctx, notice.BookingID)
	if dueAt, ok := notification.ReminderDueAt(notice.StartAt, s.lead, now); ok {
		reminderData := s.bookingData(notice)
		reminderData["timeUntil"] = humanLead(s.lead)
		s.queue(ctx, notification.KindSessionReminder, notice.ClientEmail, notice.BookingID, reminderData, dueAt)
	}
}

// BookingCancelled queues the cancellation notice and drops the reminder —
// nothing is more jarring than a reminder for a session that is off.
func (s *Service) BookingCancelled(ctx context.Context, notice ports.BookingNotice) {
	s.queue(ctx, notification.KindBookingCancelled, notice.ClientEmail, notice.BookingID,
		s.bookingData(notice), s.now())
	s.cancelReminders(ctx, notice.BookingID)
}

// FeedbackShared queues the post-session feedback email. The notes slice
// calls it only on the first share, so a client is told once.
func (s *Service) FeedbackShared(ctx context.Context, notice ports.FeedbackNotice) {
	data := map[string]string{
		"clientName":       notice.ClientName,
		"practitionerName": notice.PractitionerName,
		"serviceName":      notice.ServiceName,
		"sessionDate":      s.formatDate(notice.SessionDate, notice.Timezone),
	}
	if notice.HasResources {
		data["hasResources"] = "true"
	}
	s.queue(ctx, notification.KindFeedbackShared, notice.ClientEmail, notice.BookingID, data, s.now())
}

// EnquiryReceived queues the practitioner's alert for a website enquiry.
// The recipient is the practice inbox, never the sender: this message
// exists to tell the practitioner someone got in touch, and echoing it to
// an address a stranger typed would turn the contact form into an open
// relay for the practice's own branding.
func (s *Service) EnquiryReceived(ctx context.Context, notice ports.EnquiryNotice) {
	if s.practiceEmail == "" {
		s.report(fmt.Errorf("enquiry %s not queued: no practice inbox configured", notice.EnquiryID))
		return
	}
	s.queue(ctx, notification.KindEnquiryReceived, s.practiceEmail, notice.EnquiryID, map[string]string{
		"senderName":  notice.SenderName,
		"senderEmail": notice.SenderEmail,
		"message":     notice.Message,
	}, s.now())
}

// DispatchDue delivers up to limit due jobs. It is safe to run on a timer
// and safe to run in more than one process: ClaimDue hands each job to
// exactly one caller.
//
// One job's failure never stops the batch — a single bad address would
// otherwise block every message behind it.
func (s *Service) DispatchDue(ctx context.Context, limit int) (ports.DispatchResult, error) {
	if limit <= 0 {
		limit = defaultBatchSize
	}
	claimed, err := s.jobs.ClaimDue(ctx, s.now(), limit)
	if err != nil {
		return ports.DispatchResult{}, fmt.Errorf("claim due notifications: %w", err)
	}

	var result ports.DispatchResult
	for _, job := range claimed {
		if s.deliver(ctx, job) {
			result.Sent++
			continue
		}
		result.Failed++
	}
	return result, nil
}

// deliver sends one job and records the outcome. It reports whether the
// message went out.
func (s *Service) deliver(ctx context.Context, job notification.Job) bool {
	msg, err := s.renderer.Render(job)
	if err != nil {
		// An unrenderable job will never render; spend the whole retry
		// budget at once rather than retrying a certainty.
		s.report(fmt.Errorf("render notification %s (%s): %w", job.ID, job.Kind, err))
		s.failPermanently(ctx, job, err.Error())
		return false
	}

	if err := s.mailer.Send(ctx, msg); err != nil {
		s.report(fmt.Errorf("send notification %s (%s): %w", job.ID, job.Kind, err))
		if recordErr := job.RecordFailure(err.Error(), s.retry, s.now()); recordErr != nil {
			s.report(fmt.Errorf("record notification failure %s: %w", job.ID, recordErr))
			return false
		}
		if _, updateErr := s.jobs.Update(ctx, job); updateErr != nil {
			s.report(fmt.Errorf("persist notification failure %s: %w", job.ID, updateErr))
		}
		return false
	}

	if err := job.MarkSent(s.now()); err != nil {
		s.report(fmt.Errorf("mark notification sent %s: %w", job.ID, err))
		return false
	}
	if _, err := s.jobs.Update(ctx, job); err != nil {
		// The message is already out. Failing to record that is a
		// duplicate-send risk on the next pass, so it must be loud.
		s.report(fmt.Errorf("persist notification sent %s: %w", job.ID, err))
	}
	return true
}

// failPermanently burns the retry budget in one go for a job that cannot
// succeed on any attempt.
func (s *Service) failPermanently(ctx context.Context, job notification.Job, reason string) {
	policy := s.retry
	for job.Status == notification.StatusPending {
		if err := job.RecordFailure(reason, policy, s.now()); err != nil {
			break
		}
	}
	if _, err := s.jobs.Update(ctx, job); err != nil {
		s.report(fmt.Errorf("persist notification failure %s: %w", job.ID, err))
	}
}

// queue writes one job. Failures are reported, never returned: the business
// event that triggered this has already happened.
func (s *Service) queue(ctx context.Context, kind notification.Kind, recipient, bookingID string, data map[string]string, dueAt time.Time) {
	job, err := notification.New(kind, recipient, data, dueAt, s.now())
	if err != nil {
		s.report(fmt.Errorf("build %s notification: %w", kind, err))
		return
	}
	job.BookingID = bookingID
	if _, err := s.jobs.Create(ctx, job); err != nil {
		s.report(fmt.Errorf("queue %s notification: %w", kind, err))
	}
}

// cancelReminders drops a booking's undelivered reminders. Already-sent
// reminders are simply not in the pending set, so they are left alone.
func (s *Service) cancelReminders(ctx context.Context, bookingID string) {
	if bookingID == "" {
		return
	}
	pending, err := s.jobs.PendingByBooking(ctx, bookingID, notification.KindSessionReminder)
	if err != nil {
		s.report(fmt.Errorf("find reminders for booking %s: %w", bookingID, err))
		return
	}
	for _, job := range pending {
		if err := job.Cancel(s.now()); err != nil {
			if !errors.Is(err, notification.ErrInvalidTransition) {
				s.report(fmt.Errorf("cancel reminder %s: %w", job.ID, err))
			}
			continue
		}
		if _, err := s.jobs.Update(ctx, job); err != nil {
			s.report(fmt.Errorf("persist reminder cancellation %s: %w", job.ID, err))
		}
	}
}

// bookingData resolves the template values for the booking emails.
func (s *Service) bookingData(notice ports.BookingNotice) map[string]string {
	timezone := notice.Timezone
	if timezone == "" {
		timezone = s.timezone
	}
	return map[string]string{
		"clientName":  notice.ClientName,
		"serviceName": notice.ServiceName,
		"startTime":   s.formatTime(notice.StartAt, timezone),
		"timezone":    timezone,
	}
}

// formatTime renders an instant in the client's own timezone. An unknown
// timezone name falls back to UTC rather than dropping the time: a message
// with the wrong offset stated is recoverable, a message with no time is
// not.
func (s *Service) formatTime(at time.Time, timezone string) string {
	if at.IsZero() {
		return ""
	}
	return at.In(location(timezone)).Format("Monday 2 January 2006, 15:04")
}

// formatDate renders just the day, for messages that reference a session
// rather than schedule one.
func (s *Service) formatDate(at time.Time, timezone string) string {
	if at.IsZero() {
		return ""
	}
	return at.In(location(timezone)).Format("Monday 2 January 2006")
}

func location(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// humanLead phrases the reminder's lead time for the subject and heading.
func humanLead(lead time.Duration) string {
	switch {
	case lead >= 48*time.Hour:
		return fmt.Sprintf("in %d days", int(lead.Hours()/24))
	case lead >= 24*time.Hour:
		return "tomorrow"
	case lead >= 2*time.Hour:
		return fmt.Sprintf("in %d hours", int(lead.Hours()))
	default:
		return "coming up"
	}
}
