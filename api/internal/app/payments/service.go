// Package payments is the application service for the payments slice. It
// implements the inbound ports.PaymentService port purely against outbound
// ports — no framework, driver, or transport imports. Card and mobile-money
// details never exist here: checkout is hosted by the gateway; this service
// orchestrates initialize, webhook confirmation, and refund.
package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	notifier ports.Notifier
	now      func() time.Time
}

// Option customizes payment orchestration.
type Option func(*Service)

// WithNotifications sends the payment link after checkout initialization and
// the real confirmation only after Stripe verifies the charge.
func WithNotifications(notifier ports.Notifier) Option {
	return func(s *Service) { s.notifier = notifier }
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
	opts ...Option,
) *Service {
	s := &Service{
		payments: payments,
		bookings: bookings,
		services: services,
		users:    users,
		gateway:  gateway,
		now:      func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// InitializePayment starts a hosted checkout for the caller's own unpaid
// booking request (and tolerates legacy confirmed/unpaid records). Exactly one
// payment record exists per booking: a pending or
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
	if b.Status != booking.StatusPendingPayment && b.Status != booking.StatusConfirmed {
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
		// A service price or currency may have changed since an abandoned
		// checkout was created. The fresh Stripe session and our pending
		// snapshot must agree or the signed webhook will correctly refuse it.
		if err := existing.Reinitialize(init.Reference, svc.PriceKobo, svc.Currency, s.now()); err != nil {
			return ports.Initialization{}, err
		}
		p, err := s.payments.Update(ctx, existing)
		if err != nil {
			return ports.Initialization{}, err
		}
		result := ports.Initialization{Payment: p, AuthorizationURL: init.AuthorizationURL}
		s.notifyPaymentRequired(ctx, b, user.Name, user.Email, svc.Name, result.AuthorizationURL)
		return result, nil
	}

	p, err := payment.New(bookingID, id.UserID, svc.PriceKobo, svc.Currency, init.Reference, s.now())
	if err != nil {
		return ports.Initialization{}, err
	}
	p, err = s.payments.Create(ctx, p)
	if err != nil {
		return ports.Initialization{}, err
	}
	result := ports.Initialization{Payment: p, AuthorizationURL: init.AuthorizationURL}
	s.notifyPaymentRequired(ctx, b, user.Name, user.Email, svc.Name, result.AuthorizationURL)
	return result, nil
}

func (s *Service) notifyPaymentRequired(ctx context.Context, b booking.Booking, clientName, clientEmail, serviceName, paymentURL string) {
	if s.notifier == nil {
		return
	}
	s.notifier.BookingPaymentRequired(ctx, ports.BookingNotice{
		BookingID: b.ID, ClientName: clientName, ClientEmail: clientEmail,
		ServiceName: serviceName, StartAt: b.StartAt, PaymentURL: paymentURL,
	})
}

// stripeWebhookEvent is the minimal shape read from a Stripe delivery —
// after the signature has already authenticated the raw body. The object
// ID is the Checkout Session ID this deployment uses as the gateway
// reference.
type stripeWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
	} `json:"data"`
}

// HandleStripeWebhook processes one raw Stripe delivery: verify first, act only on
// checkout.session.completed, confirm server-side before marking paid,
// and acknowledge everything else without changes so Stripe retries are
// always safe.
func (s *Service) HandleStripeWebhook(ctx context.Context, payload []byte, signature string) error {
	if !s.gateway.VerifyWebhookSignature(payload, signature) {
		return payment.ErrInvalidWebhookSignature
	}
	var evt stripeWebhookEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return nil // authentic but unparseable — nothing to act on
	}
	if evt.Type != "checkout.session.completed" || evt.Data.Object.ID == "" {
		return nil
	}
	return s.confirmCharge(ctx, evt.Data.Object.ID)
}

// confirmCharge marks a pending payment paid after the gateway's own
// Verify confirms the charge and its amount/currency. It is idempotent on
// the payment reference; unknown references and non-pending payments are
// acknowledged with no changes. A transient Verify failure is surfaced so
// the provider retries the delivery.
func (s *Service) confirmCharge(ctx context.Context, reference string) error {
	p, err := s.payments.FindByReference(ctx, reference)
	if errors.Is(err, payment.ErrPaymentNotFound) {
		return nil // not ours — acknowledge so the retries stop
	}
	if err != nil {
		return err
	}
	if p.Status == payment.StatusSuccess {
		paidAt := s.now()
		if p.PaidAt != nil {
			paidAt = *p.PaidAt
		}
		return s.confirmBooking(ctx, &p, paidAt)
	}
	if p.Status != payment.StatusPending {
		return nil
	}

	verified, err := s.gateway.Verify(ctx, reference)
	if err != nil {
		// Transient gateway failure — surface it so the provider retries.
		return err
	}
	if verified.Status != "success" || verified.AmountKobo != p.AmountKobo || verified.Currency != p.Currency {
		return nil // verified truth disagrees with our record — do not mark paid
	}

	paidAt := verified.PaidAt
	if paidAt.IsZero() {
		paidAt = s.now()
	}
	// The delivery may quote a reference this payment was re-initialized
	// away from — the client went back to an abandoned checkout tab and
	// paid there. That reference is the transaction the gateway actually
	// holds, so it becomes the live one; a refund has to quote it.
	p.AdoptReference(reference)
	if err := p.MarkSuccess(verified.Channel, paidAt); err != nil {
		return err
	}
	if _, err := s.payments.Update(ctx, p); err != nil {
		return err
	}
	return s.confirmBooking(ctx, &p, paidAt)
}

func (s *Service) confirmBooking(ctx context.Context, p *payment.Payment, paidAt time.Time) error {
	b, err := s.bookings.FindByID(ctx, p.BookingID)
	if errors.Is(err, booking.ErrBookingNotFound) {
		return nil
	}
	if err != nil || (b.Status == booking.StatusConfirmed && b.PaymentStatus == booking.PaymentPaid) {
		return err
	}
	b.MarkPaid(paidAt)
	updated, err := s.bookings.Update(ctx, b)
	if errors.Is(err, booking.ErrSlotUnavailable) {
		// Another checkout for the same open time completed first. The user's
		// requirement deliberately leaves unpaid requests non-blocking, so this
		// race is possible; compensate immediately rather than keep money for a
		// session that cannot be placed on the calendar.
		if refundErr := s.gateway.Refund(ctx, p.ProviderReference); refundErr != nil {
			return refundErr
		}
		if refundErr := p.MarkRefunded(s.now()); refundErr != nil {
			return refundErr
		}
		if _, refundErr := s.payments.Update(ctx, *p); refundErr != nil {
			return refundErr
		}
		if cancelErr := b.Cancel(s.now()); cancelErr == nil {
			_, _ = s.bookings.Update(ctx, b)
		}
		return nil
	}
	if err != nil {
		return err
	}
	b = updated
	if s.notifier != nil {
		user, userErr := s.users.FindByID(ctx, b.ClientID)
		svc, serviceErr := s.services.FindByID(ctx, b.ServiceID)
		if userErr == nil && serviceErr == nil {
			s.notifier.BookingConfirmed(ctx, ports.BookingNotice{
				BookingID: b.ID, ClientName: user.Name, ClientEmail: user.Email,
				ServiceName: svc.Name, StartAt: b.StartAt,
			})
		}
	}
	return nil
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
	bookings, err := s.bookings.ListByPractitioner(ctx, practitionerID, ports.BookingFilter{IncludePendingPayment: true})
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
	if err := s.gateway.Refund(ctx, p.ProviderReference); err != nil {
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

// newReference generates the server-side join key sent to Stripe:
// recognizable, unique per attempt (timestamp plus random suffix, so even
// back-to-back re-initializations never collide), and valid under
// Stripe's reference alphabet (letters, digits, dash, underscore).
func (s *Service) newReference(bookingID string) string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand never fails on supported platforms; fall back to the
		// timestamp alone rather than abort a checkout.
		return fmt.Sprintf("terios_%s_%d", bookingID, s.now().UnixNano())
	}
	return fmt.Sprintf("terios_%s_%d_%s", bookingID, s.now().UnixNano(), hex.EncodeToString(buf[:]))
}
