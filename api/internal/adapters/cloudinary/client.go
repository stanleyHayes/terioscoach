// Package cloudinary is the outbound adapter for the Cloudinary media
// store — a plain net/http client, no SDK. It implements ports.MediaStore.
//
// The bytes never pass through this process. The browser uploads directly
// to Cloudinary using a signature this adapter mints, and downloads come
// from a signed delivery URL with a short expiry. The API's job is to
// decide *whether* a caller may have a URL — that decision lives in the
// documents slice, not here.
package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/xcreativs/terios/api/internal/ports"
)

// baseAPIURL is the Cloudinary REST root. It is a var so tests can point
// the client at an httptest server.
var baseAPIURL = "https://api.cloudinary.com/v1_1"

// baseDeliveryURL is the asset delivery root.
var baseDeliveryURL = "https://res.cloudinary.com"

// requestTimeout bounds every admin-API call.
const requestTimeout = 10 * time.Second

// Client signs uploads and delivery URLs for one Cloudinary account.
type Client struct {
	cloudName string
	apiKey    string
	apiSecret string
	http      *http.Client
	now       func() time.Time
}

// Compile-time check: Client satisfies the media-store port.
var _ ports.MediaStore = (*Client)(nil)

// NewClient builds a media-store client.
func NewClient(cloudName, apiKey, apiSecret string) *Client {
	return &Client{
		cloudName: cloudName,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		http:      &http.Client{Timeout: requestTimeout},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SignUpload mints the parameters a browser needs to upload one asset
// directly.
//
// The signature covers the folder and the delivery type, so a caller
// holding it cannot redirect the upload into another client's folder or
// turn a private upload into a public one — the two things that would
// break the media policy. It expires, so a leaked signature is not a
// standing invitation.
func (c *Client) SignUpload(_ context.Context, params ports.UploadParams) (ports.SignedUpload, error) {
	if params.Folder == "" {
		return ports.SignedUpload{}, fmt.Errorf("cloudinary: folder is required")
	}
	timestamp := c.now().Unix()

	toSign := map[string]string{
		"folder":    params.Folder,
		"timestamp": strconv.FormatInt(timestamp, 10),
	}
	if params.Private {
		// "authenticated" assets are never reachable by public URL: they
		// require a signed delivery URL, which is what makes a client
		// document private at the storage layer rather than by obscurity.
		toSign["type"] = "authenticated"
	}
	if params.PublicID != "" {
		toSign["public_id"] = params.PublicID
	}

	signature := c.sign(toSign)
	fields := make(map[string]string, len(toSign)+2)
	for k, v := range toSign {
		fields[k] = v
	}
	fields["api_key"] = c.apiKey
	fields["signature"] = signature

	return ports.SignedUpload{
		URL:       fmt.Sprintf("%s/%s/%s/upload", baseAPIURL, c.cloudName, string(params.ResourceType)),
		Fields:    fields,
		Signature: signature,
		Timestamp: timestamp,
		ExpiresAt: c.now().Add(ports.UploadSignatureTTL),
	}, nil
}

// SignedURL builds a short-lived delivery URL for a private asset.
//
// Cloudinary's signed delivery puts the expiry inside the signed component,
// so the link stops working on its own — a URL that leaked out of an inbox
// is not a permanent handle on someone's medical history.
func (c *Client) SignedURL(_ context.Context, asset ports.Asset, ttl time.Duration) (string, error) {
	if asset.PublicID == "" {
		return "", fmt.Errorf("cloudinary: public id is required")
	}
	if ttl <= 0 {
		ttl = ports.DefaultDeliveryTTL
	}
	expiresAt := c.now().Add(ttl).Unix()

	resourceType := string(asset.ResourceType)
	if resourceType == "" {
		resourceType = "image"
	}
	deliveryType := "upload"
	if asset.Private {
		deliveryType = "authenticated"
	}

	// Cloudinary signs "<transformations>/<public_id>" for delivery URLs;
	// the expiry rides as its own transformation component.
	target := fmt.Sprintf("exp_%d/%s", expiresAt, asset.PublicID)
	signature := signDelivery(target, c.apiSecret)

	return fmt.Sprintf("%s/%s/%s/%s/s--%s--/exp_%d/%s",
		baseDeliveryURL, c.cloudName, resourceType, deliveryType,
		signature, expiresAt, asset.PublicID), nil
}

// PublicURL builds the stable delivery URL for public CMS imagery. Unlike a
// private client document it has no expiry and is safe to persist in content.
func (c *Client) PublicURL(asset ports.Asset) (string, error) {
	if asset.PublicID == "" {
		return "", fmt.Errorf("cloudinary: public id is required")
	}
	resourceType := string(asset.ResourceType)
	if resourceType == "" {
		resourceType = "image"
	}
	return fmt.Sprintf("%s/%s/%s/upload/%s", baseDeliveryURL, c.cloudName, resourceType, asset.PublicID), nil
}

// Delete removes an asset from the store. It is used when a document
// record is deleted, so the file does not outlive the record that governed
// who could see it.
func (c *Client) Delete(ctx context.Context, asset ports.Asset) error {
	timestamp := c.now().Unix()
	params := map[string]string{
		"public_id": asset.PublicID,
		"timestamp": strconv.FormatInt(timestamp, 10),
	}
	if asset.Private {
		params["type"] = "authenticated"
	}

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("api_key", c.apiKey)
	form.Set("signature", c.sign(params))

	resourceType := string(asset.ResourceType)
	if resourceType == "" {
		resourceType = "image"
	}
	endpoint := fmt.Sprintf("%s/%s/%s/destroy", baseAPIURL, c.cloudName, resourceType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build cloudinary request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	if err != nil {
		return &ports.GatewayError{StatusCode: 0, Message: "cloudinary is unreachable: " + err.Error()}
	}
	defer func() { _ = res.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return &ports.GatewayError{
			StatusCode: res.StatusCode,
			Message:    fmt.Sprintf("cloudinary rejected the delete (HTTP %d)", res.StatusCode),
		}
	}
	return nil
}

// sign builds Cloudinary's upload/admin signature: the parameters sorted by
// key, joined as a query string, with the API secret appended, hashed with
// SHA-1. The algorithm is Cloudinary's; the sorting is what makes it
// reproducible on their side.
func (c *Client) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "&") + c.apiSecret))
	return hex.EncodeToString(sum[:])
}

// signDelivery builds the signature component of a signed delivery URL:
// SHA-1 over the target plus the secret, base64url-encoded and truncated,
// which is the form Cloudinary expects between the `s--` markers.
func signDelivery(target, secret string) string {
	sum := sha1.Sum([]byte(target + secret))
	return base64URL(sum[:])[:8]
}

// base64URL encodes without padding, matching Cloudinary's URL component.
func base64URL(raw []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var b strings.Builder
	for i := 0; i < len(raw); i += 3 {
		var chunk uint32
		remaining := len(raw) - i
		chunk |= uint32(raw[i]) << 16
		if remaining > 1 {
			chunk |= uint32(raw[i+1]) << 8
		}
		if remaining > 2 {
			chunk |= uint32(raw[i+2])
		}
		b.WriteByte(alphabet[(chunk>>18)&0x3f])
		b.WriteByte(alphabet[(chunk>>12)&0x3f])
		if remaining > 1 {
			b.WriteByte(alphabet[(chunk>>6)&0x3f])
		}
		if remaining > 2 {
			b.WriteByte(alphabet[chunk&0x3f])
		}
	}
	return b.String()
}
