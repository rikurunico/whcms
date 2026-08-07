// Package crypto implements ports.Encryptor (AES-256-GCM) and
// ports.PasswordHasher (Argon2id PHC strings).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Encryptor is an AES-256-GCM ports.Encryptor. Ciphertext format:
// base64(nonce || sealed).
type Encryptor struct {
	aead cipher.AEAD
}

// NewEncryptor builds an Encryptor from a 32-byte key.
func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	return &Encryptor{aead: aead}, nil
}

// Encrypt seals plaintext with a random nonce; output is base64.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("crypto: nonce: %w", err)
	}
	sealed := e.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. Tampered or truncated inputs return an error.
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: base64: %w", err)
	}
	ns := e.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("crypto: ciphertext too short")
	}
	plain, err := e.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plain), nil
}
