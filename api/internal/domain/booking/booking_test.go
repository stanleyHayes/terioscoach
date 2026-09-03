package booking

import (
	"errors"
	"testing"
	"time"
)

var testNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newTestBooking(t *testing.T) Booking {
	t.Helper()
	b, err := New("client-1", "prac-1", "svc-1", testNow.Add(48*time.Hour), 60, testNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b
}

func TestNewBuildsConfirmedBooking(t *testing.T) {
	b := newTestBooking(t)
	if b.Status != StatusConfirmed {
		t.Errorf("status = %q, want confirmed", b.Status)
	}
	if !b.EndAt.Equal(b.StartAt.Add(time.Hour)) {
		t.Errorf("endAt = %v, want startAt+60m", b.EndAt)
	}
	if !b.CreatedAt.Equal(testNow) || !b.UpdatedAt.Equal(testNow) {
		t.Errorf("timestamps = %v/%v, want now", b.CreatedAt, b.UpdatedAt)
	}
	if _, err := New("c", "p", "s", testNow, 0, testNow); !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("zero duration err = %v, want ErrInvalidDuration", err)
	}
}

func TestStatusLifecycle(t *testing.T) {
	t.Run("cancel frees the slot", func(t *testing.T) {
		b := newTestBooking(t)
		if err := b.Cancel(testNow.Add(time.Hour)); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		if b.Status != StatusCancelled || b.CancelledAt == nil {
			t.Errorf("after cancel: %+v, want cancelled with timestamp", b)
		}
		// Terminal: no further transitions.
		if err := b.Cancel(testNow); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("double cancel err = %v, want ErrInvalidTransition", err)
		}
		if err := b.Complete(b.EndAt); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("complete after cancel err = %v, want ErrInvalidTransition", err)
		}
		if err := b.MarkNoShow(b.StartAt); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("no-show after cancel err = %v, want ErrInvalidTransition", err)
		}
		if err := b.Reschedule(b.StartAt.Add(24*time.Hour), testNow); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("reschedule after cancel err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("complete only after endAt", func(t *testing.T) {
		b := newTestBooking(t)
		if err := b.Complete(b.EndAt.Add(-time.Minute)); !errors.Is(err, ErrTooEarly) {
			t.Fatalf("early complete err = %v, want ErrTooEarly", err)
		}
		if err := b.Complete(b.EndAt); err != nil {
			t.Fatalf("Complete at endAt: %v", err)
		}
		if b.Status != StatusCompleted || b.CompletedAt == nil {
			t.Errorf("after complete: %+v, want completed with timestamp", b)
		}
		if err := b.Cancel(b.EndAt); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("cancel after complete err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("no-show only after startAt", func(t *testing.T) {
		b := newTestBooking(t)
		if err := b.MarkNoShow(b.StartAt.Add(-time.Minute)); !errors.Is(err, ErrTooEarly) {
			t.Fatalf("early no-show err = %v, want ErrTooEarly", err)
		}
		if err := b.MarkNoShow(b.StartAt); err != nil {
			t.Fatalf("MarkNoShow at startAt: %v", err)
		}
		if b.Status != StatusNoShow {
			t.Errorf("status = %q, want no_show", b.Status)
		}
	})
}

func TestRescheduleMovesSlot(t *testing.T) {
	b := newTestBooking(t)
	newStart := b.StartAt.Add(72 * time.Hour)
	if err := b.Reschedule(newStart, testNow.Add(time.Hour)); err != nil {
		t.Fatalf("Reschedule: %v", err)
	}
	if !b.StartAt.Equal(newStart) || !b.EndAt.Equal(newStart.Add(time.Hour)) {
		t.Errorf("after reschedule [%v, %v), want [%v, %v) — duration preserved",
			b.StartAt, b.EndAt, newStart, newStart.Add(time.Hour))
	}
	if b.Status != StatusConfirmed {
		t.Errorf("status = %q, want still confirmed", b.Status)
	}
}

func TestClientCanModifyCutoff(t *testing.T) {
	policy := DefaultPolicy()
	start := testNow.Add(48 * time.Hour)
	if !policy.ClientCanModify(start, testNow) {
		t.Error("48h out: client should still modify")
	}
	if !policy.ClientCanModify(start, start.Add(-24*time.Hour).Add(-time.Second)) {
		t.Error("just outside the cutoff: client should still modify")
	}
	if policy.ClientCanModify(start, start.Add(-23*time.Hour)) {
		t.Error("inside the 24h cutoff: client must be blocked")
	}
	if policy.ClientCanModify(start, start) {
		t.Error("at startAt: client must be blocked")
	}

	// Zero-value policy falls back to the default.
	var zero ReschedulePolicy
	if zero.ClientCanModify(start, start.Add(-time.Hour)) {
		t.Error("zero policy must behave like the 24h default")
	}
}

func TestPaymentStamp(t *testing.T) {
	b := newTestBooking(t)
	if b.PaymentStatus != "" || b.PaidAt != nil {
		t.Errorf("new booking = %+v, want no payment stamp", b)
	}
	paidAt := testNow.Add(time.Hour)
	b.MarkPaid(paidAt)
	if b.PaymentStatus != PaymentPaid || b.PaidAt == nil || !b.PaidAt.Equal(paidAt) {
		t.Errorf("after MarkPaid: %+v, want paid with timestamp", b)
	}
	// A legacy/already-confirmed booking stays confirmed.
	if b.Status != StatusConfirmed {
		t.Errorf("status = %q, want still confirmed", b.Status)
	}
	// Idempotent: re-stamping just refreshes the same state.
	b.MarkPaid(paidAt)
	if b.PaymentStatus != PaymentPaid {
		t.Errorf("double MarkPaid = %q, want paid", b.PaymentStatus)
	}
	b.MarkRefunded()
	if b.PaymentStatus != PaymentRefunded {
		t.Errorf("after MarkRefunded = %q, want refunded", b.PaymentStatus)
	}
	if b.PaidAt == nil {
		t.Error("paidAt should be retained as history after refund")
	}
}

func TestPaymentRequiredBookingConfirmsOnlyAfterPayment(t *testing.T) {
	b := newTestBooking(t)
	b.RequirePayment()
	if b.Status != StatusPendingPayment {
		t.Fatalf("status = %q, want pending_payment", b.Status)
	}
	b.MarkPaid(testNow.Add(time.Hour))
	if b.Status != StatusConfirmed || b.PaymentStatus != PaymentPaid {
		t.Fatalf("after payment = %+v, want confirmed and paid", b)
	}
}
