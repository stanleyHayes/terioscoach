package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/identity"
)

/*
Booking concurrency (LCH-06).

The failure this guards against is the one a practice actually suffers:
two people looking at the same free slot at the same time, both tapping
Confirm, and both being told yes. Somebody then arrives to find the room
occupied, and no amount of apology fixes an hour that does not exist.

Correctness here cannot come from a check-then-write — between the check
and the write is exactly where the second booking lands. It comes from a
partial unique index on (practitionerId, startAt) over non-cancelled
bookings, which makes the second insert fail at the database.

What this test proves and does not:

  - **Proves:** the API is honest under contention. Exactly one caller is
    told yes; every other gets a clean 409, not a 500, not a hang.
  - **Does not prove:** that the index exists in Atlas. The fake
    repository enforces the same rule under its own lock, standing in for
    the index. `mongodb/indexes.go` declares it and the e2e suite is what
    exercises the real thing.

Run the whole file under -race. A pass without -race means very little.
*/

// TestOneSlotOneBooking fires many simultaneous requests at a single slot.
func TestOneSlotOneBooking(t *testing.T) {
	const contenders = 32

	rig := newJourneyRig(t)
	serviceID, day := seedConcurrencySchedule(t, rig)
	start := day.Add(9 * time.Hour)

	// Each contender is a distinct client, because the interesting race is
	// between different people, not one person's double-tap.
	tokens := issueClients(t, rig, contenders)

	var wg sync.WaitGroup
	statuses := make([]int, contenders)
	codes := make([]string, contenders)
	release := make(chan struct{})

	for i := range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Every goroutine waits on the same channel so they arrive
			// together rather than in the order they were started.
			<-release

			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
				"serviceId": serviceID,
				"startAt":   start.Format(time.RFC3339),
				"tz":        "UTC",
			}, bearer(tokens[i]))
			statuses[i] = rec.Code
			codes[i] = errorCodeOf(rec)
		}()
	}

	close(release)
	wg.Wait()

	created, conflicts, other := 0, 0, 0
	for i, status := range statuses {
		switch {
		case status == http.StatusCreated:
			created++
		case status == http.StatusConflict:
			conflicts++
			// The message matters: a caller has to be able to tell "that
			// slot just went" from "something broke".
			if codes[i] != "slot_unavailable" {
				t.Errorf("contender %d: 409 with code %q, want slot_unavailable", i, codes[i])
			}
		default:
			other++
			t.Errorf("contender %d: status %d (code %q) — neither a booking nor a clean refusal",
				i, status, codes[i])
		}
	}

	if created != 1 {
		t.Fatalf("%d of %d contenders were told the slot was theirs; exactly 1 may be",
			created, contenders)
	}
	if conflicts != contenders-1 {
		t.Errorf("%d clean refusals, want %d", conflicts, contenders-1)
	}
	if other != 0 {
		t.Errorf("%d requests failed in some other way", other)
	}
}

// TestConcurrentBookingsOfDifferentSlotsAllSucceed is the other half: the
// guard must not be so broad that it serializes unrelated bookings.
func TestConcurrentBookingsOfDifferentSlotsAllSucceed(t *testing.T) {
	const contenders = 12

	rig := newJourneyRig(t)
	serviceID, day := seedConcurrencySchedule(t, rig)
	tokens := issueClients(t, rig, contenders)

	var wg sync.WaitGroup
	statuses := make([]int, contenders)
	release := make(chan struct{})

	for i := range contenders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			// One per hour from 09:00 — the schedule below runs to 21:00.
			start := day.Add(time.Duration(9+i) * time.Hour)
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
				"serviceId": serviceID,
				"startAt":   start.Format(time.RFC3339),
				"tz":        "UTC",
			}, bearer(tokens[i]))
			statuses[i] = rec.Code
		}()
	}

	close(release)
	wg.Wait()

	for i, status := range statuses {
		if status != http.StatusCreated {
			t.Errorf("contender %d booking its own slot got %d, want 201", i, status)
		}
	}
}

// TestRepeatedTapsCreateOneBooking is the same person, not a race between
// people: a slow connection and an impatient thumb.
func TestRepeatedTapsCreateOneBooking(t *testing.T) {
	const taps = 8

	rig := newJourneyRig(t)
	serviceID, day := seedConcurrencySchedule(t, rig)
	start := day.Add(10 * time.Hour)

	var wg sync.WaitGroup
	statuses := make([]int, taps)
	release := make(chan struct{})

	for i := range taps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-release
			rec := doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
				"serviceId": serviceID,
				"startAt":   start.Format(time.RFC3339),
				"tz":        "UTC",
			}, bearer(rig.clientToken))
			statuses[i] = rec.Code
		}()
	}

	close(release)
	wg.Wait()

	created := 0
	for _, status := range statuses {
		if status == http.StatusCreated {
			created++
		}
	}
	// Being charged twice for one session is worse than being told the
	// button didn't work.
	if created != 1 {
		t.Errorf("%d bookings from %d taps by one client, want 1", created, taps)
	}
}

// TestReadsStayHealthyUnderWriteLoad: the public slot list is what a
// visitor sees while all this is happening. It must not error or hang.
func TestReadsStayHealthyUnderWriteLoad(t *testing.T) {
	rig := newJourneyRig(t)
	serviceID, day := seedConcurrencySchedule(t, rig)
	tokens := issueClients(t, rig, 8)

	slotsPath := "/v1/availability/slots?serviceId=" + serviceID +
		"&from=" + day.Format("2006-01-02") + "&to=" + day.Format("2006-01-02") + "&tz=UTC"

	var wg sync.WaitGroup
	readStatuses := make([]int, 24)

	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := day.Add(time.Duration(9+i) * time.Hour)
			doJSON(t, rig.srv, http.MethodPost, "/v1/bookings", map[string]any{
				"serviceId": serviceID,
				"startAt":   start.Format(time.RFC3339),
				"tz":        "UTC",
			}, bearer(tokens[i]))
		}()
	}
	for i := range readStatuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := doJSON(t, rig.srv, http.MethodGet, slotsPath, nil, nil)
			readStatuses[i] = rec.Code
		}()
	}
	wg.Wait()

	for i, status := range readStatuses {
		if status != http.StatusOK {
			t.Errorf("read %d during write load got %d, want 200", i, status)
		}
	}
}

// seedConcurrencySchedule opens a wide window a week out, so there are
// plenty of distinct slots to contend over or spread across.
func seedConcurrencySchedule(t *testing.T, rig journeyRig) (string, time.Time) {
	t.Helper()
	var created struct {
		Service struct {
			ID string `json:"id"`
		} `json:"service"`
	}
	rig.mustJSON(t, http.MethodPost, "/v1/services", map[string]any{
		"name": "Massage", "durationMinutes": 60, "priceKobo": 25000,
	}, rig.practitionerToken, http.StatusCreated, &created)

	day := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(24 * time.Hour)
	rig.mustJSON(t, http.MethodPut, "/v1/availability/rules", map[string]any{
		"rules": []map[string]any{{
			"weekday": int(day.Weekday()),
			// 09:00–21:00, no buffer: twelve back-to-back hours.
			"windows":       []map[string]int{{"startMin": 540, "endMin": 1260}},
			"bufferMinutes": 0,
		}},
	}, rig.practitionerToken, http.StatusOK, nil)

	return created.Service.ID, day
}

// issueClients seeds n client accounts and returns their access tokens.
//
// It seeds rather than registers on purpose. Thirty-two sign-ups from one
// address in one second is exactly what the auth rate limit exists to
// stop, and it stops it — but that is BE-02's test, not this one's. What
// is under test here is the booking slot, so the accounts arrive by the
// shortest honest route.
func issueClients(t *testing.T, rig journeyRig, n int) []string {
	t.Helper()
	tokens := make([]string, n)
	for i := range n {
		user, err := identity.NewUser(
			fmt.Sprintf("contender-%d@example.com", i),
			fmt.Sprintf("Contender %d", i),
			"hash", identity.RoleClient, time.Now().UTC(),
		)
		if err != nil {
			t.Fatalf("identity.NewUser: %v", err)
		}
		user, err = rig.users.Create(t.Context(), user)
		if err != nil {
			t.Fatalf("seed contender %d: %v", i, err)
		}
		tokens[i] = rig.issue(user.ID, identity.RoleClient)
	}
	return tokens
}

// errorCodeOf pulls the contract's error code out of a response, or "" if
// the body is not an error.
func errorCodeOf(rec *httptest.ResponseRecorder) string {
	body := rec.Body.String()
	const marker = `"code":"`
	i := strings.Index(body, marker)
	if i < 0 {
		return ""
	}
	rest := body[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
