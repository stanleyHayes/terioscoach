package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/form"
)

// FormRepository is the outbound port for form definitions.
type FormRepository interface {
	Create(ctx context.Context, f form.Form) (form.Form, error)
	Update(ctx context.Context, f form.Form) (form.Form, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return form.ErrFormNotFound.
	FindByID(ctx context.Context, id string) (form.Form, error)
	// List returns definitions in sortOrder then title. activeOnly hides
	// retired forms from the assign picker.
	List(ctx context.Context, activeOnly bool) ([]form.Form, error)
}

// SubmissionFilter narrows a submission listing.
type SubmissionFilter struct {
	ClientID  string
	FormID    string
	BookingID string
	Status    form.SubmissionStatus
}

// FormSubmissionRepository is the outbound port for filled-in forms.
type FormSubmissionRepository interface {
	Create(ctx context.Context, s form.Submission) (form.Submission, error)
	Update(ctx context.Context, s form.Submission) (form.Submission, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return form.ErrSubmissionNotFound.
	FindByID(ctx context.Context, id string) (form.Submission, error)
	// List returns submissions newest-first. Client-scoped queries lead
	// with clientId — the isolation rule.
	List(ctx context.Context, filter SubmissionFilter) ([]form.Submission, error)
	// HasOpenAssignment reports whether the client already has an
	// unsubmitted copy of this form, so assigning twice does not bury them
	// in duplicates.
	HasOpenAssignment(ctx context.Context, clientID, formID string) (bool, error)
}

// FormInput is the create payload for a form definition.
type FormInput struct {
	Title       string
	Description string
	Fields      []form.Field
	Template    bool
	SortOrder   int
}

// AssignInput sends a form to a client, optionally attached to a booking.
type AssignInput struct {
	FormID    string
	ClientID  string
	BookingID string
}

// SubmitInput is the client's completed form.
type SubmitInput struct {
	Answers   map[string]form.Answer
	Signature *form.SignatureInput
}

// SubmissionView is a submission plus the definition needed to render it
// and the verdict of its integrity check. The definition travels with the
// submission because a form may have been edited since it was sent, and
// what must be rendered is the version that was actually answered.
type SubmissionView struct {
	Submission form.Submission
	Form       form.Form
	// IntegrityOK is false when a signed record no longer matches its
	// digest — the record has been altered since it was signed.
	IntegrityOK bool
}

// FormService is the inbound port for the forms slice (BE-10).
type FormService interface {
	// Definitions — practitioner only.
	ListForms(ctx context.Context, activeOnly bool) ([]form.Form, error)
	GetForm(ctx context.Context, id string) (form.Form, error)
	CreateForm(ctx context.Context, in FormInput) (form.Form, error)
	UpdateForm(ctx context.Context, id string, patch form.Patch) (form.Form, error)
	DeleteForm(ctx context.Context, id string) error

	// Assignment and review — practitioner only.
	AssignForm(ctx context.Context, in AssignInput) (form.Submission, error)
	ListSubmissions(ctx context.Context, filter SubmissionFilter) ([]form.Submission, error)
	GetSubmission(ctx context.Context, id string) (SubmissionView, error)

	// The client's own forms.
	ListMySubmissions(ctx context.Context, clientID string) ([]form.Submission, error)
	GetMySubmission(ctx context.Context, clientID, submissionID string) (SubmissionView, error)
	SubmitMyForm(ctx context.Context, clientID, submissionID string, in SubmitInput) (form.Submission, error)
}
