// Package payments is the application service for the payments slice. It
// implements the inbound ports.PaymentService port purely against outbound
// ports — no framework, driver, or transport imports. Card and mobile-money
// details never exist here: checkout is hosted by the gateway; this service
// orchestrates initialize, webhook confirmation, and refund.
package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
)

// Service orchestrates the payment use cases over outbound ports.
type Service struct {
	payments ports.PaymentRepository
	bookings ports.BookingRepository
	services ports.ServiceRepository
	users    ports.UserRepository
	gateway  ports.PaymentGateway
	now      func() time.Time
}

// Compile-time check: Service satisfies the inbound port.
var _ ports.PaymentService = (*Service)(nil)

// NewService wires the use cases to their outbound ports. The booking,
// service, and user repositories are shared with their own slices: a
// payment's amount, currency, and receipt email are always server-derived
// from those records, never from client input.
func NewService(
	payments ports.PaymentRepository,
	bookings ports.BookingRepository,
	services ports.ServiceRepository,
	users ports.UserRepository,
	gateway ports.PaymentGateway,
) *Service {
	return &Service{
		payments: payments,
		bookings: bookings,
		services: services,
		users:    users,
		gateway:  gateway,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// InitializePayment starts a hosted checkout for the caller's own confirmed
// booking. Exactly one payment record exists per booking: a pending or
// failed one is re-initialized with a fresh reference (abandoned checkout),
// a successful one refuses with ErrAlreadyPaid.
func (s *Service) InitializePayment(ctx context.Context, id identity.Identity, bookingID string) (ports.Initialization, error) {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if err != nil {
		return ports.Initialization{}, err
	}
	if id.Role != identity.RoleClient || b.ClientID != id.UserID {
		// Cross-owner access is reported as not-found — no existence leak.
		return ports.Initialization{}, booking.ErrBookingNotFound
	}
	if b.Status != booking.StatusConfirmed {
		return ports.Initialization{}, booking.ErrInvalidTransition
	}

	svc, err := s.services.FindByID(ctx, b.ServiceID)
	if err != nil {
		return ports.Initialization{}, err
	}
	user, err := s.users.FindByID(ctx, id.UserID)
	if err != nil {
		return ports.Initialization{}, err
	}

	existing, findErr := s.payments.FindByBookingID(ctx, bookingID)
	if findErr != nil && !errors.Is(findErr, payment.ErrPaymentNotFound) {
		return ports.Initialization{}, findErr
	}
	hasExisting := findErr == nil
	if hasExisting && (existing.Status == payment.StatusSuccess || existing.Status == payment.StatusRefunded) {
		return ports.Initialization{}, payment.ErrAlreadyPaid
	}

	reference := s.newReference(bookingID)
	init, err := s.gateway.Initialize(ctx, ports.InitializeParams{
		Email:      user.Email,
		AmountKobo: svc.PriceKobo,
		Currency:   svc.Currency,
		Reference:  reference,
	})
	if err != nil {
		return ports.Initialization{}, err
	}

	if hasExisting {
		// Re-initialize the same record — one payment per booking.
		if err := existing.Reinitialize(init.Reference, s.now()); err != nil {
			return ports.Initialization{}, err
		}
		p, err := s.payments.Update(ctx, existing)
		if err != nil {
			return ports.Initialization{}, err
		}
		return ports.Initialization{Payment: p, AuthorizationURL: init.AuthorizationURL}, nil
	}

	p, err := payment.New(bookingID, id.UserID, svc.PriceKobo, svc.Currency, init.Reference, s.now())
	if err != nil {
		return ports.Initialization{}, err
	}
	p, err = s.payments.Create(ctx, p)
	if err != nil {
		return ports.Initialization{}, err
	}
	return ports.Initialization{Payment: p, AuthorizationURL: init.AuthorizationURL}, nil
}

// webhookEvent is the minimal shape read from a Paystack delivery — after
// the signature has already authenticated the raw body.
type webhookEvent struct {
	Event string `json:"event"`
	Data  struct {
		Reference string `json:"reference"`
	} `json:"data"`
}

// HandlePaystackWebhook processes one raw delivery. The signature is
// verified against the raw bytes before anything is parsed. Only
// charge.success mutates state, and only after a server-side Verify
// confirms the charge and its amount/currency. Every other case —
// duplicate deliveries, unknown references, other event types — is
// acknowledged with no changes, so Paystack retries are always safe.
func (s *Service) HandlePaystackWebhook(ctx context.Context, payload []byte, signature string) error {
	if !s.gateway.VerifyWebhookSignature(payload, signature) {
		return payment.ErrInvalidWebhookSignature
	}
	var evt webhookEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil // authentic but unparseable — nothing to act on
	}
	if evt.Event != "charge.success" || evt.Data.Reference == "" {
		return nil
	}

	p, err := s.payments.FindByReference(ctx, evt.Data.Reference)
	if errors.Is(err, payment.ErrPaymentNotFound) {
		return nil // not ours — acknowledge so the retries stop
	}
	if err != nil {
		return err
	}
	if p.Status == payment.StatusSuccess {
		return nil // idempotent: repeat delivery of an already-recorded charge
	}
	if p.Status != payment.StatusPending {
		return nil
	}

	verified, err := s.gateway.Verify(ctx, evt.Data.Reference)
	if err != nil {
		// Transient gateway failure — surface it so Paystack retries.
		return err
	}
	if verified.Status != "success" || verified.AmountKobo != p.AmountKobo || verified.Currency != p.Currency {
		return nil // verified truth disagrees with our record — do not mark paid
	}

	paidAt := verified.PaidAt
	if paidAt.IsZero() {
		paidAt = s.now()
	}
	if err := p.MarkSuccess(verified.Channel, paidAt); err != nil {
		return err
	}
	if _, err := s.payments.Update(ctx, p); err != nil {
		return err
	}
	return s.stampBooking(ctx, p.BookingID, func(b *booking.Booking) { b.MarkPaid(paidAt) })
}

// stampBooking applies the denormalized payment display state to the
// booking. A missing booking is tolerated (the payment is still the source
// of truth); any other failure is surfaced.
func (s *Service) stampBooking(ctx context.Context, bookingID string, stamp func(*booking.Booking)) error {
	b, err := s.bookings.FindByID(ctx, bookingID)
	if errors.Is(err, booking.ErrBookingNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	stamp(&b)
	_, err = s.bookings.Update(ctx, b)
	return err
}

// ListMine returns the client's own payments, newest first.
func (s *Service) ListMine(ctx context.Context, clientID string) ([]payment.Payment, error) {
	return s.payments.ListByClient(ctx, clientID)
}

// ListForPractitioner returns payments on the practitioner's bookings. The
// payments collection carries no practitionerId (the index design is
// fixed), so the view joins through the practitioner's booking ids.
func (s *Service) ListForPractitioner(ctx context.Context, practitionerID string, filter ports.PaymentFilter) ([]payment.Payment, error) {
	bookings, err := s.bookings.ListByPractitioner(ctx, practitionerID, ports.BookingFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(bookings))
	for _, b := range bookings {
		ids = append(ids, b.ID)
	}
	return s.payments.ListByBookingIDs(ctx, ids, filter)
}

// RefundPayment refunds a successful payment on one of the practitioner's
// bookings. Only success is refundable; the gateway call lands before any
// state change, so a failed refund leaves the payment untouched.
func (s *Service) RefundPayment(ctx context.Context, practitionerID, paymentID string) (payment.Payment, error) {
	p, err := s.payments.FindByID(ctx, paymentID)
	if err != nil {
		return payment.Payment{}, err
	}
	b, err := s.bookings.FindByID(ctx, p.BookingID)
	if errors.Is(err, booking.ErrBookingNotFound) || (err == nil && b.PractitionerID != practitionerID) {
		// Cross-practitioner access is reported as not-found.
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	if err != nil {
		return payment.Payment{}, err
	}
	if p.Status != payment.StatusSuccess {
		return payment.Payment{}, payment.ErrInvalidTransition
	}
	if err := s.gateway.Refund(ctx, p.PaystackReference); err != nil {
		return payment.Payment{}, err
	}
	if err := p.MarkRefunded(s.now()); err != nil {
		return payment.Payment{}, err
	}
	p, err = s.payments.Update(ctx, p)
	if err != nil {
		return payment.Payment{}, err
	}
	if err := s.stampBooking(ctx, p.BookingID, func(b *booking.Booking) { b.MarkRefunded() }); err != nil {
		return payment.Payment{}, err
	}
	return p, nil
}

// newReference generates the server-side join key sent to Paystack:
// recognizable, unique per attempt, and valid under Paystack's reference
// alphabet (letters, digits, dash, underscore).
func (s *Service) newReference(bookingID string) string {
	return fmt.Sprintf("terios_%s_%d", bookingID, s.now().UnixNano())
}
