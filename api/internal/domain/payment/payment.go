// Package payment is the domain core for online payments: the payment
// entity, its status lifecycle, and the money rules. Card and mobile-money
// details never exist here — checkout is hosted by Paystack; this core only
// tracks references, amounts, and states. It imports nothing outside the
// standard library — no frameworks, no drivers.
package payment

import "time"

// Status is the lifecycle state of a payment. Pending is the only
// initializable state; Success is the only refundable one; Refunded is
// terminal. Failed payments may be re-initialized back to Pending (abandoned
// checkout retry).
type Status string

const (
	StatusPending  Status = "pending"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusSuccess, StatusFailed, StatusRefunded:
		return true
	}
	return false
}

// Payment is one attempt to collect the price of one booking. Exactly one
// payment record exists per booking (the storage layer enforces it with a
// unique index on bookingId); re-initialization replaces the reference on
// the same record. AmountKobo is money in integer minor units, snapshotted
// from the service at initialize time — the domain never sees
// floating-point money.
type Payment struct {
	ID                string
	BookingID         string
	ClientID          string
	AmountKobo        int64
	Currency          string
	Status            Status
	PaystackReference string
	Channel           string
	PaidAt            *time.Time
	RefundedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// New builds a pending payment for a booking. amountKobo and currency are
// snapshotted from the booked service by the caller (app layer); reference
// is server-generated and later sent to Paystack so webhook deliveries join
// back to this record.
func New(bookingID, clientID string, amountKobo int64, currency, reference string, now time.Time) (Payment, error) {
	if amountKobo < 0 {
		return Payment{}, ErrInvalidAmount
	}
	if len(currency) != 3 {
		return Payment{}, ErrInvalidCurrency
	}
	if reference == "" {
		return Payment{}, ErrReferenceRequired
	}
	now = now.UTC()
	return Payment{
		BookingID:         bookingID,
		ClientID:          clientID,
		AmountKobo:        amountKobo,
		Currency:          currency,
		Status:            StatusPending,
		PaystackReference: reference,
		CreatedAt:         now,
		UpdatedAt:         now,
	}, nil
}

// MarkSuccess moves a pending payment to success — the webhook has
// confirmed the charge. Channel (e.g. "card", "mobile_money") and paidAt
// come from the verified transaction.
func (p *Payment) MarkSuccess(channel string, paidAt time.Time) error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	p.Status = StatusSuccess
	p.Channel = channel
	paid := paidAt.UTC()
	p.PaidAt = &paid
	p.UpdatedAt = paid
	return nil
}

// MarkFailed moves a pending payment to failed (charge attempt failed or
// was abandoned and reported as such).
func (p *Payment) MarkFailed(now time.Time) error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	now = now.UTC()
	p.Status = StatusFailed
	p.UpdatedAt = now
	return nil
}

// Reinitialize restarts a pending or failed payment with a fresh reference
// — the client abandoned checkout and is initializing again. Successful and
// refunded payments can never be re-initialized.
func (p *Payment) Reinitialize(reference string, now time.Time) error {
	if p.Status != StatusPending && p.Status != StatusFailed {
		return ErrInvalidTransition
	}
	if reference == "" {
		return ErrReferenceRequired
	}
	now = now.UTC()
	p.Status = StatusPending
	p.PaystackReference = reference
	p.Channel = ""
	p.PaidAt = nil
	p.UpdatedAt = now
	return nil
}

// MarkRefunded moves a successful payment to refunded — terminal. Only
// success is refundable.
func (p *Payment) MarkRefunded(now time.Time) error {
	if p.Status != StatusSuccess {
		return ErrInvalidTransition
	}
	now = now.UTC()
	p.Status = StatusRefunded
	p.RefundedAt = &now
	p.UpdatedAt = now
	return nil
}
