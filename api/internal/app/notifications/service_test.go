package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

type testRig struct {
	svc      *Service
	jobs     *portstest.FakeNotificationJobRepository
	mailer   *portstest.FakeMailer
	renderer *portstest.FakeEmailRenderer
	reported []error
	clock    *time.Time
}

func newTestRig(t *testing.T) *testRig {
	t.Helper()
	clock := fixedNow
	rig := &testRig{
		jobs:     portstest.NewFakeNotificationJobRepository(),
		mailer:   portstest.NewFakeMailer(),
		renderer: &portstest.FakeEmailRenderer{},
		clock:    &clock,
	}
	rig.svc = NewService(rig.jobs, rig.renderer, rig.mailer, Options{
		ReminderLead:    notification.DefaultReminderLead,
		Retry:           notification.RetryPolicy{MaxAttempts: 3, Backoffs: []time.Duration{time.Minute}},
		DefaultTimezone: "Africa/Accra",
		PracticeEmail:   "practice@terioscoach.com",
		Report:          func(err error) { rig.reported = append(rig.reported, err) },
	})
	rig.svc.now = func() time.Time { return clock }
	return rig
}

func (r *testRig) advance(d time.Duration) { *r.clock = r.clock.Add(d) }

func bookingNotice(startAt time.Time) ports.BookingNotice {
	return ports.BookingNotice{
		BookingID:   "booking-1",
		ClientName:  "Ama Serwaa",
		ClientEmail: "ama@example.com",
		ServiceName: "Deep Tissue Massage",
		StartAt:     startAt,
		Timezone:    "Africa/Accra",
	}
}

// TestBookingConfirmedQueuesConfirmationAndReminder: one immediate message
// and one scheduled for a lead time before the session.
func TestBookingConfirmedQueuesConfirmationAndReminder(t *testing.T) {
	rig := newTestRig(t)
	start := fixedNow.Add(72 * time.Hour)

	rig.svc.BookingConfirmed(context.Background(), bookingNotice(start))

	confirmations := rig.jobs.OfKind(notification.KindBookingConfirmation)
	if len(confirmations) != 1 {
		t.Fatalf("confirmations = %d, want 1", len(confirmations))
	}
	confirmation := confirmations[0]
	if confirmation.Recipient != "ama@example.com" || !confirmation.Due(fixedNow) {
		t.Errorf("confirmation = %+v, want an immediate message to the client", confirmation)
	}
	if confirmation.Data["serviceName"] != "Deep Tissue Massage" || confirmation.Data["clientName"] != "Ama Serwaa" {
		t.Errorf("data = %v, want the resolved names", confirmation.Data)
	}
	if !strings.Contains(confirmation.Data["startTime"], "2026") {
		t.Errorf("startTime = %q, want a formatted date", confirmation.Data["startTime"])
	}

	reminders := rig.jobs.OfKind(notification.KindSessionReminder)
	if len(reminders) != 1 {
		t.Fatalf("reminders = %d, want 1", len(reminders))
	}
	if !reminders[0].DueAt.Equal(start.Add(-notification.DefaultReminderLead)) {
		t.Errorf("reminder due = %v, want one lead time before %v", reminders[0].DueAt, start)
	}
	if reminders[0].Data["timeUntil"] != "tomorrow" {
		t.Errorf("timeUntil = %q, want the lead phrased for the subject line", reminders[0].Data["timeUntil"])
	}
	if reminders[0].BookingID != "booking-1" {
		t.Error("reminder is not linked to its booking; a reschedule could not find it")
	}
}

// TestShortNoticeBookingSkipsTheReminder: booking for this afternoon gets a
// confirmation only — a reminder would arrive alongside it.
func TestShortNoticeBookingSkipsTheReminder(t *testing.T) {
	rig := newTestRig(t)

	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(3*time.Hour)))

	if got := len(rig.jobs.OfKind(notification.KindSessionReminder)); got != 0 {
		t.Errorf("reminders = %d, want none inside the lead time", got)
	}
	if got := len(rig.jobs.OfKind(notification.KindBookingConfirmation)); got != 1 {
		t.Errorf("confirmations = %d, want 1", got)
	}
}

// TestRescheduleMovesTheReminder: the old reminder is cancelled and a new
// one scheduled against the new time.
func TestRescheduleMovesTheReminder(t *testing.T) {
	rig := newTestRig(t)
	original := fixedNow.Add(72 * time.Hour)
	moved := fixedNow.Add(120 * time.Hour)

	rig.svc.BookingConfirmed(context.Background(), bookingNotice(original))

	notice := bookingNotice(moved)
	notice.PreviousStartAt = original
	rig.svc.BookingRescheduled(context.Background(), notice)

	reminders := rig.jobs.OfKind(notification.KindSessionReminder)
	if len(reminders) != 2 {
		t.Fatalf("reminders = %d, want the original plus its replacement", len(reminders))
	}
	if reminders[0].Status != notification.StatusCancelled {
		t.Errorf("original reminder status = %q, want cancelled", reminders[0].Status)
	}
	if reminders[1].Status != notification.StatusPending {
		t.Errorf("new reminder status = %q, want pending", reminders[1].Status)
	}
	if !reminders[1].DueAt.Equal(moved.Add(-notification.DefaultReminderLead)) {
		t.Errorf("new reminder due = %v, want it to follow the new session time", reminders[1].DueAt)
	}

	notices := rig.jobs.OfKind(notification.KindBookingRescheduled)
	if len(notices) != 1 {
		t.Fatalf("reschedule notices = %d, want 1", len(notices))
	}
	if notices[0].Data["oldStartTime"] == "" || notices[0].Data["newStartTime"] == "" {
		t.Errorf("data = %v, want both the old and the new time", notices[0].Data)
	}
	if notices[0].Data["oldStartTime"] == notices[0].Data["newStartTime"] {
		t.Error("old and new times are identical in the reschedule notice")
	}
}

// TestCancellationDropsTheReminder: nobody should be reminded about a
// session that is off.
func TestCancellationDropsTheReminder(t *testing.T) {
	rig := newTestRig(t)
	start := fixedNow.Add(72 * time.Hour)
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(start))

	rig.svc.BookingCancelled(context.Background(), bookingNotice(start))

	for _, job := range rig.jobs.OfKind(notification.KindSessionReminder) {
		if job.Status != notification.StatusCancelled {
			t.Errorf("reminder %s status = %q, want cancelled", job.ID, job.Status)
		}
	}
	if got := len(rig.jobs.OfKind(notification.KindBookingCancelled)); got != 1 {
		t.Errorf("cancellation notices = %d, want 1", got)
	}

	// And the dispatcher must not resurrect it once the due time arrives.
	rig.advance(80 * time.Hour)
	result, err := rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	for _, msg := range rig.mailer.Sent() {
		if msg.Subject == string(notification.KindSessionReminder) {
			t.Error("a cancelled reminder was delivered")
		}
	}
	_ = result
}

// TestAlreadySentReminderIsNotCancelled: cancelling a booking after its
// reminder went out must not try to recall a sent email.
func TestAlreadySentReminderIsNotCancelled(t *testing.T) {
	rig := newTestRig(t)
	start := fixedNow.Add(48 * time.Hour)
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(start))

	rig.advance(25 * time.Hour) // the reminder is now due
	if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}

	rig.svc.BookingCancelled(context.Background(), bookingNotice(start))

	reminders := rig.jobs.OfKind(notification.KindSessionReminder)
	if reminders[0].Status != notification.StatusSent {
		t.Errorf("reminder status = %q, want it left as sent", reminders[0].Status)
	}
	for _, err := range rig.reported {
		if strings.Contains(err.Error(), "cancel reminder") {
			t.Errorf("cancelling an already-sent reminder was reported as a failure: %v", err)
		}
	}
}

// TestDispatchSendsOnlyDueJobs: a reminder scheduled for next week does not
// go out today.
func TestDispatchSendsOnlyDueJobs(t *testing.T) {
	rig := newTestRig(t)
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	result, err := rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Sent != 1 || result.Failed != 0 {
		t.Errorf("result = %+v, want only the confirmation sent", result)
	}
	if got := len(rig.mailer.Sent()); got != 1 {
		t.Fatalf("sent %d messages, want 1", got)
	}

	// Nothing is left due, so a second pass sends nothing.
	result, err = rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("second DispatchDue: %v", err)
	}
	if result.Sent != 0 {
		t.Errorf("second pass sent %d, want 0 — a sent job was re-sent", result.Sent)
	}

	// Once the reminder falls due it goes out, exactly once.
	rig.advance(49 * time.Hour)
	if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
		t.Fatalf("third DispatchDue: %v", err)
	}
	if got := len(rig.mailer.Sent()); got != 2 {
		t.Errorf("sent %d messages in total, want 2", got)
	}
	if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
		t.Fatalf("fourth DispatchDue: %v", err)
	}
	if got := len(rig.mailer.Sent()); got != 2 {
		t.Errorf("sent %d messages, want the reminder delivered only once", got)
	}
}

// TestFailedSendRetriesThenGivesUp: a provider outage reschedules with
// backoff; a permanent one eventually stops.
func TestFailedSendRetriesThenGivesUp(t *testing.T) {
	rig := newTestRig(t)
	rig.mailer.Err = &ports.GatewayError{StatusCode: 0, Message: "resend is unreachable"}
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	result, err := rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Failed != 1 || result.Sent != 0 {
		t.Errorf("result = %+v, want the send counted as failed", result)
	}

	job := rig.jobs.OfKind(notification.KindBookingConfirmation)[0]
	if job.Status != notification.StatusPending || job.Attempts != 1 {
		t.Fatalf("job = %+v, want it pending for a retry", job)
	}
	if !job.DueAt.Equal(fixedNow.Add(time.Minute)) {
		t.Errorf("dueAt = %v, want the backoff applied", job.DueAt)
	}
	if !strings.Contains(job.LastError, "unreachable") {
		t.Errorf("lastError = %q, want the provider's reason recorded", job.LastError)
	}

	// Retry until the budget is spent.
	for i := 0; i < 3; i++ {
		rig.advance(2 * time.Minute)
		if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
			t.Fatalf("retry DispatchDue: %v", err)
		}
	}
	job = rig.jobs.OfKind(notification.KindBookingConfirmation)[0]
	if job.Status != notification.StatusFailed {
		t.Errorf("status = %q after the retry budget, want failed", job.Status)
	}

	// A failed job stays out of every later pass.
	rig.advance(time.Hour)
	result, err = rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Failed != 0 {
		t.Errorf("a failed job was retried again: %+v", result)
	}
}

// TestRecoveredProviderDeliversTheRetry: the point of retrying is that the
// message eventually arrives.
func TestRecoveredProviderDeliversTheRetry(t *testing.T) {
	rig := newTestRig(t)
	rig.mailer.Err = errors.New("provider down")
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}

	rig.mailer.Err = nil
	rig.advance(2 * time.Minute)
	result, err := rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Sent != 1 {
		t.Errorf("result = %+v, want the retry delivered once the provider recovered", result)
	}
}

// TestOneBadRecipientDoesNotBlockTheBatch: a single undeliverable address
// must not hold up everyone else's mail.
func TestOneBadRecipientDoesNotBlockTheBatch(t *testing.T) {
	rig := newTestRig(t)
	rig.mailer.FailFor["broken@example.com"] = &ports.GatewayError{StatusCode: 422, Message: "invalid address"}

	broken := bookingNotice(fixedNow.Add(72 * time.Hour))
	broken.BookingID = "booking-broken"
	broken.ClientEmail = "broken@example.com"
	rig.svc.BookingConfirmed(context.Background(), broken)
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	result, err := rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Sent != 1 || result.Failed != 1 {
		t.Errorf("result = %+v, want the good message through and the bad one failed", result)
	}
	sent := rig.mailer.Sent()
	if len(sent) != 1 || sent[0].To != "ama@example.com" {
		t.Errorf("sent = %+v, want only the deliverable message", sent)
	}
}

// TestUnrenderableJobFailsImmediately: a job with no template will never
// render, so it must not consume the queue on a timer forever.
func TestUnrenderableJobFailsImmediately(t *testing.T) {
	rig := newTestRig(t)
	rig.renderer.FailKind = notification.KindBookingConfirmation
	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	if _, err := rig.svc.DispatchDue(context.Background(), 10); err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}

	job := rig.jobs.OfKind(notification.KindBookingConfirmation)[0]
	if job.Status != notification.StatusFailed {
		t.Errorf("status = %q, want failed on the first pass — retrying an unrenderable job cannot help", job.Status)
	}
	if len(rig.reported) == 0 {
		t.Error("an unrenderable job was not reported")
	}
	if len(rig.mailer.Sent()) != 0 {
		t.Error("an unrendered message was sent")
	}
}

// TestQueueFailureIsReportedNotReturned: the booking has already happened,
// so a broken outbox must be loud but must not be thrown back at the
// caller — the Notifier port has no error to return.
func TestQueueFailureIsReportedNotReturned(t *testing.T) {
	rig := newTestRig(t)
	rig.jobs.CreateErr = errors.New("mongo is down")

	rig.svc.BookingConfirmed(context.Background(), bookingNotice(fixedNow.Add(72*time.Hour)))

	if len(rig.reported) == 0 {
		t.Fatal("a failed queue write was silently swallowed")
	}
	if !strings.Contains(rig.reported[0].Error(), "mongo is down") {
		t.Errorf("reported = %v, want the underlying cause", rig.reported[0])
	}
}

// TestFeedbackSharedCarriesItsFlag: the optional "and resources" clause is
// driven by whether resources were actually shared.
func TestFeedbackSharedCarriesItsFlag(t *testing.T) {
	rig := newTestRig(t)

	rig.svc.FeedbackShared(context.Background(), ports.FeedbackNotice{
		BookingID:        "booking-1",
		ClientName:       "Ama",
		ClientEmail:      "ama@example.com",
		PractitionerName: "Terios",
		ServiceName:      "Massage",
		SessionDate:      fixedNow,
		HasResources:     true,
		Timezone:         "Africa/Accra",
	})
	rig.svc.FeedbackShared(context.Background(), ports.FeedbackNotice{
		BookingID:   "booking-2",
		ClientName:  "Koffi",
		ClientEmail: "koffi@example.com",
		SessionDate: fixedNow,
	})

	jobs := rig.jobs.OfKind(notification.KindFeedbackShared)
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	if jobs[0].Data["hasResources"] != "true" {
		t.Error("hasResources not set when resources were shared")
	}
	if jobs[1].Data["hasResources"] != "" {
		t.Error("hasResources set when no resources were shared")
	}
	if jobs[0].Data["sessionDate"] == "" {
		t.Error("sessionDate missing from the feedback email data")
	}
}

// TestEnquiryGoesToThePracticeNotTheSender: the alert tells the
// practitioner someone got in touch. Sending it to the address a stranger
// typed would let the contact form emit branded mail to anywhere.
func TestEnquiryGoesToThePracticeNotTheSender(t *testing.T) {
	rig := newTestRig(t)

	rig.svc.EnquiryReceived(context.Background(), ports.EnquiryNotice{
		EnquiryID:   "enquiry-1",
		SenderName:  "Ama",
		SenderEmail: "stranger@example.com",
		Message:     "Do you offer prenatal massage?",
	})

	jobs := rig.jobs.OfKind(notification.KindEnquiryReceived)
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if jobs[0].Recipient != "practice@terioscoach.com" {
		t.Errorf("recipient = %q, want the practice inbox", jobs[0].Recipient)
	}
	if jobs[0].Data["senderEmail"] != "stranger@example.com" {
		t.Errorf("data = %v, want the sender's address carried as content", jobs[0].Data)
	}
}

// TestEnquiryWithoutAPracticeInboxIsReported: silently dropping the alert
// would mean enquiries pile up unseen.
func TestEnquiryWithoutAPracticeInboxIsReported(t *testing.T) {
	rig := newTestRig(t)
	rig.svc.practiceEmail = ""

	rig.svc.EnquiryReceived(context.Background(), ports.EnquiryNotice{EnquiryID: "enquiry-1"})

	if len(rig.jobs.OfKind(notification.KindEnquiryReceived)) != 0 {
		t.Error("queued an enquiry alert with no recipient")
	}
	if len(rig.reported) == 0 {
		t.Fatal("a dropped enquiry alert was not reported")
	}
}

// TestTimesRenderInTheClientTimezone: a Ghana client and a London client
// see their own wall clock for the same instant.
func TestTimesRenderInTheClientTimezone(t *testing.T) {
	rig := newTestRig(t)
	start := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)

	accra := bookingNotice(start)
	accra.Timezone = "Africa/Accra" // UTC+0
	rig.svc.BookingConfirmed(context.Background(), accra)

	london := bookingNotice(start)
	london.BookingID = "booking-2"
	london.Timezone = "Europe/London" // UTC+1 in August
	rig.svc.BookingConfirmed(context.Background(), london)

	jobs := rig.jobs.OfKind(notification.KindBookingConfirmation)
	if len(jobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(jobs))
	}
	if !strings.Contains(jobs[0].Data["startTime"], "09:00") {
		t.Errorf("Accra startTime = %q, want 09:00", jobs[0].Data["startTime"])
	}
	if !strings.Contains(jobs[1].Data["startTime"], "10:00") {
		t.Errorf("London startTime = %q, want 10:00 (BST)", jobs[1].Data["startTime"])
	}
	if jobs[1].Data["timezone"] != "Europe/London" {
		t.Errorf("timezone = %q, want it stated alongside the time", jobs[1].Data["timezone"])
	}
}

// TestUnknownTimezoneFallsBackToUTC: a bad timezone must not cost the
// client the time itself.
func TestUnknownTimezoneFallsBackToUTC(t *testing.T) {
	rig := newTestRig(t)
	notice := bookingNotice(time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC))
	notice.Timezone = "Mars/Olympus_Mons"

	rig.svc.BookingConfirmed(context.Background(), notice)

	job := rig.jobs.OfKind(notification.KindBookingConfirmation)[0]
	if !strings.Contains(job.Data["startTime"], "09:00") {
		t.Errorf("startTime = %q, want the UTC time rather than nothing", job.Data["startTime"])
	}
}

// TestDispatchRespectsTheBatchLimit: a backlog is worked in bounded passes.
func TestDispatchRespectsTheBatchLimit(t *testing.T) {
	rig := newTestRig(t)
	for i := 0; i < 5; i++ {
		notice := bookingNotice(fixedNow.Add(72 * time.Hour))
		notice.BookingID = "booking-" + string(rune('a'+i))
		rig.svc.BookingConfirmed(context.Background(), notice)
	}

	result, err := rig.svc.DispatchDue(context.Background(), 2)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Sent != 2 {
		t.Errorf("sent = %d, want the batch limit honoured", result.Sent)
	}

	result, err = rig.svc.DispatchDue(context.Background(), 10)
	if err != nil {
		t.Fatalf("DispatchDue: %v", err)
	}
	if result.Sent != 3 {
		t.Errorf("sent = %d on the second pass, want the remaining 3", result.Sent)
	}
}
