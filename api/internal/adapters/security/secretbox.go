package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// SecretBox protects reversible TOTP seeds with AES-256-GCM. The key is
// supplied by the deployment secret store and never persisted with users.
type SecretBox struct{ aead cipher.AEAD }

func NewSecretBox(encodedKey string) (*SecretBox, error) {
	key, err := base64.RawStdEncoding.DecodeString(encodedKey)
	if err != nil {
		key, err = base64.StdEncoding.DecodeString(encodedKey)
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("MFA_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create MFA cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create MFA GCM: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate MFA nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b *SecretBox) Decrypt(encoded string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(sealed) < b.aead.NonceSize() {
		return "", fmt.Errorf("invalid encrypted MFA secret")
	}
	nonce, ciphertext := sealed[:b.aead.NonceSize()], sealed[b.aead.NonceSize():]
	plain, err := b.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt MFA secret: %w", err)
	}
	return string(plain), nil
}
