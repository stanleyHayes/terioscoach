package document

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var fixedNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func newClientDoc(t *testing.T) Document {
	t.Helper()
	d, err := New(KindClientDocument, "client-1", "prac-1", "terios/clients/client-1/documents/abc", "wellness-plan.pdf", 2048, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

// TestNewClassifiesAndDefaults.
func TestNewClassifiesAndDefaults(t *testing.T) {
	d := newClientDoc(t)
	if d.ResourceType != ResourceRaw || d.Format != "pdf" {
		t.Errorf("document = %+v, want a raw pdf", d)
	}
	if !d.VisibleToClient {
		t.Error("a document uploaded for a client is hidden from them by default")
	}
	if d.Title != "wellness-plan.pdf" {
		t.Errorf("title = %q, want it to default to the filename", d.Title)
	}
}

// TestPrivateDocumentsNeedAnOwner: a private asset with no owner could not
// be scoped to anyone, which is the same as being unprotected.
func TestPrivateDocumentsNeedAnOwner(t *testing.T) {
	for _, kind := range []Kind{KindClientDocument, KindSignedForm} {
		if _, err := New(kind, "", "prac-1", "public-id", "plan.pdf", 10, fixedNow); !errors.Is(err, ErrClientRequired) {
			t.Errorf("%s with no client err = %v, want ErrClientRequired", kind, err)
		}
	}
	// CMS imagery has no client and that is correct.
	if _, err := New(KindCMSImage, "", "prac-1", "public-id", "cover.jpg", 10, fixedNow); err != nil {
		t.Errorf("cms image err = %v, want it accepted without a client", err)
	}
}

// TestSignedFormsAreNotSharedByDefault: the client already has their copy
// in the portal; the archived PDF is a practice record.
func TestSignedFormsAreNotSharedByDefault(t *testing.T) {
	d, err := New(KindSignedForm, "client-1", "prac-1", "public-id", "consent.pdf", 1024, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d.VisibleToClient {
		t.Error("a signed-form archive is shared with the client by default")
	}
}

// TestFileTypeAllowList: only the types the media policy accepts.
func TestFileTypeAllowList(t *testing.T) {
	accepted := map[string]ResourceType{
		"plan.pdf":   ResourceRaw,
		"scan.JPG":   ResourceImage,
		"photo.jpeg": ResourceImage,
		"cover.png":  ResourceImage,
		"hero.webp":  ResourceImage,
	}
	for filename, want := range accepted {
		resourceType, _, err := ClassifyFilename(filename)
		if err != nil {
			t.Errorf("ClassifyFilename(%q) err = %v, want it accepted", filename, err)
			continue
		}
		if resourceType != want {
			t.Errorf("ClassifyFilename(%q) = %q, want %q", filename, resourceType, want)
		}
	}

	for _, filename := range []string{
		"video.mp4", "audio.mp3", "script.js", "archive.zip",
		"payload.svg", "page.html", "noextension", "app.exe",
	} {
		if _, _, err := ClassifyFilename(filename); !errors.Is(err, ErrUnsupportedFileType) {
			t.Errorf("ClassifyFilename(%q) err = %v, want ErrUnsupportedFileType", filename, err)
		}
	}
}

// TestFilenameSanitization: an uploaded name is attacker-controlled text
// that ends up in a header and on a disk.
func TestFilenameSanitization(t *testing.T) {
	for raw, want := range map[string]string{
		"plan.pdf":                     "plan.pdf",
		"  plan.pdf  ":                 "plan.pdf",
		"../../../etc/passwd":          "passwd",
		`..\..\windows\system32\a.pdf`: "a.pdf",
		"/absolute/path/plan.pdf":      "plan.pdf",
		"quote\"name.pdf":              "quotename.pdf",
		"newline\nname.pdf":            "newlinename.pdf",
		"..":                           "",
	} {
		if got := SanitizeFilename(raw); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", raw, got, want)
		}
	}

	// A traversal attempt must not survive into a stored record.
	if _, err := New(KindClientDocument, "client-1", "prac-1", "public-id", "../../../etc/passwd", 10, fixedNow); !errors.Is(err, ErrUnsupportedFileType) {
		t.Errorf("traversal filename err = %v, want it rejected on type", err)
	}
	d, err := New(KindClientDocument, "client-1", "prac-1", "public-id", "../../plan.pdf", 10, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if strings.Contains(d.Filename, "/") || strings.Contains(d.Filename, "..") {
		t.Errorf("filename = %q, want the path component stripped", d.Filename)
	}
}

// TestSizeLimit.
func TestSizeLimit(t *testing.T) {
	if _, err := New(KindClientDocument, "client-1", "prac-1", "public-id", "plan.pdf", MaxBytes+1, fixedNow); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("err = %v, want ErrFileTooLarge", err)
	}
	if _, err := New(KindClientDocument, "client-1", "prac-1", "public-id", "plan.pdf", MaxBytes, fixedNow); err != nil {
		t.Errorf("err = %v, want a file at the limit accepted", err)
	}
}

// TestReadableBy is the whole access rule: the file must be theirs, and it
// must have been shared with them.
func TestReadableBy(t *testing.T) {
	d := newClientDoc(t)

	if !d.ReadableBy("client-1") {
		t.Error("the owning client cannot read their own shared document")
	}
	if d.ReadableBy("client-2") {
		t.Error("another client can read this document")
	}
	if d.ReadableBy("") {
		t.Error("an empty client id can read this document")
	}

	hidden := false
	if err := d.Apply(Patch{VisibleToClient: &hidden}, fixedNow); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if d.ReadableBy("client-1") {
		t.Error("an unshared document is still readable by its client")
	}

	// A CMS asset has no owner, so no client reads it through this rule.
	cms, err := New(KindCMSImage, "", "prac-1", "public-id", "cover.jpg", 10, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if cms.ReadableBy("") || cms.ReadableBy("client-1") {
		t.Error("an ownerless asset matched a client's read rule")
	}
}

// TestPatchCannotRepointTheAsset: Patch has no ClientID or PublicID field,
// so a record can never be aimed at a different client or object.
func TestPatchCannotRepointTheAsset(t *testing.T) {
	d := newClientDoc(t)
	title := "Your wellness plan"
	if err := d.Apply(Patch{Title: &title}, fixedNow.Add(time.Hour)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if d.Title != "Your wellness plan" {
		t.Errorf("title = %q, want the patch applied", d.Title)
	}
	if d.ClientID != "client-1" || d.PublicID != "terios/clients/client-1/documents/abc" {
		t.Errorf("document = %+v, want owner and asset untouched", d)
	}
	if !d.UpdatedAt.Equal(fixedNow.Add(time.Hour)) {
		t.Errorf("updatedAt = %v, want it stamped", d.UpdatedAt)
	}

	blank := "  "
	if err := d.Apply(Patch{Title: &blank}, fixedNow); !errors.Is(err, ErrInvalidTitle) {
		t.Errorf("blank title err = %v, want ErrInvalidTitle", err)
	}
}

// TestFolderPolicy matches design/cloudinary-policy.md.
func TestFolderPolicy(t *testing.T) {
	for _, tc := range []struct {
		kind     Kind
		clientID string
		want     string
	}{
		{KindClientDocument, "client-1", "terios/clients/client-1/documents"},
		{KindSignedForm, "client-1", "terios/clients/client-1/signed-forms"},
		{KindCMSImage, "", "terios/cms"},
	} {
		if got := Folder(tc.kind, tc.clientID); got != tc.want {
			t.Errorf("Folder(%q, %q) = %q, want %q", tc.kind, tc.clientID, got, tc.want)
		}
	}
}

// TestKindPrivacy.
func TestKindPrivacy(t *testing.T) {
	if !KindClientDocument.Private() || !KindSignedForm.Private() {
		t.Error("a client asset is not marked private")
	}
	if KindCMSImage.Private() {
		t.Error("public website imagery is marked private")
	}
	if !KindClientDocument.Valid() || Kind("secret").Valid() {
		t.Error("Kind.Valid does not match the known set")
	}
	if !ResourceImage.Valid() || ResourceType("video").Valid() {
		t.Error("ResourceType.Valid accepted a type outside the policy")
	}
}
