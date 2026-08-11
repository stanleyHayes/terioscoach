// Package security is the outbound adapter for credential primitives:
// Argon2id password hashing and JWT/opaque-token issuance.
package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters per OWASP guidance for interactive logins.
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 2
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Argon2Hasher hashes passwords with Argon2id in the standard encoded
// `$argon2id$v=19$m=...,t=...,p=...$salt$key` form.
type Argon2Hasher struct{}

// NewArgon2Hasher returns the shared hasher; it holds no state.
func NewArgon2Hasher() *Argon2Hasher {
	return &Argon2Hasher{}
}

// Hash salts and hashes a password, returning the self-describing encoding.
func (Argon2Hasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("argon2 salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify recomputes the hash from the encoded parameters and compares in
// constant time. A mismatch is (false, nil); a malformed encoding is an error.
func (Argon2Hasher) Verify(encodedHash, password string) (bool, error) {
	memory, iterations, threads, salt, key, err := decodeArgon2(encodedHash)
	if err != nil {
		return false, err
	}
	candidate := argon2.IDKey([]byte(password), salt, iterations, memory, uint8(threads), uint32(len(key)))
	return subtle.ConstantTimeCompare(candidate, key) == 1, nil
}

// decodeArgon2 parses the standard encoded form back into parameters.
func decodeArgon2(encoded string) (memory, iterations, threads uint32, salt, key []byte, err error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, key]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return 0, 0, 0, nil, nil, fmt.Errorf("argon2: malformed encoded hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return 0, 0, 0, nil, nil, fmt.Errorf("argon2: unsupported version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("argon2: malformed parameters")
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("argon2: malformed salt")
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return 0, 0, 0, nil, nil, fmt.Errorf("argon2: malformed key")
	}
	return memory, iterations, threads, salt, key, nil
}
