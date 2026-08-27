package payments

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

// fixedNow keeps reference generation and timestamps deterministic.
var fixedNow = time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

const webhookSecret = "sk_test_fake_secret"

type testRig struct {
	svc      *Service
	payments *portstest.FakePaymentRepository
	bookings *portstest.FakeBookingRepository
	services *portstest.FakeServiceRepository
	users    *portstest.FakeUserRepository
	gateway  *portstest.FakePaymentGateway
}

func newTestRig() testRig {
	rig := testRig{
		payments: portstest.NewFakePaymentRepository(),
		bookings: portstest.NewFakeBookingRepository(),
		services: portstest.NewFakeServiceRepository(),
		users:    portstest.NewFakeUserRepository(),
		gateway:  portstest.NewFakePaymentGateway(webhookSecret),
	}
	rig.svc = NewService(rig.payments, rig.bookings, rig.services, rig.users, rig.gateway)
	rig.svc.now = func() time.Time { return fixedNow }
	return rig
}

// seedBooked wires a client account, a priced service, and a confirmed
// booking, returning the client identity and the booking.
func seedBooked(t *testing.T, rig testRig) (identity.Identity, booking.Booking) {
	t.Helper()
	ctx := context.Background()
	user, err := rig.users.Create(ctx, mustUser(t, "client@example.com", identity.RoleClient))
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	svc, err := catalog.NewService("prac-1", "Massage", "", 60, 25000, "GHS", 1, fixedNow)
	if err != nil {
		t.Fatalf("domain NewService: %v", err)
	}
	svc, err = rig.services.Create(ctx, svc)
	if err != nil {
		t.Fatalf("seed service: %v", err)
	}
	b, err := booking.New(user.ID, "prac-1", svc.ID, fixedNow.Add(48*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("domain booking New: %v", err)
	}
	b, err = rig.bookings.Create(ctx, b)
	if err != nil {
		t.Fatalf("seed booking: %v", err)
	}
	return identity.Identity{UserID: user.ID, Role: identity.RoleClient}, b
}

func mustUser(t *testing.T, email string, role identity.Role) identity.User {
	t.Helper()
	u, err := identity.NewUser(email, "Test User", "fakehash:password", role, fixedNow)
	if err != nil {
		t.Fatalf("domain NewUser: %v", err)
	}
	return u
}

func initialize(t *testing.T, rig testRig, id identity.Identity, bookingID string) ports.Initialization {
	t.Helper()
	init, err := rig.svc.InitializePayment(context.Background(), id, bookingID)
	if err != nil {
		t.Fatalf("InitializePayment: %v", err)
	}
	return init
}

// chargeSuccessPayload builds a signed-ready charge.success event body.
func chargeSuccessPayload(reference string) []byte {
	return []byte(fmt.Sprintf(`{"event":"charge.success","data":{"reference":%q}}`, reference))
}

func deliver(t *testing.T, rig testRig, payload []byte) error {
	t.Helper()
	return rig.svc.HandlePaystackWebhook(context.Background(), payload, rig.gateway.SignWebhook(payload))
}

func TestInitializeDerivesEverythingServerSide(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)

	init := initialize(t, rig, id, b.ID)
	if init.AuthorizationURL == "" {
		t.Error("authorizationUrl should come from the gateway")
	}
	if init.Payment.ID == "" || init.Payment.Status != payment.StatusPending {
		t.Errorf("payment = %+v, want id assigned, status pending", init.Payment)
	}
	if init.Payment.PaystackReference == "" || init.Payment.PaystackReference != init.Payment.PaystackReference {
		t.Errorf("reference = %q, want server-generated and stored", init.Payment.PaystackReference)
	}
	if init.Payment.AmountKobo != 25000 || init.Payment.Currency != "GHS" {
		t.Errorf("payment = %+v, want amount/currency snapshotted from the service", init.Payment)
	}

	if len(rig.gateway.InitializeCalls) != 1 {
		t.Fatalf("gateway calls = %d, want 1", len(rig.gateway.InitializeCalls))
	}
	call := rig.gateway.InitializeCalls[0]
	if call.Email != "client@example.com" || call.AmountKobo != 25000 || call.Currency != "GHS" {
		t.Errorf("gateway call = %+v, want email from account, amount/currency from service", call)
	}
	if call.Reference != init.Payment.PaystackReference {
		t.Errorf("gateway reference %q != stored %q", call.Reference, init.Payment.PaystackReference)
	}
}

func TestInitializeOwnershipAndIsolation(t *testing.T) {
	rig := newTestRig()
	_, b := seedBooked(t, rig)
	other := identity.Identity{UserID: "client-999", Role: identity.RoleClient}
	prac := identity.Identity{UserID: "prac-1", Role: identity.RolePractitioner}

	if _, err := rig.svc.InitializePayment(context.Background(), other, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("other client err = %v, want ErrBookingNotFound (no leak)", err)
	}
	if _, err := rig.svc.InitializePayment(context.Background(), prac, b.ID); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("practitioner err = %v, want ErrBookingNotFound", err)
	}
	if _, err := rig.svc.InitializePayment(context.Background(), other, "booking-999"); !errors.Is(err, booking.ErrBookingNotFound) {
		t.Errorf("unknown booking err = %v, want ErrBookingNotFound", err)
	}
}

func TestInitializeGuardsBookingStatus(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	if err := b.Cancel(fixedNow.Add(time.Hour)); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := rig.bookings.Update(context.Background(), b); err != nil {
		t.Fatalf("update booking: %v", err)
	}
	if _, err := rig.svc.InitializePayment(context.Background(), id, b.ID); !errors.Is(err, booking.ErrInvalidTransition) {
		t.Errorf("cancelled booking err = %v, want ErrInvalidTransition", err)
	}
}

func TestInitializeAlreadyPaid(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)

	if err := deliver(t, rig, chargeSuccessPayload(init.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	if _, err := rig.svc.InitializePayment(context.Background(), id, b.ID); !errors.Is(err, payment.ErrAlreadyPaid) {
		t.Errorf("second initialize err = %v, want ErrAlreadyPaid", err)
	}
}

func TestReinitializeReusesPendingRecord(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	first := initialize(t, rig, id, b.ID)
	second := initialize(t, rig, id, b.ID)

	if second.Payment.ID != first.Payment.ID {
		t.Errorf("re-initialize created a new record %q, want reuse of %q", second.Payment.ID, first.Payment.ID)
	}
	if second.Payment.PaystackReference == first.Payment.PaystackReference {
		t.Error("re-initialize should mint a fresh reference")
	}
	if second.Payment.Status != payment.StatusPending {
		t.Errorf("status = %q, want pending", second.Payment.Status)
	}
}

// Re-initializing does not cancel the checkout it replaces. The gateway page
// for the old reference is still open in whichever tab the client left it in,
// and paying there is a real charge — one that arrives quoting a reference the
// record no longer advertises. Dropping it on the floor takes the client's
// money and leaves the booking reading unpaid.
func TestWebhookOnAbandonedCheckoutStillRecordsThePayment(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	abandoned := initialize(t, rig, id, b.ID)
	current := initialize(t, rig, id, b.ID)

	// The client went back to the first tab and paid there.
	if err := deliver(t, rig, chargeSuccessPayload(abandoned.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	p, err := rig.payments.FindByID(context.Background(), current.Payment.ID)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusSuccess || p.PaidAt == nil {
		t.Fatalf("payment = %+v, want success — the charge was real", p)
	}
	// The charged reference becomes the live one: it is the transaction the
	// gateway holds, and a refund has to quote it.
	if p.PaystackReference != abandoned.Payment.PaystackReference {
		t.Errorf("reference = %q, want the charged one %q",
			p.PaystackReference, abandoned.Payment.PaystackReference)
	}
	if !p.KnownReference(current.Payment.PaystackReference) {
		t.Errorf("displaced reference %q was forgotten", current.Payment.PaystackReference)
	}

	updated, err := rig.bookings.FindByID(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if updated.PaymentStatus != booking.PaymentPaid {
		t.Errorf("booking paymentStatus = %q, want paid", updated.PaymentStatus)
	}
}

// The refund has to reach the transaction that was actually charged, not the
// abandoned checkout that replaced it in the record.
func TestRefundQuotesTheChargedReference(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	abandoned := initialize(t, rig, id, b.ID)
	initialize(t, rig, id, b.ID)
	if err := deliver(t, rig, chargeSuccessPayload(abandoned.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	p, err := rig.payments.FindByBookingID(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if _, err := rig.svc.RefundPayment(context.Background(), "prac-1", p.ID); err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	if got := rig.gateway.RefundCalls; len(got) != 1 || got[0] != abandoned.Payment.PaystackReference {
		t.Errorf("refund calls = %v, want [%q]", got, abandoned.Payment.PaystackReference)
	}
}

func TestWebhookRejectsBadSignatures(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	payload := chargeSuccessPayload(init.Payment.PaystackReference)

	cases := []struct {
		name      string
		payload   []byte
		signature string
	}{
		{"tampered body", []byte(`{"event":"charge.success","data":{"reference":"forged"}}`), rig.gateway.SignWebhook(payload)},
		{"wrong key", payload, signWithKey("sk_test_wrong", payload)},
		{"missing signature", payload, ""},
		{"garbage signature", payload, "not-hex-at-all"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := rig.svc.HandlePaystackWebhook(context.Background(), tc.payload, tc.signature)
			if !errors.Is(err, payment.ErrInvalidWebhookSignature) {
				t.Errorf("err = %v, want ErrInvalidWebhookSignature", err)
			}
		})
	}
	// None of the forged deliveries touched the payment.
	p, err := rig.payments.FindByReference(context.Background(), init.Payment.PaystackReference)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusPending {
		t.Errorf("status = %q, want still pending after forged webhooks", p.Status)
	}
}

func signWithKey(key string, payload []byte) string {
	return portstest.NewFakePaymentGateway(key).SignWebhook(payload)
}

func TestWebhookChargeSuccessMarksPaid(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)

	if err := deliver(t, rig, chargeSuccessPayload(init.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	p, err := rig.payments.FindByReference(context.Background(), init.Payment.PaystackReference)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusSuccess || p.Channel != "card" || p.PaidAt == nil {
		t.Errorf("payment = %+v, want success with channel and paidAt", p)
	}

	updated, err := rig.bookings.FindByID(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if updated.PaymentStatus != booking.PaymentPaid || updated.PaidAt == nil {
		t.Errorf("booking = %+v, want denormalized paymentStatus paid", updated)
	}
	if updated.Status != booking.StatusConfirmed {
		t.Errorf("booking status = %q, want unchanged confirmed", updated.Status)
	}
}

func TestWebhookIsIdempotent(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	payload := chargeSuccessPayload(init.Payment.PaystackReference)

	for i := 0; i < 3; i++ {
		if err := deliver(t, rig, payload); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}

	payments, err := rig.payments.ListByClient(context.Background(), id.UserID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("payments = %d, want exactly one record", len(payments))
	}
	if payments[0].Status != payment.StatusSuccess {
		t.Errorf("status = %q, want success", payments[0].Status)
	}
}

func TestWebhookIgnoresUnknownReferencesAndOtherEvents(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)

	if err := deliver(t, rig, chargeSuccessPayload("terios_unknown_1")); err != nil {
		t.Errorf("unknown reference should be acknowledged, got %v", err)
	}
	other := []byte(`{"event":"charge.failed","data":{"reference":"` + init.Payment.PaystackReference + `"}}`)
	if err := deliver(t, rig, other); err != nil {
		t.Errorf("non-charge.success event should be acknowledged, got %v", err)
	}
	unparseable := []byte(`not json`)
	if err := deliver(t, rig, unparseable); err != nil {
		t.Errorf("authentic unparseable body should be acknowledged, got %v", err)
	}

	p, err := rig.payments.FindByReference(context.Background(), init.Payment.PaystackReference)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusPending {
		t.Errorf("status = %q, want still pending", p.Status)
	}
}

func TestWebhookVerifyMismatchDoesNotMarkPaid(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	ref := init.Payment.PaystackReference

	// Gateway verify disagrees with the stored amount — possible tampering.
	rig.gateway.VerifyResults[ref] = ports.VerifiedTransaction{
		Reference: ref, Status: "success", AmountKobo: 100, Currency: "GHS", Channel: "card", PaidAt: fixedNow,
	}
	if err := deliver(t, rig, chargeSuccessPayload(ref)); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	p, err := rig.payments.FindByReference(context.Background(), ref)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusPending {
		t.Errorf("status = %q, want still pending on amount mismatch", p.Status)
	}

	// Gateway verify reports failure — also no paid stamp.
	rig.gateway.VerifyResults[ref] = ports.VerifiedTransaction{
		Reference: ref, Status: "failed", AmountKobo: 25000, Currency: "GHS",
	}
	if err := deliver(t, rig, chargeSuccessPayload(ref)); err != nil {
		t.Fatalf("webhook: %v", err)
	}
	p, _ = rig.payments.FindByReference(context.Background(), ref)
	if p.Status != payment.StatusPending {
		t.Errorf("status = %q, want still pending on gateway failure status", p.Status)
	}
}

func TestWebhookTransientVerifyFailureSurfaces(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	rig.gateway.VerifyErr = &ports.GatewayError{StatusCode: 0, Message: "paystack is unreachable"}

	err := deliver(t, rig, chargeSuccessPayload(init.Payment.PaystackReference))
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Errorf("err = %v, want GatewayError so the transport can 502 and Paystack retries", err)
	}
}

func TestRefundGuards(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)

	// Pending is not refundable.
	if _, err := rig.svc.RefundPayment(context.Background(), "prac-1", init.Payment.ID); !errors.Is(err, payment.ErrInvalidTransition) {
		t.Errorf("refund pending err = %v, want ErrInvalidTransition", err)
	}

	if err := deliver(t, rig, chargeSuccessPayload(init.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	// A practitioner who does not own the booking gets not-found.
	if _, err := rig.svc.RefundPayment(context.Background(), "prac-2", init.Payment.ID); !errors.Is(err, payment.ErrPaymentNotFound) {
		t.Errorf("cross-practitioner err = %v, want ErrPaymentNotFound", err)
	}

	refunded, err := rig.svc.RefundPayment(context.Background(), "prac-1", init.Payment.ID)
	if err != nil {
		t.Fatalf("RefundPayment: %v", err)
	}
	if refunded.Status != payment.StatusRefunded || refunded.RefundedAt == nil {
		t.Errorf("payment = %+v, want refunded with timestamp", refunded)
	}
	if len(rig.gateway.RefundCalls) != 1 || rig.gateway.RefundCalls[0] != init.Payment.PaystackReference {
		t.Errorf("refund calls = %v, want one call with the stored reference", rig.gateway.RefundCalls)
	}

	updated, err := rig.bookings.FindByID(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if updated.PaymentStatus != booking.PaymentRefunded {
		t.Errorf("booking paymentStatus = %q, want refunded", updated.PaymentStatus)
	}

	// Refunded is terminal.
	if _, err := rig.svc.RefundPayment(context.Background(), "prac-1", init.Payment.ID); !errors.Is(err, payment.ErrInvalidTransition) {
		t.Errorf("double refund err = %v, want ErrInvalidTransition", err)
	}
}

func TestRefundGatewayFailureLeavesPaymentUntouched(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	if err := deliver(t, rig, chargeSuccessPayload(init.Payment.PaystackReference)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	rig.gateway.RefundErr = &ports.GatewayError{StatusCode: 400, Message: "transaction not successful"}
	if _, err := rig.svc.RefundPayment(context.Background(), "prac-1", init.Payment.ID); err == nil {
		t.Fatal("expected the gateway error to surface")
	}
	p, err := rig.payments.FindByID(context.Background(), init.Payment.ID)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusSuccess {
		t.Errorf("status = %q, want still success after failed refund", p.Status)
	}
}

func TestListIsolationAndFilter(t *testing.T) {
	rig := newTestRig()
	ctx := context.Background()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)

	// A second practitioner's payment must not appear in prac-1's list.
	otherBooking, err := booking.New("client-x", "prac-2", b.ServiceID, fixedNow.Add(72*time.Hour), 60, fixedNow)
	if err != nil {
		t.Fatalf("domain booking New: %v", err)
	}
	otherBooking, err = rig.bookings.Create(ctx, otherBooking)
	if err != nil {
		t.Fatalf("seed other booking: %v", err)
	}
	otherPayment, err := payment.New(otherBooking.ID, "client-x", 25000, "GHS", "terios_other_1", fixedNow)
	if err != nil {
		t.Fatalf("domain payment New: %v", err)
	}
	if _, err := rig.payments.Create(ctx, otherPayment); err != nil {
		t.Fatalf("seed other payment: %v", err)
	}

	mine, err := rig.svc.ListMine(ctx, id.UserID)
	if err != nil {
		t.Fatalf("ListMine: %v", err)
	}
	if len(mine) != 1 || mine[0].ID != init.Payment.ID {
		t.Errorf("mine = %+v, want exactly the client's own payment", mine)
	}

	forPrac, err := rig.svc.ListForPractitioner(ctx, "prac-1", ports.PaymentFilter{})
	if err != nil {
		t.Fatalf("ListForPractitioner: %v", err)
	}
	if len(forPrac) != 1 || forPrac[0].ID != init.Payment.ID {
		t.Errorf("practitioner list = %+v, want only own bookings' payments", forPrac)
	}

	// The from/to filter bounds createdAt.
	after := fixedNow.Add(time.Hour)
	filtered, err := rig.svc.ListForPractitioner(ctx, "prac-1", ports.PaymentFilter{From: &after})
	if err != nil {
		t.Fatalf("ListForPractitioner filtered: %v", err)
	}
	if len(filtered) != 0 {
		t.Errorf("filtered list = %d items, want 0 outside the window", len(filtered))
	}
}

// stripeSessionPayload builds a signed-ready checkout.session.completed
// event body; the object ID is the session ID used as the gateway
// reference.
func stripeSessionPayload(eventType, reference string) []byte {
	return []byte(fmt.Sprintf(`{"type":%q,"data":{"object":{"id":%q}}}`, eventType, reference))
}

func deliverStripe(t *testing.T, rig testRig, payload []byte) error {
	t.Helper()
	return rig.svc.HandleStripeWebhook(context.Background(), payload, rig.gateway.SignWebhook(payload))
}

func TestStripeWebhookSessionCompletedMarksPaid(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	ref := init.Payment.PaystackReference

	if err := deliverStripe(t, rig, stripeSessionPayload("checkout.session.completed", ref)); err != nil {
		t.Fatalf("webhook: %v", err)
	}

	p, err := rig.payments.FindByReference(context.Background(), ref)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusSuccess || p.PaidAt == nil {
		t.Errorf("payment = %+v, want success with paidAt", p)
	}

	updated, err := rig.bookings.FindByID(context.Background(), b.ID)
	if err != nil {
		t.Fatalf("find booking: %v", err)
	}
	if updated.PaymentStatus != booking.PaymentPaid || updated.PaidAt == nil {
		t.Errorf("booking = %+v, want denormalized paymentStatus paid", updated)
	}
}

func TestStripeWebhookRejectsBadSignatures(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	payload := stripeSessionPayload("checkout.session.completed", init.Payment.PaystackReference)

	cases := []struct {
		name      string
		payload   []byte
		signature string
	}{
		{"tampered body", stripeSessionPayload("checkout.session.completed", "forged"), rig.gateway.SignWebhook(payload)},
		{"wrong key", payload, signWithKey("sk_test_wrong", payload)},
		{"missing signature", payload, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := rig.svc.HandleStripeWebhook(context.Background(), tc.payload, tc.signature)
			if !errors.Is(err, payment.ErrInvalidWebhookSignature) {
				t.Errorf("err = %v, want ErrInvalidWebhookSignature", err)
			}
		})
	}
}

func TestStripeWebhookIgnoresOtherEventsAndUnknownReferences(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	ref := init.Payment.PaystackReference

	if err := deliverStripe(t, rig, stripeSessionPayload("checkout.session.completed", "cs_unknown_1")); err != nil {
		t.Errorf("unknown reference should be acknowledged, got %v", err)
	}
	if err := deliverStripe(t, rig, stripeSessionPayload("checkout.session.expired", ref)); err != nil {
		t.Errorf("non-completed event should be acknowledged, got %v", err)
	}
	if err := deliverStripe(t, rig, []byte(`not json`)); err != nil {
		t.Errorf("authentic unparseable body should be acknowledged, got %v", err)
	}
	if err := deliverStripe(t, rig, stripeSessionPayload("checkout.session.completed", "")); err != nil {
		t.Errorf("missing object id should be acknowledged, got %v", err)
	}

	p, err := rig.payments.FindByReference(context.Background(), ref)
	if err != nil {
		t.Fatalf("find payment: %v", err)
	}
	if p.Status != payment.StatusPending {
		t.Errorf("status = %q, want still pending", p.Status)
	}
}

func TestStripeWebhookIsIdempotent(t *testing.T) {
	rig := newTestRig()
	id, b := seedBooked(t, rig)
	init := initialize(t, rig, id, b.ID)
	payload := stripeSessionPayload("checkout.session.completed", init.Payment.PaystackReference)

	for i := 0; i < 3; i++ {
		if err := deliverStripe(t, rig, payload); err != nil {
			t.Fatalf("delivery %d: %v", i+1, err)
		}
	}

	payments, err := rig.payments.ListByClient(context.Background(), id.UserID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(payments) != 1 || payments[0].Status != payment.StatusSuccess {
		t.Errorf("payments = %+v, want exactly one success record", payments)
	}
}
