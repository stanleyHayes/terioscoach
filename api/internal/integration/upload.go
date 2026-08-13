package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/xcreativs/terios/api/internal/ports"
)

// uploadResult is the part of Cloudinary's upload response the apps use.
type uploadResult struct {
	PublicID  string `json:"public_id"`
	SecureURL string `json:"secure_url"`
	Bytes     int64  `json:"bytes"`
}

// postSignedUpload performs the upload a browser would perform, using the
// signature the API minted.
//
// This is the step no unit test can stand in for: the signature is
// computed by our code and checked by Cloudinary's, and the only way to
// know the two agree is to ask Cloudinary.
func postSignedUpload(ctx context.Context, signed ports.SignedUpload, base64PNG string) (uploadResult, error) {
	raw, err := base64.StdEncoding.DecodeString(base64PNG)
	if err != nil {
		return uploadResult{}, fmt.Errorf("decode fixture: %w", err)
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for name, value := range signed.Fields {
		if err := form.WriteField(name, value); err != nil {
			return uploadResult{}, fmt.Errorf("write field %s: %w", name, err)
		}
	}
	part, err := form.CreateFormFile("file", "pixel.png")
	if err != nil {
		return uploadResult{}, fmt.Errorf("create file part: %w", err)
	}
	if _, err := part.Write(raw); err != nil {
		return uploadResult{}, fmt.Errorf("write file part: %w", err)
	}
	if err := form.Close(); err != nil {
		return uploadResult{}, fmt.Errorf("close form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, signed.URL, &body)
	if err != nil {
		return uploadResult{}, err
	}
	req.Header.Set("Content-Type", form.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return uploadResult{}, err
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return uploadResult{}, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// Cloudinary's message is genuinely useful here ("Invalid
		// Signature", "Folder is not allowed") and carries no secret.
		return uploadResult{}, fmt.Errorf("upload: status %d: %s", res.StatusCode, payload)
	}

	var out uploadResult
	if err := json.Unmarshal(payload, &out); err != nil {
		return uploadResult{}, fmt.Errorf("decode upload response: %w", err)
	}
	return out, nil
}
