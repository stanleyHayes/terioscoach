package httpapi

import (
	"net/http/httptest"
	"testing"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

func TestPermissionForRequest(t *testing.T) {
	t.Parallel()
	cases := map[string]identity.Permission{
		"/v1/admin/team":               identity.PermissionTeam,
		"/v1/admin/content/posts":      identity.PermissionContent,
		"/v1/admin/forms":              identity.PermissionForms,
		"/v1/admin/enquiries":          identity.PermissionEnquiries,
		"/v1/admin/reviews":            identity.PermissionReviews,
		"/v1/admin/payments":           identity.PermissionPayments,
		"/v1/admin/reports/practice":   identity.PermissionReports,
		"/v1/admin/documents":          identity.PermissionDocuments,
		"/v1/services/all":             identity.PermissionServices,
		"/v1/clients/client-1":         identity.PermissionClients,
		"/v1/bookings/booking-1/notes": identity.PermissionClients,
		"/v1/sessions/booking-1/join":  identity.PermissionSchedule,
		"/v1/availability/rules":       identity.PermissionSchedule,
		"/v1/bookings":                 identity.PermissionSchedule,
	}
	for path, want := range cases {
		path, want := path, want
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			if got := permissionForRequest(httptest.NewRequest("GET", path, nil)); got != want {
				t.Fatalf("permissionForRequest(%q) = %q, want %q", path, got, want)
			}
		})
	}
}
