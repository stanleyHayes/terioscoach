package cloudinary

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xcreativs/terios/api/internal/domain/document"
	"github.com/xcreativs/terios/api/internal/ports"
)

var testNow = time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC)

func testClient() *Client {
	c := NewClient("terios", "test-key", "test-secret")
	c.now = func() time.Time { return testNow }
	return c
}

// TestSignUploadCoversTheFolderAndType is the property that makes the
// signature safe to hand to a browser: the caller cannot change where the
// upload lands or how it is delivered without invalidating it.
func TestSignUploadCoversTheFolderAndType(t *testing.T) {
	c := testClient()

	signed, err := c.SignUpload(context.Background(), ports.UploadParams{
		Folder:       "terios/clients/client-1/documents",
		ResourceType: document.ResourceRaw,
		Private:      true,
	})
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}

	if !strings.HasSuffix(signed.URL, "/terios/raw/upload") {
		t.Errorf("url = %q, want the raw upload endpoint for this cloud", signed.URL)
	}
	if signed.Fields["api_key"] != "test-key" {
		t.Errorf("api_key = %q, want the configured key", signed.Fields["api_key"])
	}
	if signed.Fields["type"] != "authenticated" {
		t.Errorf("type = %q, want authenticated for a private upload", signed.Fields["type"])
	}
	if signed.Fields["folder"] != "terios/clients/client-1/documents" {
		t.Errorf("folder = %q, want the requested folder", signed.Fields["folder"])
	}
	if !signed.ExpiresAt.After(testNow) {
		t.Errorf("expiresAt = %v, want a future expiry", signed.ExpiresAt)
	}

	// Recompute the signature the way Cloudinary does: sorted params, then
	// the secret, SHA-1. Anything else would be rejected at their end.
	want := sha1.Sum([]byte(
		"folder=terios/clients/client-1/documents&timestamp=" +
			strconv.FormatInt(testNow.Unix(), 10) + "&type=authenticated" + "test-secret"))
	if signed.Signature != hex.EncodeToString(want[:]) {
		t.Errorf("signature = %q, want the sorted-parameter digest %q",
			signed.Signature, hex.EncodeToString(want[:]))
	}
}

// TestSignatureChangesWithTheFolder: two folders must never share a
// signature, or one could be swapped for the other.
func TestSignatureChangesWithTheFolder(t *testing.T) {
	c := testClient()

	first, err := c.SignUpload(context.Background(), ports.UploadParams{
		Folder: "terios/clients/client-1/documents", ResourceType: document.ResourceRaw, Private: true,
	})
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	second, err := c.SignUpload(context.Background(), ports.UploadParams{
		Folder: "terios/clients/client-2/documents", ResourceType: document.ResourceRaw, Private: true,
	})
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	if first.Signature == second.Signature {
		t.Error("two client folders share one signature — an upload could be redirected")
	}

	public, err := c.SignUpload(context.Background(), ports.UploadParams{
		Folder: "terios/clients/client-1/documents", ResourceType: document.ResourceRaw,
	})
	if err != nil {
		t.Fatalf("SignUpload: %v", err)
	}
	if public.Signature == first.Signature {
		t.Error("private and public uploads share one signature — privacy could be downgraded")
	}
}

// TestSignUploadRequiresAFolder: an unfoldered upload would land at the
// account root, outside the media policy entirely.
func TestSignUploadRequiresAFolder(t *testing.T) {
	if _, err := testClient().SignUpload(context.Background(), ports.UploadParams{
		ResourceType: document.ResourceRaw,
	}); err == nil {
		t.Error("signed an upload with no folder")
	}
}

// TestSignedURLExpires: a link that leaked out of an inbox must stop
// working on its own.
func TestSignedURLExpires(t *testing.T) {
	c := testClient()

	link, err := c.SignedURL(context.Background(), ports.Asset{
		PublicID:     "terios/clients/client-1/documents/abc",
		ResourceType: document.ResourceRaw,
		Private:      true,
	}, time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if parsed.Host == "" {
		t.Fatalf("url = %q, want an absolute delivery url", link)
	}
	if !strings.Contains(link, "/authenticated/") {
		t.Errorf("url = %q, want authenticated delivery for a private asset", link)
	}
	if !strings.Contains(link, "s--") {
		t.Errorf("url = %q, want a signature component", link)
	}
	expiry := strconv.FormatInt(testNow.Add(time.Hour).Unix(), 10)
	if !strings.Contains(link, "exp_"+expiry) {
		t.Errorf("url = %q, want the expiry %s embedded", link, expiry)
	}
	if !strings.Contains(link, "terios/clients/client-1/documents/abc") {
		t.Errorf("url = %q, want the asset id", link)
	}
}

// TestSignedURLDiffersPerAssetAndExpiry: the signature must cover both, or
// one link could be edited into another.
func TestSignedURLDiffersPerAssetAndExpiry(t *testing.T) {
	c := testClient()
	asset := ports.Asset{PublicID: "a", ResourceType: document.ResourceRaw, Private: true}
	other := ports.Asset{PublicID: "b", ResourceType: document.ResourceRaw, Private: true}

	first, err := c.SignedURL(context.Background(), asset, time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	second, err := c.SignedURL(context.Background(), other, time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	longer, err := c.SignedURL(context.Background(), asset, 2*time.Hour)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}

	signature := func(link string) string {
		start := strings.Index(link, "s--")
		end := strings.Index(link[start+3:], "--")
		return link[start : start+3+end]
	}
	if signature(first) == signature(second) {
		t.Error("two assets share a delivery signature — one link could be edited into the other")
	}
	if signature(first) == signature(longer) {
		t.Error("the expiry is not covered by the signature — a link could be extended")
	}
}

// TestSignedURLNeedsAnAsset.
func TestSignedURLNeedsAnAsset(t *testing.T) {
	if _, err := testClient().SignedURL(context.Background(), ports.Asset{}, time.Hour); err == nil {
		t.Error("signed a url for no asset")
	}
}

// TestSignedURLDefaultsTheTTL: a caller passing zero gets the platform
// default, not a link that expired in the past.
func TestSignedURLDefaultsTheTTL(t *testing.T) {
	link, err := testClient().SignedURL(context.Background(), ports.Asset{
		PublicID: "a", ResourceType: document.ResourceImage,
	}, 0)
	if err != nil {
		t.Fatalf("SignedURL: %v", err)
	}
	expiry := strconv.FormatInt(testNow.Add(ports.DefaultDeliveryTTL).Unix(), 10)
	if !strings.Contains(link, "exp_"+expiry) {
		t.Errorf("url = %q, want the default ttl applied", link)
	}
}

func TestPublicURLBuildsDurableImageDeliveryURL(t *testing.T) {
	link, err := testClient().PublicURL(ports.Asset{
		PublicID:     "terios/cms/about-portrait",
		ResourceType: document.ResourceImage,
	})
	if err != nil {
		t.Fatalf("PublicURL: %v", err)
	}
	if link != "https://res.cloudinary.com/terios/image/upload/terios/cms/about-portrait" {
		t.Errorf("url = %q, want durable public delivery url", link)
	}
}

func TestPublicURLRequiresAnAsset(t *testing.T) {
	if _, err := testClient().PublicURL(ports.Asset{}); err == nil {
		t.Error("built a public url without an asset id")
	}
}

// TestDeleteSignsTheRequest.
func TestDeleteSignsTheRequest(t *testing.T) {
	var form url.Values
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = r.ParseForm()
		form = r.PostForm
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	original := baseAPIURL
	baseAPIURL = srv.URL
	defer func() { baseAPIURL = original }()

	err := testClient().Delete(context.Background(), ports.Asset{
		PublicID: "terios/clients/client-1/documents/abc", ResourceType: document.ResourceRaw, Private: true,
	})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if !strings.HasSuffix(path, "/terios/raw/destroy") {
		t.Errorf("path = %q, want the raw destroy endpoint", path)
	}
	if form.Get("signature") == "" || form.Get("api_key") != "test-key" {
		t.Errorf("form = %v, want it signed and keyed", form)
	}
	if form.Get("type") != "authenticated" {
		t.Errorf("type = %q, want authenticated for a private asset", form.Get("type"))
	}
}

// TestDeleteSurfacesProviderFailures: a failed delete must not read as a
// success, or a file would outlive the record governing it.
func TestDeleteSurfacesProviderFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	original := baseAPIURL
	baseAPIURL = srv.URL
	defer func() { baseAPIURL = original }()

	err := testClient().Delete(context.Background(), ports.Asset{
		PublicID: "abc", ResourceType: document.ResourceRaw,
	})
	if err == nil {
		t.Fatal("Delete reported success on a 500")
	}
	var gatewayErr *ports.GatewayError
	if !asGateway(err, &gatewayErr) {
		t.Fatalf("err = %v, want a *ports.GatewayError", err)
	}
	if gatewayErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", gatewayErr.StatusCode)
	}
}

// asGateway is errors.As, spelled out to keep the import list short.
func asGateway(err error, target **ports.GatewayError) bool {
	for err != nil {
		if typed, ok := err.(*ports.GatewayError); ok {
			*target = typed
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}

// TestDeleteHandlesAnUnreachableProvider.
func TestDeleteHandlesAnUnreachableProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	original := baseAPIURL
	baseAPIURL = srv.URL
	srv.Close()
	defer func() { baseAPIURL = original }()

	err := testClient().Delete(context.Background(), ports.Asset{PublicID: "abc"})
	if err == nil {
		t.Fatal("Delete reported success against an unreachable provider")
	}
}

// TestUploadPostsSignedMultipart: a file the API produced goes up under the
// same signed parameters a browser upload would carry, into the private
// folder the documents slice chose.
func TestUploadPostsSignedMultipart(t *testing.T) {
	var gotPath, gotFolder, gotType, gotSignature, gotFilename string
	var gotFile []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		gotFolder = r.FormValue("folder")
		gotType = r.FormValue("type")
		gotSignature = r.FormValue("signature")
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("form file: %v", err)
		} else {
			defer func() { _ = file.Close() }()
			gotFilename = header.Filename
			gotFile, _ = io.ReadAll(file)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"public_id":"terios/clients/client-1/signed-forms/abc","bytes":9}`))
	}))
	defer srv.Close()

	original := baseAPIURL
	baseAPIURL = srv.URL
	defer func() { baseAPIURL = original }()

	asset, err := testClient().Upload(context.Background(),
		ports.UploadParams{
			Folder:       "terios/clients/client-1/signed-forms",
			ResourceType: document.ResourceRaw,
			Private:      true,
		},
		ports.UploadFile{Filename: "agreement.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4\n")},
	)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	if !strings.HasSuffix(gotPath, "/terios/raw/upload") {
		t.Errorf("path = %q, want the raw upload endpoint", gotPath)
	}
	if gotFolder != "terios/clients/client-1/signed-forms" {
		t.Errorf("folder = %q, want the client's signed-forms folder", gotFolder)
	}
	// "authenticated" is what makes the file unreachable by public URL —
	// a signed agreement must never be guessable.
	if gotType != "authenticated" {
		t.Errorf("type = %q, want authenticated", gotType)
	}
	if gotSignature == "" {
		t.Error("the upload should be signed")
	}
	if gotFilename != "agreement.pdf" || string(gotFile) != "%PDF-1.4\n" {
		t.Errorf("file = %q/%q, want the PDF as sent", gotFilename, gotFile)
	}
	if asset.PublicID != "terios/clients/client-1/signed-forms/abc" || asset.Bytes != 9 {
		t.Errorf("asset = %+v, want what the store reported", asset)
	}
}

// TestUploadReportsAGatewayFailure: the store refusing must surface as a
// gateway error, not a generic one, so callers can tell it apart.
func TestUploadReportsAGatewayFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad signature"}}`))
	}))
	defer srv.Close()

	original := baseAPIURL
	baseAPIURL = srv.URL
	defer func() { baseAPIURL = original }()

	_, err := testClient().Upload(context.Background(),
		ports.UploadParams{Folder: "terios/cms", ResourceType: document.ResourceRaw},
		ports.UploadFile{Filename: "a.pdf", Data: []byte("x")},
	)
	var gatewayErr *ports.GatewayError
	if !errors.As(err, &gatewayErr) || gatewayErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("err = %v, want a GatewayError carrying 401", err)
	}
}
