package payment

import (
	"errors"
	"testing"
	"time"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newTestPayment(t *testing.T) Payment {
	t.Helper()
	p, err := New("booking-1", "client-1", 25000, "GHS", "terios_booking-1_1", testNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestNewBuildsPendingPayment(t *testing.T) {
	p := newTestPayment(t)
	if p.Status != StatusPending {
		t.Errorf("status = %q, want pending", p.Status)
	}
	if p.PaystackReference != "terios_booking-1_1" {
		t.Errorf("reference = %q, want the supplied reference", p.PaystackReference)
	}
	if !p.CreatedAt.Equal(testNow) || !p.UpdatedAt.Equal(testNow) {
		t.Errorf("timestamps = %v/%v, want now", p.CreatedAt, p.UpdatedAt)
	}
}

func TestNewValidation(t *testing.T) {
	cases := []struct {
		name      string
		amount    int64
		currency  string
		reference string
		want      error
	}{
		{"negative amount", -1, "GHS", "ref", ErrInvalidAmount},
		{"short currency", 100, "GH", "ref", ErrInvalidCurrency},
		{"long currency", 100, "GHSS", "ref", ErrInvalidCurrency},
		{"missing reference", 100, "GHS", "", ErrReferenceRequired},
		{"zero amount is valid", 0, "GHS", "ref", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New("b", "c", tc.amount, tc.currency, tc.reference, testNow)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMarkSuccess(t *testing.T) {
	p := newTestPayment(t)
	paidAt := testNow.Add(time.Hour)
	if err := p.MarkSuccess("card", paidAt); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}
	if p.Status != StatusSuccess || p.Channel != "card" {
		t.Errorf("after success: %+v, want success via card", p)
	}
	if p.PaidAt == nil || !p.PaidAt.Equal(paidAt) {
		t.Errorf("paidAt = %v, want %v", p.PaidAt, paidAt)
	}
	// Terminal for charges: no re-success, no failure after success.
	if err := p.MarkSuccess("card", paidAt); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("double success err = %v, want ErrInvalidTransition", err)
	}
	if err := p.MarkFailed(testNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("fail after success err = %v, want ErrInvalidTransition", err)
	}
}

func TestMarkFailedAndReinitialize(t *testing.T) {
	p := newTestPayment(t)
	if err := p.MarkFailed(testNow.Add(time.Hour)); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	if p.Status != StatusFailed {
		t.Errorf("status = %q, want failed", p.Status)
	}
	// Abandoned checkout: a failed payment may restart with a fresh reference.
	if err := p.Reinitialize("terios_booking-1_2", testNow.Add(2*time.Hour)); err != nil {
		t.Fatalf("Reinitialize: %v", err)
	}
	if p.Status != StatusPending || p.PaystackReference != "terios_booking-1_2" {
		t.Errorf("after reinit: %+v, want pending with new reference", p)
	}
	if err := p.Reinitialize("", testNow); !errors.Is(err, ErrReferenceRequired) {
		t.Errorf("empty reference err = %v, want ErrReferenceRequired", err)
	}
}

func TestMarkRefundedSuccessOnly(t *testing.T) {
	p := newTestPayment(t)
	if err := p.MarkRefunded(testNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("refund pending err = %v, want ErrInvalidTransition", err)
	}
	if err := p.MarkSuccess("mobile_money", testNow); err != nil {
		t.Fatalf("MarkSuccess: %v", err)
	}
	if err := p.MarkRefunded(testNow.Add(time.Hour)); err != nil {
		t.Fatalf("MarkRefunded: %v", err)
	}
	if p.Status != StatusRefunded || p.RefundedAt == nil {
		t.Errorf("after refund: %+v, want refunded with timestamp", p)
	}
	// Refunded is terminal.
	if err := p.MarkRefunded(testNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("double refund err = %v, want ErrInvalidTransition", err)
	}
	if err := p.Reinitialize("ref-2", testNow); !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("reinit refunded err = %v, want ErrInvalidTransition", err)
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range []Status{StatusPending, StatusSuccess, StatusFailed, StatusRefunded} {
		if !s.Valid() {
			t.Errorf("status %q should be valid", s)
		}
	}
	if Status("bogus").Valid() {
		t.Error("bogus status should be invalid")
	}
}
