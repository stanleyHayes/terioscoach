// Package agreements is the application service for service agreements:
// the contracts a client accepts before their first session under a line of
// work, and the signatures recorded against them.
//
// The slice exists to answer one question cheaply and in one place — "may
// this client book this service yet?" — and to make the answer the same
// whether it is asked by the portal deciding which step to show or by
// booking creation deciding whether to accept. A gate enforced in only one
// of those two places is not a gate.
package agreements

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/pdf"
	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Options carries the collaborators a deployment supplies.
type Options struct {
	// Users resolves the signatory's name and address. The token identity
	// carries neither, and both belong on the signature: what the practice
	// has on file, next to what the client actually typed.
	Users ports.UserRepository
	// Notifier is told when a client signs. Optional.
	Notifier ports.AgreementNotifier
	// Archivist files a copy of the signed agreement on the client's
	// record. Optional: a failure to archive never fails a signature.
	Archivist ports.AgreementArchivist
	// Now supplies the clock; nil means the real one.
	Now func() time.Time
}

// Service implements ports.AgreementService.
type Service struct {
	repo      ports.AgreementRepository
	services  ports.ServiceRepository
	users     ports.UserRepository
	notifier  ports.AgreementNotifier
	archivist ports.AgreementArchivist
	now       func() time.Time
}

var _ ports.AgreementService = (*Service)(nil)

func NewService(repo ports.AgreementRepository, services ports.ServiceRepository, opts Options) *Service {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		repo:      repo,
		services:  services,
		users:     opts.Users,
		notifier:  opts.Notifier,
		archivist: opts.Archivist,
		now:       now,
	}
}

func (s *Service) List(ctx context.Context, practitionerID string, includeInactive bool) ([]agreement.Agreement, error) {
	return s.repo.List(ctx, practitionerID, includeInactive)
}

func (s *Service) Get(ctx context.Context, id string) (agreement.Agreement, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, practitionerID string, draft ports.AgreementDraft) (agreement.Agreement, error) {
	a, err := agreement.New(practitionerID, draft.Key, draft.Title, draft.Body, draft.RequiresCountersignature, s.now())
	if err != nil {
		return agreement.Agreement{}, err
	}
	a.CollectionID = draft.CollectionID
	return s.repo.Create(ctx, a)
}

func (s *Service) Update(ctx context.Context, id string, patch ports.AgreementPatch) (agreement.Agreement, error) {
	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return agreement.Agreement{}, err
	}
	updated, err := current.Apply(agreement.Patch{
		Title:                    patch.Title,
		Body:                     patch.Body,
		RequiresCountersignature: patch.RequiresCountersignature,
		CollectionID:             patch.CollectionID,
		Active:                   patch.Active,
	}, s.now())
	if err != nil {
		return agreement.Agreement{}, err
	}
	return s.repo.Update(ctx, updated)
}

// StatusesForService returns the agreement statuses for all agreements required by the service.
func (s *Service) StatusesForService(ctx context.Context, clientID, serviceID string) ([]ports.AgreementStatus, error) {
	svc, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	agreementIDs := svc.RequiredAgreementIDs()
	if len(agreementIDs) == 0 {
		return nil, nil
	}

	var results []ports.AgreementStatus
	for _, aid := range agreementIDs {
		a, err := s.repo.GetByID(ctx, aid)
		if err != nil {
			if errors.Is(err, agreement.ErrAgreementNotFound) {
				continue
			}
			return nil, err
		}
		if !a.Active {
			continue
		}
		if !agreement.RequiresClientSignature(a.Key) {
			continue
		}

		status := ports.AgreementStatus{Agreement: &a}
		if clientID != "" {
			sig, err := s.repo.SignatureFor(ctx, clientID, a.ID)
			if err != nil {
				if !errors.Is(err, agreement.ErrAgreementNotFound) {
					return nil, err
				}
			} else {
				status.Signature = &sig
			}
		}
		results = append(results, status)
	}
	return results, nil
}

// StatusForService reports the next unsigned agreement a service requires,
// or the last signed agreement if all are signed, or empty if none required.
func (s *Service) StatusForService(ctx context.Context, clientID, serviceID string) (ports.AgreementStatus, error) {
	statuses, err := s.StatusesForService(ctx, clientID, serviceID)
	if err != nil {
		return ports.AgreementStatus{}, err
	}
	if len(statuses) == 0 {
		return ports.AgreementStatus{}, nil
	}
	for _, st := range statuses {
		if !st.Signed() {
			return st, nil
		}
	}
	return statuses[0], nil
}

// RequireSigned is the booking gate. It returns ErrAgreementRequired when
// the service is covered by an agreement this client has not signed.
func (s *Service) RequireSigned(ctx context.Context, clientID, serviceID string) error {
	statuses, err := s.StatusesForService(ctx, clientID, serviceID)
	if err != nil {
		return err
	}
	for _, st := range statuses {
		if !st.Signed() {
			return fmt.Errorf("%w: %s", agreement.ErrAgreementRequired, st.Agreement.Title)
		}
	}
	return nil
}

// Countersign records the practitioner countersigning a client's signature.
func (s *Service) Countersign(ctx context.Context, id identity.Identity, signatureID string, signedName string) (agreement.Signature, error) {
	if id.Role != identity.RolePractitioner {
		return agreement.Signature{}, agreement.ErrAgreementNotFound
	}
	sig, err := s.repo.SignatureByID(ctx, signatureID)
	if err != nil {
		return agreement.Signature{}, err
	}
	countersigned, err := sig.Countersign(signedName, s.now())
	if err != nil {
		return agreement.Signature{}, err
	}
	stored, err := s.repo.UpdateSignature(ctx, countersigned)
	if err == nil {
		ports.NotifyActivity(ctx, s.notifier, ports.ActivityNotice{EventID: "agreement:" + stored.ID + ":countersigned", ClientID: stored.ClientID, PractitionerID: id.UserID, Title: "Your coaching agreement was countersigned", ClientLink: "/portal/documents", PracticeLink: "/clients/" + stored.ClientID})
	}
	return stored, err
}

// Sign records the client's acceptance. It is idempotent: signing an
// agreement already on file returns the original signature, timestamp
// included, and notifies no one a second time.
func (s *Service) Sign(ctx context.Context, req ports.SignRequest) (agreement.Signature, error) {
	a, err := s.repo.GetByID(ctx, req.AgreementID)
	if err != nil {
		return agreement.Signature{}, err
	}

	// Checked before building the signature so a repeat never depends on
	// the typed name being valid a second time — the client has already
	// signed, and what they typed then is what stands.
	if existing, err := s.repo.SignatureFor(ctx, req.ClientID, a.ID); err == nil {
		return existing, nil
	} else if !errors.Is(err, agreement.ErrAgreementNotFound) {
		return agreement.Signature{}, err
	}

	name, email := req.ClientName, req.ClientEmail
	if s.users != nil {
		// The account is the source of truth for who this is; a caller
		// cannot claim to be someone else by putting a name in the body.
		if user, err := s.users.FindByID(ctx, req.ClientID); err == nil {
			name, email = user.Name, user.Email
		}
	}

	var sig agreement.Signature
	if agreement.IsStatementOfWork(a.Key) {
		if req.StatementOfWork == nil || strings.TrimSpace(req.SignedName) != "" {
			return agreement.Signature{}, agreement.ErrInvalidStatementOfWork
		}
		sig, err = a.SubmitStatementOfWork(req.ClientID, name, email, req.BookingID, *req.StatementOfWork, s.now())
	} else {
		if req.StatementOfWork != nil {
			return agreement.Signature{}, agreement.ErrInvalidStatementOfWork
		}
		sig, err = a.Sign(req.ClientID, name, email, req.SignedName, req.BookingID, s.now())
	}
	if err != nil {
		return agreement.Signature{}, err
	}
	stored, err := s.repo.CreateSignature(ctx, sig)
	if err != nil {
		return agreement.Signature{}, err
	}

	// Neither the practice's copy nor the archive is on the client's
	// critical path: the signature is recorded, and a mail or storage
	// failure must not tell them their booking did not go through.
	noticeTime := stored.SignedAt
	if stored.SubmittedAt != nil {
		noticeTime = *stored.SubmittedAt
	}
	if s.notifier != nil {
		s.notifier.AgreementSigned(ctx, ports.AgreementSignedNotice{
			Submitted:      stored.StatementOfWork != nil,
			ClientID:       stored.ClientID,
			ClientName:     stored.ClientName,
			ClientEmail:    stored.ClientEmail,
			AgreementTitle: stored.AgreementTitle,
			SignedName:     stored.SignedName,
			SignedAt:       noticeTime.Format(time.RFC1123),
		})
	}
	if s.archivist != nil {
		s.archivist.ArchiveSignedAgreement(ctx, a, stored)
	}
	return stored, nil
}

func (s *Service) SignaturesForClient(ctx context.Context, clientID string) ([]agreement.Signature, error) {
	return s.repo.SignaturesForClient(ctx, clientID)
}

// SignedDocument renders a signature as a PDF.
//
// The document is reproduced on demand rather than stored, because every
// input to it is already frozen: the agreement text is pinned to the
// version signed, the typed name is kept verbatim, and the timestamp does
// not move. One record of truth, rendered identically every time, beats a
// duplicate blob that can drift away from it.
//
// A signature belonging to someone else is reported as missing rather than
// forbidden — the same isolation rule the rest of the client-scoped API
// follows.
func (s *Service) SignedDocument(ctx context.Context, id identity.Identity, signatureID string) (ports.SignedDocument, error) {
	sig, err := s.repo.SignatureByID(ctx, signatureID)
	if err != nil {
		return ports.SignedDocument{}, err
	}
	if id.Role != identity.RolePractitioner && sig.ClientID != id.UserID {
		return ports.SignedDocument{}, agreement.ErrAgreementNotFound
	}
	// The wording rendered comes off the signature, not the live
	// agreement: this document has to show what was actually accepted,
	// however the text has been edited since.
	signed := agreement.Agreement{
		ID:      sig.AgreementID,
		Key:     sig.AgreementKey,
		Title:   sig.AgreementTitle,
		Body:    sig.AgreementBody,
		Version: sig.AgreementVersion,
	}
	if signed.Body == "" {
		// Signatures taken before the snapshot existed fall back to the
		// live text; it is the only copy there is.
		if current, err := s.repo.GetByID(ctx, sig.AgreementID); err == nil {
			signed.Body = current.Body
		}
	}

	return ports.SignedDocument{
		Filename: filename(sig),
		Data:     pdf.SignedAgreement(signed, sig),
	}, nil
}

// filename names the download after what it is and who signed it.
func filename(sig agreement.Signature) string {
	slug := func(in string) string {
		var b strings.Builder
		for _, r := range strings.ToLower(in) {
			switch {
			case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
				b.WriteRune(r)
			case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
				b.WriteByte('-')
			}
		}
		return strings.Trim(b.String(), "-")
	}
	recordedAt := sig.SignedAt
	if sig.SubmittedAt != nil {
		recordedAt = *sig.SubmittedAt
	}
	return fmt.Sprintf("%s-%s-%s.pdf",
		slug(sig.AgreementTitle), slug(sig.ClientName), recordedAt.Format("2006-01-02"))
}
