package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/app/auth"
	documentsapp "github.com/xcreativs/terios/api/internal/app/documents"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// documentTestRig bundles a server with both document surfaces over an
// in-memory repository and a media store that records what it was asked.
type documentTestRig struct {
	srv               *Server
	media             *portstest.FakeMediaStore
	practitionerToken string
	clientToken       string
	otherClientToken  string
}

func newDocumentTestRig(t *testing.T) documentTestRig {
	t.Helper()
	issuer := portstest.NewFakeTokenIssuer(15 * time.Minute)
	authSvc := auth.NewService(
		portstest.NewFakeUserRepository(),
		portstest.NewFakeRefreshTokenRepository(),
		portstest.FakeHasher{},
		issuer,
		30*24*time.Hour,
	)
	media := portstest.NewFakeMediaStore()
	svc := documentsapp.NewService(portstest.NewFakeDocumentRepository(), media, documentsapp.Options{})

	issue := func(userID string, role identity.Role) string {
		token, _, err := issuer.IssueAccessToken(identity.Identity{UserID: userID, Role: role})
		if err != nil {
			t.Fatalf("issue token: %v", err)
		}
		return token
	}
	return documentTestRig{
		srv:               NewServer(WithAuth(authSvc), WithDocuments(svc, authSvc)),
		media:             media,
		practitionerToken: issue("prac-1", identity.RolePractitioner),
		clientToken:       issue("client-1", identity.RoleClient),
		otherClientToken:  issue("client-2", identity.RoleClient),
	}
}

type documentTestBody struct {
	ID              string `json:"id"`
	Kind            string `json:"kind"`
	ClientID        string `json:"clientId"`
	Title           string `json:"title"`
	Filename        string `json:"filename"`
	VisibleToClient bool   `json:"visibleToClient"`
}

func recordDocument(t *testing.T, rig documentTestRig, clientID, filename string) documentTestBody {
	t.Helper()
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/documents", map[string]any{
		"kind":     "client_document",
		"clientId": clientID,
		"publicId": "terios/clients/" + clientID + "/documents/abc",
		"filename": filename,
		"bytes":    2048,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("record status = %d, body %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Document documentTestBody `json:"document"`
	}
	decodeBody(t, rec, &res)
	return res.Document
}

// TestSignUploadDerivesTheFolder: the folder comes from the kind and the
// client, never from the request, so a signature cannot be aimed at
// another client's folder.
func TestSignUploadDerivesTheFolder(t *testing.T) {
	rig := newDocumentTestRig(t)

	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/documents/sign-upload", map[string]any{
		"kind": "client_document", "clientId": "client-1", "filename": "wellness-plan.pdf",
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("sign status = %d, body %s", rec.Code, rec.Body.String())
	}

	if len(rig.media.Uploads) != 1 {
		t.Fatalf("upload signings = %d, want 1", len(rig.media.Uploads))
	}
	params := rig.media.Uploads[0]
	if params.Folder != "terios/clients/client-1/documents" {
		t.Errorf("folder = %q, want the policy folder for this client", params.Folder)
	}
	if !params.Private {
		t.Error("a client document was signed for public delivery")
	}
	if params.ResourceType != "raw" {
		t.Errorf("resourceType = %q, want raw for a pdf", params.ResourceType)
	}

	var res struct {
		URL       string            `json:"url"`
		Fields    map[string]string `json:"fields"`
		ExpiresAt time.Time         `json:"expiresAt"`
	}
	decodeBody(t, rec, &res)
	if res.URL == "" || res.Fields["signature"] == "" {
		t.Errorf("response = %+v, want an upload target and signature", res)
	}
	if res.ExpiresAt.IsZero() {
		t.Error("the signature has no stated expiry")
	}
}

// TestSignUploadRejectsUnsupportedTypes: refusing before the upload beats
// refusing after it.
func TestSignUploadRejectsUnsupportedTypes(t *testing.T) {
	rig := newDocumentTestRig(t)

	for _, filename := range []string{"movie.mp4", "payload.svg", "script.js", "noextension"} {
		rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/documents/sign-upload", map[string]any{
			"kind": "client_document", "clientId": "client-1", "filename": filename,
		}, bearer(rig.practitionerToken))
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s status = %d, want 415 (body %s)", filename, rec.Code, rec.Body.String())
		}
	}
	if len(rig.media.Uploads) != 0 {
		t.Errorf("signings = %d, want none for rejected types", len(rig.media.Uploads))
	}
}

// TestDocumentIsolation is the slice's central guarantee.
func TestDocumentIsolation(t *testing.T) {
	rig := newDocumentTestRig(t)
	mine := recordDocument(t, rig, "client-1", "wellness-plan.pdf")

	// The owner can list and download it.
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list mine status = %d", rec.Code)
	}
	var mineList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rec, &mineList)
	if len(mineList.Items) != 1 {
		t.Fatalf("client's documents = %d, want 1", len(mineList.Items))
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine/"+mine.ID+"/url", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("download status = %d, body %s", rec.Code, rec.Body.String())
	}
	var download struct {
		URL       string `json:"url"`
		ExpiresIn int    `json:"expiresIn"`
	}
	decodeBody(t, rec, &download)
	if download.URL == "" || download.ExpiresIn <= 0 {
		t.Errorf("download = %+v, want a signed url with an expiry", download)
	}

	// Another client cannot — and is told not-found, not forbidden.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine/"+mine.ID+"/url", nil, bearer(rig.otherClientToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("other client download = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
	var errRes errorBody
	decodeBody(t, rec, &errRes)
	if errRes.Error.Code != "document_not_found" {
		t.Errorf("code = %q, want document_not_found", errRes.Error.Code)
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine", nil, bearer(rig.otherClientToken))
	decodeBody(t, rec, &mineList)
	if len(mineList.Items) != 0 {
		t.Errorf("other client's list = %+v, want it empty", mineList.Items)
	}
}

// TestUnsharedDocumentsAreInvisible: a practitioner can hold a file against
// a client record without sharing it, and the client learns nothing.
func TestUnsharedDocumentsAreInvisible(t *testing.T) {
	rig := newDocumentTestRig(t)
	doc := recordDocument(t, rig, "client-1", "internal-notes.pdf")

	rec := doJSON(t, rig.srv, http.MethodPatch, "/v1/admin/documents/"+doc.ID, map[string]any{
		"visibleToClient": false,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine", nil, bearer(rig.clientToken))
	var mineList struct {
		Items []map[string]any `json:"items"`
	}
	decodeBody(t, rec, &mineList)
	if len(mineList.Items) != 0 {
		t.Errorf("client's documents = %+v, want the unshared file hidden", mineList.Items)
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/documents/mine/"+doc.ID+"/url", nil, bearer(rig.clientToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("download of an unshared file = %d, want 404", rec.Code)
	}

	// The practitioner still sees it on the client's file.
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/documents?clientId=client-1", nil, bearer(rig.practitionerToken))
	var adminList struct {
		Items []documentTestBody `json:"items"`
	}
	decodeBody(t, rec, &adminList)
	if len(adminList.Items) != 1 || adminList.Items[0].VisibleToClient {
		t.Errorf("practice list = %+v, want the unshared file present and marked unshared", adminList.Items)
	}
}

// TestPublicIDNeverLeaves: the storage handle is the API's alone.
func TestPublicIDNeverLeaves(t *testing.T) {
	rig := newDocumentTestRig(t)
	doc := recordDocument(t, rig, "client-1", "wellness-plan.pdf")

	for _, tc := range []struct{ path, token string }{
		{"/v1/admin/documents?clientId=client-1", rig.practitionerToken},
		{"/v1/documents/mine", rig.clientToken},
	} {
		rec := doJSON(t, rig.srv, http.MethodGet, tc.path, nil, bearer(tc.token))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", tc.path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "publicId") {
			t.Errorf("GET %s leaked the storage id: %s", tc.path, rec.Body.String())
		}
	}
	_ = doc
}

func TestCMSMediaLibraryReturnsReusablePublicImages(t *testing.T) {
	rig := newDocumentTestRig(t)
	rec := doJSON(t, rig.srv, http.MethodPost, "/v1/admin/documents", map[string]any{
		"kind": "cms_image", "publicId": "terios/cms/about-portrait",
		"filename": "about-portrait.webp", "bytes": 2048,
	}, bearer(rig.practitionerToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("record cms image = %d, body %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/documents/media", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("list media = %d, body %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Items []struct {
			ID, URL, Title, Filename string
			Bytes                    int64
		} `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 {
		t.Fatalf("media items = %d, want 1", len(list.Items))
	}
	item := list.Items[0]
	if item.URL != "https://delivery.test/terios/cms/about-portrait" || item.Filename != "about-portrait.webp" || item.Bytes != 2048 {
		t.Errorf("media item = %+v, want reusable delivery metadata", item)
	}
	if strings.Contains(rec.Body.String(), "publicId") {
		t.Errorf("media library leaked provider public id: %s", rec.Body.String())
	}
}

// TestDeleteRemovesTheStoredFile: a file must not outlive the record that
// governed who could see it.
func TestDeleteRemovesTheStoredFile(t *testing.T) {
	rig := newDocumentTestRig(t)
	doc := recordDocument(t, rig, "client-1", "wellness-plan.pdf")

	rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/documents/"+doc.ID, nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body %s", rec.Code, rec.Body.String())
	}
	if len(rig.media.Deleted) != 1 {
		t.Fatalf("media deletions = %d, want 1", len(rig.media.Deleted))
	}
	if !rig.media.Deleted[0].Private {
		t.Error("the stored file was deleted as a public asset, want authenticated")
	}

	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/documents/"+doc.ID+"/url", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusNotFound {
		t.Errorf("download after delete = %d, want 404", rec.Code)
	}
}

// TestDeleteKeepsTheRecordWhenStorageFails: a record with no file is a
// broken link; a file with no record is an ungoverned asset. The record
// stays so the practitioner can retry.
func TestDeleteKeepsTheRecordWhenStorageFails(t *testing.T) {
	rig := newDocumentTestRig(t)
	doc := recordDocument(t, rig, "client-1", "wellness-plan.pdf")
	rig.media.DeleteErr = &portstestGatewayError{}

	rec := doJSON(t, rig.srv, http.MethodDelete, "/v1/admin/documents/"+doc.ID, nil, bearer(rig.practitionerToken))
	if rec.Code == http.StatusNoContent {
		t.Fatal("delete reported success while the stored file survived")
	}

	rig.media.DeleteErr = nil
	rec = doJSON(t, rig.srv, http.MethodGet, "/v1/admin/documents?clientId=client-1", nil, bearer(rig.practitionerToken))
	var list struct {
		Items []documentTestBody `json:"items"`
	}
	decodeBody(t, rec, &list)
	if len(list.Items) != 1 {
		t.Errorf("documents = %d after a failed delete, want the record kept for a retry", len(list.Items))
	}
}

// portstestGatewayError stands in for a provider failure.
type portstestGatewayError struct{}

func (*portstestGatewayError) Error() string { return "media store unavailable" }

// TestDocumentRoleGuards.
func TestDocumentRoleGuards(t *testing.T) {
	rig := newDocumentTestRig(t)

	for _, tc := range []struct{ method, path, token string }{
		{http.MethodPost, "/v1/admin/documents/sign-upload", rig.clientToken},
		{http.MethodPost, "/v1/admin/documents", rig.clientToken},
		{http.MethodGet, "/v1/admin/documents", rig.clientToken},
		{http.MethodGet, "/v1/admin/documents/media", rig.clientToken},
		{http.MethodGet, "/v1/documents/mine", rig.practitionerToken},
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

// TestAdminListRequiresAClient: an unscoped listing of every client's files
// is not a query this API offers.
func TestAdminListRequiresAClient(t *testing.T) {
	rig := newDocumentTestRig(t)
	rec := doJSON(t, rig.srv, http.MethodGet, "/v1/admin/documents", nil, bearer(rig.practitionerToken))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// TestDocumentsUnavailableWithoutStorage.
func TestDocumentsUnavailableWithoutStorage(t *testing.T) {
	srv := NewServer(WithDocuments(nil, nil))
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/v1/documents/mine"},
		{http.MethodGet, "/v1/admin/documents"},
		{http.MethodPost, "/v1/admin/documents/sign-upload"},
	} {
		rec := doJSON(t, srv, tc.method, tc.path, nil, nil)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s = %d, want 503 (body %s)", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}
