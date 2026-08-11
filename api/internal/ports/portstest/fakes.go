// Package portstest provides in-memory fakes of the outbound ports for
// unit and handler tests. No live MongoDB or real crypto required.
package portstest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/booking"
	"github.com/xcreativs/terios/api/internal/domain/catalog"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/domain/scheduling"
	"github.com/xcreativs/terios/api/internal/ports"
)

// FakeUserRepository stores users in memory.
type FakeUserRepository struct {
	mu      sync.Mutex
	byID    map[string]identity.User
	byEmail map[string]string // email -> id
	next    int
}

var _ ports.UserRepository = (*FakeUserRepository)(nil)

func NewFakeUserRepository() *FakeUserRepository {
	return &FakeUserRepository{
		byID:    make(map[string]identity.User),
		byEmail: make(map[string]string),
	}
}

func (f *FakeUserRepository) Create(_ context.Context, user identity.User) (identity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byEmail[user.Email]; exists {
		return identity.User{}, identity.ErrEmailTaken
	}
	f.next++
	user.ID = fmt.Sprintf("user-%d", f.next)
	f.byID[user.ID] = user
	f.byEmail[user.Email] = user.ID
	return user, nil
}

func (f *FakeUserRepository) FindByEmail(_ context.Context, email string) (identity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byEmail[email]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return f.byID[id], nil
}

func (f *FakeUserRepository) FindByID(_ context.Context, id string) (identity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[id]
	if !ok {
		return identity.User{}, identity.ErrUserNotFound
	}
	return user, nil
}

// FakeRefreshTokenRepository stores refresh sessions in memory.
type FakeRefreshTokenRepository struct {
	mu     sync.Mutex
	byHash map[string]identity.RefreshToken
}

var _ ports.RefreshTokenRepository = (*FakeRefreshTokenRepository)(nil)

func NewFakeRefreshTokenRepository() *FakeRefreshTokenRepository {
	return &FakeRefreshTokenRepository{byHash: make(map[string]identity.RefreshToken)}
}

func (f *FakeRefreshTokenRepository) Store(_ context.Context, token identity.RefreshToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byHash[token.TokenHash] = token
	return nil
}

func (f *FakeRefreshTokenRepository) FindByHash(_ context.Context, tokenHash string) (identity.RefreshToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[tokenHash]
	if !ok {
		return identity.RefreshToken{}, identity.ErrTokenInvalid
	}
	return token, nil
}

func (f *FakeRefreshTokenRepository) Revoke(_ context.Context, tokenHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[tokenHash]
	if !ok {
		return nil
	}
	token.Revoked = true
	f.byHash[tokenHash] = token
	return nil
}

// FakeHasher is a deterministic, obviously-insecure hasher for tests.
type FakeHasher struct{}

var _ ports.PasswordHasher = FakeHasher{}

func (FakeHasher) Hash(password string) (string, error) {
	return "fakehash:" + password, nil
}

func (FakeHasher) Verify(encodedHash, password string) (bool, error) {
	return encodedHash == "fakehash:"+password, nil
}

// FakeTokenIssuer issues deterministic access tokens and random-looking
// refresh tokens, honoring expiry for Authenticate tests.
type FakeTokenIssuer struct {
	mu        sync.Mutex
	next      int
	access    map[string]fakeAccess
	AccessTTL time.Duration
}

type fakeAccess struct {
	id        identity.Identity
	expiresAt time.Time
}

var _ ports.TokenIssuer = (*FakeTokenIssuer)(nil)

func NewFakeTokenIssuer(accessTTL time.Duration) *FakeTokenIssuer {
	return &FakeTokenIssuer{access: make(map[string]fakeAccess), AccessTTL: accessTTL}
}

func (f *FakeTokenIssuer) IssueAccessToken(id identity.Identity) (string, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	token := fmt.Sprintf("access-%d:%s:%s", f.next, id.UserID, id.Role)
	expiresAt := time.Now().UTC().Add(f.AccessTTL)
	f.access[token] = fakeAccess{id: id, expiresAt: expiresAt}
	return token, expiresAt, nil
}

func (f *FakeTokenIssuer) VerifyAccessToken(token string) (identity.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.access[token]
	if !ok {
		return identity.Identity{}, identity.ErrTokenInvalid
	}
	if !time.Now().UTC().Before(entry.expiresAt) {
		return identity.Identity{}, identity.ErrTokenExpired
	}
	return entry.id, nil
}

func (f *FakeTokenIssuer) NewRefreshToken() (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	plain := fmt.Sprintf("refresh-%d", f.next)
	return plain, f.HashRefreshToken(plain), nil
}

func (f *FakeTokenIssuer) HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// SplitAccessToken exposes the (userID, role) embedded in a fake access
// token, for assertions in handler tests.
func SplitAccessToken(token string) (userID, role string, ok bool) {
	rest, found := strings.CutPrefix(token, "access-")
	if !found {
		return "", "", false
	}
	parts := strings.SplitN(rest, ":", 3)
	if len(parts) != 3 {
		return "", "", false
	}
	return parts[1], parts[2], true
}

// FakeServiceRepository stores services in memory. BookedServiceIDs marks
// services as booking-referenced, driving the soft-delete path.
type FakeServiceRepository struct {
	mu               sync.Mutex
	byID             map[string]catalog.Service
	next             int
	BookedServiceIDs map[string]bool
}

var _ ports.ServiceRepository = (*FakeServiceRepository)(nil)

func NewFakeServiceRepository() *FakeServiceRepository {
	return &FakeServiceRepository{
		byID:             make(map[string]catalog.Service),
		BookedServiceIDs: make(map[string]bool),
	}
}

func (f *FakeServiceRepository) Create(_ context.Context, svc catalog.Service) (catalog.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	svc.ID = fmt.Sprintf("service-%d", f.next)
	f.byID[svc.ID] = svc
	return svc, nil
}

func (f *FakeServiceRepository) FindByID(_ context.Context, id string) (catalog.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	svc, ok := f.byID[id]
	if !ok || svc.DeletedAt != nil {
		return catalog.Service{}, catalog.ErrServiceNotFound
	}
	return svc, nil
}

func (f *FakeServiceRepository) ListByPractitioner(_ context.Context, practitionerID string, activeOnly bool) ([]catalog.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []catalog.Service
	for _, svc := range f.byID {
		if svc.DeletedAt != nil {
			continue
		}
		if practitionerID != "" && svc.PractitionerID != practitionerID {
			continue
		}
		if activeOnly && !svc.Active {
			continue
		}
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *FakeServiceRepository) Update(_ context.Context, svc catalog.Service) (catalog.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[svc.ID]; !ok {
		return catalog.Service{}, catalog.ErrServiceNotFound
	}
	f.byID[svc.ID] = svc
	return svc, nil
}

func (f *FakeServiceRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeServiceRepository) HasBookings(_ context.Context, serviceID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.BookedServiceIDs[serviceID], nil
}

// Raw exposes a stored record including soft-deleted ones, for assertions
// in application tests (mirrors SplitAccessToken's helper role).
func (f *FakeServiceRepository) Raw(id string) (catalog.Service, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	svc, ok := f.byID[id]
	return svc, ok
}

// FakeAvailabilityRepository stores weekly rules and time-off in memory.
type FakeAvailabilityRepository struct {
	mu      sync.Mutex
	rules   map[string][]scheduling.WeeklyRule // practitionerID -> rules
	timeOff []scheduling.TimeOff
	next    int
}

var _ ports.AvailabilityRepository = (*FakeAvailabilityRepository)(nil)

func NewFakeAvailabilityRepository() *FakeAvailabilityRepository {
	return &FakeAvailabilityRepository{rules: make(map[string][]scheduling.WeeklyRule)}
}

func (f *FakeAvailabilityRepository) GetRules(_ context.Context, practitionerID string) ([]scheduling.WeeklyRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	rules := append([]scheduling.WeeklyRule(nil), f.rules[practitionerID]...)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Weekday < rules[j].Weekday })
	return rules, nil
}

func (f *FakeAvailabilityRepository) ReplaceRules(_ context.Context, practitionerID string, rules []scheduling.WeeklyRule) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rules[practitionerID] = append([]scheduling.WeeklyRule(nil), rules...)
	return nil
}

func (f *FakeAvailabilityRepository) CreateTimeOff(_ context.Context, t scheduling.TimeOff) (scheduling.TimeOff, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	t.ID = fmt.Sprintf("timeoff-%d", f.next)
	f.timeOff = append(f.timeOff, t)
	return t, nil
}

func (f *FakeAvailabilityRepository) ListTimeOff(_ context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	query := scheduling.Interval{Start: from, End: to}
	var out []scheduling.Interval
	for _, t := range f.timeOff {
		if t.PractitionerID != practitionerID {
			continue
		}
		if iv := (scheduling.Interval{Start: t.StartAt, End: t.EndAt}); iv.Overlaps(query) {
			out = append(out, iv)
		}
	}
	return out, nil
}

// FakeBusyIntervalReader returns a fixed set of busy intervals.
type FakeBusyIntervalReader struct {
	Intervals []scheduling.Interval
}

var _ ports.BusyIntervalReader = (*FakeBusyIntervalReader)(nil)

func (f *FakeBusyIntervalReader) BusyIntervals(_ context.Context, _ string, from, to time.Time) ([]scheduling.Interval, error) {
	query := scheduling.Interval{Start: from, End: to}
	var out []scheduling.Interval
	for _, iv := range f.Intervals {
		if iv.Overlaps(query) {
			out = append(out, iv)
		}
	}
	return out, nil
}

// FakeBookingRepository stores bookings in memory and enforces the same
// confirmed-slot uniqueness the partial unique index enforces in MongoDB:
// a second confirmed booking for the same practitioner+startAt fails with
// booking.ErrSlotUnavailable. It also implements ports.BusyIntervalReader,
// so tests can wire it as the slot engine's busy source and watch a fresh
// booking immediately block its slot.
type FakeBookingRepository struct {
	mu   sync.Mutex
	byID map[string]booking.Booking
	next int
}

var _ ports.BookingRepository = (*FakeBookingRepository)(nil)
var _ ports.BusyIntervalReader = (*FakeBookingRepository)(nil)

func NewFakeBookingRepository() *FakeBookingRepository {
	return &FakeBookingRepository{byID: make(map[string]booking.Booking)}
}

// slotTaken mirrors the unique_confirmed_slot partial index.
func (f *FakeBookingRepository) slotTaken(b booking.Booking) bool {
	if b.Status != booking.StatusConfirmed {
		return false
	}
	for _, other := range f.byID {
		if other.ID != b.ID && other.PractitionerID == b.PractitionerID &&
			other.Status == booking.StatusConfirmed && other.StartAt.Equal(b.StartAt) {
			return true
		}
	}
	return false
}

func (f *FakeBookingRepository) Create(_ context.Context, b booking.Booking) (booking.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.slotTaken(b) {
		return booking.Booking{}, booking.ErrSlotUnavailable
	}
	f.next++
	b.ID = fmt.Sprintf("booking-%d", f.next)
	f.byID[b.ID] = b
	return b, nil
}

func (f *FakeBookingRepository) FindByID(_ context.Context, id string) (booking.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.byID[id]
	if !ok {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	return b, nil
}

func (f *FakeBookingRepository) Update(_ context.Context, b booking.Booking) (booking.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[b.ID]; !ok {
		return booking.Booking{}, booking.ErrBookingNotFound
	}
	if f.slotTaken(b) {
		return booking.Booking{}, booking.ErrSlotUnavailable
	}
	f.byID[b.ID] = b
	return b, nil
}

func (f *FakeBookingRepository) ListByClient(_ context.Context, clientID string) ([]booking.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []booking.Booking
	for _, b := range f.byID {
		if b.ClientID == clientID {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}

func (f *FakeBookingRepository) ListByPractitioner(_ context.Context, practitionerID string, filter ports.BookingFilter) ([]booking.Booking, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []booking.Booking
	for _, b := range f.byID {
		if b.PractitionerID != practitionerID {
			continue
		}
		if filter.From != nil && b.StartAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && !b.StartAt.Before(*filter.To) {
			continue
		}
		if filter.Status != "" && b.Status != filter.Status {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartAt.Before(out[j].StartAt) })
	return out, nil
}

// BusyIntervals returns confirmed bookings overlapping [from, to) — the
// same view the MongoDB BusyIntervalReader gives the slot engine.
func (f *FakeBookingRepository) BusyIntervals(_ context.Context, practitionerID string, from, to time.Time) ([]scheduling.Interval, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	query := scheduling.Interval{Start: from, End: to}
	var out []scheduling.Interval
	for _, b := range f.byID {
		if b.PractitionerID != practitionerID || b.Status != booking.StatusConfirmed {
			continue
		}
		if iv := (scheduling.Interval{Start: b.StartAt, End: b.EndAt}); iv.Overlaps(query) {
			out = append(out, iv)
		}
	}
	return out, nil
}

// FakePaymentRepository stores payments in memory and enforces the same
// uniqueness the MongoDB indexes enforce: one payment per booking, and a
// unique Paystack reference.
type FakePaymentRepository struct {
	mu   sync.Mutex
	byID map[string]payment.Payment
	next int
}

var _ ports.PaymentRepository = (*FakePaymentRepository)(nil)

func NewFakePaymentRepository() *FakePaymentRepository {
	return &FakePaymentRepository{byID: make(map[string]payment.Payment)}
}

func (f *FakePaymentRepository) Create(_ context.Context, p payment.Payment) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, other := range f.byID {
		if other.BookingID == p.BookingID {
			return payment.Payment{}, payment.ErrAlreadyPaid
		}
	}
	f.next++
	p.ID = fmt.Sprintf("payment-%d", f.next)
	f.byID[p.ID] = p
	return p, nil
}

func (f *FakePaymentRepository) FindByID(_ context.Context, id string) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	return p, nil
}

func (f *FakePaymentRepository) FindByBookingID(_ context.Context, bookingID string) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.byID {
		if p.BookingID == bookingID {
			return p, nil
		}
	}
	return payment.Payment{}, payment.ErrPaymentNotFound
}

func (f *FakePaymentRepository) FindByReference(_ context.Context, reference string) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.byID {
		if p.PaystackReference == reference {
			return p, nil
		}
	}
	return payment.Payment{}, payment.ErrPaymentNotFound
}

func (f *FakePaymentRepository) Update(_ context.Context, p payment.Payment) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[p.ID]; !ok {
		return payment.Payment{}, payment.ErrPaymentNotFound
	}
	f.byID[p.ID] = p
	return p, nil
}

func (f *FakePaymentRepository) ListByClient(_ context.Context, clientID string) ([]payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []payment.Payment
	for _, p := range f.byID {
		if p.ClientID == clientID {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *FakePaymentRepository) ListByBookingIDs(_ context.Context, bookingIDs []string, filter ports.PaymentFilter) ([]payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	wanted := make(map[string]bool, len(bookingIDs))
	for _, id := range bookingIDs {
		wanted[id] = true
	}
	var out []payment.Payment
	for _, p := range f.byID {
		if !wanted[p.BookingID] {
			continue
		}
		if filter.From != nil && p.CreatedAt.Before(*filter.From) {
			continue
		}
		if filter.To != nil && !p.CreatedAt.Before(*filter.To) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// FakePaymentGateway is a scriptable in-memory gateway. SecretKey keys the
// webhook HMAC exactly like the real adapter, so tests can sign valid and
// tampered payloads. The Verify/Refund/Initialize error fields force
// failure paths; call slices record what the app layer asked for.
type FakePaymentGateway struct {
	mu              sync.Mutex
	SecretKey       string
	AuthorizeURL    string
	VerifyResults   map[string]ports.VerifiedTransaction
	InitializeErr   error
	VerifyErr       error
	RefundErr       error
	InitializeCalls []ports.InitializeParams
	RefundCalls     []string
}

var _ ports.PaymentGateway = (*FakePaymentGateway)(nil)

func NewFakePaymentGateway(secretKey string) *FakePaymentGateway {
	return &FakePaymentGateway{
		SecretKey:     secretKey,
		AuthorizeURL:  "https://checkout.paystack.com/fake",
		VerifyResults: make(map[string]ports.VerifiedTransaction),
	}
}

func (f *FakePaymentGateway) Initialize(_ context.Context, params ports.InitializeParams) (ports.InitializedTransaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.InitializeErr != nil {
		return ports.InitializedTransaction{}, f.InitializeErr
	}
	f.InitializeCalls = append(f.InitializeCalls, params)
	return ports.InitializedTransaction{AuthorizationURL: f.AuthorizeURL, Reference: params.Reference}, nil
}

func (f *FakePaymentGateway) Verify(_ context.Context, reference string) (ports.VerifiedTransaction, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.VerifyErr != nil {
		return ports.VerifiedTransaction{}, f.VerifyErr
	}
	if vt, ok := f.VerifyResults[reference]; ok {
		return vt, nil
	}
	// Default: a successful charge matching whatever was initialized.
	for _, call := range f.InitializeCalls {
		if call.Reference == reference {
			return ports.VerifiedTransaction{
				Reference:  reference,
				Status:     "success",
				AmountKobo: call.AmountKobo,
				Currency:   call.Currency,
				Channel:    "card",
				PaidAt:     time.Now().UTC(),
			}, nil
		}
	}
	return ports.VerifiedTransaction{}, &ports.GatewayError{StatusCode: 404, Message: "transaction not found"}
}

func (f *FakePaymentGateway) Refund(_ context.Context, reference string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.RefundErr != nil {
		return f.RefundErr
	}
	f.RefundCalls = append(f.RefundCalls, reference)
	return nil
}

func (f *FakePaymentGateway) VerifyWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha512.New, []byte(f.SecretKey))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// SignWebhook computes the x-paystack-signature header for a payload under
// the fake's key — the valid-signature side of webhook tests.
func (f *FakePaymentGateway) SignWebhook(payload []byte) string {
	mac := hmac.New(sha512.New, []byte(f.SecretKey))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
