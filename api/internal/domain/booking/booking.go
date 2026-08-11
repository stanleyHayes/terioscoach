// Package booking is the domain core for appointments: the booking entity,
// its status lifecycle, and the reschedule/cancel cutoff policy. It imports
// nothing outside the standard library — no frameworks, no drivers.
package booking

import "time"

// Status is the lifecycle state of a booking. Confirmed is the only
// non-terminal state and the only one that blocks a slot (mirroring the
// partial unique index in the storage layer). Cancelled, Completed, and
// NoShow are terminal: no transition leaves them.
type Status string

const (
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
	StatusCompleted Status = "completed"
	StatusNoShow    Status = "no_show"
)

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusConfirmed, StatusCancelled, StatusCompleted, StatusNoShow:
		return true
	}
	return false
}

// PaymentStatus is the denormalized payment state stamped on a booking for
// list display (BE-06). It is additive display state only — the payments
// collection is the source of truth, and the booking lifecycle Status above
// is untouched by it. Empty means no payment activity yet.
type PaymentStatus string

const (
	PaymentPaid    PaymentStatus = "paid"
	PaymentRefunded PaymentStatus = "refunded"
)

// Booking is an appointment: a client holding one slot of one service with
// one practitioner. The storage shape ({clientId, practitionerId, serviceId,
// startAt, endAt, status}) is what the slot engine reads as busy intervals
// and what the double-booking unique index guards — field names are fixed
// by BE-03/BE-04. PaymentStatus/PaidAt are the additive BE-06 stamp.
type Booking struct {
	ID             string
	ClientID       string
	PractitionerID string
	ServiceID      string
	StartAt        time.Time
	EndAt          time.Time
	Status         Status
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CancelledAt    *time.Time
	CompletedAt    *time.Time
	PaymentStatus  PaymentStatus
	PaidAt         *time.Time
}

// New builds a confirmed booking for a slot. durationMinutes comes from the
// service; the caller (app layer) has already proven startAt is a
// generatable slot for that duration.
func New(clientID, practitionerID, serviceID string, startAt time.Time, durationMinutes int, now time.Time) (Booking, error) {
	if durationMinutes < 1 {
		return Booking{}, ErrInvalidDuration
	}
	now = now.UTC()
	startAt = startAt.UTC()
	return Booking{
		ClientID:       clientID,
		PractitionerID: practitionerID,
		ServiceID:      serviceID,
		StartAt:        startAt,
		EndAt:          startAt.Add(time.Duration(durationMinutes) * time.Minute),
		Status:         StatusConfirmed,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

// Cancel moves a confirmed booking to cancelled, freeing its slot. Any
// other source state is terminal and rejected.
func (b *Booking) Cancel(now time.Time) error {
	if b.Status != StatusConfirmed {
		return ErrInvalidTransition
	}
	now = now.UTC()
	b.Status = StatusCancelled
	b.CancelledAt = &now
	b.UpdatedAt = now
	return nil
}

// Complete moves a confirmed booking to completed — only once the
// appointment has ended.
func (b *Booking) Complete(now time.Time) error {
	if b.Status != StatusConfirmed {
		return ErrInvalidTransition
	}
	if now.Before(b.EndAt) {
		return ErrTooEarly
	}
	now = now.UTC()
	b.Status = StatusCompleted
	b.CompletedAt = &now
	b.UpdatedAt = now
	return nil
}

// MarkNoShow moves a confirmed booking to no_show — only once the
// appointment should have started. Practitioner-only; the role check lives
// in the app/transport layer.
func (b *Booking) MarkNoShow(now time.Time) error {
	if b.Status != StatusConfirmed {
		return ErrInvalidTransition
	}
	if now.Before(b.StartAt) {
		return ErrTooEarly
	}
	now = now.UTC()
	b.Status = StatusNoShow
	b.CompletedAt = &now
	b.UpdatedAt = now
	return nil
}

// MarkPaid stamps the denormalized payment state after a charge succeeds
// (BE-06). It is idempotent and deliberately independent of the lifecycle
// Status: paying does not confirm, complete, or cancel anything.
func (b *Booking) MarkPaid(paidAt time.Time) {
	paid := paidAt.UTC()
	b.PaymentStatus = PaymentPaid
	b.PaidAt = &paid
}

// MarkRefunded stamps the denormalized payment state after a refund. The
// PaidAt stamp is retained as history.
func (b *Booking) MarkRefunded() {
	b.PaymentStatus = PaymentRefunded
}

// Reschedule moves a confirmed booking to a new slot, freeing the old one.
// durationMinutes is unchanged (same service); the app layer has already
// proven the new start is a generatable slot.
func (b *Booking) Reschedule(startAt time.Time, now time.Time) error {
	if b.Status != StatusConfirmed {
		return ErrInvalidTransition
	}
	duration := b.EndAt.Sub(b.StartAt)
	now = now.UTC()
	startAt = startAt.UTC()
	b.StartAt = startAt
	b.EndAt = startAt.Add(duration)
	b.UpdatedAt = now
	return nil
}

// DefaultCutoff is how long before an appointment a client may still
// reschedule or cancel. It is a per-practice setting; 24h is the platform
// default.
const DefaultCutoff = 24 * time.Hour

// ReschedulePolicy carries the per-practice modification rules.
type ReschedulePolicy struct {
	Cutoff time.Duration
}

// DefaultPolicy returns the platform-default policy.
func DefaultPolicy() ReschedulePolicy {
	return ReschedulePolicy{Cutoff: DefaultCutoff}
}

// ClientCanModify reports whether a client may still reschedule or cancel
// an appointment starting at startAt. Practitioners skip this check
// entirely — the policy binds clients only.
func (p ReschedulePolicy) ClientCanModify(startAt, now time.Time) bool {
	cutoff := p.Cutoff
	if cutoff <= 0 {
		cutoff = DefaultCutoff
	}
	return now.UTC().Before(startAt.UTC().Add(-cutoff))
}
