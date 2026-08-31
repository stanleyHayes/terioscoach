// Package payment is the domain core for online payments: the payment
// entity, its status lifecycle, and the money rules. Card and mobile-money
// details never exist here — checkout is hosted by Stripe; this core only
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
	ProviderReference string
	// PreviousReferences are references this payment was initialized under
	// before, oldest first. Re-initialization does not cancel the checkout
	// it replaces: the gateway page for the old reference is still open in
	// whichever tab the client left it in, and paying there produces a real
	// charge that arrives as a webhook for a reference this record no
	// longer advertises. Keeping them is what lets that charge still find
	// its payment instead of being acknowledged as somebody else's.
	PreviousReferences []string
	Channel            string
	PaidAt             *time.Time
	RefundedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// MaxPreviousReferences bounds the retained history. A client who abandons
// checkout twenty times over is not going to come back to the first page,
// and an unbounded list is a document that grows on a stranger's clicks.
const MaxPreviousReferences = 20

// KnownReference reports whether reference identifies this payment, under
// its current reference or any it was initialized under before.
func (p Payment) KnownReference(reference string) bool {
	if reference == "" {
		return false
	}
	if p.ProviderReference == reference {
		return true
	}
	for _, previous := range p.PreviousReferences {
		if previous == reference {
			return true
		}
	}
	return false
}

// New builds a pending payment for a booking. amountKobo and currency are
// snapshotted from the booked service by the caller (app layer); reference
// is server-generated and later sent to Stripe so webhook deliveries join
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
		ProviderReference: reference,
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
//
// The outgoing reference is retained rather than dropped: the checkout it
// belongs to is still live at the gateway, and a client who returns to that
// abandoned tab and pays produces a charge under it. Forgetting it would
// leave that charge with no record to join to — money taken and a booking
// still showing as unpaid.
func (p *Payment) Reinitialize(reference string, now time.Time) error {
	if p.Status != StatusPending && p.Status != StatusFailed {
		return ErrInvalidTransition
	}
	if reference == "" {
		return ErrReferenceRequired
	}
	now = now.UTC()
	if p.ProviderReference != "" && p.ProviderReference != reference {
		p.PreviousReferences = appendReference(p.PreviousReferences, p.ProviderReference)
	}
	p.Status = StatusPending
	p.ProviderReference = reference
	p.Channel = ""
	p.PaidAt = nil
	p.UpdatedAt = now
	return nil
}

// AdoptReference makes reference the payment's live reference, moving the
// one it displaces into the retained history. It is a no-op when reference
// is already live or is not one of this payment's references at all.
//
// This is what a charge confirmed under a superseded reference needs: the
// client paid on the abandoned checkout page, so that reference — not the
// newer unpaid one — is the transaction the gateway holds, and a later
// refund has to quote it.
func (p *Payment) AdoptReference(reference string) {
	if reference == "" || p.ProviderReference == reference || !p.KnownReference(reference) {
		return
	}
	displaced := p.ProviderReference
	kept := p.PreviousReferences[:0:0]
	for _, previous := range p.PreviousReferences {
		if previous != reference {
			kept = append(kept, previous)
		}
	}
	p.PreviousReferences = kept
	p.ProviderReference = reference
	if displaced != "" {
		p.PreviousReferences = appendReference(p.PreviousReferences, displaced)
	}
}

// appendReference adds reference to the retained history, skipping
// duplicates and dropping the oldest entries once the cap is reached.
func appendReference(history []string, reference string) []string {
	for _, previous := range history {
		if previous == reference {
			return history
		}
	}
	history = append(history, reference)
	if len(history) > MaxPreviousReferences {
		history = history[len(history)-MaxPreviousReferences:]
	}
	return history
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
