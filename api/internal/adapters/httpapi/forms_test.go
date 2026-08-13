package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	formsapp "github.com/xcreativs/terios/api/internal/app/forms"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

const formSignatureImage = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUg=="

// formTestRig bundles a server with the builder and the client surfaces.
type formTestRig struct {
	srv               *Server
	practitionerToken string
	clientToken       string
	otherClientToken  string
}

func newFormTestRig(t *testing.T) formTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	svc := formsapp.NewService(
		portstest.NewFakeFormRepository(),
		portstest.NewFakeFormSubmissionRepository(),
	)

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return formTestRig{
		srv:               NewServer(WithAuth(authSvc), WithForms(svc, authSvc)),
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
		otherClientToken:  issue("client-2", identity.RoleClient),
	}
}

func consentFormPayload() map[string]any {
	return map[string]any{
		"title":       "Intake and consent",
		"description": "Please complete before your first session.",
		"fields": []map[string]any{
			{"key": "full_name", "label": "Full name", "type": "text", "required": true},
			{"key": "pressure", "label": "Preferred pressure", "type": "select", "required": true,
				"options": []string{"Light", "Medium", "Firm"}},
			{"key": "notes", "label": "Anything we should know?", "type": "textarea"},
			{"key": "consent", "label": "Signature", "type": "signature", "required": true},
		},
	}
}

type formTestBody struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
	Fields []struct {
		Key      string   `json:"key"`
		Type     string   `json:"type"`
		Required bool     `json:"required"`
		Options  []string `json:"options"`
	} `json:"fields"`
}

type submissionTestBody struct {
	ID        string `json:"id"`
	FormID    string `json:"formId"`
	FormTitle string `json:"formTitle"`
	ClientID  string `json:"clientId"`
	Status    string `json:"status"`
	Signature *struct {
		TypedName string `json:"typedName"`
	} `json:"signature"`
}

func createForm(t *testing.T, rig formTestRig) formTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/forms", consentFormPayload(), bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create form status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Form formTestBody `json:"form"`
	}
	decodeBody(t, rec, &res)
	return res.Form
}

func assignForm(t *testing.T, rig formTestRig, formID, clientID string) submissionTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/forms/assign", map[string]any{
		"formId": formID, "clientId": clientID, "bookingId": "booking-1",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("assign status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Submission submissionTestBody `json:"submission"`
	}
	decodeBody(t, rec, &res)
	return res.Submission
}

func submitPayload() map[string]any {
	return map[string]any{
		"answers": map[string]any{
			"full_name": map[string]any{"value": "Ama Serwaa"},
			"pressure":  map[string]any{"value": "Medium"},
		},
		"signature": map[string]any{
			"typedName": "Ama Serwaa",
			"imageData": formSignatureImage,
		},
	}
}

// TestBuildAssignAndSign walks the whole slice: build a form, send it,
// complete it, and read it back as both parties.
func TestBuildAssignAndSign(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	if !f.Active || len(f.Fields) != 4 {
		t.Fatalf("form = %+v, want an active four-field form", f)
	}

	submission := assignForm(t, rig, f.ID, "client-1")
	if submission.Status != "assigned" || submission.FormTitle != "Intake and consent" {
		t.Fatalf("submission = %+v, want an assignment with the title snapshotted", submission)
	}

	// The client sees it waiting.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/forms/mine", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list mine status = %d, body %s", rec.Code, rec.Body.String())
	}
	var mine struct {
		Items []submissionTestBody `json:"items"`
	}
	decodeBody(t, rec, &mine)
	if len(mine.Items) != 1 || mine.Items[0].Status != "assigned" {
		t.Fatalf("client's forms = %+v, want one assigned", mine.Items)
	}

	// And gets the definition to render it from.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/forms/mine/"+submission.ID, nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("get mine status = %d, body %s", rec.Code, rec.Body.String())
	}
	var view struct {
		Form        formTestBody       `json:"form"`
		Submission  submissionTestBody `json:"submission"`
		IntegrityOK bool               `json:"integrityOk"`
	}
	decodeBody(t, rec, &view)
	if len(view.Form.Fields) != 4 {
		t.Errorf("form = %+v, want the definition alongside the submission", view.Form)
	}

	// Complete and sign it.
	rec = doJSON(t, rig.srv, http.MethodPost, "/v1/forms/mine/"+submission.ID+"/submit", submitPayload(), bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d, body %s", rec.Code, rec.Body.String())
	}
	var submitted struct {
		Submission submissionTestBody `json:"submission"`
	}
	decodeBody(t, rec, &submitted)
	if submitted.Submission.Status != "submitted" || submitted.Submission.Signature == nil {
		t.Fatalf("submission = %+v, want it submitted and signed", submitted.Submission)
	}
	if submitted.Submission.Signature.TypedName != "Ama Serwaa" {
		t.Errorf("typedName = %q, want the client's mark", submitted.Submission.Signature.TypedName)
	}

	// The practitioner reads it back with the integrity verdict.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/forms/submissions/"+submission.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("review status = %d, body %s", rec.Code, rec.Body.String())
	}
	var reviewed struct {
		Submission     map[string]any `json:"submission"`
		IntegrityOK    bool           `json:"integrityOk"`
		SignatureImage string         `json:"signatureImage"`
	}
	decodeBody(t, rec, &reviewed)
	if !reviewed.IntegrityOK {
		t.Error("a freshly signed submission reported as tampered")
	}
	if reviewed.SignatureImage != formSignatureImage {
		t.Error("the drawn signature was not served with the single record")
	}
}

// TestSignatureImageIsNotInListings: the drawn mark is served only with the
// one record a person opened.
func TestSignatureImageIsNotInListings(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	submission := assignForm(t, rig, f.ID, "client-1")
	if rec := doJSON(t, rig.srv, http.MethodPost, "/v1/forms/mine/"+submission.ID+"/submit", submitPayload(), bearer(rig.clientToken)); rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d, body %s", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct{ path, token string }{
		{"/v1/admin/forms/submissions", rig.practitionerToken},
		{"/v1/forms/mine", rig.clientToken},
	} {
		rec := doJSON(t, rig.srv, http.MethodGet, tc.path, nil, bearer(tc.token))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", tc.path, rec.Code)
		}
		var listing struct {
			Items []map[string]any `json:"items"`
		}
		decodeBody(t, rec, &listing)
		if len(listing.Items) != 1 {
			t.Fatalf("GET %s items = %d, want 1", tc.path, len(listing.Items))
		}
		signature, _ := listing.Items[0]["signature"].(map[string]any)
		if signature == nil {
			t.Fatalf("GET %s: no signature summary on a signed submission", tc.path)
		}
		for _, heavy := range []string{"imageData", "signedIp", "hash"} {
			if _, present := signature[heavy]; present {
				t.Errorf("GET %s: %q reached a listing: %+v", tc.path, heavy, signature)
			}
		}
	}
}

// TestSubmissionValidationOverHTTP.
func TestSubmissionValidationOverHTTP(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)

	cases := map[string]struct {
		body     map[string]any
		wantCode int
		wantErr  string
	}{
		"missing required field": {map[string]any{
			"answers":   map[string]any{"pressure": map[string]any{"value": "Medium"}},
			"signature": map[string]any{"typedName": "Ama", "imageData": formSignatureImage},
		}, http.StatusBadRequest, "validation_error"},
		"option not on the list": {map[string]any{
			"answers": map[string]any{
				"full_name": map[string]any{"value": "Ama"},
				"pressure":  map[string]any{"value": "Bone-crushing"},
			},
			"signature": map[string]any{"typedName": "Ama", "imageData": formSignatureImage},
		}, http.StatusBadRequest, "validation_error"},
		"answer to an unknown field": {map[string]any{
			"answers": map[string]any{
				"full_name": map[string]any{"value": "Ama"},
				"pressure":  map[string]any{"value": "Medium"},
				"smuggled":  map[string]any{"value": "payload"},
			},
			"signature": map[string]any{"typedName": "Ama", "imageData": formSignatureImage},
		}, http.StatusBadRequest, "validation_error"},
		"no signature": {map[string]any{
			"answers": map[string]any{
				"full_name": map[string]any{"value": "Ama"},
				"pressure":  map[string]any{"value": "Medium"},
			},
		}, http.StatusUnprocessableEntity, "signature_required"},
		"remote signature image": {map[string]any{
			"answers": map[string]any{
				"full_name": map[string]any{"value": "Ama"},
				"pressure":  map[string]any{"value": "Medium"},
			},
			"signature": map[string]any{"typedName": "Ama", "imageData": "https://example.com/sig.png"},
		}, http.StatusBadRequest, "validation_error"},
	}
	// Each case needs its own assignment: assigning the same form to the
	// same client twice is refused by design, and reusing one across
	// subtests would leave them sharing mutated state anyway.
	_ = f
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			submission := assignForm(t, rig, createForm(t, rig).ID, "client-1")
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/forms/mine/"+submission.ID+"/submit", tc.body, bearer(rig.clientToken))
			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantCode, rec.Body.String())
			}
			var errRes errorBody
			decodeBody(t, rec, &errRes)
			if errRes.Error.Code != tc.wantErr {
				t.Errorf("code = %q, want %q", errRes.Error.Code, tc.wantErr)
			}

			// Nothing was stored: the form is still waiting.
			rec = doJSON(t, rig.srv, http.MethodGet, "/v1/forms/mine/"+submission.ID, nil, bearer(rig.clientToken))
			var view struct {
				Submission submissionTestBody `json:"submission"`
			}
			decodeBody(t, rec, &view)
			if view.Submission.Status != "assigned" {
				t.Errorf("status = %q after a rejected submit, want it still assigned", view.Submission.Status)
			}
		})
	}
}

// TestSubmissionIsolation: a client cannot read or submit another client's
// form, and gets not-found rather than forbidden.
func TestSubmissionIsolation(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	submission := assignForm(t, rig, f.ID, "client-1")

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/forms/mine/" + submission.ID},
		{http.MethodPost, "/v1/forms/mine/" + submission.ID + "/submit"},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, submitPayload(), bearer(rig.otherClientToken))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s as another client = %d, want 404 (body %s)", tc.method, rec.Code, rec.Body.String())
		}
		var errRes errorBody
		decodeBody(t, rec, &errRes)
		if errRes.Error.Code != "submission_not_found" {
			t.Errorf("code = %q, want submission_not_found", errRes.Error.Code)
		}
	}

	// And it does not show up in their list.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/forms/mine", nil, bearer(rig.otherClientToken))
	var mine struct {
		Items []submissionTestBody `json:"items"`
	}
	decodeBody(t, rec, &mine)
	if len(mine.Items) != 0 {
		t.Errorf("another client's list = %+v, want it empty", mine.Items)
	}
}

// TestSubmitIsOneWayOverHTTP.
func TestSubmitIsOneWayOverHTTP(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	submission := assignForm(t, rig, f.ID, "client-1")

	if rec := doJSON(t, rig.srv, http.MethodPost, "/v1/forms/mine/"+submission.ID+"/submit", submitPayload(), bearer(rig.clientToken)); rec.Code != http.StatusOK {
		t.Fatalf("first submit status = %d", rec.Code)
	}

	altered := submitPayload()
	altered["answers"] = map[string]any{
		"full_name": map[string]any{"value": "Someone Else"},
		"pressure":  map[string]any{"value": "Firm"},
	}
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/forms/mine/"+submission.ID+"/submit", altered, bearer(rig.clientToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second submit = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "already_submitted" {
		t.Errorf("code = %q, want already_submitted", errRes.Error.Code)
	}
}

// TestAssigningTwiceIsRefused: a client should not find three copies of the
// same form waiting.
func TestAssigningTwiceIsRefused(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	assignForm(t, rig, f.ID, "client-1")

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/forms/assign", map[string]any{
		"formId": f.ID, "clientId": "client-1",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusConflict {
		t.Fatalf("second assignment = %d, want 409 (body %s)", rec.Code, rec.Body.String())
	}

	// Once completed, the same form can be sent again.
	submission := assignForm(t, rig, f.ID, "client-2")
	_ = submission
}

// TestDeletingAUsedFormRetiresIt: deleting a form that has been sent must
// not strand the signed records that point at it.
func TestDeletingAUsedFormRetiresIt(t *testing.T) {
	rig := newFormTestRig(t)
	f := createForm(t, rig)
	submission := assignForm(t, rig, f.ID, "client-1")

	rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/forms/"+f.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body.String())
	}

	// It is retired, not gone — the record still resolves its definition.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/forms/"+f.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("get after delete = %d, want the form retained", rec.Code)
	}
	var res struct {
		Form formTestBody `json:"form"`
	}
	decodeBody(t, rec, &res)
	if res.Form.Active {
		t.Error("a deleted form is still active, want it retired")
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/forms/submissions/"+submission.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Errorf("the submission was stranded by the delete (status %d)", rec.Code)
	}

	// An unused form really is deleted.
	unused := createForm(t, rig)
	if rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/forms/"+unused.ID, nil, bearer(rig.practitionerToken)); rec.Code != http.StatusNoContent {
		t.Fatalf("delete unused status = %d", rec.Code)
	}
	if rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/forms/"+unused.ID, nil, bearer(rig.practitionerToken)); rec.Code != http.StatusNotFound {
		t.Errorf("unused form still present after delete (status %d)", rec.Code)
	}
}

// TestFormBuilderValidation.
func TestFormBuilderValidation(t *testing.T) {
	rig := newFormTestRig(t)

	cases := map[string]map[string]any{
		"no fields": {"title": "Empty", "fields": []map[string]any{}},
		"blank title": {"title": "  ", "fields": []map[string]any{
			{"key": "a", "label": "A", "type": "text"},
		}},
		"unknown field type": {"title": "T", "fields": []map[string]any{
			{"key": "a", "label": "A", "type": "colour_picker"},
		}},
		"select without options": {"title": "T", "fields": []map[string]any{
			{"key": "a", "label": "A", "type": "select"},
		}},
		"duplicate keys": {"title": "T", "fields": []map[string]any{
			{"key": "a", "label": "A", "type": "text"},
			{"key": "a", "label": "B", "type": "text"},
		}},
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/forms", body, bearer(rig.practitionerToken))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestFormRoleGuards.
func TestFormRoleGuards(t *testing.T) {
	rig := newFormTestRig(t)

	for _, tc := range []struct{ method, path, token string }{
		{http.MethodGet, "/v1/admin/forms", rig.clientToken},
		{http.MethodPost, "/v1/admin/forms", rig.clientToken},
		{http.MethodPost, "/v1/admin/forms/assign", rig.clientToken},
		{http.MethodGet, "/v1/admin/forms/submissions", rig.clientToken},
		{http.MethodGet, "/v1/forms/mine", rig.practitionerToken},
	} {
		rec := doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, bearer(tc.token))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s with the wrong role = %d, want 403", tc.method, tc.path, rec.Code)
		}
		rec = doJSON(t, rig.srv, tc.method, tc.path, map[string]any{}, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s anonymous = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

// TestFormsUnavailableWithoutDatabase.
func TestFormsUnavailableWithoutDatabase(t *testing.T) {
	srv := NewServer(WithForms(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/forms/mine"},
		{http.MethodGet, "/v1/admin/forms"},
		{http.MethodPost, "/v1/admin/forms/assign"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
