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
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
)

// defaultBatchSize bounds one dispatcher pass.
const defaultBatchSize = 50

// Service queues and delivers notifications.
type Service struct {
	archiveExecution func(context.Context, string) error
	events           ports.WorkflowEventRepository
	bookings         ports.BookingRepository
	catalog          ports.ServiceRepository
	journalOnly      bool
	users            ports.UserRepository
	jobs             ports.NotificationJobRepository
	inApp            ports.InAppNotificationRepository
	renderer         ports.EmailRenderer
	mailer           ports.Mailer
	retry            notification.RetryPolicy
	lead             time.Duration
	timezone         string
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
	Events      ports.WorkflowEventRepository
	Bookings    ports.BookingRepository
	Catalog     ports.ServiceRepository
	JournalOnly bool
	Users       ports.UserRepository
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
	inApp ports.InAppNotificationRepository,
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
		events: opts.Events, bookings: opts.Bookings, catalog: opts.Catalog, journalOnly: opts.JournalOnly,
		jobs:          jobs,
		users:         opts.Users,
		inApp:         inApp,
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

// BookingPaymentRequired queues the checkout link without claiming that the
// appointment is confirmed. Confirmation and reminders are queued only after
// the signed Stripe webhook promotes the booking.
func (s *Service) BookingPaymentRequired(ctx context.Context, notice ports.BookingNotice) {
	data := s.bookingData(notice)
	data["paymentUrl"] = notice.PaymentURL
	s.queue(ctx, notification.KindBookingPaymentRequired, notice.ClientEmail, notice.BookingID, data, s.now())
}

// BookingConfirmed queues the confirmation and schedules the reminder.
// A session booked inside the reminder lead gets no reminder — it would
// land alongside the confirmation.
func (s *Service) BookingConfirmed(ctx context.Context, notice ports.BookingNotice) {
	now := s.now()
	data := s.bookingData(notice)
	s.queue(ctx, notification.KindBookingConfirmation, notice.ClientEmail, notice.BookingID, data, now)
	if s.practiceAddress(ctx) != "" && s.practiceAddress(ctx) != notice.ClientEmail {
		s.queue(ctx, notification.KindBookingConfirmation, s.practiceAddress(ctx), notice.BookingID, data, now)
	}

	if dueAt, ok := notification.ReminderDueAt(notice.StartAt, s.lead, now); ok {
		reminderData := s.bookingData(notice)
		reminderData["timeUntil"] = humanLead(s.lead)
		s.queue(ctx, notification.KindSessionReminder, notice.ClientEmail, notice.BookingID, reminderData, dueAt)
		if s.practiceAddress(ctx) != "" && s.practiceAddress(ctx) != notice.ClientEmail {
			s.queue(ctx, notification.KindSessionReminder, s.practiceAddress(ctx), notice.BookingID, reminderData, dueAt)
		}
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
	if s.practiceAddress(ctx) != "" && s.practiceAddress(ctx) != notice.ClientEmail {
		s.queue(ctx, notification.KindBookingRescheduled, s.practiceAddress(ctx), notice.BookingID, data, now)
	}

	s.cancelReminders(ctx, notice.BookingID)
	if dueAt, ok := notification.ReminderDueAt(notice.StartAt, s.lead, now); ok {
		reminderData := s.bookingData(notice)
		reminderData["timeUntil"] = humanLead(s.lead)
		s.queue(ctx, notification.KindSessionReminder, notice.ClientEmail, notice.BookingID, reminderData, dueAt)
		if s.practiceAddress(ctx) != "" && s.practiceAddress(ctx) != notice.ClientEmail {
			s.queue(ctx, notification.KindSessionReminder, s.practiceAddress(ctx), notice.BookingID, reminderData, dueAt)
		}
	}
}

// BookingCancelled queues the cancellation notice and drops the reminder —
// nothing is more jarring than a reminder for a session that is off.
func (s *Service) BookingCancelled(ctx context.Context, notice ports.BookingNotice) {
	s.queue(ctx, notification.KindBookingCancelled, notice.ClientEmail, notice.BookingID,
		s.bookingData(notice), s.now())
	if s.practiceAddress(ctx) != "" && s.practiceAddress(ctx) != notice.ClientEmail {
		s.queue(ctx, notification.KindBookingCancelled, s.practiceAddress(ctx), notice.BookingID,
			s.bookingData(notice), s.now())
	}
	s.cancelReminders(ctx, notice.BookingID)
}

// BookingChangeRequested tells the practice a client has asked to move or
// cancel a session. It does not notify the client or change reminders because
// the session is still unchanged until the practitioner acts.
func (s *Service) BookingChangeRequested(ctx context.Context, notice ports.BookingChangeRequestNotice) {
	if s.practiceAddress(ctx) == "" {
		s.report(fmt.Errorf("booking change request for %s not queued: no practice inbox configured", notice.BookingID))
		return
	}
	data := map[string]string{
		"bookingId":    notice.BookingID,
		"clientName":   notice.ClientName,
		"clientEmail":  notice.ClientEmail,
		"serviceName":  notice.ServiceName,
		"requestType":  notice.RequestType,
		"currentTime":  s.formatTime(notice.StartAt, notice.Timezone),
		"currentStart": s.formatTime(notice.StartAt, notice.Timezone),
		"startTime":    s.formatTime(notice.StartAt, notice.Timezone),
		"reason":       notice.Reason,
		"timezone":     notice.Timezone,
	}
	if data["reason"] == "" {
		data["reason"] = "No reason provided — reschedule request."
	}
	if !notice.ProposedStartAt.IsZero() {
		proposed := s.formatTime(notice.ProposedStartAt, notice.Timezone)
		data["proposedTime"] = proposed
		data["newStartTime"] = proposed
	} else {
		data["proposedTime"] = "No new time requested — cancellation requested."
	}
	s.queue(ctx, notification.KindBookingChangeRequested, s.practiceAddress(ctx), notice.BookingID, data, s.now())
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
	if s.practiceAddress(ctx) == "" {
		s.report(fmt.Errorf("enquiry %s not queued: no practice inbox configured", notice.EnquiryID))
		return
	}
	s.queue(ctx, notification.KindEnquiryReceived, s.practiceAddress(ctx), notice.EnquiryID, map[string]string{
		"senderName":  notice.SenderName,
		"senderEmail": notice.SenderEmail,
		"message":     notice.Message,
	}, s.now())
}

// AgreementSigned tells the practice a client has accepted a service
// agreement. Like the enquiry alert, this one goes to the practice inbox:
// the client already has their own copy on their portal, and this message
// exists so the practitioner has the signature in writing without having to
// go looking for it.
func (s *Service) AgreementSigned(ctx context.Context, notice ports.AgreementSignedNotice) {
	if s.practiceAddress(ctx) == "" {
		s.report(fmt.Errorf("agreement signature for %s not queued: no practice inbox configured", notice.ClientID))
		return
	}
	action := "signed"
	if notice.Submitted {
		action = "submitted"
	}
	s.queue(ctx, notification.KindAgreementSigned, s.practiceAddress(ctx), notice.ClientID, map[string]string{
		"clientName":     notice.ClientName,
		"clientEmail":    notice.ClientEmail,
		"agreementTitle": notice.AgreementTitle,
		"action":         action,
		"signedName":     notice.SignedName,
		"signedAt":       notice.SignedAt,
	}, s.now())
}

func (s *Service) FormAssigned(ctx context.Context, notice ports.FormAssignedNotice) {
	if notice.ClientEmail == "" {
		s.report(fmt.Errorf("form assignment %s not queued: no client email", notice.SubmissionID))
		return
	}
	s.queue(ctx, notification.KindFormAssigned, notice.ClientEmail, notice.SubmissionID, map[string]string{
		"clientName":   notice.ClientName,
		"formTitle":    notice.FormTitle,
		"assignedAt":   notice.AssignedAt,
		"submissionId": notice.SubmissionID,
	}, s.now())
}

func (s *Service) FormSubmitted(ctx context.Context, notice ports.FormSubmittedNotice) {
	if s.practiceAddress(ctx) == "" {
		s.report(fmt.Errorf("form submission %s not queued: no practice inbox configured", notice.SubmissionID))
		return
	}
	s.queue(ctx, notification.KindFormSubmitted, s.practiceAddress(ctx), notice.SubmissionID, map[string]string{
		"clientName":   notice.ClientName,
		"clientEmail":  notice.ClientEmail,
		"formTitle":    notice.FormTitle,
		"submittedAt":  notice.SubmittedAt,
		"submissionId": notice.SubmissionID,
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
	if s.events != nil {
		s.reconcile(ctx, limit)
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
	if job.Kind == notification.KindSessionReminder && s.bookings != nil {
		b, err := s.bookings.FindByID(ctx, job.BookingID)
		if err != nil {
			s.report(err)
			_ = job.RecordFailure("booking unavailable", s.retry, s.now())
			_, _ = s.jobs.Update(ctx, job)
			return false
		}
		expected, err := time.Parse(time.RFC3339Nano, job.Data["startAt"])
		if b.Status != "confirmed" || (!expected.IsZero() && !b.StartAt.Equal(expected)) || !b.StartAt.After(s.now()) {
			_ = job.Cancel(s.now())
			_, err = s.jobs.Update(ctx, job)
			if err != nil {
				s.report(err)
			}
			return false
		}
		if uid := job.Data["recipientId"]; uid != "" && s.users != nil && job.Data["renderedTimezone"] == "" {
			u, e := s.users.FindByID(ctx, uid)
			if e != nil {
				s.report(e)
				_ = job.RecordFailure("recipient unavailable", s.retry, s.now())
				_, _ = s.jobs.Update(ctx, job)
				return false
			}
			zone := u.Timezone
			if zone == "" {
				zone = "UTC"
			}
			job.Data["timezone"] = zone
			job.Data["startTime"] = s.formatTime(b.StartAt, zone)
			job.Data["renderedTimezone"] = zone
			// Persist rendered-zone evidence before sending; sent records never re-render.

		}
	}

	if err := s.recordInApp(ctx, job); err != nil {
		s.report(err)
		_ = job.RecordFailure(err.Error(), s.retry, s.now())
		_, updateErr := s.jobs.Update(ctx, job)
		if updateErr != nil {
			s.report(updateErr)
		}
		return false
	}
	if job.Kind == notification.KindActivity {
		_ = job.MarkSent(s.now())
		if _, err := s.jobs.Update(ctx, job); err != nil {
			s.report(err)
			return false
		}
		return true
	}
	msg, err := s.renderer.Render(job)
	if job.Data["deliveryPrepared"] == "true" {
		msg = ports.EmailMessage{To: job.Recipient, Subject: job.Data["renderedSubject"], HTML: job.Data["renderedHTML"], Text: job.Data["renderedText"]}
		err = nil
	}
	if err != nil {
		// An unrenderable job will never render; spend the whole retry
		// budget at once rather than retrying a certainty.
		s.report(fmt.Errorf("render notification %s (%s): %w", job.ID, job.Kind, err))
		s.failPermanently(ctx, job, err.Error())
		return false
	}

	if prep, ok := s.jobs.(ports.NotificationDeliveryPreparer); ok {
		job.Data["renderedSubject"] = msg.Subject
		job.Data["renderedHTML"] = msg.HTML
		job.Data["renderedText"] = msg.Text
		job.Data["deliveryPrepared"] = "true"
		frozen, e := prep.PrepareDelivery(ctx, job)
		if e != nil {
			s.report(e)
			return false
		}
		job = frozen
		msg = ports.EmailMessage{To: job.Recipient, Subject: job.Data["renderedSubject"], HTML: job.Data["renderedHTML"], Text: job.Data["renderedText"]}
	}
	msg.IdempotencyKey = "notification-" + job.ID
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

// recordInApp persists a due event before email delivery. Immediate events
// also use this path at queue time; EventID deduplicates dispatch retries.
func (s *Service) recordInApp(ctx context.Context, job notification.Job) error {
	if s.inApp == nil {
		return nil
	}
	title, body, link := inAppContent(job)
	if job.Kind == notification.KindActionRequired {
		title = job.Data["title"]
		body = job.Data["body"]
		link = job.Data["link"]
	}
	if title == "" {
		return nil
	}
	recipient := job.Recipient
	if job.Data["recipientId"] == "" && strings.EqualFold(recipient, s.practiceAddress(ctx)) && s.users != nil {
		practitioner, err := s.users.FindFirstByRole(ctx, identity.RolePractitioner)
		if err != nil {
			return fmt.Errorf("resolve practitioner notification: %w", err)
		}
		recipient = practitioner.Email
	}
	if (job.Data["audience"] == "practitioner" || strings.EqualFold(job.Recipient, s.practiceAddress(ctx))) && strings.HasPrefix(link, "/portal/") {
		link = "/calendar"
		if job.Kind == notification.KindBookingPaymentRequired {
			link = "/payments"
		}
		title = strings.ReplaceAll(title, "Your session", "Client session")
		title = strings.ReplaceAll(title, "your session", "client session")
	}
	entry, err := notification.NewInApp(recipient, job.Kind, title, body, link, job.BookingID, s.now())
	if err != nil {
		return fmt.Errorf("build in-app notification %s (%s): %w", job.ID, job.Kind, err)
	}
	entry.EventID = job.ID
	if key := job.Data["eventId"]; key != "" {
		entry.EventID = key
	}
	if _, err := s.inApp.Create(ctx, entry); err != nil {
		return fmt.Errorf("persist in-app notification %s (%s): %w", job.ID, job.Kind, err)
	}
	// Preserve an acknowledgement for the other participant as well. These
	// copies are in-app only and share the originating job's retry lifecycle.
	var receiptRecipient, receiptTitle, receiptLink string
	switch job.Kind {
	case notification.KindFormAssigned:
		receiptRecipient, receiptTitle, receiptLink = s.practiceAddress(ctx), "A form was assigned to a client", "/forms"
	case notification.KindFormSubmitted:
		receiptRecipient, receiptTitle, receiptLink = job.Data["clientEmail"], "Your form was submitted", "/portal/forms"
	case notification.KindAgreementSigned:
		action := job.Data["action"]
		if action == "" {
			action = "signed"
		}
		receiptRecipient, receiptTitle, receiptLink = job.Data["clientEmail"], "Your "+job.Data["agreementTitle"]+" was "+action, "/portal/documents"
	case notification.KindFeedbackShared:
		receiptRecipient, receiptTitle, receiptLink = s.practiceAddress(ctx), "Session feedback and resources were shared", "/clients"
	case notification.KindBookingChangeRequested:
		receiptRecipient, receiptTitle, receiptLink = job.Data["clientEmail"], "Your session change request was sent to the practice", "/portal/sessions"
	}
	if receiptRecipient != "" && !strings.EqualFold(receiptRecipient, job.Recipient) {
		receipt := job
		receipt.Kind = notification.KindActivity
		receipt.Recipient = receiptRecipient
		receipt.Data = map[string]string{"eventId": job.ID + ":receipt", "title": receiptTitle, "link": receiptLink}
		return s.recordInApp(ctx, receipt)
	}
	return nil
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
	if s.journalOnly {
		return
	}
	job, err := notification.New(kind, recipient, data, dueAt, s.now())
	if err != nil {
		s.report(fmt.Errorf("build %s notification: %w", kind, err))
		return
	}
	job.BookingID = bookingID
	stored, err := s.jobs.Create(ctx, job)
	if err != nil {
		s.report(fmt.Errorf("queue %s notification: %w", kind, err))
		return
	}
	if !dueAt.After(s.now()) {
		if err := s.recordInApp(ctx, stored); err != nil {
			s.report(err)
		}
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
	return at.In(location(timezone)).Format("Monday 2 January 2006, 15:04 MST (UTC-07:00)")
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
	case lead >= time.Minute:
		return fmt.Sprintf("in %d minutes", int(lead.Minutes()))
	default:
		return "coming up"
	}
}

// inAppContent resolves a delivered job into the (title, body, link) shown
// in the recipient's in-app feed. The title reuses the same wording as the
// email subject so the two channels never disagree about what happened;
// the link is a route within whichever app that recipient actually uses —
// the portal for a client-facing kind, the dashboard for a practice-facing
// one — since a job's recipient is always addressed to exactly one of the
// two audiences.
func inAppContent(job notification.Job) (title, body, link string) {
	data := job.Data
	switch job.Kind {
	case notification.KindActivity:
		return data["title"], "", data["link"]
	case notification.KindBookingPaymentRequired:
		return "Payment required to confirm your session", data["serviceName"] + " · " + data["startTime"], "/portal/payments"
	case notification.KindBookingConfirmation:
		return "Your session is confirmed", data["serviceName"] + " · " + data["startTime"], "/portal/sessions"
	case notification.KindSessionReminder:
		title := "Reminder: your session is coming up"
		if until := data["timeUntil"]; until != "" {
			title = "Reminder: your session is " + until
		}
		link := "/portal/sessions"
		if job.BookingID != "" && data["recipientId"] != "" {
			link += "/" + job.BookingID + "/room"
			if data["audience"] == "practitioner" {
				link = "/sessions/" + job.BookingID + "/room"
			}
		}
		return title, data["serviceName"] + " · " + data["startTime"], link
	case notification.KindBookingRescheduled:
		return "Your session has been rescheduled", data["serviceName"] + " is now " + data["newStartTime"], "/portal/sessions"
	case notification.KindBookingCancelled:
		return "Your session has been cancelled", data["serviceName"] + " · " + data["startTime"], "/portal/sessions"
	case notification.KindBookingChangeRequested:
		title := "Client requested a session change"
		if name := data["clientName"]; name != "" {
			title = name + " requested a session " + data["requestType"]
		}
		return title, data["serviceName"] + " · " + data["currentTime"], "/calendar"
	case notification.KindFeedbackShared:
		return "Notes and resources from your session", data["serviceName"] + " · " + data["sessionDate"], "/portal/sessions"
	case notification.KindEnquiryReceived:
		title := "New enquiry from your website"
		if name := data["senderName"]; name != "" {
			title = "New enquiry from " + name
		}
		return title, data["message"], "/enquiries"
	case notification.KindAgreementSigned:
		title := "A client signed a service agreement"
		if name := data["clientName"]; name != "" {
			action := "signed"
			if data["action"] == "submitted" {
				action = "submitted"
			}
			title = name + " " + action + " the " + data["agreementTitle"]
		}
		return title, "", "/clients"
	case notification.KindFormAssigned:
		title := "A form is ready for you"
		if formTitle := data["formTitle"]; formTitle != "" {
			title = "Please complete " + formTitle
		}
		link := "/portal/forms"
		if id := data["submissionId"]; id != "" {
			link = "/portal/forms/" + id
		}
		return title, "", link
	case notification.KindFormSubmitted:
		title := "A client submitted a form"
		if name := data["clientName"]; name != "" {
			title = name + " submitted " + data["formTitle"]
		}
		return title, "", "/forms"
	default:
		return "", "", ""
	}
}

// SetExecutionArchiver is called once during composition, before the worker starts.
func (s *Service) SetExecutionArchiver(fn func(context.Context, string) error) {
	s.archiveExecution = fn
}
