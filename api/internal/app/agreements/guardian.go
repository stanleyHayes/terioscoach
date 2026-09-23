package agreements

import (
	"context"
	"errors"
	"fmt"

	"github.com/xcreativs/terios/api/internal/domain/agreement"
	"github.com/xcreativs/terios/api/internal/ports"
)

const DefaultGuardianConsent = "I am the parent or legal guardian of the participant named above, who will be under 18 on the appointment date. I confirm that I have authority to consent on their behalf, have reviewed the applicable documents, and consent to the services described. I intend my signature to record this consent."

// GuardianConsent uses the existing versioned agreement editor and immutable
// execution store. A missing policy is initialized once with the owner-authorized
// recommended text. Concurrent initialization converges on its unique stable key.
func (s *Service) GuardianConsent(ctx context.Context, practitionerID string) (agreement.Agreement, error) {
	a, err := s.repo.GetByKey(ctx, practitionerID, "guardian_consent")
	if err == nil {
		return a, nil
	}
	if !errors.Is(err, agreement.ErrAgreementNotFound) {
		return a, err
	}
	draft, err := agreement.New(practitionerID, "guardian_consent", "Parent / guardian consent", DefaultGuardianConsent, false, s.now())
	if err != nil {
		return a, err
	}
	a, err = s.repo.Create(ctx, draft)
	if err != nil {
		if existing, e := s.repo.GetByKey(ctx, practitionerID, "guardian_consent"); e == nil {
			return existing, nil
		}
	}
	return a, err
}
func (s *Service) GuardianConsentForService(ctx context.Context, serviceID string) (agreement.Agreement, error) {
	svc, err := s.services.FindByID(ctx, serviceID)
	if err != nil {
		return agreement.Agreement{}, err
	}
	return s.GuardianConsent(ctx, svc.PractitionerID)
}
func (s *Service) UpdateGuardianConsent(ctx context.Context, practitionerID, body string) (agreement.Agreement, error) {
	a, err := s.GuardianConsent(ctx, practitionerID)
	if err != nil {
		return a, err
	}
	active := true
	return s.Update(ctx, a.ID, ports.AgreementPatch{Body: &body, Active: &active})
}
func consentVersion(a agreement.Agreement) string { return fmt.Sprintf("%s:v%d", a.Key, a.Version) }
