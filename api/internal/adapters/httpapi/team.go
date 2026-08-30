package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
)

func WithTeam(service ports.TeamService, auth ports.AuthService) Option {
	return func(server *Server) {
		server.Router.Route("/v1/admin/team", func(router chi.Router) {
			if service == nil || auth == nil {
				router.HandleFunc("/*", handleAuthUnavailable)
				return
			}
			router.Use(RequireAuth(auth), RequireRole(identity.RolePractitioner))
			handler := &teamHandler{service: service}
			router.Get("/", handler.list)
			router.Post("/", handler.create)
			router.Patch("/{id}", handler.update)
		})
	}
}

type teamHandler struct{ service ports.TeamService }

type staffRequest struct {
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	RoleName    string   `json:"roleName"`
	Permissions []string `json:"permissions"`
	Disabled    bool     `json:"disabled"`
}

type staffBody struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	RoleName    string   `json:"roleName"`
	Permissions []string `json:"permissions"`
	Disabled    bool     `json:"disabled"`
	MFAEnabled  bool     `json:"mfaEnabled"`
}

func permissionsFromBody(values []string) []identity.Permission {
	permissions := make([]identity.Permission, len(values))
	for i, value := range values {
		permissions[i] = identity.Permission(value)
	}
	return permissions
}
func staffResponse(user identity.User) staffBody {
	permissionList := user.Permissions.List()
	permissions := make([]string, len(permissionList))
	for i, permission := range permissionList {
		permissions[i] = string(permission)
	}
	roleName := user.RoleName
	if user.Role == identity.RolePractitioner {
		roleName = "Owner"
	}
	return staffBody{ID: user.ID, Email: user.Email, Name: user.Name, Role: string(user.Role), RoleName: roleName, Permissions: permissions, Disabled: user.Disabled, MFAEnabled: user.MFAEnabled}
}

func actor(r *http.Request) identity.Identity {
	value, _ := IdentityFromContext(r.Context())
	return value
}

func (handler *teamHandler) list(w http.ResponseWriter, r *http.Request) {
	users, err := handler.service.List(r.Context(), actor(r))
	if err != nil {
		writeDomainError(w, err)
		return
	}
	items := make([]staffBody, len(users))
	for i, user := range users {
		items[i] = staffResponse(user)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (handler *teamHandler) create(w http.ResponseWriter, r *http.Request) {
	var request staffRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	result, err := handler.service.Create(r.Context(), actor(r), ports.CreateStaffInput{Email: request.Email, Name: request.Name, RoleName: request.RoleName, Permissions: permissionsFromBody(request.Permissions)})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"member": staffResponse(result.User), "temporaryPassword": result.TemporaryPassword})
}

func (handler *teamHandler) update(w http.ResponseWriter, r *http.Request) {
	var request staffRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	user, err := handler.service.Update(r.Context(), actor(r), chi.URLParam(r, "id"), ports.UpdateStaffInput{Name: request.Name, RoleName: request.RoleName, Permissions: permissionsFromBody(request.Permissions), Disabled: request.Disabled})
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"member": staffResponse(user)})
}
