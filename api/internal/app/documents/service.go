// Package documents is the application service for client files. It
// implements the inbound ports.DocumentService port purely against outbound
// ports — no framework, driver, or provider imports.
//
// The slice's whole job is the access decision. The bytes live in the media
// store and the browser talks to it directly; what the API controls is who
// gets a signed URL, and for how long. Every client-facing read leads with
// the caller's own id, and a document that is not theirs is reported as
// missing rather than forbidden.
package documents

import (
	"context"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the document use cases over outbound ports.
type Service struct {
	documents ports.DocumentRepository
	media     ports.MediaStore
	ttl       time.Duration
	now       func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.DocumentService = (*Service)(nil)

// Options configure a Service.
type Options struct {
	// DeliveryTTL is how long a signed download link lives.
	DeliveryTTL time.Duration
}

// NewService wires the use cases to their outbound ports.
func NewService(documents ports.DocumentRepository, media ports.MediaStore, opts Options) *Service {
	ttl := opts.DeliveryTTL
	if ttl <= 0 {
		ttl = ports.DefaultDeliveryTTL
	}
	return &Service{
		documents: documents,
		media:     media,
		ttl:       ttl,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SignUpload authorizes one direct upload.
//
// The folder is derived from the kind and the client — never taken from
// the request — so a caller cannot aim an upload at another client's
// folder. The file type is checked before signing, so an unsupported file
// is refused before it is uploaded rather than after.
func (s *Service) SignUpload(ctx context.Context, in ports.UploadRequest) (ports.SignedUpload, error) {
	if !in.Kind.Valid() {
		return ports.SignedUpload{}, document.ErrInvalidKind
	}
	if in.Kind.Private() && in.ClientID == "" {
		return ports.SignedUpload{}, document.ErrClientRequired
	}
	filename := document.SanitizeFilename(in.Filename)
	if filename == "" {
		return ports.SignedUpload{}, document.ErrInvalidFilename
	}
	resourceType, _, err := document.ClassifyFilename(filename)
	if err != nil {
		return ports.SignedUpload{}, err
	}

	return s.media.SignUpload(ctx, ports.UploadParams{
		Folder:       document.Folder(in.Kind, in.ClientID),
		ResourceType: resourceType,
		Private:      in.Kind.Private(),
	})
}

// RecordUpload registers a finished upload as a document record.
func (s *Service) RecordUpload(ctx context.Context, uploadedBy string, in ports.RecordUploadInput) (document.Document, error) {
	d, err := document.New(in.Kind, in.ClientID, uploadedBy, in.PublicID, in.Filename, in.Bytes, s.now())
	if err != nil {
		return document.Document{}, err
	}
	return s.documents.Create(ctx, d)
}

// ListForClient returns every document held against a client, shared or
// not — the practitioner's view of the file.
func (s *Service) ListForClient(ctx context.Context, clientID string) ([]document.Document, error) {
	return s.documents.ListByClient(ctx, clientID)
}

// UpdateDocument edits the title or the sharing flag.
func (s *Service) UpdateDocument(ctx context.Context, id string, patch document.Patch) (document.Document, error) {
	d, err := s.documents.FindByID(ctx, id)
	if err != nil {
		return document.Document{}, err
	}
	if err := d.Apply(patch, s.now()); err != nil {
		return document.Document{}, err
	}
	return s.documents.Update(ctx, d)
}

// DeleteDocument removes the record and the stored file.
//
// The stored object goes first. If that fails the record stays, and the
// practitioner sees an error and can retry — which is the safe way round:
// a record with no file is a broken link, but a file with no record is an
// asset nobody is governing.
func (s *Service) DeleteDocument(ctx context.Context, id string) error {
	d, err := s.documents.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.media.Delete(ctx, assetOf(d)); err != nil {
		return err
	}
	return s.documents.Delete(ctx, id)
}

// ListMine returns the caller's own documents — shared ones only. A file
// the practitioner is holding but has not shared is not the client's to
// see, and its existence is not theirs to know either.
func (s *Service) ListMine(ctx context.Context, clientID string) ([]document.Document, error) {
	all, err := s.documents.ListByClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	shared := make([]document.Document, 0, len(all))
	for _, d := range all {
		if d.ReadableBy(clientID) {
			shared = append(shared, d)
		}
	}
	return shared, nil
}

// DownloadURLForClient issues a signed link for the caller's own document.
func (s *Service) DownloadURLForClient(ctx context.Context, clientID, documentID string) (string, error) {
	d, err := s.documents.FindByID(ctx, documentID)
	if err != nil {
		return "", err
	}
	// Not-found rather than forbidden: whether a document exists is itself
	// something only its owner and the practice may learn.
	if !d.ReadableBy(clientID) {
		return "", document.ErrDocumentNotFound
	}
	return s.media.SignedURL(ctx, assetOf(d), s.ttl)
}

// DownloadURLForPractitioner issues a signed link for any document.
func (s *Service) DownloadURLForPractitioner(ctx context.Context, documentID string) (string, error) {
	d, err := s.documents.FindByID(ctx, documentID)
	if err != nil {
		return "", err
	}
	return s.media.SignedURL(ctx, assetOf(d), s.ttl)
}

// assetOf projects a record onto the storage identity.
func assetOf(d document.Document) ports.Asset {
	return ports.Asset{
		PublicID:     d.PublicID,
		ResourceType: d.ResourceType,
		Private:      d.Kind.Private(),
	}
}
