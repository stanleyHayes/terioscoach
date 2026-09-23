package agreements

import (
	"context"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

func bookingContext(b booking.Booking) string {
	return fmt.Sprintf("booking:%s:participant:%d", b.ID, b.Participant.Revision)
}

func (s *Service) signingBooking(ctx context.Context, clientID, bookingID string) (booking.Booking, error) {
	if s.bookings == nil {
		return booking.Booking{}, agreement.ErrAgreementRequired
	}
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return b, err
	}
	if b.ClientID != clientID {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	if b.Status != booking.StatusConfirmed && b.Status != booking.StatusPendingPayment {
		return b, booking.ErrInvalidTransition
	}
	if b.Participant == nil || !b.Participant.AppointmentAt.Equal(b.StartAt) {
		return b, fmt.Errorf("%w: confirm participant details for this appointment date", agreement.ErrAgreementRequired)
	}
	if err = b.Participant.Validate(); err != nil {
		return b, fmt.Errorf("%w: participant declaration missing", agreement.ErrAgreementRequired)
	}
	return b, nil
}

func (s *Service) requiredForBooking(ctx context.Context, b booking.Booking) ([]agreement.Agreement, error) {
	svc, err := s.services.FindByID(ctx, b.ServiceID)
	if err != nil {
		return nil, err
	}
	out := []agreement.Agreement{}
	for _, id := range svc.RequiredAgreementIDs() {
		a, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if a.Active {
			out = append(out, a)
		}
	}
	if b.Participant != nil && b.Participant.Under18 {
		guardian, err := s.GuardianConsent(ctx, b.PractitionerID)
		if err != nil {
			return nil, err
		}
		found := false
		for _, a := range out {
			if a.ID == guardian.ID {
				found = true
			}
		}
		if !found {
			out = append(out, guardian)
		}
	}
	return out, nil
}

func (s *Service) StatusesForBooking(ctx context.Context, id identity.Identity, bookingID string) ([]ports.AgreementStatus, error) {
	if s.bookings == nil {
		return nil, agreement.ErrAgreementRequired
	}
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return nil, err
	}
	if (id.Role == identity.RoleClient && b.ClientID != id.UserID) || (id.Role == identity.RolePractitioner && b.PractitionerID != id.UserID) || (id.Role != identity.RoleClient && id.Role != identity.RolePractitioner) {
		return nil, booking.ErrBookingNotFound
	}
	docs, err := s.requiredForBooking(ctx, b)
	if err != nil {
		return nil, err
	}
	sigs, err := s.repo.SignaturesForClient(ctx, b.ClientID)
	if err != nil {
		return nil, err
	}
	out := make([]ports.AgreementStatus, 0, len(docs))
	for _, doc := range docs {
		svc, err := s.services.FindByID(ctx, b.ServiceID)
		if err != nil {
			return nil, err
		}
		st := ports.AgreementStatus{Agreement: &doc, Fee: fmt.Sprintf("%s %d.%02d", svc.Currency, svc.PriceKobo/100, svc.PriceKobo%100), GuardianConsentReady: true}
		if b.Participant != nil && b.Participant.Under18 && st.GuardianConsentReady {
			consent, err := s.GuardianConsent(ctx, b.PractitionerID)
			if err != nil {
				return nil, err
			}
			st.ConsentVersion = consentVersion(consent)
			if doc.Key != "guardian_consent" {
				doc.Body += "\n\n## Parent / guardian consent (" + consentVersion(consent) + ")\n\n" + consent.Body
			}
		}
		if b.Participant != nil && b.Participant.AppointmentAt.Equal(b.StartAt) {
			role := "client"
			if b.Participant.Under18 {
				role = "guardian"
			}
			for _, sig := range sigs {
				if sig.ContextID == executionContext(b, st.ConsentVersion) && sig.AgreementID == doc.ID && sig.AgreementVersion == doc.Version && sig.SignerRole == role && (role != "guardian" || sig.ConsentVersion == st.ConsentVersion) && sig.Acknowledged && !sig.SignedAt.IsZero() && sig.VerifyIntegrity() {
					st.Signature = &sig
					break
				}
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) RequireBookingReady(ctx context.Context, bookingID string) error {
	if s.bookings == nil {
		return agreement.ErrAgreementRequired
	}
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return err
	}
	// Historical records remain readable; every session entry requires an explicit current declaration.
	b, err = s.signingBooking(ctx, b.ClientID, bookingID)
	if err != nil {
		return err
	}
	statuses, err := s.StatusesForBooking(ctx, identity.Identity{UserID: b.ClientID, Role: identity.RoleClient}, bookingID)
	if err != nil {
		return err
	}
	if s.forms != nil {
		outstanding, err := s.forms.List(ctx, ports.SubmissionFilter{ClientID: b.ClientID, BookingID: b.ID, Status: form.StatusAssigned})
		if err != nil {
			return err
		}
		if len(outstanding) > 0 {
			return fmt.Errorf("%w: complete assigned session forms", agreement.ErrAgreementRequired)
		}
	}
	guardianSigned := !b.Participant.Under18
	for _, st := range statuses {
		if !st.Signed() {
			return fmt.Errorf("%w: %s", agreement.ErrAgreementRequired, st.Agreement.Title)
		}
		if st.Signature != nil && agreement.RequiresPractitionerSignature(st.Signature.AgreementKey) && st.Signature.PractitionerSignedAt == nil {
			return fmt.Errorf("%w: awaiting practitioner countersignature for %s", agreement.ErrAgreementRequired, st.Agreement.Title)
		}
		if st.Signature != nil && st.Signature.SignerRole == "guardian" && st.Signature.ConsentBody != "" && st.Signature.ConsentVersion != "" {
			guardianSigned = true
		}
	}
	if !guardianSigned {
		return fmt.Errorf("%w: guardian consent required", agreement.ErrAgreementRequired)
	}
	if b.Status == booking.StatusPendingPayment {
		return fmt.Errorf("%w: complete payment to confirm this appointment", agreement.ErrAgreementRequired)
	}
	return nil
}

func executionContext(b booking.Booking, consent string) string {
	key := bookingContext(b)
	if b.Participant.Under18 {
		key += ":consent:" + consent
	}
	return key
}
