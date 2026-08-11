package catalog

import (
	"context"
	"errors"
	"testing"

	domain "github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

func newTestService() (*Service, *portstest.FakeServiceRepository) {
	repo := portstest.NewFakeServiceRepository()
	return NewService(repo), repo
}

func createOne(t *testing.T, svc *Service, practitionerID, name string, sortOrder int) domain.Service {
	t.Helper()
	created, err := svc.CreateService(context.Background(), practitionerID, ports.ServiceInput{
		Name:            name,
		DurationMinutes: 60,
		PriceKobo:       1000,
		SortOrder:       sortOrder,
	})
	if err != nil {
		t.Fatalf("CreateService(%q): %v", name, err)
	}
	return created
}

func TestListActiveHidesInactiveAndOrders(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	second := createOne(t, svc, "prac-1", "Second", 2)
	first := createOne(t, svc, "prac-1", "First", 1)
	inactive := createOne(t, svc, "prac-1", "Inactive", 0)
	otherPractitioner := createOne(t, svc, "prac-2", "Other", 3)

	if _, err := svc.UpdateService(ctx, "prac-1", inactive.ID, ports.ServicePatch{Active: ptr(false)}); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	public, err := svc.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	// Active services across practitioners, sorted by sortOrder.
	if len(public) != 3 || public[0].ID != first.ID || public[1].ID != second.ID || public[2].ID != otherPractitioner.ID {
		t.Fatalf("ListActive = %+v, want First, Second, Other (inactive hidden)", names(public))
	}

	all, err := svc.ListAll(ctx, "prac-1")
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListAll = %+v, want all 3 incl. inactive", names(all))
	}
}

func TestCreateServiceValidation(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.CreateService(context.Background(), "prac-1", ports.ServiceInput{
		Name:            "",
		DurationMinutes: 60,
	})
	if !errors.Is(err, domain.ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
}

func TestUpdateService(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	created := createOne(t, svc, "prac-1", "Massage", 1)

	updated, err := svc.UpdateService(ctx, "prac-1", created.ID, ports.ServicePatch{
		PriceKobo: ptr(int64(5000)),
		Active:    ptr(false),
	})
	if err != nil {
		t.Fatalf("UpdateService: %v", err)
	}
	if updated.PriceKobo != 5000 || updated.Active {
		t.Errorf("updated = %+v, want price 5000 and inactive", updated)
	}
	if updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Errorf("UpdatedAt moved backwards: %v -> %v", created.UpdatedAt, updated.UpdatedAt)
	}

	// Invalid patch value is rejected before persistence.
	if _, err := svc.UpdateService(ctx, "prac-1", created.ID, ports.ServicePatch{DurationMinutes: ptr(3)}); !errors.Is(err, domain.ErrInvalidDuration) {
		t.Fatalf("bad duration err = %v, want ErrInvalidDuration", err)
	}

	// Unknown id and foreign ownership are both plain not-found.
	if _, err := svc.UpdateService(ctx, "prac-1", "nope", ports.ServicePatch{}); !errors.Is(err, domain.ErrServiceNotFound) {
		t.Fatalf("unknown id err = %v, want ErrServiceNotFound", err)
	}
	if _, err := svc.UpdateService(ctx, "prac-2", created.ID, ports.ServicePatch{}); !errors.Is(err, domain.ErrServiceNotFound) {
		t.Fatalf("foreign owner err = %v, want ErrServiceNotFound", err)
	}
}

func TestDeleteServiceHardWithoutBookings(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	created := createOne(t, svc, "prac-1", "Massage", 1)

	if err := svc.DeleteService(ctx, "prac-1", created.ID); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}
	if _, ok := repo.Raw(created.ID); ok {
		t.Error("hard delete must remove the record entirely")
	}
}

func TestDeleteServiceSoftWithBookings(t *testing.T) {
	svc, repo := newTestService()
	ctx := context.Background()
	created := createOne(t, svc, "prac-1", "Massage", 1)
	repo.BookedServiceIDs[created.ID] = true

	if err := svc.DeleteService(ctx, "prac-1", created.ID); err != nil {
		t.Fatalf("DeleteService: %v", err)
	}
	raw, ok := repo.Raw(created.ID)
	if !ok {
		t.Fatal("soft delete must retain the record for booking history")
	}
	if raw.DeletedAt == nil || raw.Active {
		t.Errorf("soft-deleted record = %+v, want deletedAt set and inactive", raw)
	}

	// Hidden from every list afterwards.
	public, _ := svc.ListActive(ctx)
	all, _ := svc.ListAll(ctx, "prac-1")
	if len(public) != 0 || len(all) != 0 {
		t.Errorf("soft-deleted service still listed: public %d, all %d", len(public), len(all))
	}
	// And treated as gone by further mutations.
	if err := svc.DeleteService(ctx, "prac-1", created.ID); !errors.Is(err, domain.ErrServiceNotFound) {
		t.Errorf("second delete err = %v, want ErrServiceNotFound", err)
	}
}

func ptr[T any](v T) *T { return &v }

func names(services []domain.Service) []string {
	out := make([]string, 0, len(services))
	for _, s := range services {
		out = append(out, s.Name)
	}
	return out
}
