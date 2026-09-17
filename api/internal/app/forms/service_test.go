package forms

import (
	"context"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var testNow = time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)

type recordingFormNotifier struct {
	assigned  []ports.FormAssignedNotice
	submitted []ports.FormSubmittedNotice
}

func (r *recordingFormNotifier) FormAssigned(_ context.Context, notice ports.FormAssignedNotice) {
	r.assigned = append(r.assigned, notice)
}

func (r *recordingFormNotifier) FormSubmitted(_ context.Context, notice ports.FormSubmittedNotice) {
	r.submitted = append(r.submitted, notice)
}

func newFormRig(t *testing.T) (*Service, *recordingFormNotifier, form.Form) {
	t.Helper()
	users := portstest.NewFakeUserRepository()
	if _, err := users.Create(context.Background(), identity.User{
		Email: "ama@example.com",
		Name:  "Ama Serwaa",
		Role:  identity.RoleClient,
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	forms := portstest.NewFakeFormRepository()
	submissions := portstest.NewFakeFormSubmissionRepository()
	def, err := forms.Create(context.Background(), form.Form{
		Title: "Health Intake",
		Fields: []form.Field{{
			Key:      "goal",
			Label:    "Goal",
			Type:     form.FieldText,
			Required: true,
		}},
		Active:    true,
		CreatedAt: testNow,
		UpdatedAt: testNow,
	})
	if err != nil {
		t.Fatalf("seed form: %v", err)
	}
	notifier := &recordingFormNotifier{}
	svc := NewService(forms, submissions, Options{
		Users:    users,
		Notifier: notifier,
		Now:      func() time.Time { return testNow },
	})
	return svc, notifier, def
}

func TestAssignFormNotifiesTheClient(t *testing.T) {
	svc, notifier, def := newFormRig(t)

	submission, err := svc.AssignForm(context.Background(), ports.AssignInput{
		FormID:   def.ID,
		ClientID: "user-1",
	})
	if err != nil {
		t.Fatalf("AssignForm: %v", err)
	}
	if len(notifier.assigned) != 1 {
		t.Fatalf("assigned notices = %d, want 1", len(notifier.assigned))
	}
	notice := notifier.assigned[0]
	if notice.SubmissionID != submission.ID || notice.ClientEmail != "ama@example.com" || notice.FormTitle != "Health Intake" {
		t.Errorf("notice = %+v, want resolved client and form details", notice)
	}
}

func TestSubmitMyFormNotifiesThePractice(t *testing.T) {
	svc, notifier, def := newFormRig(t)
	submission, err := svc.AssignForm(context.Background(), ports.AssignInput{
		FormID:   def.ID,
		ClientID: "user-1",
	})
	if err != nil {
		t.Fatalf("AssignForm: %v", err)
	}

	updated, err := svc.SubmitMyForm(context.Background(), "user-1", submission.ID, ports.SubmitInput{
		Answers: map[string]form.Answer{"goal": {Value: "Improve sleep"}},
	})
	if err != nil {
		t.Fatalf("SubmitMyForm: %v", err)
	}
	if updated.SubmittedAt == nil {
		t.Fatal("submission was not completed")
	}
	if len(notifier.submitted) != 1 {
		t.Fatalf("submitted notices = %d, want 1", len(notifier.submitted))
	}
	notice := notifier.submitted[0]
	if notice.SubmissionID != submission.ID || notice.ClientName != "Ama Serwaa" || notice.FormTitle != "Health Intake" {
		t.Errorf("notice = %+v, want resolved client and form details", notice)
	}
}
