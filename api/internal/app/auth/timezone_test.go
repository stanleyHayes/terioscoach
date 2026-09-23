package auth

import (
	"context"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"testing"
)

func TestTimezonePreferenceIsValidatedAndPersists(t *testing.T) {
	svc, users, _ := newTestService()
	result := registerClient(t, svc, "zone@example.com", "a long enough password")
	id := identity.Identity{UserID: result.User.ID, Role: identity.RoleClient}
	for _, zone := range []string{"America/New_York", "America/Chicago", "America/Los_Angeles", "Pacific/Honolulu", "Africa/Accra", "UTC"} {
		user, err := svc.UpdateTimezone(context.Background(), id, zone)
		if err != nil {
			t.Fatal(err)
		}
		stored, _ := users.FindByID(context.Background(), id.UserID)
		if stored.Timezone != zone || user.Timezone != zone {
			t.Fatal("preference not persisted")
		}
	}
	for _, zone := range []string{"", "Local", "EST", "PST", "-05:00", "America/Imaginary", " UTC "} {
		if _, err := svc.UpdateTimezone(context.Background(), id, zone); err == nil {
			t.Errorf("accepted %q", zone)
		}
	}
	stored, _ := users.FindByID(context.Background(), id.UserID)
	if stored.Timezone != "UTC" {
		t.Fatal("invalid selection overwrote preference")
	}
}
