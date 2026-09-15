package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/identity"
)

// AgreementDraft is the input for creating an agreement.
type AgreementDraft struct {
	Key                      string
	Title                    string
	Body                     string
	RequiresCountersignature bool
	CollectionID             string
}

// AgreementPatch is a partial edit. Body changes bump the version.
type AgreementPatch struct {
	Title                    *string
	Body                     *string
	RequiresCountersignature *bool
	CollectionID             *string
	Active                   *bool
}

// SignRequest is one client accepting one agreement.
type SignRequest struct {
	AgreementID string
	ClientID    string
	ClientName  string
	ClientEmail string
	SignedName  string
	/** The booking the client was making, when the signature came from the
	 * booking flow. Optional. */
	BookingID string
}

// AgreementStatus answers "may this client book this service yet?" in one
// round trip: the agreement the service requires, and whether it is signed.
// Agreement is nil when the service needs no agreement at all.
type AgreementStatus struct {
	Agreement *agreement.Agreement
	Signature *agreement.Signature
}

// Signed reports whether nothing stands in the way of booking.
func (s AgreementStatus) Signed() bool {
	return s.Agreement == nil || s.Signature != nil
}

// AgreementService is the application service for agreements.
type AgreementService interface {
	List(ctx context.Context, practitionerID string, includeInactive bool) ([]agreement.Agreement, error)
	Get(ctx context.Context, id string) (agreement.Agreement, error)
	Create(ctx context.Context, practitionerID string, draft AgreementDraft) (agreement.Agreement, error)
	Update(ctx context.Context, id string, patch AgreementPatch) (agreement.Agreement, error)

	// StatusForService is what the booking flow asks before showing the
	// agreement step, and what booking creation asks before accepting.
	StatusForService(ctx context.Context, clientID, serviceID string) (AgreementStatus, error)
	StatusesForService(ctx context.Context, clientID, serviceID string) ([]AgreementStatus, error)
	Sign(ctx context.Context, req SignRequest) (agreement.Signature, error)
	Countersign(ctx context.Context, id identity.Identity, signatureID string, signedName string) (agreement.Signature, error)
	SignaturesForClient(ctx context.Context, clientID string) ([]agreement.Signature, error)

	// SignedDocument renders one signature as a PDF, for whoever is
	// entitled to it: the signatory, or the practice.
	SignedDocument(ctx context.Context, id identity.Identity, signatureID string) (SignedDocument, error)
}

// SignedDocument is a rendered signed agreement.
type SignedDocument struct {
	Filename string
	Data     []byte
}

// AgreementRepository stores agreements and the signatures against them.
type AgreementRepository interface {
	List(ctx context.Context, practitionerID string, includeInactive bool) ([]agreement.Agreement, error)
	GetByID(ctx context.Context, id string) (agreement.Agreement, error)
	GetByKey(ctx context.Context, practitionerID, key string) (agreement.Agreement, error)
	Create(ctx context.Context, a agreement.Agreement) (agreement.Agreement, error)
	Update(ctx context.Context, a agreement.Agreement) (agreement.Agreement, error)

	// CreateSignature is idempotent by (clientID, agreementID): a repeat
	// returns the signature already on file rather than a second one.
	CreateSignature(ctx context.Context, sig agreement.Signature) (agreement.Signature, error)
	UpdateSignature(ctx context.Context, sig agreement.Signature) (agreement.Signature, error)
	SignatureFor(ctx context.Context, clientID, agreementID string) (agreement.Signature, error)
	SignatureByID(ctx context.Context, id string) (agreement.Signature, error)
	SignaturesForClient(ctx context.Context, clientID string) ([]agreement.Signature, error)
}

// AgreementGate is the booking slice's view of agreements: one question,
// asked on the write path. Kept narrow on purpose — booking has no business
// listing or editing contracts.
type AgreementGate interface {
	// RequireSigned returns agreement.ErrAgreementRequired when the service
	// is covered by an agreement this client has not signed.
	RequireSigned(ctx context.Context, clientID, serviceID string) error
}

// AgreementNotifier tells the practice a client has signed. Implemented by
// the notifications service; a no-op is a valid implementation.
type AgreementNotifier interface {
	AgreementSigned(ctx context.Context, notice AgreementSignedNotice)
}

// AgreementArchivist files a permanent copy of a signed agreement on the
// client's record. Implemented over the documents slice; a no-op is a valid
// implementation, and a failure must never fail the signature itself.
type AgreementArchivist interface {
	ArchiveSignedAgreement(ctx context.Context, a agreement.Agreement, sig agreement.Signature)
}

// AgreementSignedNotice is what the practice is told when a client signs.
type AgreementSignedNotice struct {
	ClientID       string
	ClientName     string
	ClientEmail    string
	AgreementTitle string
	SignedName     string
	SignedAt       string
}
