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
	"github.com/xcreativs/terios/api/internal/domain/client"
	"github.com/xcreativs/terios/api/internal/domain/cms"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/domain/enquiry"
	"github.com/xcreativs/terios/api/internal/domain/form"
	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/domain/note"
	"github.com/xcreativs/terios/api/internal/domain/notification"
	"github.com/xcreativs/terios/api/internal/domain/payment"
	"github.com/xcreativs/terios/api/internal/domain/review"
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

func (f *FakeUserRepository) FindFirstByRole(_ context.Context, role identity.Role) (identity.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, user := range f.byID {
		if user.Role == role {
			return user, nil
		}
	}
	return identity.User{}, identity.ErrUserNotFound
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

func (f *FakeUserRepository) SetPasswordReset(_ context.Context, userID, tokenHash string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[userID]
	if !ok {
		return identity.ErrUserNotFound
	}
	user.PasswordResetTokenHash = tokenHash
	user.PasswordResetExpiresAt = expiresAt
	f.byID[userID] = user
	return nil
}

func (f *FakeUserRepository) ResetPassword(_ context.Context, tokenHash, passwordHash string, now time.Time) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, user := range f.byID {
		if user.PasswordResetTokenHash == tokenHash && now.Before(user.PasswordResetExpiresAt) {
			user.PasswordHash = passwordHash
			user.PasswordResetTokenHash = ""
			user.PasswordResetExpiresAt = time.Time{}
			f.byID[id] = user
			return id, nil
		}
	}
	return "", identity.ErrPasswordResetInvalid
}

func (f *FakeUserRepository) SetMFAPending(_ context.Context, userID, encryptedSecret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[userID]
	if !ok {
		return identity.ErrUserNotFound
	}
	user.MFASecret, user.MFAEnabled = encryptedSecret, false
	f.byID[userID] = user
	return nil
}

func (f *FakeUserRepository) EnableMFA(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[userID]
	if !ok {
		return identity.ErrUserNotFound
	}
	if user.MFASecret == "" {
		return identity.ErrMFANotPending
	}
	user.MFAEnabled = true
	f.byID[userID] = user
	return nil
}

func (f *FakeUserRepository) DisableMFA(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	user, ok := f.byID[userID]
	if !ok {
		return identity.ErrUserNotFound
	}
	user.MFASecret, user.MFAEnabled = "", false
	f.byID[userID] = user
	return nil
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

// RevokeAllForUser mirrors the family-wide revocation the reuse detector
// triggers: every session of the account dies, revoked or not.
func (f *FakeRefreshTokenRepository) RevokeAllForUser(_ context.Context, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		if token.UserID == userID {
			token.Revoked = true
			f.byHash[hash] = token
		}
	}
	return nil
}

// FakeLoginAttemptStore records failed-login accounting in memory, keyed
// on the submitted identifier exactly like the MongoDB adapter. Retention
// is honoured on read so tests can prove records expire.
type FakeLoginAttemptStore struct {
	mu       sync.Mutex
	byKey    map[string]identity.LoginAttempts
	expiries map[string]time.Time
	// Now lets a test drive expiry without sleeping; nil means real time.
	Now func() time.Time
}

var _ ports.LoginAttemptStore = (*FakeLoginAttemptStore)(nil)

func NewFakeLoginAttemptStore() *FakeLoginAttemptStore {
	return &FakeLoginAttemptStore{
		byKey:    make(map[string]identity.LoginAttempts),
		expiries: make(map[string]time.Time),
	}
}

func (f *FakeLoginAttemptStore) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now().UTC()
}

func (f *FakeLoginAttemptStore) Get(_ context.Context, identifier string) (identity.LoginAttempts, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	attempts, ok := f.byKey[identifier]
	if !ok {
		return identity.LoginAttempts{Identifier: identifier}, nil
	}
	if expiry, ok := f.expiries[identifier]; ok && !f.now().Before(expiry) {
		delete(f.byKey, identifier)
		delete(f.expiries, identifier)
		return identity.LoginAttempts{Identifier: identifier}, nil
	}
	return attempts, nil
}

func (f *FakeLoginAttemptStore) Save(_ context.Context, attempts identity.LoginAttempts, retainFor time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byKey[attempts.Identifier] = attempts
	f.expiries[attempts.Identifier] = attempts.LastAt.Add(retainFor)
	return nil
}

func (f *FakeLoginAttemptStore) Reset(_ context.Context, identifier string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byKey, identifier)
	delete(f.expiries, identifier)
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

// FindByReference matches the current reference or any the payment was
// initialized under before — the same rule the Mongo adapter applies, so a
// webhook for an abandoned checkout finds its record here too.
func (f *FakePaymentRepository) FindByReference(_ context.Context, reference string) (payment.Payment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.byID {
		if p.KnownReference(reference) {
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

// FakeClientProfileRepository stores practice profiles in memory, keyed on
// the client's user id like the unique index in MongoDB.
type FakeClientProfileRepository struct {
	mu       sync.Mutex
	byUserID map[string]client.Profile
	next     int
}

var _ ports.ClientProfileRepository = (*FakeClientProfileRepository)(nil)

func NewFakeClientProfileRepository() *FakeClientProfileRepository {
	return &FakeClientProfileRepository{byUserID: make(map[string]client.Profile)}
}

func (f *FakeClientProfileRepository) Upsert(_ context.Context, profile client.Profile) (client.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if existing, ok := f.byUserID[profile.UserID]; ok {
		profile.ID = existing.ID
		profile.CreatedAt = existing.CreatedAt
	} else {
		f.next++
		profile.ID = fmt.Sprintf("profile-%d", f.next)
	}
	f.byUserID[profile.UserID] = profile
	return profile, nil
}

func (f *FakeClientProfileRepository) FindByUserID(_ context.Context, userID string) (client.Profile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile, ok := f.byUserID[userID]
	if !ok {
		return client.Profile{}, client.ErrProfileNotFound
	}
	return profile, nil
}

// FakeDocumentCounter returns scripted document counts per client.
type FakeDocumentCounter struct {
	Counts map[string]int
}

var _ ports.DocumentCounter = (*FakeDocumentCounter)(nil)

func (f *FakeDocumentCounter) CountByClient(_ context.Context, clientID string) (int, error) {
	return f.Counts[clientID], nil
}

// FakeFormSubmissionCounter returns scripted submission counts per client.
type FakeFormSubmissionCounter struct {
	Counts map[string]int
}

var _ ports.FormSubmissionCounter = (*FakeFormSubmissionCounter)(nil)

func (f *FakeFormSubmissionCounter) CountByClient(_ context.Context, clientID string) (int, error) {
	return f.Counts[clientID], nil
}

// FakeSessionNoteRepository stores session notes in memory and enforces the
// one-note-per-booking uniqueness the MongoDB bookingId index enforces.
type FakeSessionNoteRepository struct {
	mu          sync.Mutex
	byID        map[string]note.SessionNote
	byBookingID map[string]string // bookingID -> note id
	next        int
}

var _ ports.SessionNoteRepository = (*FakeSessionNoteRepository)(nil)

func NewFakeSessionNoteRepository() *FakeSessionNoteRepository {
	return &FakeSessionNoteRepository{
		byID:        make(map[string]note.SessionNote),
		byBookingID: make(map[string]string),
	}
}

func (f *FakeSessionNoteRepository) Create(_ context.Context, n note.SessionNote) (note.SessionNote, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byBookingID[n.BookingID]; exists {
		return note.SessionNote{}, note.ErrNoteExists
	}
	f.next++
	n.ID = fmt.Sprintf("note-%d", f.next)
	f.byID[n.ID] = n
	f.byBookingID[n.BookingID] = n.ID
	return n, nil
}

func (f *FakeSessionNoteRepository) FindByBookingID(_ context.Context, bookingID string) (note.SessionNote, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byBookingID[bookingID]
	if !ok {
		return note.SessionNote{}, note.ErrNoteNotFound
	}
	return f.byID[id], nil
}

func (f *FakeSessionNoteRepository) Update(_ context.Context, n note.SessionNote) (note.SessionNote, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[n.ID]; !ok {
		return note.SessionNote{}, note.ErrNoteNotFound
	}
	f.byID[n.ID] = n
	return n, nil
}

// FakeNotificationJobRepository stores the delivery outbox in memory and
// mirrors the atomic claim the MongoDB adapter performs: a claimed job is
// handed to exactly one caller.
type FakeNotificationJobRepository struct {
	mu      sync.Mutex
	byID    map[string]notification.Job
	claimed map[string]bool
	order   []string
	next    int
	// CreateErr, when set, fails every Create — for proving that a
	// business action survives an unwritable outbox.
	CreateErr error
}

var _ ports.NotificationJobRepository = (*FakeNotificationJobRepository)(nil)

func NewFakeNotificationJobRepository() *FakeNotificationJobRepository {
	return &FakeNotificationJobRepository{
		byID:    make(map[string]notification.Job),
		claimed: make(map[string]bool),
	}
}

func (f *FakeNotificationJobRepository) Create(_ context.Context, job notification.Job) (notification.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateErr != nil {
		return notification.Job{}, f.CreateErr
	}
	f.next++
	job.ID = fmt.Sprintf("job-%d", f.next)
	f.byID[job.ID] = job
	f.order = append(f.order, job.ID)
	return job, nil
}

func (f *FakeNotificationJobRepository) Update(_ context.Context, job notification.Job) (notification.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[job.ID]; !ok {
		return notification.Job{}, notification.ErrJobNotFound
	}
	f.byID[job.ID] = job
	delete(f.claimed, job.ID)
	return job, nil
}

func (f *FakeNotificationJobRepository) ClaimDue(_ context.Context, now time.Time, limit int) ([]notification.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var due []notification.Job
	for _, id := range f.order {
		job := f.byID[id]
		if f.claimed[id] || !job.Due(now) {
			continue
		}
		due = append(due, job)
	}
	notification.SortByDue(due)
	if limit > 0 && len(due) > limit {
		due = due[:limit]
	}
	for _, job := range due {
		f.claimed[job.ID] = true
	}
	return due, nil
}

func (f *FakeNotificationJobRepository) PendingByBooking(_ context.Context, bookingID string, kind notification.Kind) ([]notification.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []notification.Job
	for _, id := range f.order {
		job := f.byID[id]
		if job.BookingID == bookingID && job.Kind == kind && job.Status == notification.StatusPending {
			out = append(out, job)
		}
	}
	return out, nil
}

// All returns every job in insertion order, for assertions.
func (f *FakeNotificationJobRepository) All() []notification.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notification.Job, 0, len(f.order))
	for _, id := range f.order {
		out = append(out, f.byID[id])
	}
	return out
}

// OfKind returns every job of one kind, in insertion order.
func (f *FakeNotificationJobRepository) OfKind(kind notification.Kind) []notification.Job {
	var out []notification.Job
	for _, job := range f.All() {
		if job.Kind == kind {
			out = append(out, job)
		}
	}
	return out
}

// FakeMailer records what was sent and can be scripted to fail.
type FakeMailer struct {
	mu   sync.Mutex
	sent []ports.EmailMessage
	// Err, when set, fails every send.
	Err error
	// FailFor fails only messages to these recipients, so a test can prove
	// one bad address does not block the batch behind it.
	FailFor map[string]error
}

var _ ports.Mailer = (*FakeMailer)(nil)

func NewFakeMailer() *FakeMailer { return &FakeMailer{FailFor: map[string]error{}} }

func (f *FakeMailer) Send(_ context.Context, msg ports.EmailMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	if err, ok := f.FailFor[msg.To]; ok {
		return err
	}
	f.sent = append(f.sent, msg)
	return nil
}

// Sent returns the messages delivered so far.
func (f *FakeMailer) Sent() []ports.EmailMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ports.EmailMessage(nil), f.sent...)
}

// FakeEmailRenderer produces a trivially identifiable message per job and
// can be scripted to fail for one kind.
type FakeEmailRenderer struct {
	// FailKind renders as an error, standing in for a missing template.
	FailKind notification.Kind
}

var _ ports.EmailRenderer = (*FakeEmailRenderer)(nil)

func (f *FakeEmailRenderer) Render(job notification.Job) (ports.EmailMessage, error) {
	if f.FailKind != "" && job.Kind == f.FailKind {
		return ports.EmailMessage{}, notification.ErrTemplateNotFound
	}
	return ports.EmailMessage{
		To:      job.Recipient,
		Subject: string(job.Kind),
		HTML:    "<p>" + string(job.Kind) + "</p>",
		Text:    string(job.Kind),
	}, nil
}

// FakeNotifier records what the other slices announced, so their tests can
// assert on the notice without a mail provider anywhere in sight.
type FakeNotifier struct {
	mu          sync.Mutex
	Confirmed   []ports.BookingNotice
	Rescheduled []ports.BookingNotice
	Cancelled   []ports.BookingNotice
	Feedback    []ports.FeedbackNotice
	Enquiries   []ports.EnquiryNotice
}

var _ ports.Notifier = (*FakeNotifier)(nil)

func NewFakeNotifier() *FakeNotifier { return &FakeNotifier{} }

func (f *FakeNotifier) BookingConfirmed(_ context.Context, notice ports.BookingNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Confirmed = append(f.Confirmed, notice)
}

func (f *FakeNotifier) BookingRescheduled(_ context.Context, notice ports.BookingNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Rescheduled = append(f.Rescheduled, notice)
}

func (f *FakeNotifier) BookingCancelled(_ context.Context, notice ports.BookingNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Cancelled = append(f.Cancelled, notice)
}

func (f *FakeNotifier) FeedbackShared(_ context.Context, notice ports.FeedbackNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Feedback = append(f.Feedback, notice)
}

func (f *FakeNotifier) EnquiryReceived(_ context.Context, notice ports.EnquiryNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Enquiries = append(f.Enquiries, notice)
}

// FakePageRepository stores site pages in memory and enforces the unique
// slug the MongoDB index enforces.
type FakePageRepository struct {
	mu    sync.Mutex
	byID  map[string]cms.Page
	order []string
	next  int
}

var _ ports.PageRepository = (*FakePageRepository)(nil)

func NewFakePageRepository() *FakePageRepository {
	return &FakePageRepository{byID: make(map[string]cms.Page)}
}

func (f *FakePageRepository) slugTaken(slug, exceptID string) bool {
	for id, page := range f.byID {
		if id != exceptID && page.Slug == slug {
			return true
		}
	}
	return false
}

func (f *FakePageRepository) Create(_ context.Context, page cms.Page) (cms.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.slugTaken(page.Slug, "") {
		return cms.Page{}, cms.ErrSlugTaken
	}
	f.next++
	page.ID = fmt.Sprintf("page-%d", f.next)
	f.byID[page.ID] = page
	f.order = append(f.order, page.ID)
	return page, nil
}

func (f *FakePageRepository) Update(_ context.Context, page cms.Page) (cms.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[page.ID]; !ok {
		return cms.Page{}, cms.ErrPageNotFound
	}
	if f.slugTaken(page.Slug, page.ID) {
		return cms.Page{}, cms.ErrSlugTaken
	}
	f.byID[page.ID] = page
	return page, nil
}

func (f *FakePageRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakePageRepository) FindByID(_ context.Context, id string) (cms.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	page, ok := f.byID[id]
	if !ok {
		return cms.Page{}, cms.ErrPageNotFound
	}
	return page, nil
}

func (f *FakePageRepository) FindBySlug(_ context.Context, slug string, publishedOnly bool) (cms.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		page, ok := f.byID[id]
		if !ok || page.Slug != slug {
			continue
		}
		if publishedOnly && page.Status != cms.StatusPublished {
			return cms.Page{}, cms.ErrPageNotFound
		}
		return page, nil
	}
	return cms.Page{}, cms.ErrPageNotFound
}

func (f *FakePageRepository) List(_ context.Context, filter ports.ContentFilter) ([]cms.Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]cms.Page, 0, len(f.order))
	for _, id := range f.order {
		page, ok := f.byID[id]
		if !ok {
			continue
		}
		if filter.PublishedOnly && page.Status != cms.StatusPublished {
			continue
		}
		out = append(out, page)
	}
	return out, nil
}

// FakePostRepository stores blog posts in memory, newest-published first.
type FakePostRepository struct {
	mu    sync.Mutex
	byID  map[string]cms.Post
	order []string
	next  int
}

var _ ports.PostRepository = (*FakePostRepository)(nil)

func NewFakePostRepository() *FakePostRepository {
	return &FakePostRepository{byID: make(map[string]cms.Post)}
}

func (f *FakePostRepository) slugTaken(slug, exceptID string) bool {
	for id, post := range f.byID {
		if id != exceptID && post.Slug == slug {
			return true
		}
	}
	return false
}

func (f *FakePostRepository) Create(_ context.Context, post cms.Post) (cms.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.slugTaken(post.Slug, "") {
		return cms.Post{}, cms.ErrSlugTaken
	}
	f.next++
	post.ID = fmt.Sprintf("post-%d", f.next)
	f.byID[post.ID] = post
	f.order = append(f.order, post.ID)
	return post, nil
}

func (f *FakePostRepository) Update(_ context.Context, post cms.Post) (cms.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[post.ID]; !ok {
		return cms.Post{}, cms.ErrPostNotFound
	}
	if f.slugTaken(post.Slug, post.ID) {
		return cms.Post{}, cms.ErrSlugTaken
	}
	f.byID[post.ID] = post
	return post, nil
}

func (f *FakePostRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakePostRepository) FindByID(_ context.Context, id string) (cms.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	post, ok := f.byID[id]
	if !ok {
		return cms.Post{}, cms.ErrPostNotFound
	}
	return post, nil
}

func (f *FakePostRepository) FindBySlug(_ context.Context, slug string, publishedOnly bool) (cms.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		post, ok := f.byID[id]
		if !ok || post.Slug != slug {
			continue
		}
		if publishedOnly && post.Status != cms.StatusPublished {
			return cms.Post{}, cms.ErrPostNotFound
		}
		return post, nil
	}
	return cms.Post{}, cms.ErrPostNotFound
}

func (f *FakePostRepository) List(_ context.Context, filter ports.ContentFilter) ([]cms.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []cms.Post
	for _, id := range f.order {
		post, ok := f.byID[id]
		if !ok {
			continue
		}
		if filter.PublishedOnly && post.Status != cms.StatusPublished {
			continue
		}
		if filter.Category != "" && post.Category != filter.Category {
			continue
		}
		if filter.Tag != "" && !containsTag(post.Tags, filter.Tag) {
			continue
		}
		out = append(out, post)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return postOrderKey(out[i]).After(postOrderKey(out[j]))
	})
	return out, nil
}

func containsTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

// postOrderKey is publishedAt when live, else createdAt — the same
// newest-first ordering the MongoDB adapter produces.
func postOrderKey(p cms.Post) time.Time {
	if p.PublishedAt != nil {
		return *p.PublishedAt
	}
	return p.CreatedAt
}

// FakeFAQRepository stores FAQ entries in memory, sortOrder first.
type FakeFAQRepository struct {
	mu    sync.Mutex
	byID  map[string]cms.FAQ
	order []string
	next  int
}

var _ ports.FAQRepository = (*FakeFAQRepository)(nil)

func NewFakeFAQRepository() *FakeFAQRepository {
	return &FakeFAQRepository{byID: make(map[string]cms.FAQ)}
}

func (f *FakeFAQRepository) Create(_ context.Context, faq cms.FAQ) (cms.FAQ, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	faq.ID = fmt.Sprintf("faq-%d", f.next)
	f.byID[faq.ID] = faq
	f.order = append(f.order, faq.ID)
	return faq, nil
}

func (f *FakeFAQRepository) Update(_ context.Context, faq cms.FAQ) (cms.FAQ, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[faq.ID]; !ok {
		return cms.FAQ{}, cms.ErrFAQNotFound
	}
	f.byID[faq.ID] = faq
	return faq, nil
}

func (f *FakeFAQRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeFAQRepository) FindByID(_ context.Context, id string) (cms.FAQ, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	faq, ok := f.byID[id]
	if !ok {
		return cms.FAQ{}, cms.ErrFAQNotFound
	}
	return faq, nil
}

func (f *FakeFAQRepository) List(_ context.Context, filter ports.ContentFilter) ([]cms.FAQ, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []cms.FAQ
	for _, id := range f.order {
		faq, ok := f.byID[id]
		if !ok {
			continue
		}
		if filter.PublishedOnly && !faq.Active {
			continue
		}
		if filter.Category != "" && faq.Category != filter.Category {
			continue
		}
		out = append(out, faq)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

// FakeTestimonialRepository stores testimonials in memory.
type FakeTestimonialRepository struct {
	mu    sync.Mutex
	byID  map[string]cms.Testimonial
	order []string
	next  int
}

var _ ports.TestimonialRepository = (*FakeTestimonialRepository)(nil)

func NewFakeTestimonialRepository() *FakeTestimonialRepository {
	return &FakeTestimonialRepository{byID: make(map[string]cms.Testimonial)}
}

func (f *FakeTestimonialRepository) Create(_ context.Context, t cms.Testimonial) (cms.Testimonial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	t.ID = fmt.Sprintf("testimonial-%d", f.next)
	f.byID[t.ID] = t
	f.order = append(f.order, t.ID)
	return t, nil
}

func (f *FakeTestimonialRepository) Update(_ context.Context, t cms.Testimonial) (cms.Testimonial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[t.ID]; !ok {
		return cms.Testimonial{}, cms.ErrTestimonialNotFound
	}
	f.byID[t.ID] = t
	return t, nil
}

func (f *FakeTestimonialRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeTestimonialRepository) FindByID(_ context.Context, id string) (cms.Testimonial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.byID[id]
	if !ok {
		return cms.Testimonial{}, cms.ErrTestimonialNotFound
	}
	return t, nil
}

func (f *FakeTestimonialRepository) List(_ context.Context, filter ports.ContentFilter, status cms.Moderation) ([]cms.Testimonial, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []cms.Testimonial
	for _, id := range f.order {
		t, ok := f.byID[id]
		if !ok {
			continue
		}
		if status != "" && t.Status != status {
			continue
		}
		if filter.PublishedOnly && t.Status != cms.ModerationApproved {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

// FakeEnquiryRepository stores contact-form enquiries in memory,
// newest-first like the MongoDB adapter.
type FakeEnquiryRepository struct {
	mu    sync.Mutex
	byID  map[string]enquiry.Enquiry
	order []string
	next  int
}

var _ ports.EnquiryRepository = (*FakeEnquiryRepository)(nil)

func NewFakeEnquiryRepository() *FakeEnquiryRepository {
	return &FakeEnquiryRepository{byID: make(map[string]enquiry.Enquiry)}
}

func (f *FakeEnquiryRepository) Create(_ context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	e.ID = fmt.Sprintf("enquiry-%d", f.next)
	f.byID[e.ID] = e
	f.order = append(f.order, e.ID)
	return e, nil
}

func (f *FakeEnquiryRepository) Update(_ context.Context, e enquiry.Enquiry) (enquiry.Enquiry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[e.ID]; !ok {
		return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
	}
	f.byID[e.ID] = e
	return e, nil
}

func (f *FakeEnquiryRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeEnquiryRepository) FindByID(_ context.Context, id string) (enquiry.Enquiry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.byID[id]
	if !ok {
		return enquiry.Enquiry{}, enquiry.ErrEnquiryNotFound
	}
	return e, nil
}

func (f *FakeEnquiryRepository) List(_ context.Context, filter ports.EnquiryFilter) ([]enquiry.Enquiry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []enquiry.Enquiry
	for i := len(f.order) - 1; i >= 0; i-- {
		e, ok := f.byID[f.order[i]]
		if !ok {
			continue
		}
		if filter.Status != "" && e.Status != filter.Status {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *FakeEnquiryRepository) CountByStatus(_ context.Context, status enquiry.Status) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, e := range f.byID {
		if e.Status == status {
			count++
		}
	}
	return count, nil
}

// FakeReviewRepository stores post-session reviews in memory and enforces
// the one-review-per-booking uniqueness the MongoDB index enforces.
type FakeReviewRepository struct {
	mu          sync.Mutex
	byID        map[string]review.Review
	byBookingID map[string]string
	order       []string
	next        int
}

var _ ports.ReviewRepository = (*FakeReviewRepository)(nil)

func NewFakeReviewRepository() *FakeReviewRepository {
	return &FakeReviewRepository{
		byID:        make(map[string]review.Review),
		byBookingID: make(map[string]string),
	}
}

func (f *FakeReviewRepository) Create(_ context.Context, r review.Review) (review.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.byBookingID[r.BookingID]; exists {
		return review.Review{}, review.ErrReviewExists
	}
	f.next++
	r.ID = fmt.Sprintf("review-%d", f.next)
	f.byID[r.ID] = r
	f.byBookingID[r.BookingID] = r.ID
	f.order = append(f.order, r.ID)
	return r, nil
}

func (f *FakeReviewRepository) Update(_ context.Context, r review.Review) (review.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[r.ID]; !ok {
		return review.Review{}, review.ErrReviewNotFound
	}
	f.byID[r.ID] = r
	return r, nil
}

func (f *FakeReviewRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.byID[id]; ok {
		delete(f.byBookingID, r.BookingID)
	}
	delete(f.byID, id)
	return nil
}

func (f *FakeReviewRepository) FindByID(_ context.Context, id string) (review.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.byID[id]
	if !ok {
		return review.Review{}, review.ErrReviewNotFound
	}
	return r, nil
}

func (f *FakeReviewRepository) FindByBookingID(_ context.Context, bookingID string) (review.Review, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.byBookingID[bookingID]
	if !ok {
		return review.Review{}, review.ErrReviewNotFound
	}
	return f.byID[id], nil
}

// selectReviews returns matching reviews newest-first.
func (f *FakeReviewRepository) selectReviews(match func(review.Review) bool, limit int) []review.Review {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []review.Review
	for i := len(f.order) - 1; i >= 0; i-- {
		r, ok := f.byID[f.order[i]]
		if !ok || !match(r) {
			continue
		}
		out = append(out, r)
		if limit > 0 && len(out) == limit {
			break
		}
	}
	return out
}

func (f *FakeReviewRepository) ListByClient(_ context.Context, clientID string) ([]review.Review, error) {
	return f.selectReviews(func(r review.Review) bool { return r.ClientID == clientID }, 0), nil
}

func (f *FakeReviewRepository) ListByPractitioner(_ context.Context, practitionerID string, filter ports.ReviewFilter) ([]review.Review, error) {
	return f.selectReviews(func(r review.Review) bool {
		if r.PractitionerID != practitionerID {
			return false
		}
		if filter.ApprovedOnly {
			return r.Status == review.StatusApproved
		}
		return filter.Status == "" || r.Status == filter.Status
	}, 0), nil
}

func (f *FakeReviewRepository) ListPublic(_ context.Context, limit int) ([]review.Review, error) {
	return f.selectReviews(func(r review.Review) bool { return r.Status == review.StatusApproved }, limit), nil
}

// FakeFormRepository stores form definitions in memory.
type FakeFormRepository struct {
	mu    sync.Mutex
	byID  map[string]form.Form
	order []string
	next  int
}

var _ ports.FormRepository = (*FakeFormRepository)(nil)

func NewFakeFormRepository() *FakeFormRepository {
	return &FakeFormRepository{byID: make(map[string]form.Form)}
}

func (f *FakeFormRepository) Create(_ context.Context, def form.Form) (form.Form, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	def.ID = fmt.Sprintf("form-%d", f.next)
	f.byID[def.ID] = def
	f.order = append(f.order, def.ID)
	return def, nil
}

func (f *FakeFormRepository) Update(_ context.Context, def form.Form) (form.Form, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[def.ID]; !ok {
		return form.Form{}, form.ErrFormNotFound
	}
	f.byID[def.ID] = def
	return def, nil
}

func (f *FakeFormRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeFormRepository) FindByID(_ context.Context, id string) (form.Form, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	def, ok := f.byID[id]
	if !ok {
		return form.Form{}, form.ErrFormNotFound
	}
	return def, nil
}

func (f *FakeFormRepository) List(_ context.Context, activeOnly bool) ([]form.Form, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]form.Form, 0, len(f.order))
	for _, id := range f.order {
		def, ok := f.byID[id]
		if !ok || (activeOnly && !def.Active) {
			continue
		}
		out = append(out, def)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out, nil
}

// FakeFormSubmissionRepository stores filled-in forms in memory,
// newest-first like the MongoDB adapter.
type FakeFormSubmissionRepository struct {
	mu    sync.Mutex
	byID  map[string]form.Submission
	order []string
	next  int
}

var _ ports.FormSubmissionRepository = (*FakeFormSubmissionRepository)(nil)

func NewFakeFormSubmissionRepository() *FakeFormSubmissionRepository {
	return &FakeFormSubmissionRepository{byID: make(map[string]form.Submission)}
}

func (f *FakeFormSubmissionRepository) Create(_ context.Context, s form.Submission) (form.Submission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	s.ID = fmt.Sprintf("submission-%d", f.next)
	f.byID[s.ID] = s
	f.order = append(f.order, s.ID)
	return s, nil
}

func (f *FakeFormSubmissionRepository) Update(_ context.Context, s form.Submission) (form.Submission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[s.ID]; !ok {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	f.byID[s.ID] = s
	return s, nil
}

func (f *FakeFormSubmissionRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeFormSubmissionRepository) FindByID(_ context.Context, id string) (form.Submission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[id]
	if !ok {
		return form.Submission{}, form.ErrSubmissionNotFound
	}
	return s, nil
}

func (f *FakeFormSubmissionRepository) List(_ context.Context, filter ports.SubmissionFilter) ([]form.Submission, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []form.Submission
	for i := len(f.order) - 1; i >= 0; i-- {
		s, ok := f.byID[f.order[i]]
		if !ok {
			continue
		}
		if filter.ClientID != "" && s.ClientID != filter.ClientID {
			continue
		}
		if filter.FormID != "" && s.FormID != filter.FormID {
			continue
		}
		if filter.BookingID != "" && s.BookingID != filter.BookingID {
			continue
		}
		if filter.Status != "" && s.Status != filter.Status {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (f *FakeFormSubmissionRepository) HasOpenAssignment(ctx context.Context, clientID, formID string) (bool, error) {
	open, err := f.List(ctx, ports.SubmissionFilter{
		ClientID: clientID,
		FormID:   formID,
		Status:   form.StatusAssigned,
	})
	if err != nil {
		return false, err
	}
	return len(open) > 0, nil
}

// FakeDocumentRepository stores document records in memory, newest-first
// per client like the MongoDB adapter.
type FakeDocumentRepository struct {
	mu    sync.Mutex
	byID  map[string]document.Document
	order []string
	next  int
}

var _ ports.DocumentRepository = (*FakeDocumentRepository)(nil)

func NewFakeDocumentRepository() *FakeDocumentRepository {
	return &FakeDocumentRepository{byID: make(map[string]document.Document)}
}

func (f *FakeDocumentRepository) Create(_ context.Context, d document.Document) (document.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	d.ID = fmt.Sprintf("document-%d", f.next)
	f.byID[d.ID] = d
	f.order = append(f.order, d.ID)
	return d, nil
}

func (f *FakeDocumentRepository) Update(_ context.Context, d document.Document) (document.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	existing, ok := f.byID[d.ID]
	if !ok {
		return document.Document{}, document.ErrDocumentNotFound
	}
	// Mirror the adapter: only the editable fields are persisted, so a
	// test cannot accidentally prove that re-pointing an asset works.
	existing.Title = d.Title
	existing.VisibleToClient = d.VisibleToClient
	existing.UpdatedAt = d.UpdatedAt
	f.byID[d.ID] = existing
	return existing, nil
}

func (f *FakeDocumentRepository) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *FakeDocumentRepository) FindByID(_ context.Context, id string) (document.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d, ok := f.byID[id]
	if !ok {
		return document.Document{}, document.ErrDocumentNotFound
	}
	return d, nil
}

func (f *FakeDocumentRepository) ListByClient(_ context.Context, clientID string) ([]document.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []document.Document
	for i := len(f.order) - 1; i >= 0; i-- {
		d, ok := f.byID[f.order[i]]
		if ok && d.ClientID == clientID {
			out = append(out, d)
		}
	}
	return out, nil
}

// FakeMediaStore records what was signed and deleted, without any provider.
type FakeMediaStore struct {
	mu        sync.Mutex
	Uploads   []ports.UploadParams
	Signed    []ports.Asset
	Deleted   []ports.Asset
	SignErr   error
	DeleteErr error
}

var _ ports.MediaStore = (*FakeMediaStore)(nil)

func NewFakeMediaStore() *FakeMediaStore { return &FakeMediaStore{} }

func (f *FakeMediaStore) SignUpload(_ context.Context, params ports.UploadParams) (ports.SignedUpload, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SignErr != nil {
		return ports.SignedUpload{}, f.SignErr
	}
	f.Uploads = append(f.Uploads, params)
	return ports.SignedUpload{
		URL: "https://upload.test/" + string(params.ResourceType),
		Fields: map[string]string{
			"folder":    params.Folder,
			"signature": "fake-signature",
		},
		Signature: "fake-signature",
		ExpiresAt: time.Now().UTC().Add(ports.UploadSignatureTTL),
	}, nil
}

func (f *FakeMediaStore) SignedURL(_ context.Context, asset ports.Asset, ttl time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SignErr != nil {
		return "", f.SignErr
	}
	f.Signed = append(f.Signed, asset)
	return fmt.Sprintf("https://delivery.test/%s?expires=%d", asset.PublicID, int(ttl.Seconds())), nil
}

func (f *FakeMediaStore) Delete(_ context.Context, asset ports.Asset) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	f.Deleted = append(f.Deleted, asset)
	return nil
}
