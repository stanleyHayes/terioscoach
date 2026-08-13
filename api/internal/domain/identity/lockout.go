package identity

import "time"

// Lockout defaults. Six wrong passwords in a rolling fifteen minutes locks
// the identifier for fifteen minutes — enough to stop online guessing
// without punishing a person who mistyped their password a few times.
const (
	DefaultMaxLoginAttempts = 6
	DefaultAttemptWindow    = 15 * time.Minute
	DefaultLockoutCooldown  = 15 * time.Minute
)

// LoginAttempts is the recorded failure history for one login identifier.
// The identifier is the normalized email — never the account id — because
// failures are recorded for unknown emails too (see LockoutPolicy).
type LoginAttempts struct {
	Identifier string
	Count      int
	// FirstAt starts the rolling window; a failure arriving after the
	// window has elapsed starts a fresh one.
	FirstAt time.Time
	// LastAt is when the most recent failure landed. The cooldown is
	// measured from it, so each new attempt while locked extends the lock.
	LastAt time.Time
}

// LockoutPolicy is the brute-force rule: how many failures inside a rolling
// window trip a lock, and how long the lock then holds.
//
// The policy is deliberately keyed on the submitted email whether or not an
// account exists. Locking only real accounts would turn the lockout response
// into an account-existence oracle — exactly the enumeration leak the
// uniform ErrInvalidCredentials answer exists to prevent.
type LockoutPolicy struct {
	MaxAttempts int
	Window      time.Duration
	Cooldown    time.Duration
}

// DefaultLockoutPolicy is the platform default.
func DefaultLockoutPolicy() LockoutPolicy {
	return LockoutPolicy{
		MaxAttempts: DefaultMaxLoginAttempts,
		Window:      DefaultAttemptWindow,
		Cooldown:    DefaultLockoutCooldown,
	}
}

// normalized fills in zero/negative fields with the platform defaults so a
// partially configured policy can never disable the protection.
func (p LockoutPolicy) normalized() LockoutPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = DefaultMaxLoginAttempts
	}
	if p.Window <= 0 {
		p.Window = DefaultAttemptWindow
	}
	if p.Cooldown <= 0 {
		p.Cooldown = DefaultLockoutCooldown
	}
	return p
}

// Locked reports whether the identifier is currently locked out, and for how
// much longer. An attempt history whose window has already elapsed is stale
// and never locks: the count belongs to a window that has closed.
func (p LockoutPolicy) Locked(attempts LoginAttempts, now time.Time) (bool, time.Duration) {
	p = p.normalized()
	if attempts.Count < p.MaxAttempts {
		return false, 0
	}
	if now.Sub(attempts.FirstAt) > p.Window {
		return false, 0
	}
	retryAfter := attempts.LastAt.Add(p.Cooldown).Sub(now)
	if retryAfter <= 0 {
		return false, 0
	}
	return true, retryAfter
}

// Record folds one new failure into the history, rolling the window over
// when the previous one has closed. It returns the updated history — the
// caller persists it through the LoginAttemptStore port.
func (p LockoutPolicy) Record(attempts LoginAttempts, now time.Time) LoginAttempts {
	p = p.normalized()
	if attempts.Count == 0 || now.Sub(attempts.FirstAt) > p.Window {
		return LoginAttempts{
			Identifier: attempts.Identifier,
			Count:      1,
			FirstAt:    now,
			LastAt:     now,
		}
	}
	attempts.Count++
	attempts.LastAt = now
	return attempts
}

// RetentionAfter is how long a failure record stays useful: the window plus
// the cooldown. Storage adapters expire records on this so the collection
// never grows without bound.
func (p LockoutPolicy) RetentionAfter() time.Duration {
	p = p.normalized()
	return p.Window + p.Cooldown
}
