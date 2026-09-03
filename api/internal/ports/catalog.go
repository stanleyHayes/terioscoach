package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/catalog"
)

// ServiceInput carries what a service is created with.
type ServiceInput struct {
	Name            string
	Description     string
	ImageURL        string
	DurationMinutes int
	PriceKobo       int64
	Currency        string // empty defaults to catalog.DefaultCurrency
	SortOrder       int
}

// ServicePatch is a partial update: nil fields stay untouched.
type ServicePatch struct {
	Name            *string
	Description     *string
	ImageURL        *string
	DurationMinutes *int
	PriceKobo       *int64
	Currency        *string
	Active          *bool
	SortOrder       *int
}

// CatalogService is the inbound port for the services slice.
type CatalogService interface {
	// ListActive returns the public catalog: active, non-deleted services
	// ordered by sortOrder then createdAt.
	ListActive(ctx context.Context) ([]catalog.Service, error)
	// ListAll returns the practitioner's full catalog incl. inactive.
	ListAll(ctx context.Context, practitionerID string) ([]catalog.Service, error)
	// CreateService validates input and persists a new active service.
	CreateService(ctx context.Context, practitionerID string, in ServiceInput) (catalog.Service, error)
	// UpdateService applies a partial update. Misses (unknown, deleted,
	// or another practitioner's service) return catalog.ErrServiceNotFound.
	UpdateService(ctx context.Context, practitionerID, id string, patch ServicePatch) (catalog.Service, error)
	// DeleteService removes a service: hard delete when nothing references
	// it, soft delete (deletedAt + deactivate) once bookings exist.
	DeleteService(ctx context.Context, practitionerID, id string) error
}

// ServiceRepository is the outbound port for service persistence.
type ServiceRepository interface {
	// Create persists a new service, assigning its ID.
	Create(ctx context.Context, svc catalog.Service) (catalog.Service, error)
	// FindByID looks up a non-deleted service; misses return
	// catalog.ErrServiceNotFound.
	FindByID(ctx context.Context, id string) (catalog.Service, error)
	// ListByPractitioner returns non-deleted services, ordered by
	// sortOrder then createdAt; activeOnly filters to the public catalog.
	// An empty practitionerID lists across practitioners (single-user
	// platform: the public catalog has exactly one owner).
	ListByPractitioner(ctx context.Context, practitionerID string, activeOnly bool) ([]catalog.Service, error)
	// Update persists an existing service (including soft-delete via
	// DeletedAt). Misses return catalog.ErrServiceNotFound.
	Update(ctx context.Context, svc catalog.Service) (catalog.Service, error)
	// Delete hard-deletes a service. Misses are not an error.
	Delete(ctx context.Context, id string) error
	// HasBookings reports whether any booking references the service —
	// the soft-delete trigger. Backed by the bookings collection (BE-05).
	HasBookings(ctx context.Context, serviceID string) (bool, error)
}
