package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

// Encryptor provides AES-256-GCM encryption/decryption for event data.
// If nil, all operations are no-ops (pass-through).
type Encryptor struct {
	gcm cipher.AEAD
}

// NewEncryptor creates an Encryptor from a hex-encoded 32-byte key.
// Returns nil if hexKey is empty (encryption disabled).
// Returns error if hexKey is invalid.
func NewEncryptor(hexKey string) (*Encryptor, error) {
	if hexKey == "" {
		return nil, nil
	}

	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: not valid hex: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key: must be 32 bytes (64 hex chars), got %d bytes", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns nonce || ciphertext (nonce is 12 bytes prepended).
// If encryptor is nil, returns plaintext unchanged.
func (e *Encryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if e == nil {
		return plaintext, nil
	}

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Seal appends ciphertext+tag to nonce
	return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts AES-256-GCM ciphertext (nonce || ciphertext).
// If data is too short or decryption fails, returns raw data as-is
// (backward compatibility with unencrypted events).
// If encryptor is nil, returns ciphertext unchanged.
func (e *Encryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if e == nil {
		return ciphertext, nil
	}

	nonceSize := e.gcm.NonceSize()
	// Too short to be encrypted: nonce (12) + tag (16) = 28 bytes minimum
	if len(ciphertext) < nonceSize+e.gcm.Overhead() {
		return ciphertext, nil
	}

	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		// Decryption failed — likely unencrypted data, return as-is
		return ciphertext, nil
	}

	return plaintext, nil
}
