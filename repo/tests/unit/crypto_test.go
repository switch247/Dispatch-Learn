package unit

import (
	"testing"

	"dispatchlearn/internal/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	enc, err := crypto.NewEncryptor("0123456789abcdef0123456789abcdef")
	require.NoError(t, err)

	t.Run("encrypt and decrypt text", func(t *testing.T) {
		plaintext := "sensitive payment reference"
		encrypted, err := enc.Encrypt(plaintext)
		require.NoError(t, err)
		assert.NotEqual(t, plaintext, encrypted)

		decrypted, err := enc.Decrypt(encrypted)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})

	t.Run("encrypt empty string", func(t *testing.T) {
		encrypted, err := enc.Encrypt("")
		require.NoError(t, err)
		assert.Equal(t, "", encrypted)
	})

	t.Run("different encryptions produce different ciphertext", func(t *testing.T) {
		text := "same text"
		enc1, _ := enc.Encrypt(text)
		enc2, _ := enc.Encrypt(text)
		assert.NotEqual(t, enc1, enc2) // Due to random nonce
	})

	t.Run("invalid key length rejected", func(t *testing.T) {
		_, err := crypto.NewEncryptor("tooshort")
		assert.Error(t, err)
	})

	t.Run("key rotation", func(t *testing.T) {
		plaintext := "before rotation"
		encrypted, err := enc.Encrypt(plaintext)
		require.NoError(t, err)

		newKey := "abcdef0123456789abcdef0123456789"
		err = enc.RotateKey(newKey)
		require.NoError(t, err)

		// Old ciphertext should fail with new key
		_, err = enc.Decrypt(encrypted)
		assert.Error(t, err)

		// New encryption should work
		encrypted2, err := enc.Encrypt(plaintext)
		require.NoError(t, err)
		decrypted, err := enc.Decrypt(encrypted2)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestMaskString(t *testing.T) {
	t.Run("mask long string", func(t *testing.T) {
		masked := crypto.MaskString("123 Main Street", 3)
		assert.Equal(t, "123*********eet", masked)
	})

	t.Run("mask short string", func(t *testing.T) {
		masked := crypto.MaskString("abc", 3)
		assert.Equal(t, "****", masked)
	})
}
