package notifications

import (
	"context"
	"errors"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
	"testing"
	"time"
)

func TestFeedDoesNotDependOnEmailAndRetriesDoNotDuplicate(t *testing.T) {
	r := newTestRig(t)
	ctx := context.Background()
	r.mailer.FailFor["ama@example.com"] = errors.New("email offline")
	r.svc.BookingConfirmed(ctx, bookingNotice(fixedNow.Add(time.Hour)))
	items, _ := r.inApp.ListForRecipient(ctx, "ama@example.com", 50)
	if len(items) != 1 {
		t.Fatalf("immediate feed entries = %d", len(items))
	}
	if items[0].Link != "/portal/sessions" {
		t.Fatal(items[0])
	}
	_ = r.inApp.MarkRead(ctx, items[0].ID, "ama@example.com")
	_, _ = r.svc.DispatchDue(ctx, 50)
	r.advance(2 * time.Minute)
	_, _ = r.svc.DispatchDue(ctx, 50)
	items, _ = r.inApp.ListForRecipient(ctx, "ama@example.com", 50)
	if len(items) != 1 || !items[0].Read {
		t.Fatalf("retry duplicated or reset read state: %+v", items)
	}
	practice, _ := r.inApp.ListForRecipient(ctx, "practice@terioscoach.com", 50)
	if len(practice) != 1 || practice[0].Link != "/calendar" {
		t.Fatalf("wrong practice destination: %+v", practice)
	}
}

func TestRemindersAppearOnlyWhenDueAndCancelledOnesNeverAppear(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		r := newTestRig(t)
		ctx := context.Background()
		r.svc.BookingConfirmed(ctx, bookingNotice(fixedNow.Add(time.Hour)))
		if cancel {
			r.svc.BookingCancelled(ctx, bookingNotice(fixedNow.Add(time.Hour)))
		}
		_, _ = r.svc.DispatchDue(ctx, 50)
		items, _ := r.inApp.ListForRecipient(ctx, "ama@example.com", 50)
		for _, item := range items {
			if item.Kind == notification.KindSessionReminder {
				t.Fatal("early reminder")
			}
		}
		r.advance(51 * time.Minute)
		_, _ = r.svc.DispatchDue(ctx, 50)
		items, _ = r.inApp.ListForRecipient(ctx, "ama@example.com", 50)
		reminders := 0
		for _, item := range items {
			if item.Kind == notification.KindSessionReminder {
				reminders++
			}
		}
		if (!cancel && reminders != 1) || (cancel && reminders != 0) {
			t.Fatalf("cancel=%v reminders=%d", cancel, reminders)
		}
	}
}

func TestActivityIsDurableWithoutSendingEmailAndResolvesAccounts(t *testing.T) {
	r := newTestRig(t)
	ctx := context.Background()
	users := portstest.NewFakeUserRepository()
	r.svc.users = users
	client, _ := users.Create(ctx, identity.User{Email: "client@example.com", Role: identity.RoleClient})
	practitioner, _ := users.Create(ctx, identity.User{Email: "owner@example.com", Role: identity.RolePractitioner})
	notice := ports.ActivityNotice{EventID: "recording:1", ClientID: client.ID, PractitionerID: practitioner.ID, Title: "Recording ready", ClientLink: "/portal/sessions", PracticeLink: "/clients"}
	r.svc.Activity(ctx, notice)
	r.svc.Activity(ctx, notice)
	_, _ = r.svc.DispatchDue(ctx, 50)
	for _, email := range []string{client.Email, practitioner.Email} {
		items, _ := r.inApp.ListForRecipient(ctx, email, 50)
		if len(items) != 1 {
			t.Fatalf("%s entries=%d", email, len(items))
		}
	}
	if len(r.mailer.Sent()) != 0 {
		t.Fatal("in-app activity sent email")
	}
	r.svc.FormSubmitted(ctx, ports.FormSubmittedNotice{SubmissionID: "f1", ClientName: "Client", FormTitle: "Intake"})
	items, _ := r.inApp.ListForRecipient(ctx, practitioner.Email, 50)
	if len(items) != 2 {
		t.Fatalf("practice mailbox alias not resolved to owner: %+v", items)
	}
}

func TestPracticeFeedDoesNotRequireEmailRoutingSetting(t *testing.T) {
	r := newTestRig(t)
	r.svc.practiceEmail = ""
	r.svc.users = portstest.NewFakeUserRepository()
	_, err := r.svc.users.Create(t.Context(), identity.User{Email: "owner@example.com", Role: identity.RolePractitioner})
	if err != nil {
		t.Fatal(err)
	}
	r.svc.FormSubmitted(t.Context(), ports.FormSubmittedNotice{SubmissionID: "form", ClientName: "Client", FormTitle: "Intake"})
	items, _ := r.inApp.ListForRecipient(t.Context(), "owner@example.com", 50)
	if len(items) != 1 || items[0].Link != "/forms" {
		t.Fatalf("practice notification lost without configured mailbox: %+v", items)
	}
}

func TestFormAssignmentAndSubmissionAppearForBothParticipants(t *testing.T) {
	r := newTestRig(t)
	ctx := t.Context()
	r.svc.FormAssigned(ctx, ports.FormAssignedNotice{ClientEmail: "client@example.com", FormTitle: "Intake", SubmissionID: "f1"})
	r.svc.FormSubmitted(ctx, ports.FormSubmittedNotice{ClientEmail: "client@example.com", ClientName: "Client", FormTitle: "Intake", SubmissionID: "f1"})
	for _, recipient := range []string{"client@example.com", "practice@terioscoach.com"} {
		items, _ := r.inApp.ListForRecipient(ctx, recipient, 50)
		if len(items) != 2 {
			t.Fatalf("%s feed entries=%d", recipient, len(items))
		}
	}
	_, _ = r.svc.DispatchDue(ctx, 50)
	for _, recipient := range []string{"client@example.com", "practice@terioscoach.com"} {
		items, _ := r.inApp.ListForRecipient(ctx, recipient, 50)
		if len(items) != 2 {
			t.Fatalf("duplicate receipt for %s", recipient)
		}
	}
}
