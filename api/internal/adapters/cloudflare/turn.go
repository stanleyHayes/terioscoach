// Package cloudflare is the outbound adapter for Cloudflare Realtime's
// managed TURN service — a plain net/http client, no SDK. It implements
// ports.ICEProvider.
//
// Cloudflare does not issue a static TURN username and password. A
// deployment holds a long-term key (an id and an API token) and exchanges
// it, per use, for credentials that expire. That exchange is what this
// adapter does, and it is why the ICE list is a port call rather than a
// value read out of the environment: a fixed username lifted from config
// authenticates against nothing.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

// baseURL is the Realtime API root. A var so tests can point at httptest.
var baseURL = "https://rtc.live.cloudflare.com/v1"

// requestTimeout bounds the credential call. It sits in front of a client
// waiting to join a session, so it is short: a slow answer is worse than
// falling back to STUN and connecting anyway.
const requestTimeout = 5 * time.Second

// maxResponseBytes caps the response body. The real payload is well under
// a kilobyte.
const maxResponseBytes = 1 << 16

// minTTL is the shortest credential lifetime worth asking for. Cloudflare
// bills nothing per credential, and a credential that expires mid-session
// drops the relay under a live call.
const minTTL = 5 * time.Minute

// Client mints short-lived TURN credentials from one Cloudflare TURN key.
type Client struct {
	keyID    string
	apiToken string
	http     *http.Client
	now      func() time.Time

	// Credentials are cached and shared. Every participant in every
	// session can use the same ones — they are not per-user secrets, and
	// minting a fresh pair per join would put a network round trip in
	// front of every "Join session" tap for no benefit.
	mu       sync.Mutex
	cached   []ports.ICEServer
	cachedTo time.Time
}

var _ ports.ICEProvider = (*Client)(nil)

// NewClient builds a TURN credential client.
//
// keyID and apiToken are the two values a TURN key yields in the
// Cloudflare dashboard under Realtime → TURN Keys.
func NewClient(keyID, apiToken string) *Client {
	return &Client{
		keyID:    keyID,
		apiToken: apiToken,
		http:     &http.Client{Timeout: requestTimeout},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// iceServersResponse is Cloudflare's reply, which is already in the shape
// an RTCPeerConnection wants — STUN entries with no credentials, then the
// TURN entry carrying them.
type iceServersResponse struct {
	ICEServers []struct {
		URLs       []string `json:"urls"`
		Username   string   `json:"username,omitempty"`
		Credential string   `json:"credential,omitempty"`
	} `json:"iceServers"`
}

// ICEServers returns servers valid for at least ttl.
//
// Cached credentials are reused while they have comfortably more than ttl
// left. "Comfortably" matters: handing back a credential with four minutes
// remaining to a session that will run for an hour is the same as handing
// back nothing, and the failure would appear forty minutes in, as a call
// that silently stops relaying.
func (c *Client) ICEServers(ctx context.Context, ttl time.Duration) ([]ports.ICEServer, error) {
	if ttl < minTTL {
		ttl = minTTL
	}

	c.mu.Lock()
	if c.cached != nil && c.cachedTo.After(c.now().Add(ttl)) {
		servers := c.cached
		c.mu.Unlock()
		return servers, nil
	}
	c.mu.Unlock()

	// Asked for double what the caller needs, so one mint covers a session
	// that overruns and the next few that follow it.
	requested := ttl * 2

	servers, err := c.generate(ctx, requested)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cached = servers
	c.cachedTo = c.now().Add(requested)
	c.mu.Unlock()

	return servers, nil
}

// generate performs the credential exchange.
func (c *Client) generate(ctx context.Context, ttl time.Duration) ([]ports.ICEServer, error) {
	body, err := json.Marshal(map[string]int{"ttl": int(ttl.Seconds())})
	if err != nil {
		return nil, fmt.Errorf("cloudflare: encode request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/turn/keys/%s/credentials/generate-ice-servers", baseURL, c.keyID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cloudflare: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare: turn credentials: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("cloudflare: read response: %w", err)
	}

	if res.StatusCode != http.StatusCreated && res.StatusCode != http.StatusOK {
		// The body is not echoed. A 401 here means the API token is wrong,
		// and tokens have a way of ending up in logs when errors quote
		// what the server said.
		return nil, fmt.Errorf("cloudflare: turn credentials: status %d", res.StatusCode)
	}

	var parsed iceServersResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("cloudflare: decode response: %w", err)
	}
	if len(parsed.ICEServers) == 0 {
		return nil, fmt.Errorf("cloudflare: turn credentials: empty server list")
	}

	servers := make([]ports.ICEServer, 0, len(parsed.ICEServers))
	for _, server := range parsed.ICEServers {
		if len(server.URLs) == 0 {
			continue
		}
		servers = append(servers, ports.ICEServer{
			URLs:       server.URLs,
			Username:   server.Username,
			Credential: server.Credential,
		})
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("cloudflare: turn credentials: no usable servers")
	}
	return servers, nil
}

// StaticProvider serves a fixed ICE list — the STUN-only fallback for a
// deployment with no TURN key, and what development runs on.
//
// It exists so the signaling service always has a provider and never has
// to branch on nil. What it cannot do is relay: two clients behind
// symmetric NAT will fail to connect, which on ordinary home broadband is
// a real and common case, not an edge one.
type StaticProvider struct {
	Servers []ports.ICEServer
}

var _ ports.ICEProvider = StaticProvider{}

func (s StaticProvider) ICEServers(context.Context, time.Duration) ([]ports.ICEServer, error) {
	return s.Servers, nil
}
