package scheduling

import (
	"errors"
	"testing"
	"time"
)

func TestWindowValidate(t *testing.T) {
	cases := []struct {
		name    string
		window  Window
		wantErr bool
	}{
		{"valid", Window{StartMin: 540, EndMin: 1020}, false},
		{"midnight close allowed", Window{StartMin: 1380, EndMin: 1440}, false},
		{"whole day", Window{StartMin: 0, EndMin: 1440}, false},
		{"overnight rejected", Window{StartMin: 1200, EndMin: 600}, true},
		{"empty rejected", Window{StartMin: 540, EndMin: 540}, true},
		{"negative start", Window{StartMin: -1, EndMin: 60}, true},
		{"end past midnight", Window{StartMin: 1380, EndMin: 1441}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.window.Validate()
			if tc.wantErr && !errors.Is(err, ErrInvalidWindow) {
				t.Fatalf("err = %v, want ErrInvalidWindow", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
		})
	}
}

func TestWeeklyRuleValidate(t *testing.T) {
	cases := []struct {
		name    string
		rule    WeeklyRule
		wantErr error
	}{
		{
			"valid split day",
			WeeklyRule{Weekday: time.Monday, Windows: []Window{{540, 720}, {780, 1020}}, BufferMinutes: 15},
			nil,
		},
		{
			"no windows means closed rule is pointless but valid",
			WeeklyRule{Weekday: time.Monday, BufferMinutes: 0},
			nil,
		},
		{
			"overlapping windows",
			WeeklyRule{Weekday: time.Monday, Windows: []Window{{540, 800}, {780, 1020}}},
			ErrInvalidWindow,
		},
		{
			"unsorted windows",
			WeeklyRule{Weekday: time.Monday, Windows: []Window{{780, 1020}, {540, 720}}},
			ErrInvalidWindow,
		},
		{
			"buffer too large",
			WeeklyRule{Weekday: time.Monday, Windows: []Window{{540, 720}}, BufferMinutes: 121},
			ErrInvalidBuffer,
		},
		{
			"negative buffer",
			WeeklyRule{Weekday: time.Monday, Windows: []Window{{540, 720}}, BufferMinutes: -1},
			ErrInvalidBuffer,
		},
		{
			"weekday out of range",
			WeeklyRule{Weekday: time.Weekday(7), Windows: []Window{{540, 720}}},
			ErrInvalidWeekday,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rule.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateRulesRejectsDuplicateWeekday(t *testing.T) {
	rules := []WeeklyRule{
		{Weekday: time.Monday, Windows: []Window{{540, 720}}},
		{Weekday: time.Monday, Windows: []Window{{780, 1020}}},
	}
	if err := ValidateRules(rules); !errors.Is(err, ErrDuplicateWeekday) {
		t.Fatalf("err = %v, want ErrDuplicateWeekday", err)
	}
}

func TestNewTimeOff(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(24 * time.Hour)
	end := now.Add(48 * time.Hour)

	off, err := NewTimeOff("prac-1", start, end, "holiday", now)
	if err != nil {
		t.Fatalf("NewTimeOff: %v", err)
	}
	if off.StartAt != start || off.EndAt != end || off.Reason != "holiday" {
		t.Errorf("unexpected time-off: %+v", off)
	}

	if _, err := NewTimeOff("prac-1", end, start, "", now); !errors.Is(err, ErrInvalidTimeOffRange) {
		t.Fatalf("inverted range err = %v, want ErrInvalidTimeOffRange", err)
	}
	if _, err := NewTimeOff("prac-1", start, start, "", now); !errors.Is(err, ErrInvalidTimeOffRange) {
		t.Fatalf("empty range err = %v, want ErrInvalidTimeOffRange", err)
	}
}
