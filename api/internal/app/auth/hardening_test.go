package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
	"github.com/xcreativs/terios/api/internal/ports"
	"github.com/xcreativs/terios/api/internal/ports/portstest"
)

var hardeningNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

// hardenedRig is a service with brute-force lockout wired and a clock the
// test drives, so cooldowns are proven without sleeping.
type hardenedRig struct {
	svc      *Service
	sessions *portstest.FakeRefreshTokenRepository
	attempts *portstest.FakeLoginAttemptStore
	policy   identity.LockoutPolicy
	clock    *time.Time
}

func newHardenedRig() hardenedRig {
	clock := hardeningNow
	attempts := portstest.NewFakeLoginAttemptStore()
	attempts.Now = func() time.Time { return clock }
	sessions := portstest.NewFakeRefreshTokenRepository()
	policy := identity.LockoutPolicy{MaxAttempts: 3, Window: 10 * time.Minute, Cooldown: 5 * time.Minute}

	svc := NewService(
		portstest.NewFakeUserRepository(),
		sessions,
		portstest.FakeHasher{},
		portstest.NewFakeTokenIssuer(15*time.Minute),
		testRefreshTTL,
		WithLockout(attempts, policy),
	)
	svc.now = func() time.Time { return clock }

	return hardenedRig{svc: svc, sessions: sessions, attempts: attempts, policy: policy, clock: &clock}
}

func (r hardenedRig) advance(d time.Duration) { *r.clock = r.clock.Add(d) }

// failLogin attempts a login with the wrong password and returns the error.
func failLogin(rig hardenedRig, email string) error {
	_, err := rig.svc.Login(context.Background(), email, "definitely-the-wrong-password")
	return err
}

// retryAfter extracts the cooldown a lockout error carries.
func retryAfter(t *testing.T, err error) time.Duration {
	t.Helper()
	var retry *identity.RetryAfterError
	if !errors.As(err, &retry) {
		t.Fatalf("err = %v, want a RetryAfterError carrying the cooldown", err)
	}
	return retry.RetryAfter
}

// TestLockoutTripsAndReleases: repeated failures lock the identifier, the
// lock reports its cooldown, and it lifts once the cooldown elapses.
func TestLockoutTripsAndReleases(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "ama@example.com", "a long enough password")

	for i := 1; i < rig.policy.MaxAttempts; i++ {
		if err := failLogin(rig, "ama@example.com"); !errors.Is(err, identity.ErrInvalidCredentials) {
			t.Fatalf("attempt %d err = %v, want ErrInvalidCredentials", i, err)
		}
	}

	err := failLogin(rig, "ama@example.com")
	if !errors.Is(err, identity.ErrTooManyAttempts) {
		t.Fatalf("attempt %d err = %v, want ErrTooManyAttempts", rig.policy.MaxAttempts, err)
	}
	if got := retryAfter(t, err); got != rig.policy.Cooldown {
		t.Errorf("retryAfter = %v, want %v", got, rig.policy.Cooldown)
	}

	// While locked, even the *correct* password is refused — that is the
	// whole point of a lockout.
	if _, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password"); !errors.Is(err, identity.ErrTooManyAttempts) {
		t.Errorf("correct password while locked = %v, want ErrTooManyAttempts", err)
	}

	rig.advance(rig.policy.Cooldown)
	if _, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password"); err != nil {
		t.Fatalf("login after cooldown: %v", err)
	}
}

// TestSuccessfulLoginClearsAttempts: a person who mistypes then gets it
// right is not left one attempt away from a lock.
func TestSuccessfulLoginClearsAttempts(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "ama@example.com", "a long enough password")

	if err := failLogin(rig, "ama@example.com"); !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Fatalf("first failure = %v", err)
	}
	if _, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password"); err != nil {
		t.Fatalf("login: %v", err)
	}

	history, err := rig.attempts.Get(context.Background(), "ama@example.com")
	if err != nil {
		t.Fatalf("read attempts: %v", err)
	}
	if history.Count != 0 {
		t.Errorf("count = %d after a successful login, want the history cleared", history.Count)
	}
}

// TestLockoutIsNotAnEnumerationOracle: an email with no account behaves
// exactly like one with an account — same errors, same lock, same timing
// signal. Otherwise the 429 would answer "does this address exist?".
func TestLockoutIsNotAnEnumerationOracle(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "real@example.com", "a long enough password")

	var realErrs, unknownErrs []error
	for i := 0; i < rig.policy.MaxAttempts; i++ {
		realErrs = append(realErrs, failLogin(rig, "real@example.com"))
		unknownErrs = append(unknownErrs, failLogin(rig, "ghost@example.com"))
	}

	for i := range realErrs {
		realLocked := errors.Is(realErrs[i], identity.ErrTooManyAttempts)
		unknownLocked := errors.Is(unknownErrs[i], identity.ErrTooManyAttempts)
		if realLocked != unknownLocked {
			t.Fatalf("attempt %d: real locked=%v, unknown locked=%v — the responses diverge and leak account existence",
				i+1, realLocked, unknownLocked)
		}
		if !realLocked && !errors.Is(unknownErrs[i], identity.ErrInvalidCredentials) {
			t.Errorf("attempt %d unknown err = %v, want ErrInvalidCredentials", i+1, unknownErrs[i])
		}
	}
	if !errors.Is(unknownErrs[len(unknownErrs)-1], identity.ErrTooManyAttempts) {
		t.Error("an unknown email never locked, want the same lock as a real account")
	}
}

// TestLockoutIsPerIdentifier: locking one account does not lock the rest of
// the practice out.
func TestLockoutIsPerIdentifier(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "ama@example.com", "a long enough password")
	registerClient(t, rig.svc, "koffi@example.com", "another long password")

	for i := 0; i < rig.policy.MaxAttempts; i++ {
		failLogin(rig, "ama@example.com")
	}

	if _, err := rig.svc.Login(context.Background(), "koffi@example.com", "another long password"); err != nil {
		t.Fatalf("second account login = %v, want the lockout scoped to the first identifier", err)
	}
}

// TestLockoutNormalizesIdentifier: casing and surrounding space must not
// buy an attacker a fresh set of attempts.
func TestLockoutNormalizesIdentifier(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "ama@example.com", "a long enough password")

	for i := 1; i < rig.policy.MaxAttempts; i++ {
		failLogin(rig, "ama@example.com")
	}
	err := failLogin(rig, "  AMA@Example.COM  ")
	if !errors.Is(err, identity.ErrTooManyAttempts) {
		t.Errorf("err = %v, want the differently-cased address to share the counter", err)
	}
}

// TestRefreshReuseRevokesEverySession: replaying a rotated refresh token
// means it leaked, so every session of that account dies at once.
func TestRefreshReuseRevokesEverySession(t *testing.T) {
	rig := newHardenedRig()
	first := registerClient(t, rig.svc, "ama@example.com", "a long enough password")

	// A second device: an independent, live session for the same account.
	second, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password")
	if err != nil {
		t.Fatalf("second session login: %v", err)
	}

	rotated, err := rig.svc.Refresh(context.Background(), first.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The attacker replays the token the victim already spent.
	if _, err := rig.svc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("reuse err = %v, want ErrTokenInvalid", err)
	}

	// Both the rotated token and the untouched second session are now dead.
	for name, token := range map[string]string{
		"rotated token":  rotated.RefreshToken,
		"second session": second.RefreshToken,
	} {
		if _, err := rig.svc.Refresh(context.Background(), token); !errors.Is(err, identity.ErrTokenInvalid) {
			t.Errorf("%s after reuse detection = %v, want every session revoked", name, err)
		}
	}
}

// TestRefreshReuseLeavesOtherAccountsAlone: the blast radius of reuse
// detection is exactly one account.
func TestRefreshReuseLeavesOtherAccountsAlone(t *testing.T) {
	rig := newHardenedRig()
	victim := registerClient(t, rig.svc, "ama@example.com", "a long enough password")
	bystander := registerClient(t, rig.svc, "koffi@example.com", "another long password")

	if _, err := rig.svc.Refresh(context.Background(), victim.RefreshToken); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, err := rig.svc.Refresh(context.Background(), victim.RefreshToken); !errors.Is(err, identity.ErrTokenInvalid) {
		t.Fatalf("reuse err = %v, want ErrTokenInvalid", err)
	}

	if _, err := rig.svc.Refresh(context.Background(), bystander.RefreshToken); err != nil {
		t.Errorf("bystander refresh = %v, want an unaffected session", err)
	}
}

// TestLogoutThenReuseIsNotAFamilyKill would be wrong to assert: a logged-out
// token that comes back *is* indistinguishable from a stolen one, so the
// family kill is correct there too. What must hold is that logout itself
// leaves the account's other sessions alive.
func TestLogoutLeavesOtherSessionsAlive(t *testing.T) {
	rig := newHardenedRig()
	phone := registerClient(t, rig.svc, "ama@example.com", "a long enough password")
	laptop, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password")
	if err != nil {
		t.Fatalf("second session: %v", err)
	}

	if err := rig.svc.Logout(context.Background(), phone.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := rig.svc.Refresh(context.Background(), laptop.RefreshToken); err != nil {
		t.Errorf("other session after logout = %v, want it still usable", err)
	}
}

// TestLockoutStoreFailuresSurface: a degraded attempt store must fail the
// login rather than silently disable the protection.
func TestLockoutStoreFailuresSurface(t *testing.T) {
	rig := newHardenedRig()
	registerClient(t, rig.svc, "ama@example.com", "a long enough password")
	rig.svc.attempts = failingAttemptStore{}

	_, err := rig.svc.Login(context.Background(), "ama@example.com", "a long enough password")
	if err == nil {
		t.Fatal("login succeeded with a broken attempt store, want the failure surfaced")
	}
	if errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("err = %v, want a store failure, not a credentials answer", err)
	}
}

// failingAttemptStore reports a broken backing store on every call.
type failingAttemptStore struct{}

var _ ports.LoginAttemptStore = failingAttemptStore{}

var errStoreDown = errors.New("attempt store unavailable")

func (failingAttemptStore) Get(context.Context, string) (identity.LoginAttempts, error) {
	return identity.LoginAttempts{}, errStoreDown
}

func (failingAttemptStore) Save(context.Context, identity.LoginAttempts, time.Duration) error {
	return errStoreDown
}

func (failingAttemptStore) Reset(context.Context, string) error { return errStoreDown }
