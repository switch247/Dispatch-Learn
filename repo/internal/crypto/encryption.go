package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"
)

type Encryptor struct {
	mu  sync.RWMutex
	key []byte
}

func NewEncryptor(masterKey string) (*Encryptor, error) {
	key := []byte(masterKey)
	if len(key) != 32 {
		return nil, errors.New("master key must be 32 bytes for AES-256")
	}
	return &Encryptor{key: key}, nil
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *Encryptor) Decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func (e *Encryptor) RotateKey(newKey string) error {
	if len([]byte(newKey)) != 32 {
		return errors.New("new key must be 32 bytes for AES-256")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.key = []byte(newKey)
	return nil
}

// MaskString masks sensitive data, showing first and last chars
func MaskString(s string, visibleChars int) string {
	if len(s) <= visibleChars*2 {
		return "****"
	}
	prefix := s[:visibleChars]
	suffix := s[len(s)-visibleChars:]
	masked := prefix
	for i := 0; i < len(s)-visibleChars*2; i++ {
		masked += "*"
	}
	masked += suffix
	return masked
}
