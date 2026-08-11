package catalog

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func TestNewServiceDefaults(t *testing.T) {
	svc, err := NewService("prac-1", "  Swedish Massage  ", " Relaxing. ", 60, 25_000_00, "", 1, testNow)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if svc.Name != "Swedish Massage" || svc.Description != "Relaxing." {
		t.Errorf("expected trimmed name/description, got %+v", svc)
	}
	if svc.Currency != DefaultCurrency {
		t.Errorf("currency = %q, want default %q", svc.Currency, DefaultCurrency)
	}
	if !svc.Active {
		t.Error("new services must start active")
	}
	if svc.CreatedAt != testNow || svc.UpdatedAt != testNow {
		t.Errorf("timestamps = %v/%v, want %v", svc.CreatedAt, svc.UpdatedAt, testNow)
	}
	if svc.DeletedAt != nil {
		t.Error("new services must not be deleted")
	}
}

func TestNewServiceValidation(t *testing.T) {
	longName := strings.Repeat("x", 201)
	cases := []struct {
		name     string
		svcName  string
		duration int
		price    int64
		currency string
		wantErr  error
	}{
		{"empty name", "   ", 60, 100, "GHS", ErrInvalidName},
		{"overlong name", longName, 60, 100, "GHS", ErrInvalidName},
		{"duration too short", "X", 4, 100, "GHS", ErrInvalidDuration},
		{"duration too long", "X", 481, 100, "GHS", ErrInvalidDuration},
		{"negative price", "X", 60, -1, "GHS", ErrInvalidPrice},
		{"short currency", "X", 60, 100, "GH", ErrInvalidCurrency},
		{"non-alpha currency", "X", 60, 100, "GH1", ErrInvalidCurrency},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService("prac-1", tc.svcName, "", tc.duration, tc.price, tc.currency, 0, testNow)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewServiceBoundaryValues(t *testing.T) {
	// Bounds are inclusive: 5 and 480 minutes, zero price, lowercase code
	// normalizes to uppercase.
	svc, err := NewService("prac-1", "Quick", "", 5, 0, "ghs", 0, testNow)
	if err != nil {
		t.Fatalf("NewService min bounds: %v", err)
	}
	if svc.Currency != "GHS" {
		t.Errorf("currency = %q, want GHS (normalized)", svc.Currency)
	}
	if _, err := NewService("prac-1", "Long", "", 480, 0, "USD", 0, testNow); err != nil {
		t.Fatalf("NewService max duration: %v", err)
	}
}
