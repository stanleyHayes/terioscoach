package identity

import (
	"testing"
	"time"
)

var lockoutNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func policy() LockoutPolicy {
	return LockoutPolicy{MaxAttempts: 3, Window: 10 * time.Minute, Cooldown: 5 * time.Minute}
}

// TestRecordCountsWithinWindow: consecutive failures accumulate and the
// window's start stays pinned to the first one.
func TestRecordCountsWithinWindow(t *testing.T) {
	p := policy()
	attempts := LoginAttempts{Identifier: "ama@example.com"}

	attempts = p.Record(attempts, lockoutNow)
	if attempts.Count != 1 || !attempts.FirstAt.Equal(lockoutNow) {
		t.Fatalf("first record = %+v, want count 1 opening the window", attempts)
	}

	later := lockoutNow.Add(2 * time.Minute)
	attempts = p.Record(attempts, later)
	if attempts.Count != 2 {
		t.Errorf("count = %d, want 2", attempts.Count)
	}
	if !attempts.FirstAt.Equal(lockoutNow) {
		t.Errorf("firstAt = %v, want the window start %v kept", attempts.FirstAt, lockoutNow)
	}
	if !attempts.LastAt.Equal(later) {
		t.Errorf("lastAt = %v, want %v", attempts.LastAt, later)
	}
	if attempts.Identifier != "ama@example.com" {
		t.Errorf("identifier = %q, want it carried through", attempts.Identifier)
	}
}

// TestRecordRollsStaleWindow: a failure arriving after the window closed
// starts a fresh count rather than inheriting old attempts.
func TestRecordRollsStaleWindow(t *testing.T) {
	p := policy()
	attempts := LoginAttempts{Identifier: "ama@example.com", Count: 2, FirstAt: lockoutNow, LastAt: lockoutNow}

	rolled := p.Record(attempts, lockoutNow.Add(p.Window+time.Second))
	if rolled.Count != 1 {
		t.Errorf("count = %d, want a fresh window of 1", rolled.Count)
	}
	if !rolled.FirstAt.Equal(lockoutNow.Add(p.Window + time.Second)) {
		t.Errorf("firstAt = %v, want the new attempt's time", rolled.FirstAt)
	}
}

// TestLockedTripsAtMaxAttempts: the lock engages on the max-th failure and
// reports the remaining cooldown.
func TestLockedTripsAtMaxAttempts(t *testing.T) {
	p := policy()
	attempts := LoginAttempts{Identifier: "ama@example.com"}

	for i := 1; i < p.MaxAttempts; i++ {
		attempts = p.Record(attempts, lockoutNow)
		if locked, _ := p.Locked(attempts, lockoutNow); locked {
			t.Fatalf("locked after %d attempts, want the lock to hold until %d", i, p.MaxAttempts)
		}
	}

	attempts = p.Record(attempts, lockoutNow)
	locked, retryAfter := p.Locked(attempts, lockoutNow)
	if !locked {
		t.Fatalf("not locked after %d attempts, want locked", p.MaxAttempts)
	}
	if retryAfter != p.Cooldown {
		t.Errorf("retryAfter = %v, want the full cooldown %v", retryAfter, p.Cooldown)
	}
}

// TestLockedReleasesAfterCooldown: the lock lifts once the cooldown has
// elapsed since the last attempt, and each new attempt extends it.
func TestLockedReleasesAfterCooldown(t *testing.T) {
	p := policy()
	attempts := LoginAttempts{
		Identifier: "ama@example.com",
		Count:      p.MaxAttempts,
		FirstAt:    lockoutNow,
		LastAt:     lockoutNow,
	}

	if locked, _ := p.Locked(attempts, lockoutNow.Add(p.Cooldown-time.Second)); !locked {
		t.Error("released one second early, want the lock to hold for the full cooldown")
	}
	if locked, _ := p.Locked(attempts, lockoutNow.Add(p.Cooldown)); locked {
		t.Error("still locked at the cooldown boundary, want release")
	}

	// Hammering while locked pushes the release out.
	attempts.LastAt = lockoutNow.Add(4 * time.Minute)
	if locked, retryAfter := p.Locked(attempts, lockoutNow.Add(5*time.Minute)); !locked || retryAfter != 4*time.Minute {
		t.Errorf("locked=%v retryAfter=%v, want locked with 4m left (the lock extends)", locked, retryAfter)
	}
}

// TestLockedIgnoresStaleWindow: attempts belonging to a window that has
// already closed never lock, even at max count.
func TestLockedIgnoresStaleWindow(t *testing.T) {
	p := policy()
	attempts := LoginAttempts{
		Identifier: "ama@example.com",
		Count:      p.MaxAttempts + 5,
		FirstAt:    lockoutNow,
		LastAt:     lockoutNow,
	}

	if locked, _ := p.Locked(attempts, lockoutNow.Add(p.Window+p.Cooldown+time.Minute)); locked {
		t.Error("locked on a closed window, want the stale count ignored")
	}
}

// TestZeroPolicyFallsBackToDefaults: a misconfigured policy can never
// disable the protection.
func TestZeroPolicyFallsBackToDefaults(t *testing.T) {
	var zero LockoutPolicy
	attempts := LoginAttempts{Identifier: "ama@example.com"}
	for i := 0; i < DefaultMaxLoginAttempts; i++ {
		attempts = zero.Record(attempts, lockoutNow)
	}

	locked, retryAfter := zero.Locked(attempts, lockoutNow)
	if !locked {
		t.Fatal("zero policy did not lock, want the platform defaults applied")
	}
	if retryAfter != DefaultLockoutCooldown {
		t.Errorf("retryAfter = %v, want the default cooldown %v", retryAfter, DefaultLockoutCooldown)
	}
	if zero.RetentionAfter() != DefaultAttemptWindow+DefaultLockoutCooldown {
		t.Errorf("retention = %v, want window + cooldown", zero.RetentionAfter())
	}
}
