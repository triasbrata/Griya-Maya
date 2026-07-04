package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// encrypt seals plaintext with AES-256-GCM under key (32 bytes) and returns a
// base64 (std) string of nonce||ciphertext. Used to keep connection secrets and
// tokens encrypted at rest in D1.
func encrypt(key []byte, plaintext string) (string, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// decrypt reverses encrypt. An empty ciphertext decrypts to the empty string so
// callers can round-trip optional/unset secret fields.
func decrypt(key []byte, ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	gcm, err := newGCM(key)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto decode: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("crypto open: %w", err)
	}
	return string(plain), nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
