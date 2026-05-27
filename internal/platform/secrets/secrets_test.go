package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
)

// testKey returns a deterministic base64-encoded 32-byte key for tests.
func testKey(fill byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

// TestCipherEncryptDecrypt verifies encrypted values round-trip and use random nonces.
func TestCipherEncryptDecrypt(t *testing.T) {
	cipher, err := NewCipher(testKey(1))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}

	first, err := cipher.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt first: %v", err)
	}
	second, err := cipher.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt second: %v", err)
	}
	if first == second {
		t.Fatal("expected non-deterministic ciphertext")
	}

	plain, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "secret" {
		t.Fatalf("expected secret, got %q", plain)
	}
}

// TestCipherRejectsWrongKeyAndInvalidFormat verifies invalid envelopes and wrong keys fail.
func TestCipherRejectsWrongKeyAndInvalidFormat(t *testing.T) {
	cipher, err := NewCipher(testKey(1))
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	ciphertext, err := cipher.Encrypt("secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	wrong, err := NewCipher(testKey(2))
	if err != nil {
		t.Fatalf("new wrong cipher: %v", err)
	}
	if _, err := wrong.Decrypt(ciphertext); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected invalid ciphertext for wrong key, got %v", err)
	}
	if _, err := cipher.Decrypt("not-versioned"); !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("expected invalid ciphertext for bad format, got %v", err)
	}
}
