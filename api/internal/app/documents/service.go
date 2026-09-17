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
	activity  ports.ActivityNotifier
	documents ports.DocumentRepository
	media     ports.MediaStore
	ttl       time.Duration
	now       func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.DocumentService = (*Service)(nil)

// Options configure a Service.
type Options struct {
	Activity ports.ActivityNotifier
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
		activity:  opts.Activity,
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
	stored, err := s.documents.Create(ctx, d)
	if err == nil && stored.VisibleToClient {
		s.notify(ctx, stored, "A document was shared with you", "shared")
	}
	return stored, err
}

// StoreDocument files a document the API produced itself: it uploads the
// bytes and records them in one step, because there is no browser in the
// middle to come back and tell us what landed.
//
// The document is created with the visibility its kind implies — a signed
// form is a practice record, not something shared back to the client — so
// callers cannot accidentally publish one by omission.
func (s *Service) StoreDocument(ctx context.Context, uploadedBy string, in ports.StoreDocumentInput) (document.Document, error) {
	if !in.Kind.Valid() {
		return document.Document{}, document.ErrInvalidKind
	}
	if in.Kind.Private() && in.ClientID == "" {
		return document.Document{}, document.ErrClientRequired
	}
	filename := document.SanitizeFilename(in.Filename)
	if filename == "" {
		return document.Document{}, document.ErrInvalidFilename
	}
	resourceType, _, err := document.ClassifyFilename(filename)
	if err != nil {
		return document.Document{}, err
	}
	if len(in.Data) == 0 {
		return document.Document{}, document.ErrInvalidFilename
	}
	if int64(len(in.Data)) > document.MaxBytes {
		return document.Document{}, document.ErrFileTooLarge
	}

	uploaded, err := s.media.Upload(ctx,
		ports.UploadParams{
			Folder:       document.Folder(in.Kind, in.ClientID),
			ResourceType: resourceType,
			Private:      in.Kind.Private(),
		},
		ports.UploadFile{
			Filename:    filename,
			ContentType: in.ContentType,
			Data:        in.Data,
		},
	)
	if err != nil {
		return document.Document{}, err
	}

	d, err := document.New(in.Kind, in.ClientID, uploadedBy, uploaded.PublicID, filename, uploaded.Bytes, s.now())
	if err != nil {
		return document.Document{}, err
	}
	if in.Title != "" {
		d.Title = in.Title
	}
	stored, err := s.documents.Create(ctx, d)
	if err == nil && stored.VisibleToClient {
		s.notify(ctx, stored, "A document was shared with you", "shared")
	}
	return stored, err
}

// ListForClient returns every document held against a client, shared or
// not — the practitioner's view of the file.
func (s *Service) ListForClient(ctx context.Context, clientID string) ([]document.Document, error) {
	return s.documents.ListByClient(ctx, clientID)
}

func (s *Service) ListCMSImages(ctx context.Context) ([]ports.CMSImage, error) {
	documents, err := s.documents.ListByKind(ctx, document.KindCMSImage)
	if err != nil {
		return nil, err
	}
	items := make([]ports.CMSImage, 0, len(documents))
	for _, d := range documents {
		url, err := s.media.PublicURL(assetOf(d))
		if err != nil {
			return nil, err
		}
		items = append(items, ports.CMSImage{
			ID: d.ID, URL: url, Title: d.Title, Filename: d.Filename,
			Bytes: d.Bytes, CreatedAt: d.CreatedAt,
		})
	}
	return items, nil
}

// UpdateDocument edits the title or the sharing flag.
func (s *Service) UpdateDocument(ctx context.Context, id string, patch document.Patch) (document.Document, error) {
	d, err := s.documents.FindByID(ctx, id)
	if err != nil {
		return document.Document{}, err
	}
	wasVisible := d.VisibleToClient
	if err := d.Apply(patch, s.now()); err != nil {
		return document.Document{}, err
	}
	stored, err := s.documents.Update(ctx, d)
	if err == nil && (wasVisible || stored.VisibleToClient) {
		title := "A shared document was updated"
		if !wasVisible {
			title = "A document was shared with you"
		}
		if !stored.VisibleToClient {
			title = "A document is no longer shared"
		}
		s.notify(ctx, stored, title, "updated:"+stored.UpdatedAt.Format(time.RFC3339Nano))
	}
	return stored, err
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
	if err := s.documents.Delete(ctx, id); err != nil {
		return err
	}
	if d.VisibleToClient {
		s.notify(ctx, d, "A shared document was removed", "removed")
	}
	return nil
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

func (s *Service) notify(ctx context.Context, d document.Document, title, event string) {
	ports.NotifyActivity(ctx, s.activity, ports.ActivityNotice{EventID: "document:" + d.ID + ":" + event, ClientID: d.ClientID, Title: title, ClientLink: "/portal/documents", PracticeLink: "/clients/" + d.ClientID})
}
