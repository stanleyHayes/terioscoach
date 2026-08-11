package scheduling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/catalog"
	domain "github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

type testRig struct {
	svc      *Service
	services *portstest.FakeServiceRepository
	avail    *portstest.FakeAvailabilityRepository
	busy     *portstest.FakeBusyIntervalReader
}

func newTestRig() testRig {
	rig := testRig{
		services: portstest.NewFakeServiceRepository(),
		avail:    portstest.NewFakeAvailabilityRepository(),
		busy:     &portstest.FakeBusyIntervalReader{},
	}
	rig.svc = NewService(rig.services, rig.avail, rig.busy)
	return rig
}

func seedService(t *testing.T, rig testRig, practitionerID string, active bool) catalog.Service {
	t.Helper()
	svc, err := catalog.NewService(practitionerID, "Massage", "", 60, 1000, "GHS", 1, time.Now().UTC())
	if err != nil {
		t.Fatalf("domain NewService: %v", err)
	}
	svc.Active = active
	created, err := rig.services.Create(context.Background(), svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	return created
}

func TestReplaceRulesValidates(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()

	rules := []domain.WeeklyRule{{
		Weekday: time.Monday,
		Windows: []domain.Window{{StartMin: 540, EndMin: 720}},
	}}
	stored, err := rig.svc.ReplaceRules(ctx, "prac-1", rules)
	if err != nil {
		t.Fatalf("ReplaceRules: %v", err)
	}
	if len(stored) != 1 || stored[0].PractitionerID != "prac-1" {
		t.Errorf("stored rules = %+v, want practitioner stamped", stored)
	}

	// Invalid input leaves the previous schedule untouched.
	bad := []domain.WeeklyRule{{
		Weekday: time.Tuesday,
		Windows: []domain.Window{{StartMin: 1200, EndMin: 600}}, // overnight
	}}
	if _, err := rig.svc.ReplaceRules(ctx, "prac-1", bad); !errors.Is(err, domain.ErrInvalidWindow) {
		t.Fatalf("overnight err = %v, want ErrInvalidWindow", err)
	}
	current, _ := rig.svc.GetRules(ctx, "prac-1")
	if len(current) != 1 || current[0].Weekday != time.Monday {
		t.Errorf("rules after failed replace = %+v, want the Monday rule kept", current)
	}
}

func TestAddTimeOff(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()

	start := time.Now().UTC().Add(48 * time.Hour)
	off, err := rig.svc.AddTimeOff(ctx, "prac-1", start, start.Add(24*time.Hour), "holiday")
	if err != nil {
		t.Fatalf("AddTimeOff: %v", err)
	}
	if off.ID == "" || off.PractitionerID != "prac-1" {
		t.Errorf("time-off = %+v, want id assigned and practitioner stamped", off)
	}
	if _, err := rig.svc.AddTimeOff(ctx, "prac-1", start, start, ""); !errors.Is(err, domain.ErrInvalidTimeOffRange) {
		t.Fatalf("empty range err = %v, want ErrInvalidTimeOffRange", err)
	}
}

func TestGetSlots(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()
	svc := seedService(t, rig, "prac-1", true)

	// Open the queried day's weekday 09:00-12:00.
	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	_, err := rig.svc.ReplaceRules(ctx, "prac-1", []domain.WeeklyRule{{
		Weekday: day.Weekday(),
		Windows: []domain.Window{{StartMin: 540, EndMin: 720}},
	}})
	if err != nil {
		t.Fatalf("ReplaceRules: %v", err)
	}

	res, err := rig.svc.GetSlots(ctx, svc.ID, day, day, "UTC")
	if err != nil {
		t.Fatalf("GetSlots: %v", err)
	}
	if res.DurationMinutes != 60 {
		t.Errorf("duration = %d, want 60", res.DurationMinutes)
	}
	if len(res.Slots) != 3 {
		t.Fatalf("slots = %v, want 3 hourly starts 09:00-12:00", res.Slots)
	}
	for _, iv := range res.Slots {
		if iv.Start.Location() != time.UTC {
			t.Errorf("slot start location = %v, want UTC", iv.Start.Location())
		}
	}

	// A busy interval removes the overlapping starts.
	rig.busy.Intervals = []domain.Interval{{
		Start: day.Add(9*time.Hour + 30*time.Minute),
		End:   day.Add(10*time.Hour + 30*time.Minute),
	}}
	res, err = rig.svc.GetSlots(ctx, svc.ID, day, day, "UTC")
	if err != nil {
		t.Fatalf("GetSlots with busy: %v", err)
	}
	if len(res.Slots) != 1 || !res.Slots[0].Start.Equal(day.Add(11*time.Hour)) {
		t.Errorf("slots with busy = %v, want only 11:00", res.Slots)
	}
}

func TestGetSlotsMisses(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()
	day := time.Now().UTC().Add(7 * 24 * time.Hour)

	if _, err := rig.svc.GetSlots(ctx, "unknown", day, day, "UTC"); !errors.Is(err, catalog.ErrServiceNotFound) {
		t.Errorf("unknown service err = %v, want ErrServiceNotFound", err)
	}

	inactive := seedService(t, rig, "prac-1", false)
	if _, err := rig.svc.GetSlots(ctx, inactive.ID, day, day, "UTC"); !errors.Is(err, catalog.ErrServiceNotFound) {
		t.Errorf("inactive service err = %v, want ErrServiceNotFound (no existence leak)", err)
	}

	active := seedService(t, rig, "prac-1", true)
	if _, err := rig.svc.GetSlots(ctx, active.ID, day, day, "Mars/Olympus"); !errors.Is(err, domain.ErrInvalidTimezone) {
		t.Errorf("bad tz err = %v, want ErrInvalidTimezone", err)
	}
}

// Compile-time check that the test rig satisfies the ports it fakes —
// guards against port drift in test wiring.
var _ ports.ServiceRepository = (*portstest.FakeServiceRepository)(nil)
