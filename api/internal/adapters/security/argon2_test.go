package security

import (
	"strings"
	"testing"
)

func TestArgon2RoundTrip(t *testing.T) {
	h := NewArgon2Hasher()
	encoded, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Errorf("unexpected encoding prefix: %q", encoded)
	}
	if strings.Contains(encoded, "correct horse") {
		t.Error("encoded hash leaks plaintext")
	}

	ok, err := h.Verify(encoded, "correct horse battery")
	if err != nil || !ok {
		t.Errorf("Verify(same password) = %v, %v; want true, nil", ok, err)
	}
}

func TestArgon2SaltsAreRandom(t *testing.T) {
	h := NewArgon2Hasher()
	a, err := h.Hash("same password here")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	b, err := h.Hash("same password here")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if a == b {
		t.Error("identical hashes for identical passwords: salt is not random")
	}
}

func TestArgon2RejectsWrongPassword(t *testing.T) {
	h := NewArgon2Hasher()
	encoded, err := h.Hash("right password ok")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := h.Verify(encoded, "wrong password ok")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("wrong password accepted")
	}
}

func TestArgon2RejectsTamperedHash(t *testing.T) {
	h := NewArgon2Hasher()
	encoded, err := h.Hash("some password ok!")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	// Flip the final key character — same length, valid base64, wrong key.
	tampered := encoded[:len(encoded)-2] + "AA"
	ok, err := h.Verify(tampered, "some password ok!")
	if err != nil {
		t.Fatalf("Verify(tampered): %v", err)
	}
	if ok {
		t.Error("tampered hash verified successfully")
	}

	// Structurally malformed encodings must error, not panic.
	malformed := []string{
		"",
		"not-a-hash",
		"$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$a2V5",  // wrong variant
		"$argon2id$v=17$m=65536,t=3,p=2$c2FsdA$a2V5", // wrong version
		"$argon2id$v=19$m=65536,t=3$c2FsdA$a2V5",     // missing param
		"$argon2id$v=19$m=65536,t=3,p=2$!!!$a2V5",    // bad base64 salt
		"$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!",  // bad base64 key
	}
	for _, m := range malformed {
		if _, err := h.Verify(m, "some password ok!"); err == nil {
			t.Errorf("Verify(%q) did not error", m)
		}
	}
}
