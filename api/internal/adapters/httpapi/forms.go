package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// WithForms mounts the form routes backed by the form port (BE-10).
//
// /v1/admin/forms is the practitioner's builder and review surface;
// /v1/forms is the client's own assigned forms. There is no public
// surface — an intake form is never anonymous. A nil service keeps the
// routes mounted but answering 503.
func WithForms(svc ports.FormService, auth ports.AuthService) Option {
	return func(s *Server) {
		s.Router.Route("/v1/forms", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleFormsUnavailable)
				r.HandleFunc("/", handleFormsUnavailable)
				return
			}
			h := &formHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RoleClient))
			r.Get("/mine", h.listMine)
			r.Get("/mine/{id}", h.getMine)
			r.Post("/mine/{id}/submit", h.submitMine)
		})

		s.Router.Route("/v1/admin/forms", func(r chi.Router) {
			if svc == nil {
				r.HandleFunc("/*", handleFormsUnavailable)
				r.HandleFunc("/", handleFormsUnavailable)
				return
			}
			h := &formHandler{svc: svc}
			r.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/submissions", h.listSubmissions)
			r.Get("/submissions/{id}", h.getSubmission)
			r.Post("/assign", h.assign)
			r.Get("/{id}", h.get)
			r.Patch("/{id}", h.patch)
			r.Delete("/{id}", h.delete)
		})
	}
}

// handleFormsUnavailable answers every form route when the database is not
// configured.
func handleFormsUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusServiceUnavailable, "service_unavailable", "forms are unavailable: database not connected")
}

type formHandler struct {
	svc ports.FormService
}

// ---- Response shapes ----

type fieldBody struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	HelpText string   `json:"helpText,omitempty"`
	Options  []string `json:"options"`
}

type formBody struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	Fields      []fieldBody `json:"fields"`
	Template    bool        `json:"template"`
	SortOrder   int         `json:"sortOrder"`
	Active      bool        `json:"active"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

func newFormBody(f form.Form) formBody {
	fields := make([]fieldBody, 0, len(f.Fields))
	for _, field := range f.Fields {
		options := field.Options
		if options == nil {
			options = []string{}
		}
		fields = append(fields, fieldBody{
			Key:      field.Key,
			Label:    field.Label,
			Type:     string(field.Type),
			Required: field.Required,
			HelpText: field.HelpText,
			Options:  options,
		})
	}
	return formBody{
		ID:          f.ID,
		Title:       f.Title,
		Description: f.Description,
		Fields:      fields,
		Template:    f.Template,
		SortOrder:   f.SortOrder,
		Active:      f.Active,
		CreatedAt:   f.CreatedAt.UTC(),
		UpdatedAt:   f.UpdatedAt.UTC(),
	}
}

type answerBody struct {
	Value  string   `json:"value,omitempty"`
	Values []string `json:"values,omitempty"`
}

// signatureBody deliberately omits the drawn image and the signer's IP.
// The image is only served with the single submission a person opened, and
// the IP is evidence held in the record, not a field for a UI.
type signatureBody struct {
	TypedName string    `json:"typedName"`
	SignedAt  time.Time `json:"signedAt"`
}

type submissionBody struct {
	ID          string                `json:"id"`
	FormID      string                `json:"formId"`
	FormTitle   string                `json:"formTitle"`
	ClientID    string                `json:"clientId"`
	BookingID   string                `json:"bookingId,omitempty"`
	Status      string                `json:"status"`
	Answers     map[string]answerBody `json:"answers"`
	Signature   *signatureBody        `json:"signature,omitempty"`
	AssignedAt  time.Time             `json:"assignedAt"`
	SubmittedAt *time.Time            `json:"submittedAt,omitempty"`
}

func newSubmissionBody(s form.Submission) submissionBody {
	answers := make(map[string]answerBody, len(s.Answers))
	for key, answer := range s.Answers {
		answers[key] = answerBody{Value: answer.Value, Values: answer.Values}
	}
	body := submissionBody{
		ID:          s.ID,
		FormID:      s.FormID,
		FormTitle:   s.FormTitle,
		ClientID:    s.ClientID,
		BookingID:   s.BookingID,
		Status:      string(s.Status),
		Answers:     answers,
		AssignedAt:  s.AssignedAt.UTC(),
		SubmittedAt: s.SubmittedAt,
	}
	if s.Signature != nil {
		body.Signature = &signatureBody{
			TypedName: s.Signature.TypedName,
			SignedAt:  s.Signature.SignedAt.UTC(),
		}
	}
	return body
}

// submissionViewBody carries the definition alongside the answers, so the
// UI renders the version that was actually answered, plus the integrity
// verdict for a signed record.
type submissionViewBody struct {
	Submission  submissionBody `json:"submission"`
	Form        formBody       `json:"form"`
	IntegrityOK bool           `json:"integrityOk"`
	// SignatureImage is the drawn mark, served only on this single-record
	// route — never in a listing.
	SignatureImage string `json:"signatureImage,omitempty"`
}

func newSubmissionViewBody(view ports.SubmissionView) submissionViewBody {
	body := submissionViewBody{
		Submission:  newSubmissionBody(view.Submission),
		Form:        newFormBody(view.Form),
		IntegrityOK: view.IntegrityOK,
	}
	if view.Submission.Signature != nil {
		body.SignatureImage = view.Submission.Signature.ImageData
	}
	return body
}

// decodeFields turns the request's field list into domain fields.
func decodeFields(raw []fieldBody) []form.Field {
	fields := make([]form.Field, 0, len(raw))
	for _, field := range raw {
		fields = append(fields, form.Field{
			Key:      field.Key,
			Label:    field.Label,
			Type:     form.FieldType(field.Type),
			Required: field.Required,
			HelpText: field.HelpText,
			Options:  field.Options,
		})
	}
	return fields
}

// ---- Practitioner: definitions ----

func (h *formHandler) list(w http.ResponseWriter, r *http.Request) {
	forms, err := h.svc.ListForms(r.Context(), r.URL.Query().Get("active") == "true")
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]formBody, 0, len(forms))
	for _, f := range forms {
		items = append(items, newFormBody(f))
	}
	writeJSON(w, http.StatusOK, map[string][]formBody{"items": items})
}

func (h *formHandler) get(w http.ResponseWriter, r *http.Request) {
	f, err := h.svc.GetForm(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]formBody{"form": newFormBody(f)})
}

func (h *formHandler) create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Fields      []fieldBody `json:"fields"`
		Template    bool        `json:"template"`
		SortOrder   int         `json:"sortOrder"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	f, err := h.svc.CreateForm(r.Context(), ports.FormInput{
		Title:       req.Title,
		Description: req.Description,
		Fields:      decodeFields(req.Fields),
		Template:    req.Template,
		SortOrder:   req.SortOrder,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]formBody{"form": newFormBody(f)})
}

func (h *formHandler) patch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       *string      `json:"title"`
		Description *string      `json:"description"`
		Fields      *[]fieldBody `json:"fields"`
		Template    *bool        `json:"template"`
		SortOrder   *int         `json:"sortOrder"`
		Active      *bool        `json:"active"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	patch := form.Patch{
		Title:       req.Title,
		Description: req.Description,
		Template:    req.Template,
		SortOrder:   req.SortOrder,
		Active:      req.Active,
	}
	if req.Fields != nil {
		fields := decodeFields(*req.Fields)
		patch.Fields = &fields
	}
	f, err := h.svc.UpdateForm(r.Context(), chi.URLParam(r, "id"), patch)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]formBody{"form": newFormBody(f)})
}

func (h *formHandler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteForm(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Practitioner: assignment and review ----

func (h *formHandler) assign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FormID    string `json:"formId"`
		ClientID  string `json:"clientId"`
		BookingID string `json:"bookingId"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	submission, err := h.svc.AssignForm(r.Context(), ports.AssignInput{
		FormID:    req.FormID,
		ClientID:  req.ClientID,
		BookingID: req.BookingID,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]submissionBody{"submission": newSubmissionBody(submission)})
}

func (h *formHandler) listSubmissions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	status := form.SubmissionStatus(query.Get("status"))
	if status != "" && !status.Valid() {
		writeError(w, http.StatusBadRequest, "validation_error", "status must be assigned or submitted")
		return
	}
	items, err := h.svc.ListSubmissions(r.Context(), ports.SubmissionFilter{
		ClientID:  query.Get("clientId"),
		FormID:    query.Get("formId"),
		BookingID: query.Get("bookingId"),
		Status:    status,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]submissionBody, 0, len(items))
	for _, s := range items {
		out = append(out, newSubmissionBody(s))
	}
	writeJSON(w, http.StatusOK, map[string][]submissionBody{"items": out})
}

func (h *formHandler) getSubmission(w http.ResponseWriter, r *http.Request) {
	view, err := h.svc.GetSubmission(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newSubmissionViewBody(view))
}

// ---- Client: own forms ----

func (h *formHandler) listMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListMySubmissions(r.Context(), id.UserID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]submissionBody, 0, len(items))
	for _, s := range items {
		out = append(out, newSubmissionBody(s))
	}
	writeJSON(w, http.StatusOK, map[string][]submissionBody{"items": out})
}

func (h *formHandler) getMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	view, err := h.svc.GetMySubmission(r.Context(), id.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newSubmissionViewBody(view))
}

func (h *formHandler) submitMine(w http.ResponseWriter, r *http.Request) {
	id, ok := identityOr401(w, r)
	if !ok {
		return
	}
	var req struct {
		Answers   map[string]answerBody `json:"answers"`
		Signature *struct {
			TypedName string `json:"typedName"`
			ImageData string `json:"imageData"`
		} `json:"signature"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	answers := make(map[string]form.Answer, len(req.Answers))
	for key, answer := range req.Answers {
		answers[key] = form.Answer{Value: answer.Value, Values: answer.Values}
	}

	in := ports.SubmitInput{Answers: answers}
	if req.Signature != nil {
		in.Signature = &form.SignatureInput{
			TypedName: req.Signature.TypedName,
			ImageData: req.Signature.ImageData,
			// The signing address is observed, never submitted: a client
			// cannot choose what a consent record says about where it was
			// signed from.
			IP: clientKey(r),
		}
	}

	submission, err := h.svc.SubmitMyForm(r.Context(), id.UserID, chi.URLParam(r, "id"), in)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]submissionBody{"submission": newSubmissionBody(submission)})
}
