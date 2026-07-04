package service

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	for _, pt := range []string{"", "hello", "a much longer secret value 12345"} {
		ct, err := encrypt(testKey, pt)
		require.NoError(t, err)
		if pt != "" {
			assert.NotEqual(t, pt, ct, "ciphertext must differ from plaintext")
		}
		got, err := decrypt(testKey, ct)
		require.NoError(t, err)
		assert.Equal(t, pt, got)
	}
}

func TestEncrypt_NonDeterministic(t *testing.T) {
	a, err := encrypt(testKey, "same")
	require.NoError(t, err)
	b, err := encrypt(testKey, "same")
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "random nonce should make ciphertexts differ")
}

func TestDecrypt_Empty(t *testing.T) {
	got, err := decrypt(testKey, "")
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestDecrypt_Errors(t *testing.T) {
	// Not valid base64.
	_, err := decrypt(testKey, "!!!not base64!!!")
	assert.Error(t, err)

	// Valid base64 but shorter than a nonce.
	short := base64.StdEncoding.EncodeToString([]byte("x"))
	_, err = decrypt(testKey, short)
	assert.Error(t, err)

	// Tampered ciphertext fails the GCM auth tag.
	ct, err := encrypt(testKey, "secret")
	require.NoError(t, err)
	raw, _ := base64.StdEncoding.DecodeString(ct)
	raw[len(raw)-1] ^= 0xff
	_, err = decrypt(testKey, base64.StdEncoding.EncodeToString(raw))
	assert.Error(t, err)
}

func TestEncrypt_BadKey(t *testing.T) {
	_, err := encrypt([]byte("short"), "x")
	assert.Error(t, err)
}
