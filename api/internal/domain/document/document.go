// Package document is the domain core for client files: the record of an
// asset held in the media store, who it belongs to, and who may see it. It
// imports nothing outside the standard library — no frameworks, no drivers,
// and no knowledge of Cloudinary.
//
// The bytes never pass through this package or the API: the browser uploads
// straight to the media store with a signature the API issued, and delivery
// is a short-lived signed URL. What lives here is the record that says a
// given asset belongs to a given client — which is the thing that has to be
// right for a private file to stay private.
package document

import (
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

// Field limits.
const (
	MaxFilenameLen = 255
	MaxPublicIDLen = 500
	MaxTitleLen    = 200
	MaxBytes       = 10 << 20 // 10 MB, matching the upload preset
)

// Kind is what an asset is for. It decides the folder it lives in and who
// may see it.
type Kind string

const (
	// KindClientDocument is a file shared with one client (a wellness
	// plan, a handout). Private: signed delivery only.
	KindClientDocument Kind = "client_document"
	// KindSignedForm is a completed consent form archived as a PDF.
	// Private, and never client-deletable — it is a practice record.
	KindSignedForm Kind = "signed_form"
	// KindCMSImage is public website imagery. It carries no client.
	KindCMSImage Kind = "cms_image"
)

// Valid reports whether k is a known kind.
func (k Kind) Valid() bool {
	switch k {
	case KindClientDocument, KindSignedForm, KindCMSImage:
		return true
	}
	return false
}

// Private reports whether the asset must be delivered through a signed,
// short-lived URL rather than a public one.
func (k Kind) Private() bool {
	return k == KindClientDocument || k == KindSignedForm
}

// ResourceType mirrors the media store's own split. Only images and raw
// files (PDFs) are accepted; video and audio are out of scope for v1 and
// are rejected rather than silently stored.
type ResourceType string

const (
	ResourceImage ResourceType = "image"
	ResourceRaw   ResourceType = "raw"
)

// Valid reports whether t is an accepted resource type.
func (t ResourceType) Valid() bool {
	return t == ResourceImage || t == ResourceRaw
}

// allowedExtensions is the accepted file types per the media policy.
var allowedExtensions = map[string]ResourceType{
	".pdf":  ResourceRaw,
	".jpg":  ResourceImage,
	".jpeg": ResourceImage,
	".png":  ResourceImage,
	".webp": ResourceImage,
}

// Document is the record of one stored asset.
type Document struct {
	ID   string
	Kind Kind
	// ClientID is the owner. Empty for CMS assets, required for anything
	// private — a private asset with no owner could not be scoped to
	// anyone, which is the same as being unprotected.
	ClientID string
	// UploadedBy is the account that put it there.
	UploadedBy string
	// PublicID is the media store's own identifier. It is never exposed to
	// a client: delivery goes through the API, which checks ownership and
	// signs a URL.
	PublicID     string
	Filename     string
	Title        string
	ResourceType ResourceType
	Format       string
	Bytes        int64
	// VisibleToClient controls whether the owning client can see it at
	// all. A practitioner can hold a file against a client record without
	// sharing it, exactly as with private session notes.
	VisibleToClient bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// New validates and builds a document record.
func New(kind Kind, clientID, uploadedBy, publicID, filename string, bytes int64, now time.Time) (Document, error) {
	if !kind.Valid() {
		return Document{}, ErrInvalidKind
	}
	if kind.Private() && clientID == "" {
		return Document{}, ErrClientRequired
	}
	if uploadedBy == "" {
		return Document{}, ErrUploaderRequired
	}

	publicID = strings.TrimSpace(publicID)
	if publicID == "" || utf8.RuneCountInString(publicID) > MaxPublicIDLen {
		return Document{}, ErrInvalidPublicID
	}

	filename = SanitizeFilename(filename)
	if filename == "" || utf8.RuneCountInString(filename) > MaxFilenameLen {
		return Document{}, ErrInvalidFilename
	}
	resourceType, format, err := ClassifyFilename(filename)
	if err != nil {
		return Document{}, err
	}
	if bytes < 0 || bytes > MaxBytes {
		return Document{}, ErrFileTooLarge
	}

	now = now.UTC()
	return Document{
		Kind:         kind,
		ClientID:     clientID,
		UploadedBy:   uploadedBy,
		PublicID:     publicID,
		Filename:     filename,
		Title:        filename,
		ResourceType: resourceType,
		Format:       format,
		Bytes:        bytes,
		// A file a practitioner uploads for a client is shared by default —
		// that is why they uploaded it. A signed form is a practice record
		// the client already has a copy of in their portal.
		VisibleToClient: kind == KindClientDocument,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// Patch is the set of editable fields. Neither the asset nor its owner can
// be changed: re-pointing a record at a different client or a different
// stored object would be a way to hand one client another's file.
type Patch struct {
	Title           *string
	VisibleToClient *bool
}

// Apply validates and applies a patch.
func (d *Document) Apply(patch Patch, now time.Time) error {
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" || utf8.RuneCountInString(title) > MaxTitleLen {
			return ErrInvalidTitle
		}
		d.Title = title
	}
	if patch.VisibleToClient != nil {
		d.VisibleToClient = *patch.VisibleToClient
	}
	d.UpdatedAt = now.UTC()
	return nil
}

// ReadableBy reports whether the given client may see this document. It is
// the whole access rule in one place: the file must be theirs, and it must
// have been shared with them.
func (d Document) ReadableBy(clientID string) bool {
	return d.ClientID != "" && d.ClientID == clientID && d.VisibleToClient
}

// Folder is where the asset lives in the media store, per the media policy.
func Folder(kind Kind, clientID string) string {
	switch kind {
	case KindClientDocument:
		return "terios/clients/" + clientID + "/documents"
	case KindSignedForm:
		return "terios/clients/" + clientID + "/signed-forms"
	default:
		return "terios/cms"
	}
}

// SanitizeFilename strips any directory component and control characters.
// An uploaded name is attacker-controlled text that ends up in a
// Content-Disposition header and on someone's disk: "../../etc/passwd" and
// a name containing a newline are both rejected here rather than trusted
// downstream.
func SanitizeFilename(raw string) string {
	raw = strings.TrimSpace(raw)
	// Both separators, because the name may come from any client OS.
	raw = raw[strings.LastIndexAny(raw, "/\\")+1:]

	var b strings.Builder
	for _, r := range raw {
		if r < 0x20 || r == 0x7f || r == '"' {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	if cleaned == "." || cleaned == ".." {
		return ""
	}
	return cleaned
}

// ClassifyFilename maps an extension to its resource type and format,
// rejecting anything outside the accepted set.
func ClassifyFilename(filename string) (ResourceType, string, error) {
	ext := strings.ToLower(path.Ext(filename))
	resourceType, ok := allowedExtensions[ext]
	if !ok {
		return "", "", ErrUnsupportedFileType
	}
	return resourceType, strings.TrimPrefix(ext, "."), nil
}
