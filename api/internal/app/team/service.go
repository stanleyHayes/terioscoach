package team

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

type Service struct {
	users  ports.UserRepository
	staff  ports.StaffRepository
	hasher ports.PasswordHasher
	now    func() time.Time
}

var _ ports.TeamService = (*Service)(nil)

func NewService(users ports.UserRepository, staff ports.StaffRepository, hasher ports.PasswordHasher) *Service {
	return &Service{users: users, staff: staff, hasher: hasher, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) List(ctx context.Context, _ identity.Identity) ([]identity.User, error) {
	return s.staff.ListStaff(ctx)
}

func validateAccess(roleName string, permissions []identity.Permission) error {
	if strings.TrimSpace(roleName) == "" {
		return identity.ErrInvalidRole
	}
	seen := map[identity.Permission]bool{}
	for _, permission := range permissions {
		if !permission.Valid() {
			return identity.ErrInvalidPermission
		}
		seen[permission] = true
	}
	if len(seen) == 0 {
		return identity.ErrInvalidPermission
	}
	return nil
}

func (s *Service) Create(ctx context.Context, _ identity.Identity, input ports.CreateStaffInput) (ports.StaffCreation, error) {
	if err := validateAccess(input.RoleName, input.Permissions); err != nil {
		return ports.StaffCreation{}, err
	}
	random := make([]byte, 18)
	if _, err := rand.Read(random); err != nil {
		return ports.StaffCreation{}, err
	}
	password := base64.RawURLEncoding.EncodeToString(random) + "!9a"
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return ports.StaffCreation{}, err
	}
	user, err := identity.NewUser(input.Email, input.Name, hash, identity.RoleStaff, s.now())
	if err != nil {
		return ports.StaffCreation{}, err
	}
	user.RoleName = strings.TrimSpace(input.RoleName)
	user.Permissions = identity.NewPermissionSet(input.Permissions...)
	created, err := s.users.Create(ctx, user)
	if err != nil {
		return ports.StaffCreation{}, err
	}
	return ports.StaffCreation{User: created, TemporaryPassword: password}, nil
}

func (s *Service) Update(ctx context.Context, actor identity.Identity, userID string, input ports.UpdateStaffInput) (identity.User, error) {
	if actor.UserID == userID {
		return identity.User{}, identity.ErrLastOwner
	}
	if err := validateAccess(input.RoleName, input.Permissions); err != nil {
		return identity.User{}, err
	}
	return s.staff.UpdateStaffAccess(ctx, userID, strings.TrimSpace(input.Name), strings.TrimSpace(input.RoleName), input.Permissions, input.Disabled)
}
