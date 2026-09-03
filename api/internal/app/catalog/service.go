// Package catalog is the application service for the services slice. It
// implements the inbound ports.CatalogService port purely against outbound
// ports — no framework, driver, or transport imports.
package catalog

import (
	"context"
	"strings"
	"time"

	domain "github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the catalog use cases over outbound ports.
type Service struct {
	services ports.ServiceRepository
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.CatalogService = (*Service)(nil)

// NewService wires the use cases to their outbound ports.
func NewService(services ports.ServiceRepository) *Service {
	return &Service{
		services: services,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// ListActive returns the public catalog across the platform's (single)
// practitioner.
func (s *Service) ListActive(ctx context.Context) ([]domain.Service, error) {
	return s.services.ListByPractitioner(ctx, "", true)
}

// ListAll returns the practitioner's full catalog incl. inactive services.
func (s *Service) ListAll(ctx context.Context, practitionerID string) ([]domain.Service, error) {
	return s.services.ListByPractitioner(ctx, practitionerID, false)
}

// CreateService validates input and persists a new active service.
func (s *Service) CreateService(ctx context.Context, practitionerID string, in ports.ServiceInput) (domain.Service, error) {
	svc, err := domain.NewService(
		practitionerID, in.Name, in.Description,
		in.DurationMinutes, in.PriceKobo, in.Currency, in.SortOrder, s.now(),
	)
	if err != nil {
		return domain.Service{}, err
	}
	imageURL := strings.TrimSpace(in.ImageURL)
	if err := domain.ValidateImageURL(imageURL); err != nil {
		return domain.Service{}, err
	}
	svc.ImageURL = imageURL
	return s.services.Create(ctx, svc)
}

// UpdateService applies a partial update after validating every supplied
// field. The service must belong to the practitioner.
func (s *Service) UpdateService(ctx context.Context, practitionerID, id string, patch ports.ServicePatch) (domain.Service, error) {
	svc, err := s.ownedService(ctx, practitionerID, id)
	if err != nil {
		return domain.Service{}, err
	}

	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if err := domain.ValidateName(name); err != nil {
			return domain.Service{}, err
		}
		svc.Name = name
	}
	if patch.Description != nil {
		svc.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.ImageURL != nil {
		imageURL := strings.TrimSpace(*patch.ImageURL)
		if err := domain.ValidateImageURL(imageURL); err != nil {
			return domain.Service{}, err
		}
		svc.ImageURL = imageURL
	}
	if patch.DurationMinutes != nil {
		if err := domain.ValidateDuration(*patch.DurationMinutes); err != nil {
			return domain.Service{}, err
		}
		svc.DurationMinutes = *patch.DurationMinutes
	}
	if patch.PriceKobo != nil {
		if err := domain.ValidatePrice(*patch.PriceKobo); err != nil {
			return domain.Service{}, err
		}
		svc.PriceKobo = *patch.PriceKobo
	}
	if patch.Currency != nil {
		currency := strings.ToUpper(strings.TrimSpace(*patch.Currency))
		if err := domain.ValidateCurrency(currency); err != nil {
			return domain.Service{}, err
		}
		svc.Currency = currency
	}
	if patch.Active != nil {
		svc.Active = *patch.Active
	}
	if patch.SortOrder != nil {
		svc.SortOrder = *patch.SortOrder
	}
	svc.UpdatedAt = s.now()

	return s.services.Update(ctx, svc)
}

// DeleteService removes a service. Once bookings reference it the delete
// is soft — the record stays for booking history but is deactivated and
// hidden from every list; otherwise it is hard-deleted.
func (s *Service) DeleteService(ctx context.Context, practitionerID, id string) error {
	svc, err := s.ownedService(ctx, practitionerID, id)
	if err != nil {
		return err
	}

	booked, err := s.services.HasBookings(ctx, svc.ID)
	if err != nil {
		return err
	}
	if !booked {
		return s.services.Delete(ctx, svc.ID)
	}

	now := s.now()
	svc.Active = false
	svc.DeletedAt = &now
	svc.UpdatedAt = now
	_, err = s.services.Update(ctx, svc)
	return err
}

// ownedService loads a service and enforces practitioner ownership. Every
// miss — unknown, soft-deleted, foreign — is domain.ErrServiceNotFound.
func (s *Service) ownedService(ctx context.Context, practitionerID, id string) (domain.Service, error) {
	svc, err := s.services.FindByID(ctx, id)
	if err != nil {
		return domain.Service{}, err
	}
	if svc.PractitionerID != practitionerID {
		return domain.Service{}, domain.ErrServiceNotFound
	}
	return svc, nil
}
