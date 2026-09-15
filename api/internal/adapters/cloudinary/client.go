// Package cloudinary is the outbound adapter for the Cloudinary media
// store — a plain net/http client, no SDK. It implements ports.MediaStore.
//
// Client uploads never pass through this process: the browser uploads
// directly to Cloudinary using a signature this adapter mints, and
// downloads come from a signed delivery URL with a short expiry. The API's
// job is to decide *whether* a caller may have a URL — that decision lives
// in the documents slice, not here.
//
// Upload is the one exception, for the few files the API produces itself
// (a signed agreement, rendered as a PDF). There is no browser in that
// story to hand a signature to.
package cloudinary

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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

// Upload stores bytes this process produced, under the same signed
// parameters a browser upload would carry.
func (c *Client) Upload(ctx context.Context, params ports.UploadParams, file ports.UploadFile) (ports.UploadedAsset, error) {
	signed, err := c.SignUpload(ctx, params)
	if err != nil {
		return ports.UploadedAsset{}, err
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for key, value := range signed.Fields {
		if err := form.WriteField(key, value); err != nil {
			return ports.UploadedAsset{}, fmt.Errorf("build cloudinary upload: %w", err)
		}
	}
	part, err := form.CreateFormFile("file", file.Filename)
	if err != nil {
		return ports.UploadedAsset{}, fmt.Errorf("build cloudinary upload: %w", err)
	}
	if _, err := part.Write(file.Data); err != nil {
		return ports.UploadedAsset{}, fmt.Errorf("build cloudinary upload: %w", err)
	}
	if err := form.Close(); err != nil {
		return ports.UploadedAsset{}, fmt.Errorf("build cloudinary upload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signed.URL, &body)
	if err != nil {
		return ports.UploadedAsset{}, fmt.Errorf("build cloudinary request: %w", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())

	res, err := c.http.Do(req)
	if err != nil {
		return ports.UploadedAsset{}, &ports.GatewayError{StatusCode: 0, Message: "cloudinary is unreachable: " + err.Error()}
	}
	defer func() { _ = res.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return ports.UploadedAsset{}, &ports.GatewayError{
			StatusCode: res.StatusCode,
			Message:    fmt.Sprintf("cloudinary rejected the upload (HTTP %d)", res.StatusCode),
		}
	}

	var decoded struct {
		PublicID string `json:"public_id"`
		Bytes    int64  `json:"bytes"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ports.UploadedAsset{}, fmt.Errorf("decode cloudinary upload: %w", err)
	}
	if decoded.PublicID == "" {
		return ports.UploadedAsset{}, fmt.Errorf("cloudinary upload returned no public id")
	}
	if decoded.Bytes == 0 {
		decoded.Bytes = int64(len(file.Data))
	}
	return ports.UploadedAsset{PublicID: decoded.PublicID, Bytes: decoded.Bytes}, nil
}

// SignedURL uses Cloudinary's authenticated download API. Unlike a signed
// CDN path, this endpoint enforces expires_at without a premium auth token.
// Raw public IDs retain their extension; image downloads preserve the source.
func (c *Client) SignedURL(_ context.Context, asset ports.Asset, ttl time.Duration) (string, error) {
	if asset.PublicID == "" {
		return "", fmt.Errorf("cloudinary: public id is required")
	}
	if ttl <= 0 {
		ttl = ports.DefaultDeliveryTTL
	}
	resourceType := string(asset.ResourceType)
	if resourceType == "" {
		resourceType = "image"
	}
	deliveryType := "upload"
	if asset.Private {
		deliveryType = "authenticated"
	}
	params := map[string]string{
		"public_id":  asset.PublicID,
		"type":       deliveryType,
		"timestamp":  strconv.FormatInt(c.now().Unix(), 10),
		"expires_at": strconv.FormatInt(c.now().Add(ttl).Unix(), 10),
	}
	if resourceType == "video" {
		// Convert existing WebM recordings as well as new uploads: mp4's
		// default codecs (H.264/AAC) are playable on iPhone, Android and
		// desktop media players. "format" is the only conversion parameter
		// this endpoint signs — an arbitrary "transformation" parameter is
		// not part of the signed set the Admin API checks, so adding one
		// here would make the signature it computes never match the one we
		// send, and every video download would fail with "Invalid Signature".
		params["format"] = "mp4"
	}
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	query.Set("api_key", c.apiKey)
	query.Set("signature", c.sign(params))
	return fmt.Sprintf("%s/%s/%s/download?%s", baseAPIURL, c.cloudName, resourceType, query.Encode()), nil
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
