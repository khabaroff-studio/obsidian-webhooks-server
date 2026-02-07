package services

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func validHexKey() string {
	// 32 bytes = 64 hex chars
	return "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}

func TestNewEncryptor_EmptyKey(t *testing.T) {
	enc, err := NewEncryptor("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enc != nil {
		t.Fatal("expected nil encryptor for empty key")
	}
}

func TestNewEncryptor_ValidKey(t *testing.T) {
	enc, err := NewEncryptor(validHexKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enc == nil {
		t.Fatal("expected non-nil encryptor")
	}
}

func TestNewEncryptor_InvalidHex(t *testing.T) {
	_, err := NewEncryptor("not-hex")
	if err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestNewEncryptor_WrongLength(t *testing.T) {
	// 16 bytes = 32 hex chars (AES-128, not AES-256)
	_, err := NewEncryptor("0123456789abcdef0123456789abcdef")
	if err == nil {
		t.Fatal("expected error for wrong key length")
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	enc, err := NewEncryptor(validHexKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	plaintext := []byte(`{"title":"Test Note","content":"Hello world"}`)

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	// Ciphertext should differ from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	// Ciphertext should be longer (nonce + tag overhead)
	if len(ciphertext) <= len(plaintext) {
		t.Fatalf("ciphertext too short: %d <= %d", len(ciphertext), len(plaintext))
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted != plaintext: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptDecrypt_EmptyPayload(t *testing.T) {
	enc, err := NewEncryptor(validHexKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ciphertext, err := enc.Encrypt([]byte{})
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty bytes, got %d bytes", len(decrypted))
	}
}

func TestNilEncryptor_Passthrough(t *testing.T) {
	var enc *Encryptor

	plaintext := []byte("hello")

	encrypted, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if !bytes.Equal(encrypted, plaintext) {
		t.Fatal("nil encryptor should pass through on encrypt")
	}

	decrypted, err := enc.Decrypt(plaintext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("nil encryptor should pass through on decrypt")
	}
}

func TestDecrypt_UnencryptedData_Passthrough(t *testing.T) {
	enc, err := NewEncryptor(validHexKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Raw JSON — not encrypted, should pass through
	raw := []byte(`{"title":"Old Event"}`)
	decrypted, err := enc.Decrypt(raw)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, raw) {
		t.Fatal("unencrypted data should pass through")
	}
}

func TestDecrypt_TooShortData_Passthrough(t *testing.T) {
	enc, err := NewEncryptor(validHexKey())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	short := []byte("hi")
	decrypted, err := enc.Decrypt(short)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, short) {
		t.Fatal("short data should pass through")
	}
}

func TestDecrypt_WrongKey_Passthrough(t *testing.T) {
	enc1, _ := NewEncryptor(validHexKey())

	// Different key
	key2 := make([]byte, 32)
	key2[0] = 0xff
	enc2, _ := NewEncryptor(hex.EncodeToString(key2))

	plaintext := []byte("secret data")
	ciphertext, _ := enc1.Encrypt(plaintext)

	// Decrypt with wrong key — should return ciphertext as-is (fallback)
	decrypted, err := enc2.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, ciphertext) {
		t.Fatal("wrong key decrypt should return ciphertext as-is")
	}
}

func TestEncrypt_UniqueNonces(t *testing.T) {
	enc, _ := NewEncryptor(validHexKey())
	plaintext := []byte("same data")

	ct1, _ := enc.Encrypt(plaintext)
	ct2, _ := enc.Encrypt(plaintext)

	if bytes.Equal(ct1, ct2) {
		t.Fatal("two encryptions of same data should produce different ciphertexts (different nonces)")
	}
}
