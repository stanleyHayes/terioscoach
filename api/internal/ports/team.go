package ports

import (
	"context"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

type StaffRepository interface {
	ListStaff(ctx context.Context) ([]identity.User, error)
	UpdateStaffAccess(ctx context.Context, userID, name, roleName string, permissions []identity.Permission, disabled bool) (identity.User, error)
}

type CreateStaffInput struct {
	Email       string
	Name        string
	RoleName    string
	Permissions []identity.Permission
}

type UpdateStaffInput struct {
	Name        string
	RoleName    string
	Permissions []identity.Permission
	Disabled    bool
}

type StaffCreation struct {
	User              identity.User
	TemporaryPassword string
}

type TeamService interface {
	List(ctx context.Context, actor identity.Identity) ([]identity.User, error)
	Create(ctx context.Context, actor identity.Identity, input CreateStaffInput) (StaffCreation, error)
	Update(ctx context.Context, actor identity.Identity, userID string, input UpdateStaffInput) (identity.User, error)
}
