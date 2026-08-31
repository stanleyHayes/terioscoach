package integration

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/adapters/cloudflare"
	"github.com/xcreativs/terios/api/internal/adapters/cloudinary"
	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
)

// env is loaded once per run.
var env map[string]string

// requireIntegration gates the whole file and loads credentials.
//
// The env-var gate is the important half. Without it these run as part of
// a plain `go test ./...`, which would mean every developer and every CI
// job silently making live calls to Cloudflare and Cloudinary —
// and, once MONGODB_URI is filled in, writing to a real database.
func requireIntegration(t *testing.T) map[string]string {
	t.Helper()
	if os.Getenv("TERIOS_INTEGRATION") != "1" {
		t.Skip("live integration tests are off; set TERIOS_INTEGRATION=1 to run them")
	}
	if env == nil {
		loaded, err := repoEnv()
		if err != nil {
			t.Skipf("no api/.env: %v", err)
		}
		env = loaded
	}
	return env
}

func TestMain(m *testing.M) {
	m.Run()
}

// ---------------------------------------------------------------------
// Cloudflare TURN (CX-02)
//
// The one adapter whose correctness cannot be inferred from anything else:
// it was written against documentation, and the endpoint, the auth header
// and the response shape are all assumptions until a real call is made.
// ---------------------------------------------------------------------

func TestLiveCloudflareTURN(t *testing.T) {
	env := requireIntegration(t)
	keyID, token := env["TURN_KEY_ID"], env["TURN_API_TOKEN"]
	if placeholder(keyID) || placeholder(token) {
		t.Skip("TURN_KEY_ID / TURN_API_TOKEN not set")
	}

	client := cloudflare.NewClient(keyID, token)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	servers, err := client.ICEServers(ctx, time.Hour)
	if err != nil {
		t.Fatalf("mint TURN credentials: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("Cloudflare returned no ICE servers")
	}

	var sawSTUN, sawTURN, sawTLS bool
	for _, server := range servers {
		for _, url := range server.URLs {
			switch {
			case strings.HasPrefix(url, "stun:"):
				sawSTUN = true
			case strings.HasPrefix(url, "turns:"):
				sawTLS = true
				sawTURN = true
			case strings.HasPrefix(url, "turn:"):
				sawTURN = true
			}
		}
		if sawTURN && server.Username == "" {
			t.Error("a TURN entry arrived with no username — it will not authenticate")
		}
	}

	if !sawTURN {
		t.Error("no turn: URLs — this is STUN-only and will fail behind symmetric NAT")
	}
	if !sawSTUN {
		t.Error("no stun: URLs — every call would be forced through the relay")
	}
	// turns: on 443 is what gets through a corporate firewall that blocks
	// UDP entirely. Its absence is not fatal but is worth knowing.
	if !sawTLS {
		t.Log("note: no turns: (TLS) URLs offered; calls from restrictive networks may fail")
	}

	// The credentials must be usable, not merely present: Cloudflare
	// returns base64, and a value that does not decode means we are
	// reading the wrong field.
	for _, server := range servers {
		if server.Username == "" {
			continue
		}
		if _, err := base64.StdEncoding.DecodeString(server.Username); err != nil {
			t.Errorf("username is not base64 as documented: %v", err)
		}
		if _, err := base64.StdEncoding.DecodeString(server.Credential); err != nil {
			t.Errorf("credential is not base64 as documented: %v", err)
		}
	}

	t.Logf("Cloudflare returned %d ICE entries (stun=%v turn=%v turns=%v)",
		len(servers), sawSTUN, sawTURN, sawTLS)
}

func TestLiveCloudflareRejectsABadToken(t *testing.T) {
	env := requireIntegration(t)
	keyID := env["TURN_KEY_ID"]
	if placeholder(keyID) {
		t.Skip("TURN_KEY_ID not set")
	}

	client := cloudflare.NewClient(keyID, "definitely-not-a-valid-token")
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()

	_, err := client.ICEServers(ctx, time.Hour)
	if err == nil {
		t.Fatal("a bogus token was accepted — the adapter is not checking the status")
	}
	// The token must not survive into the error, which ends up in logs.
	if strings.Contains(err.Error(), "definitely-not-a-valid-token") {
		t.Errorf("the token leaked into the error: %v", err)
	}
	t.Logf("bad token correctly refused: %v", err)
}

// ---------------------------------------------------------------------
// Cloudinary (BE-11)
//
// Signing is computed locally, so a wrong signature is invisible until
// Cloudinary rejects it. This uploads a real 1x1 PNG and deletes it.
// ---------------------------------------------------------------------

// onePixelPNG is the smallest valid PNG, so the upload costs nothing.
const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func TestLiveCloudinarySignedUpload(t *testing.T) {
	env := requireIntegration(t)
	name, key, secret := env["CLOUDINARY_CLOUD_NAME"], env["CLOUDINARY_API_KEY"], env["CLOUDINARY_API_SECRET"]
	if placeholder(name) || placeholder(key) || placeholder(secret) {
		t.Skip("Cloudinary credentials not set")
	}

	client := cloudinary.NewClient(name, key, secret)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	publicID := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	signed, err := client.SignUpload(ctx, ports.UploadParams{
		Folder:       "terios/integration",
		PublicID:     publicID,
		ResourceType: document.ResourceImage,
		Private:      false,
	})
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}
	if signed.URL == "" || len(signed.Fields) == 0 {
		t.Fatalf("signed upload is unusable: %+v", signed)
	}

	uploaded, err := postSignedUpload(ctx, signed, onePixelPNG)
	if err != nil {
		// This is the failure worth having a test for: the signature was
		// accepted by our own code and rejected by Cloudinary.
		t.Fatalf("Cloudinary rejected our signature: %v", err)
	}
	t.Logf("uploaded public_id=%s bytes=%d", uploaded.PublicID, uploaded.Bytes)

	t.Cleanup(func() {
		if err := client.Delete(context.Background(), ports.Asset{
			PublicID:     uploaded.PublicID,
			ResourceType: document.ResourceImage,
		}); err != nil {
			t.Errorf("leaked a test asset (%s): %v", uploaded.PublicID, err)
		}
	})

	if uploaded.PublicID == "" {
		t.Error("no public_id returned; the document record would have nothing to point at")
	}
	if uploaded.SecureURL == "" || !strings.HasPrefix(uploaded.SecureURL, "https://") {
		t.Errorf("secure_url = %q, want an https URL", uploaded.SecureURL)
	}
}

func TestLiveCloudinaryRejectsAForgedSignature(t *testing.T) {
	env := requireIntegration(t)
	name, key := env["CLOUDINARY_CLOUD_NAME"], env["CLOUDINARY_API_KEY"]
	if placeholder(name) || placeholder(key) {
		t.Skip("Cloudinary credentials not set")
	}

	// A client built with the wrong secret must not be able to upload.
	// If this passes, the folder policy is not enforced by anything.
	client := cloudinary.NewClient(name, key, "not-the-real-secret")
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	signed, err := client.SignUpload(ctx, ports.UploadParams{
		Folder:       "terios/integration",
		PublicID:     fmt.Sprintf("forged-%d", time.Now().UnixNano()),
		ResourceType: document.ResourceImage,
	})
	if err != nil {
		t.Fatalf("sign upload: %v", err)
	}

	if _, err := postSignedUpload(ctx, signed, onePixelPNG); err == nil {
		t.Fatal("Cloudinary accepted a forged signature — uploads are not actually authenticated")
	}
}

// ---------------------------------------------------------------------
