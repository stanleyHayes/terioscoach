// Package forms is the application service for the intake-and-consent
// slice. It implements the inbound ports.FormService port purely against
// outbound ports — no framework, driver, or transport imports.
//
// Two rules run through it. Client isolation: every client-facing use case
// leads with the caller's own id, and someone else's submission is reported
// as missing rather than forbidden. And validation-by-definition: a
// submission is always checked against the form it belongs to, which the
// service loads itself rather than taking from the request.
package forms

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the forms use cases over outbound ports.
type Service struct {
	forms       ports.FormRepository
	submissions ports.FormSubmissionRepository
	now         func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.FormService = (*Service)(nil)

// NewService wires the use cases to their outbound ports.
func NewService(forms ports.FormRepository, submissions ports.FormSubmissionRepository) *Service {
	return &Service{
		forms:       forms,
		submissions: submissions,
		now:         func() time.Time { return time.Now().UTC() },
	}
}

// ---- Definitions (practitioner) ----

// ListForms returns the form library.
func (s *Service) ListForms(ctx context.Context, activeOnly bool) ([]form.Form, error) {
	return s.forms.List(ctx, activeOnly)
}

// GetForm returns one definition.
func (s *Service) GetForm(ctx context.Context, id string) (form.Form, error) {
	return s.forms.FindByID(ctx, id)
}

// CreateForm adds a definition to the library.
func (s *Service) CreateForm(ctx context.Context, in ports.FormInput) (form.Form, error) {
	f, err := form.New(in.Title, in.Description, in.Fields, s.now())
	if err != nil {
		return form.Form{}, err
	}
	f.Template = in.Template
	f.SortOrder = in.SortOrder
	return s.forms.Create(ctx, f)
}

// UpdateForm edits a definition.
//
// Editing a form does not touch submissions that were already made against
// it: each carries its own snapshot of the title, and GetSubmission returns
// the definition alongside so an old answer is always rendered against the
// version that was answered. A consent record must not silently change its
// wording because the template moved on.
func (s *Service) UpdateForm(ctx context.Context, id string, patch form.Patch) (form.Form, error) {
	f, err := s.forms.FindByID(ctx, id)
	if err != nil {
		return form.Form{}, err
	}
	if err := f.Apply(patch, s.now()); err != nil {
		return form.Form{}, err
	}
	return s.forms.Update(ctx, f)
}

// DeleteForm removes a definition from the library.
//
// A form that has been sent to anyone is deactivated instead of deleted:
// deleting it would strand every signed consent record that points at it.
// Both answers look the same to the caller — the difference is invisible
// and the record is what matters.
func (s *Service) DeleteForm(ctx context.Context, id string) error {
	f, err := s.forms.FindByID(ctx, id)
	if err != nil {
		return err
	}
	used, err := s.submissions.List(ctx, ports.SubmissionFilter{FormID: id})
	if err != nil {
		return err
	}
	if len(used) > 0 {
		inactive := false
		if err := f.Apply(form.Patch{Active: &inactive}, s.now()); err != nil {
			return err
		}
		_, err = s.forms.Update(ctx, f)
		return err
	}
	return s.forms.Delete(ctx, id)
}

// ---- Assignment and review (practitioner) ----

// AssignForm sends a form to a client. Assigning a form the client already
// has open is refused rather than piling up duplicates in their portal.
func (s *Service) AssignForm(ctx context.Context, in ports.AssignInput) (form.Submission, error) {
	f, err := s.forms.FindByID(ctx, in.FormID)
	if err != nil {
		return form.Submission{}, err
	}
	open, err := s.submissions.HasOpenAssignment(ctx, in.ClientID, in.FormID)
	if err != nil {
		return form.Submission{}, err
	}
	if open {
		return form.Submission{}, form.ErrAlreadyAssigned
	}

	submission, err := form.Assign(f, in.ClientID, in.BookingID, s.now())
	if err != nil {
		return form.Submission{}, err
	}
	return s.submissions.Create(ctx, submission)
}

// ListSubmissions returns the practice's view of filled-in forms.
func (s *Service) ListSubmissions(ctx context.Context, filter ports.SubmissionFilter) ([]form.Submission, error) {
	if filter.Status != "" && !filter.Status.Valid() {
		return nil, form.ErrSubmissionNotFound
	}
	return s.submissions.List(ctx, filter)
}

// GetSubmission returns one submission with its definition and the verdict
// of its integrity check.
func (s *Service) GetSubmission(ctx context.Context, id string) (ports.SubmissionView, error) {
	submission, err := s.submissions.FindByID(ctx, id)
	if err != nil {
		return ports.SubmissionView{}, err
	}
	return s.view(ctx, submission)
}

// ---- The client's own forms ----

// ListMySubmissions returns the caller's own forms, assigned and completed.
func (s *Service) ListMySubmissions(ctx context.Context, clientID string) ([]form.Submission, error) {
	return s.submissions.List(ctx, ports.SubmissionFilter{ClientID: clientID})
}

// GetMySubmission returns one of the caller's own forms with its
// definition, which is what the portal renders the questions from.
func (s *Service) GetMySubmission(ctx context.Context, clientID, submissionID string) (ports.SubmissionView, error) {
	submission, err := s.ownedSubmission(ctx, clientID, submissionID)
	if err != nil {
		return ports.SubmissionView{}, err
	}
	return s.view(ctx, submission)
}

// SubmitMyForm completes and (where required) signs the caller's own form.
// The definition is loaded here, not taken from the request: validation
// must be against the practice's form, not the client's copy of it.
func (s *Service) SubmitMyForm(ctx context.Context, clientID, submissionID string, in ports.SubmitInput) (form.Submission, error) {
	submission, err := s.ownedSubmission(ctx, clientID, submissionID)
	if err != nil {
		return form.Submission{}, err
	}
	f, err := s.forms.FindByID(ctx, submission.FormID)
	if err != nil {
		return form.Submission{}, err
	}
	if err := submission.Submit(f, in.Answers, in.Signature, s.now()); err != nil {
		return form.Submission{}, err
	}
	return s.submissions.Update(ctx, submission)
}

// ownedSubmission loads a submission belonging to the client. Someone
// else's is reported as missing — no existence leak.
func (s *Service) ownedSubmission(ctx context.Context, clientID, submissionID string) (form.Submission, error) {
	submission, err := s.submissions.FindByID(ctx, submissionID)
	if err != nil {
		return form.Submission{}, err
	}
	if submission.ClientID != clientID {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	return submission, nil
}

// view assembles a submission with its definition and integrity verdict. A
// definition that has since been deleted does not hide the answers: the
// submission is returned with an empty form rather than an error.
func (s *Service) view(ctx context.Context, submission form.Submission) (ports.SubmissionView, error) {
	view := ports.SubmissionView{
		Submission:  submission,
		IntegrityOK: submission.VerifyIntegrity(),
	}
	if f, err := s.forms.FindByID(ctx, submission.FormID); err == nil {
		view.Form = f
	}
	return view, nil
}
