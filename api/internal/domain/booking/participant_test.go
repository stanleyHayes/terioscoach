package booking

import (
	"errors"
	"testing"
	"time"
)

func TestParticipantDeclarationValidation(t *testing.T) {
	valid := Participant{Name: "Participant", Accurate: true}
	for _, tc := range []struct {
		name string
		p    Participant
		want error
	}{
		{"adult", valid, nil}, {"unknown", Participant{Name: "Participant"}, ErrParticipantRequired},
		{"minor missing guardian", Participant{Name: "Child", Accurate: true, Under18: true}, ErrGuardianRequired},
		{"minor complete", Participant{Name: "Child", Accurate: true, Under18: true, GuardianName: "Parent", GuardianRelationship: "Parent", GuardianEmail: "parent@example.com"}, nil},
		{"contact is not a freeform identity", Participant{Name: "Child", Accurate: true, Under18: true, GuardianName: "Parent", GuardianRelationship: "Parent", GuardianEmail: "Name <parent@example.com>"}, ErrGuardianRequired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.p.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
func TestUnpaidHoldExpiresAtStartOnly(t *testing.T) {
	start := time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC)
	b := Booking{Status: StatusPendingPayment, StartAt: start}
	if b.ExpirePayment(start.Add(-time.Nanosecond)) {
		t.Fatal("expired early")
	}
	if !b.ExpirePayment(start) || b.Status != StatusCancelled || !b.PaymentExpired {
		t.Fatal("did not expire at exact start")
	}
	if b.ExpirePayment(start.Add(time.Minute)) {
		t.Fatal("expiry is not idempotent")
	}
	paid := Booking{Status: StatusConfirmed, StartAt: start}
	if paid.ExpirePayment(start) {
		t.Fatal("expired confirmed booking")
	}
}
