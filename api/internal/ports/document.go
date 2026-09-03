package ports

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/document"
)

// Signature and delivery lifetimes.
const (
	// UploadSignatureTTL is how long an upload signature stays usable.
	UploadSignatureTTL = 10 * time.Minute
	// DefaultDeliveryTTL is how long a signed download link lives. Long
	// enough to open a PDF, short enough that a link forwarded by mistake
	// stops working.
	DefaultDeliveryTTL = time.Hour
)

// UploadParams describes one upload the API is willing to authorize.
type UploadParams struct {
	Folder       string
	ResourceType document.ResourceType
	// Private selects authenticated delivery — no public URL exists.
	Private bool
	// PublicID pins the stored object's id; empty lets the store pick.
	PublicID string
}

// SignedUpload is what the browser needs to upload directly.
type SignedUpload struct {
	URL       string
	Fields    map[string]string
	Signature string
	Timestamp int64
	ExpiresAt time.Time
}

// Asset identifies one stored object for delivery or deletion.
type Asset struct {
	PublicID     string
	ResourceType document.ResourceType
	Private      bool
}

// MediaStore is the outbound port for the media store (Cloudinary).
type MediaStore interface {
	// SignUpload authorizes one direct browser upload.
	SignUpload(ctx context.Context, params UploadParams) (SignedUpload, error)
	// SignedURL builds a short-lived delivery URL for a private asset.
	SignedURL(ctx context.Context, asset Asset, ttl time.Duration) (string, error)
	// PublicURL returns the durable delivery URL for a public CMS asset.
	PublicURL(asset Asset) (string, error)
	// Delete removes an asset, so a file never outlives the record that
	// governed who could see it.
	Delete(ctx context.Context, asset Asset) error
}

// DocumentRepository is the outbound port for document records.
type DocumentRepository interface {
	Create(ctx context.Context, d document.Document) (document.Document, error)
	Update(ctx context.Context, d document.Document) (document.Document, error)
	Delete(ctx context.Context, id string) error
	// FindByID misses return document.ErrDocumentNotFound.
	FindByID(ctx context.Context, id string) (document.Document, error)
	// ListByClient returns one client's documents newest-first, leading
	// with clientId — the isolation rule.
	ListByClient(ctx context.Context, clientID string) ([]document.Document, error)
	// ListByKind returns one media class newest-first.
	ListByKind(ctx context.Context, kind document.Kind) ([]document.Document, error)
}

// CMSImage is one reusable public upload in the practitioner media library.
type CMSImage struct {
	ID        string
	URL       string
	Title     string
	Filename  string
	Bytes     int64
	CreatedAt time.Time
}

// UploadRequest asks the API to authorize an upload.
type UploadRequest struct {
	Kind     document.Kind
	ClientID string
	Filename string
}

// RecordUploadInput registers an asset the browser has finished uploading.
type RecordUploadInput struct {
	Kind     document.Kind
	ClientID string
	PublicID string
	Filename string
	Bytes    int64
}

// DocumentService is the inbound port for the documents slice (BE-11).
type DocumentService interface {
	// SignUpload authorizes a practitioner upload — practitioner only.
	SignUpload(ctx context.Context, in UploadRequest) (SignedUpload, error)
	// RecordUpload registers a finished upload — practitioner only.
	RecordUpload(ctx context.Context, uploadedBy string, in RecordUploadInput) (document.Document, error)
	// ListForClient returns a client's documents — practitioner only.
	ListForClient(ctx context.Context, clientID string) ([]document.Document, error)
	// ListCMSImages returns reusable public uploads newest-first.
	ListCMSImages(ctx context.Context) ([]CMSImage, error)
	// UpdateDocument edits the title or sharing — practitioner only.
	UpdateDocument(ctx context.Context, id string, patch document.Patch) (document.Document, error)
	// DeleteDocument removes the record and the stored file — practitioner
	// only.
	DeleteDocument(ctx context.Context, id string) error

	// ListMine returns the caller's own shared documents.
	ListMine(ctx context.Context, clientID string) ([]document.Document, error)
	// DownloadURLForClient issues a signed link for the caller's own
	// document. Another client's document is not-found, never forbidden.
	DownloadURLForClient(ctx context.Context, clientID, documentID string) (string, error)
	// DownloadURLForPractitioner issues a signed link for any document.
	DownloadURLForPractitioner(ctx context.Context, documentID string) (string, error)
}
