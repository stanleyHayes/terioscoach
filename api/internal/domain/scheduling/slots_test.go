package scheduling

import (
	"errors"
	"testing"
	"time"
)

// testDay returns noon UTC on a fixed date plus a rule opening that
// date's weekday with the given windows/buffer — so tests never depend on
// what weekday a hard-coded date falls on.
func ruleForDay(day time.Time, buffer int, windows ...Window) WeeklyRule {
	return WeeklyRule{
		PractitionerID: "prac-1",
		Weekday:        day.Weekday(),
		Windows:        windows,
		BufferMinutes:  buffer,
	}
}

func starts(slots []Interval) []time.Time {
	out := make([]time.Time, 0, len(slots))
	for _, s := range slots {
		out = append(out, s.Start)
	}
	return out
}

func TestGenerateSlotsBasic(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	req := SlotRequest{
		Rules:           []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 720})}, // 09:00-12:00
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             time.UTC,
		Now:             day.Add(-24 * time.Hour),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	want := []time.Time{
		time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 10, 11, 0, 0, 0, time.UTC),
	}
	got := starts(slots)
	if len(got) != len(want) {
		t.Fatalf("starts = %v, want %v", got, want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("slot %d = %v, want %v", i, got[i], want[i])
		}
		if !slots[i].End.Equal(want[i].Add(time.Hour)) {
			t.Errorf("slot %d end = %v, want one hour later", i, slots[i].End)
		}
	}
}

func TestGenerateSlotsSkipsClosedAndPast(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	// Only one rule: the second range day is closed.
	req := SlotRequest{
		Rules:           []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 720})},
		DurationMinutes: 60,
		From:            day,
		To:              day.AddDate(0, 0, 1),
		Loc:             time.UTC,
		Now:             time.Date(2026, 8, 10, 10, 30, 0, 0, time.UTC),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	got := starts(slots)
	if len(got) != 1 || !got[0].Equal(time.Date(2026, 8, 10, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("starts = %v, want only 11:00 (past skipped, next day closed)", got)
	}
}

func TestGenerateSlotsTimeOffExclusion(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	req := SlotRequest{
		Rules: []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 720})},
		TimeOff: []Interval{{
			Start: time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 8, 10, 11, 0, 0, 0, time.UTC),
		}},
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             time.UTC,
		Now:             day.Add(-24 * time.Hour),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	got := starts(slots)
	if len(got) != 2 || !got[0].Equal(day.Add(9*time.Hour)) || !got[1].Equal(day.Add(11*time.Hour)) {
		t.Fatalf("starts = %v, want 09:00 and 11:00 (10:00 blocked by time-off)", got)
	}
}

func TestGenerateSlotsBusyIntervalExclusion(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	req := SlotRequest{
		Rules: []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 720})},
		Busy: []Interval{{
			Start: time.Date(2026, 8, 10, 9, 30, 0, 0, time.UTC),
			End:   time.Date(2026, 8, 10, 10, 30, 0, 0, time.UTC),
		}},
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             time.UTC,
		Now:             day.Add(-24 * time.Hour),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	got := starts(slots)
	if len(got) != 1 || !got[0].Equal(day.Add(11*time.Hour)) {
		t.Fatalf("starts = %v, want only 11:00 (09:00/10:00 overlap the booking)", got)
	}
}

func TestGenerateSlotsBufferRespected(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	// 60-minute sessions with a 30-minute buffer step 09:00, 10:30, 12:00,
	// 13:30, 15:00. A booking at 11:00-12:00 expanded by the buffer on both
	// sides blocks 10:30 and 12:00 but nothing else.
	req := SlotRequest{
		Rules: []WeeklyRule{ruleForDay(day, 30, Window{StartMin: 540, EndMin: 1020})},
		Busy: []Interval{{
			Start: time.Date(2026, 8, 10, 11, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
		}},
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             time.UTC,
		Now:             day.Add(-24 * time.Hour),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	want := []time.Time{
		day.Add(9 * time.Hour),
		day.Add(13*time.Hour + 30*time.Minute),
		day.Add(15 * time.Hour),
	}
	got := starts(slots)
	if len(got) != len(want) {
		t.Fatalf("starts = %v, want %v (buffer must clear 30min around the booking)", got, want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("slot %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGenerateSlotsRejectsBadInput(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	base := SlotRequest{
		Rules:           []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 720})},
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             time.UTC,
		Now:             day.Add(-24 * time.Hour),
	}

	overnight := base
	overnight.Rules = []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 1200, EndMin: 600})}
	if _, err := GenerateSlots(overnight); !errors.Is(err, ErrInvalidWindow) {
		t.Errorf("overnight window err = %v, want ErrInvalidWindow", err)
	}

	zeroDuration := base
	zeroDuration.DurationMinutes = 0
	if _, err := GenerateSlots(zeroDuration); !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("zero duration err = %v, want ErrInvalidDuration", err)
	}

	nilLoc := base
	nilLoc.Loc = nil
	if _, err := GenerateSlots(nilLoc); !errors.Is(err, ErrInvalidTimezone) {
		t.Errorf("nil location err = %v, want ErrInvalidTimezone", err)
	}

	inverted := base
	inverted.From, inverted.To = day.AddDate(0, 0, 1), day
	if _, err := GenerateSlots(inverted); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("inverted range err = %v, want ErrInvalidRange", err)
	}

	tooLong := base
	tooLong.To = day.AddDate(0, 0, MaxSlotRangeDays+1)
	if _, err := GenerateSlots(tooLong); !errors.Is(err, ErrInvalidRange) {
		t.Errorf("overlong range err = %v, want ErrInvalidRange", err)
	}
}

func TestGenerateSlotsTimezoneConversion(t *testing.T) {
	day := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	// 09:00 in Tokyo (UTC+9, no DST) is 00:00 UTC.
	req := SlotRequest{
		Rules:           []WeeklyRule{ruleForDay(day, 0, Window{StartMin: 540, EndMin: 600})},
		DurationMinutes: 60,
		From:            day,
		To:              day,
		Loc:             tokyo,
		Now:             day.Add(-48 * time.Hour),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}
	if len(slots) != 1 || !slots[0].Start.Equal(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("starts = %v, want 09:00 JST = 00:00 UTC", starts(slots))
	}
}

func TestGenerateSlotsSpringForwardDST(t *testing.T) {
	// US spring forward: 2026-03-08, 02:00 EST -> 03:00 EDT.
	// Saturday 2026-03-07 is EST (UTC-5), Sunday 2026-03-08 is EDT (UTC-4).
	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("tzdata unavailable: %v", err)
	}
	saturday := time.Date(2026, 3, 7, 0, 0, 0, 0, nyc)
	sunday := time.Date(2026, 3, 8, 0, 0, 0, 0, nyc)
	rules := []WeeklyRule{
		ruleForDay(saturday, 0, Window{StartMin: 540, EndMin: 720}),
		ruleForDay(sunday, 0, Window{StartMin: 540, EndMin: 720}),
	}
	req := SlotRequest{
		Rules:           rules,
		DurationMinutes: 60,
		From:            saturday,
		To:              sunday,
		Loc:             nyc,
		Now:             time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}
	slots, err := GenerateSlots(req)
	if err != nil {
		t.Fatalf("GenerateSlots: %v", err)
	}

	// Both days open 09:00 local wall clock: EST Saturday -> 14:00 UTC,
	// EDT Sunday -> 13:00 UTC. If the engine converted offsets naively one
	// of these would be an hour off.
	mustContain := map[time.Time]bool{
		time.Date(2026, 3, 7, 14, 0, 0, 0, time.UTC): false, // Sat 09:00 EST
		time.Date(2026, 3, 8, 13, 0, 0, 0, time.UTC): false, // Sun 09:00 EDT
	}
	for _, s := range slots {
		if _, want := mustContain[s.Start]; want {
			mustContain[s.Start] = true
		}
		// Slot length stays a true hour even across the transition.
		if s.End.Sub(s.Start) != time.Hour {
			t.Errorf("slot %v lasts %v, want 1h", s.Start, s.End.Sub(s.Start))
		}
	}
	for want, found := range mustContain {
		if !found {
			t.Errorf("missing slot %v in %v", want, starts(slots))
		}
	}
	if len(slots) != 6 {
		t.Errorf("got %d slots, want 6 (3 per day)", len(slots))
	}
}
