package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	keySize = 32
	prefix  = "v1:"
)

var (
	ErrInvalidKey        = errors.New("invalid secrets key")
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

type Cipher struct {
	aead cipher.AEAD
}

// NewCipher builds an AES-256-GCM cipher from a base64 master key.
func NewCipher(base64Key string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil {
		return nil, fmt.Errorf("%w: must be base64", ErrInvalidKey)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("%w: must decode to 32 bytes", ErrInvalidKey)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt seals plaintext with a random nonce and a versioned envelope.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if c == nil {
		return "", ErrInvalidKey
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := append(nonce, sealed...)
	return prefix + base64.StdEncoding.EncodeToString(payload), nil
}

// Decrypt opens a versioned ciphertext envelope.
func (c *Cipher) Decrypt(ciphertext string) (string, error) {
	if c == nil {
		return "", ErrInvalidKey
	}
	if !strings.HasPrefix(ciphertext, prefix) {
		return "", ErrInvalidCiphertext
	}

	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, prefix))
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) <= nonceSize {
		return "", ErrInvalidCiphertext
	}

	plaintext, err := c.aead.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return "", ErrInvalidCiphertext
	}
	return string(plaintext), nil
}
