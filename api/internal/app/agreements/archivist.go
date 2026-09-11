package agreements

import (
	"context"
	"log/slog"

	"github.com/xcreativs/terios/api/internal/adapters/pdf"
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
)

// DocumentArchivist files each signed agreement as a PDF on the client's
// record, under the `signed_form` kind the documents slice already reserves
// for exactly this — a completed consent archived as a practice record.
//
// Two properties worth stating, because they are what make it safe to run
// this on the client's critical path:
//
//   - It never fails the signature. The signature is already stored by the
//     time this runs, and the same document can always be re-rendered from
//     it on demand, so a storage outage costs the practice a convenience
//     copy and nothing else. Failures are logged, not returned.
//   - The copy is not the record. `signed_form` documents are private and
//     never shared back to the client, and the authoritative artefact
//     remains the signature row plus the wording it snapshotted.
type DocumentArchivist struct {
	documents ports.DocumentService
	log       *slog.Logger
}

var _ ports.AgreementArchivist = (*DocumentArchivist)(nil)

func NewDocumentArchivist(documents ports.DocumentService, log *slog.Logger) *DocumentArchivist {
	if log == nil {
		log = slog.Default()
	}
	return &DocumentArchivist{documents: documents, log: log}
}

func (a *DocumentArchivist) ArchiveSignedAgreement(ctx context.Context, ag agreement.Agreement, sig agreement.Signature) {
	if a == nil || a.documents == nil {
		return
	}
	stored, err := a.documents.StoreDocument(ctx, sig.ClientID, ports.StoreDocumentInput{
		Kind:        document.KindSignedForm,
		ClientID:    sig.ClientID,
		Filename:    filename(sig),
		Title:       sig.AgreementTitle,
		ContentType: "application/pdf",
		Data:        pdf.SignedAgreement(ag, sig),
	})
	if err != nil {
		// Loud, because a practice expecting a filed copy should be able to
		// find out it did not arrive — and quiet failure is the one thing a
		// records system must not do.
		a.log.Error("archive signed agreement",
			"clientId", sig.ClientID,
			"agreement", sig.AgreementKey,
			"error", err,
		)
		return
	}
	a.log.Info("archived signed agreement",
		"clientId", sig.ClientID,
		"agreement", sig.AgreementKey,
		"documentId", stored.ID,
	)
}
