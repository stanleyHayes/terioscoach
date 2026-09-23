package notifications

import (
	"context"
	"errors"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
	"strings"
	"testing"
	"time"
)

type fakeEvents struct {
	events           []ports.WorkflowEvent
	complete, failed int
}

func (f *fakeEvents) Pending(context.Context, int) ([]ports.WorkflowEvent, error) {
	return f.events, nil
}
func (f *fakeEvents) Complete(context.Context, string) error       { f.complete++; return nil }
func (f *fakeEvents) Failed(context.Context, string, string) error { f.failed++; return nil }
func TestWorkflowReplaySurvivesQueueFailureAndDeduplicatesRecipients(t *testing.T) {
	ctx := context.Background()
	r := newTestRig(t)
	users := portstest.NewFakeUserRepository()
	client, _ := users.Create(ctx, identity.User{Name: "Client", Email: "client@example.com", Role: identity.RoleClient, Timezone: "America/New_York"})
	practitioner, _ := users.Create(ctx, identity.User{Name: "Practitioner", Email: "assigned@example.com", Role: identity.RolePractitioner, Timezone: "Pacific/Honolulu"})
	bookings := portstest.NewFakeBookingRepository()
	start := fixedNow.Add(2 * time.Hour)
	b, _ := bookings.Create(ctx, booking.Booking{ClientID: client.ID, PractitionerID: practitioner.ID, Status: booking.StatusConfirmed, StartAt: start, EndAt: start.Add(time.Hour)})
	event := ports.WorkflowEvent{ID: "event-one", Collection: "bookings", EntityID: b.ID, After: map[string]string{"clientId": client.ID, "practitionerId": practitioner.ID, "status": "confirmed", "startAt": start.Format(time.RFC3339)}}
	events := &fakeEvents{events: []ports.WorkflowEvent{event}}
	r.svc.events = events
	r.svc.users = users
	r.svc.bookings = bookings
	r.jobs.CreateErr = errors.New("outbox unavailable")
	r.svc.reconcile(ctx, 10)
	if events.failed != 1 || events.complete != 0 {
		t.Fatal("lost failed event")
	}
	r.jobs.CreateErr = nil
	r.svc.reconcile(ctx, 10)
	jobs := r.jobs.All()
	if len(jobs) != 4 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	for _, job := range jobs {
		if job.Recipient == "practice@terioscoach.com" {
			t.Fatal("routed to generic practice instead of assigned practitioner")
		}
	}
	items, _ := r.inApp.ListForRecipient(ctx, client.Email, 10)
	if len(items) != 1 {
		t.Fatalf("feed=%d", len(items))
	}
	_ = r.inApp.MarkRead(ctx, items[0].ID, client.Email)
	r.svc.reconcile(ctx, 10)
	if len(r.jobs.All()) != 4 {
		t.Fatal("duplicate jobs on replay")
	}
	items, _ = r.inApp.ListForRecipient(ctx, client.Email, 10)
	if len(items) != 1 || !items[0].Read {
		t.Fatal("read state reset")
	}
	var clientBody, practiceBody string
	for _, job := range r.jobs.OfKind(notification.KindActionRequired) {
		if job.Recipient == client.Email {
			clientBody = job.Data["body"]
		} else {
			practiceBody = job.Data["body"]
		}
	}
	if clientBody == practiceBody || !strings.Contains(clientBody, "America/New_York") {
		t.Fatal("recipient timezone ignored")
	}
	// A committed cancellation cancels both reminders before their due time.
	event.ID = "event-two"
	event.Before = event.After
	event.After = map[string]string{"clientId": client.ID, "practitionerId": practitioner.ID, "status": "cancelled", "startAt": start.Format(time.RFC3339)}
	if err := r.svc.replayMutation(ctx, event); err != nil {
		t.Fatal(err)
	}
	for _, job := range r.jobs.OfKind(notification.KindSessionReminder) {
		if job.Status != notification.StatusCancelled {
			t.Fatal("obsolete reminder retained")
		}
	}
}
func TestReminderReloadsBookingAndSavedZoneBeforeDelivery(t *testing.T) {
	ctx := context.Background()
	r := newTestRig(t)
	users := portstest.NewFakeUserRepository()
	u, _ := users.Create(ctx, identity.User{Email: "client@example.com", Role: identity.RoleClient, Timezone: "America/Chicago"})
	r.svc.users = users
	repo := portstest.NewFakeBookingRepository()
	b, _ := repo.Create(ctx, booking.Booking{ClientID: u.ID, Status: booking.StatusConfirmed, StartAt: fixedNow.Add(5 * time.Minute), EndAt: fixedNow.Add(time.Hour)})
	r.svc.bookings = repo
	job, _ := notification.New(notification.KindSessionReminder, u.Email, map[string]string{"recipientId": u.ID, "startAt": b.StartAt.Format(time.RFC3339), "timezone": "UTC"}, fixedNow, fixedNow)
	job.BookingID = b.ID
	job, _ = r.jobs.Create(ctx, job)
	if !r.svc.deliver(ctx, job) {
		t.Fatal("delivery failed")
	}
	stored := r.jobs.All()[0]
	if stored.Data["renderedTimezone"] != "America/Chicago" {
		t.Fatal("delivery ignored latest preference")
	}
	if r.mailer.Sent()[0].IdempotencyKey != "notification-"+job.ID {
		t.Fatal("provider key missing")
	}
}

type lostAckJobs struct {
	*portstest.FakeNotificationJobRepository
	loseAck bool
}

func (r *lostAckJobs) Update(ctx context.Context, j notification.Job) (notification.Job, error) {
	if r.loseAck && j.Status == notification.StatusSent {
		r.loseAck = false
		return notification.Job{}, errors.New("database acknowledgement lost")
	}
	return r.FakeNotificationJobRepository.Update(ctx, j)
}
func (r *lostAckJobs) PrepareDelivery(ctx context.Context, j notification.Job) (notification.Job, error) {
	for _, old := range r.All() {
		if old.ID == j.ID && old.Data["deliveryPrepared"] == "true" {
			return old, nil
		}
	}
	return r.FakeNotificationJobRepository.Update(ctx, j)
}
func TestProviderAcceptanceBeforeLocalAcknowledgementReusesFrozenPayload(t *testing.T) {
	ctx := context.Background()
	r := newTestRig(t)
	jobs := &lostAckJobs{FakeNotificationJobRepository: r.jobs, loseAck: true}
	r.svc.jobs = jobs
	job, _ := notification.New(notification.KindActionRequired, "client@example.com", map[string]string{"title": "Document ready", "link": "/portal/documents", "body": "Review your document"}, fixedNow, fixedNow)
	job, _ = jobs.Create(ctx, job)
	if !r.svc.deliver(ctx, job) {
		t.Fatal("provider did not accept first attempt")
	}
	stored := jobs.All()[0]
	if stored.Status != notification.StatusPending {
		t.Fatal("failure window not retained")
	}
	// Even a now-unavailable template must not alter the frozen provider request.
	r.renderer.FailKind = notification.KindActionRequired
	if !r.svc.deliver(ctx, stored) {
		t.Fatal("retry did not use persisted payload")
	}
	sent := r.mailer.Sent()
	if len(sent) != 2 || sent[0] != sent[1] || sent[0].IdempotencyKey == "" {
		t.Fatal("provider retries differ")
	}
	items, _ := r.inApp.ListForRecipient(ctx, job.Recipient, 10)
	if len(items) != 1 {
		t.Fatal("duplicate feed after lost acknowledgement")
	}
}
func TestArchiveFailureKeepsExecutionEventRecoverable(t *testing.T) {
	ctx := context.Background()
	r := newTestRig(t)
	users := portstest.NewFakeUserRepository()
	u, _ := users.Create(ctx, identity.User{Role: identity.RoleClient, Email: "client@example.com"})
	p, _ := users.Create(ctx, identity.User{Role: identity.RolePractitioner, Email: "practice@example.com"})
	r.svc.users = users
	events := &fakeEvents{events: []ports.WorkflowEvent{{ID: "signed-event", Collection: "agreement_executions", EntityID: "execution", After: map[string]string{"clientId": u.ID, "practitionerId": p.ID}}}}
	r.svc.events = events
	r.svc.archiveExecution = func(context.Context, string) error { return errors.New("storage offline") }
	r.svc.reconcile(ctx, 10)
	if events.failed != 1 || events.complete != 0 || len(r.jobs.All()) != 2 {
		t.Fatal("archive outage lost event or notification")
	}
	r.svc.archiveExecution = func(context.Context, string) error { return nil }
	r.svc.reconcile(ctx, 10)
	if events.complete != 1 || len(r.jobs.All()) != 2 {
		t.Fatal("archive replay duplicated notification")
	}
}
