package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// realResponse is the shape Cloudflare actually returns, copied from
// developers.cloudflare.com/realtime/turn/generate-credentials.
const realResponse = `{
  "iceServers": [
    {"urls": ["stun:stun.cloudflare.com:3478", "stun:stun.cloudflare.com:53"]},
    {
      "urls": [
        "turn:turn.cloudflare.com:3478?transport=udp",
        "turns:turn.cloudflare.com:5349?transport=tcp"
      ],
      "username": "dXNlcm5hbWU=",
      "credential": "Y3JlZGVudGlhbA=="
    }
  ]
}`

// newTestClient points a client at an httptest server.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	original := baseURL
	baseURL = server.URL + "/v1"
	t.Cleanup(func() { baseURL = original })

	return NewClient("key-123", "token-abc"), server
}

func TestGeneratesCredentialsFromTheKey(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		gotBody = string(body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	servers, err := client.ICEServers(t.Context(), time.Hour)
	if err != nil {
		t.Fatalf("ICEServers: %v", err)
	}

	if want := "/v1/turn/keys/key-123/credentials/generate-ice-servers"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if gotAuth != "Bearer token-abc" {
		t.Errorf("Authorization = %q, want a bearer token", gotAuth)
	}
	if !strings.Contains(gotBody, `"ttl"`) {
		t.Errorf("body = %q, want a ttl", gotBody)
	}

	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	// STUN first and credential-free; TURN carries the pair.
	if servers[0].Username != "" || servers[0].Credential != "" {
		t.Error("the STUN entry carries credentials it does not need")
	}
	if servers[1].Username == "" || servers[1].Credential == "" {
		t.Errorf("the TURN entry has no credentials: %+v", servers[1])
	}
	if !strings.HasPrefix(servers[1].URLs[0], "turn") {
		t.Errorf("second entry is not TURN: %+v", servers[1].URLs)
	}
}

// TestAsksForLongerThanTheSessionNeeds. A credential that expires mid-call
// drops the relay under a live session, and the client sees a call that
// worked for forty minutes and then stopped.
func TestAsksForLongerThanTheSessionNeeds(t *testing.T) {
	var requested struct {
		TTL int `json:"ttl"`
	}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&requested)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
		t.Fatalf("ICEServers: %v", err)
	}

	if requested.TTL <= int(time.Hour.Seconds()) {
		t.Errorf("asked for %ds to cover a 1h session — no headroom", requested.TTL)
	}
}

func TestShortTTLIsRaisedToAFloor(t *testing.T) {
	var requested struct {
		TTL int `json:"ttl"`
	}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&requested)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	// A client joining a room that closes in ten seconds still needs
	// credentials that outlast the handshake.
	if _, err := client.ICEServers(t.Context(), 10*time.Second); err != nil {
		t.Fatalf("ICEServers: %v", err)
	}
	if requested.TTL < int(minTTL.Seconds()) {
		t.Errorf("asked for %ds, want at least the %s floor", requested.TTL, minTTL)
	}
}

// TestCredentialsAreReusedWhileTheyLast: minting a pair per join would put
// a network round trip in front of every "Join session" tap, for
// credentials that are not per-user secrets anyway.
func TestCredentialsAreReusedWhileTheyLast(t *testing.T) {
	var calls int
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	for range 5 {
		if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
			t.Fatalf("ICEServers: %v", err)
		}
	}

	if calls != 1 {
		t.Errorf("made %d credential calls for 5 joins, want 1", calls)
	}
}

// TestExpiringCredentialsAreReplaced is the other half: the cache must not
// outlive what it holds.
func TestExpiringCredentialsAreReplaced(t *testing.T) {
	var calls int
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Two hours minted; three hours later they are long gone.
	now = now.Add(3 * time.Hour)
	if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
		t.Fatalf("second: %v", err)
	}

	if calls != 2 {
		t.Errorf("made %d calls, want 2 — stale credentials were served", calls)
	}
}

// TestCachedCredentialsMustCoverTheWholeSession: a cached pair with four
// minutes left is no use to a session that runs for an hour.
func TestCachedCredentialsMustCoverTheWholeSession(t *testing.T) {
	var calls int
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	// Ten-minute session → 20 minutes minted.
	if _, err := client.ICEServers(t.Context(), 10*time.Minute); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Fifteen minutes later there are 5 minutes left — fine for a short
	// call, not for an hour-long one.
	now = now.Add(15 * time.Minute)
	if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
		t.Fatalf("second: %v", err)
	}

	if calls != 2 {
		t.Errorf("made %d calls, want 2 — a nearly-expired pair was reused for a long session", calls)
	}
}

// TestApiTokenNeverAppearsInAnError. Tokens have a way of reaching logs
// when an error quotes what the server said.
func TestApiTokenNeverAppearsInAnError(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"bad token: token-abc"}`))
	})

	_, err := client.ICEServers(t.Context(), time.Hour)
	if err == nil {
		t.Fatal("a 401 produced no error")
	}
	if strings.Contains(err.Error(), "token-abc") {
		t.Errorf("the API token is in the error text: %v", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error does not say what happened: %v", err)
	}
}

func TestRejectsAnEmptyOrUselessResponse(t *testing.T) {
	for name, body := range map[string]string{
		"no servers":  `{"iceServers":[]}`,
		"no urls":     `{"iceServers":[{"username":"u","credential":"c"}]}`,
		"not json":    `<html>502</html>`,
		"wrong shape": `{"result":{"iceServers":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(body))
			})
			// Returning an empty list as success would look like STUN-only
			// and hide a broken TURN key indefinitely.
			if _, err := client.ICEServers(t.Context(), time.Hour); err == nil {
				t.Error("accepted a response with no usable servers")
			}
		})
	}
}

func TestSurvivesConcurrentJoins(t *testing.T) {
	var mu sync.Mutex
	var calls int
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(realResponse))
	})

	var wg sync.WaitGroup
	errs := make([]error, 16)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = client.ICEServers(t.Context(), time.Hour)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("join %d: %v", i, err)
		}
	}
}

func TestTimeoutIsBounded(t *testing.T) {
	// Released on cleanup: a handler that blocks on the request context
	// alone leaves httptest.Server.Close waiting on it forever.
	// Released by defer, which runs before t.Cleanup — and therefore
	// before httptest's server.Close, which blocks on live handlers.
	release := make(chan struct{})
	defer close(release)

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	client.http.Timeout = 50 * time.Millisecond

	start := time.Now()
	_, err := client.ICEServers(t.Context(), time.Hour)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("a hanging provider produced no error")
	}
	// A client is waiting to join a session behind this call.
	if elapsed > time.Second {
		t.Errorf("took %s to give up", elapsed)
	}
}

func TestCancelledContextIsRespected(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := client.ICEServers(ctx, time.Hour); err == nil {
		t.Error("a cancelled request still returned servers")
	}
}

func TestStaticProviderServesWhatItWasGiven(t *testing.T) {
	provider := StaticProvider{Servers: nil}
	servers, err := provider.ICEServers(t.Context(), time.Hour)
	if err != nil {
		t.Fatalf("StaticProvider: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("got %d servers from an empty provider", len(servers))
	}
}

// TestAcceptsBoth200And201: the documented success is 201, but an API that
// starts answering 200 must not break every video call.
func TestAcceptsBoth200And201(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(realResponse))
			})
			if _, err := client.ICEServers(t.Context(), time.Hour); err != nil {
				t.Errorf("status %d rejected: %v", status, err)
			}
		})
	}
}
