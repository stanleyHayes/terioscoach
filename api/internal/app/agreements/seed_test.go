package agreements

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

func TestSeedCreatesBothAgreementsWithTheRealWording(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &fakeServices{}, Options{Now: func() time.Time { return fixedNow }})

	created, err := svc.Seed(context.Background(), "prac-1")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if len(created) != len(SeedAgreements) {
		t.Fatalf("created = %d, want %d", len(created), len(SeedAgreements))
	}

	byKey := map[string]string{}
	for _, a := range created {
		byKey[a.Key] = a.Body
		if a.Version != 1 || !a.Active {
			t.Errorf("%s = version %d active %v, want 1/true", a.Key, a.Version, a.Active)
		}
	}

	for key, phrase := range map[string]string{
		"holistic_coaching": "not a licensed nurse",
		"nurse_coaching":    "Nurse Coaching",
	} {
		if !strings.Contains(byKey[key], phrase) {
			t.Errorf("%s is missing %q — the two agreements may have been swapped", key, phrase)
		}
	}
}

// A deploy must never restore the original wording under a client who
// signed the revised one.
func TestSeedIsIdempotentAndNeverOverwritesAnEdit(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &fakeServices{}, Options{Now: func() time.Time { return fixedNow }})
	ctx := context.Background()

	created, err := svc.Seed(ctx, "prac-1")
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}

	first := created[0]
	edited := "Our own wording now."
	if _, err := svc.Update(ctx, first.ID, ports.AgreementPatch{Body: &edited}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	again, err := svc.Seed(ctx, "prac-1")
	if err != nil {
		t.Fatalf("Seed again: %v", err)
	}
	if len(again) != 0 {
		t.Errorf("second seed created %d agreements, want 0", len(again))
	}

	current, err := svc.Get(ctx, first.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if current.Body != edited {
		t.Error("re-seeding overwrote the practitioner's edit")
	}
	if current.Version != 2 {
		t.Errorf("Version = %d, want the edit to have bumped it to 2", current.Version)
	}
}
