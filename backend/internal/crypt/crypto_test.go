package crypt_test

import (
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/crypt"
)

func TestGenerateRandomKey(t *testing.T) {
	t.Parallel()

	key, err := crypt.GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey() error = %v", err)
	}

	if len(key) != 32 {
		t.Errorf("key length = %d; want 32", len(key))
	}

	key2, err := crypt.GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey() error = %v", err)
	}

	if string(key) == string(key2) {
		t.Error("two generated keys should not be equal")
	}
}

func TestGenerateToken(t *testing.T) {
	t.Parallel()

	token, err := crypt.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == "" {
		t.Error("token should not be empty")
	}

	token2, err := crypt.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	if token == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	derived1, err := crypt.DeriveKey(key, "test@example.com")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	derived2, err := crypt.DeriveKey(key, "test@example.com")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	if string(derived1) != string(derived2) {
		t.Error("same inputs should produce same derived key")
	}
}

func TestDeriveKeyDifferentEmails(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	derived1, err := crypt.DeriveKey(key, "alice@example.com")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	derived2, err := crypt.DeriveKey(key, "bob@example.com")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	if string(derived1) == string(derived2) {
		t.Error("different emails should produce different derived keys")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("RESEND_API_KEY=re_xxxxx")

	ciphertext, nonce, err := crypt.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := crypt.Decrypt(key, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypt() = %q; want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	t.Parallel()

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("secret data")

	ciphertext, nonce, err := crypt.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	wrongKey := make([]byte, 32)
	for i := range wrongKey {
		wrongKey[i] = byte(i + 1)
	}

	_, err = crypt.Decrypt(wrongKey, ciphertext, nonce)
	if err == nil {
		t.Error("Decrypt() with wrong key should fail")
	}
}

func TestHashEmail(t *testing.T) {
	t.Parallel()

	hash1 := crypt.HashEmail("test@example.com")
	hash2 := crypt.HashEmail("test@example.com")
	hash3 := crypt.HashEmail("other@example.com")

	if hash1 != hash2 {
		t.Error("same email should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different emails should produce different hashes")
	}

	if len(hash1) != 64 {
		t.Errorf("hash length = %d; want 64 (hex-encoded SHA-256)", len(hash1))
	}
}

func TestEncodeDecodeKeyRoundTrip(t *testing.T) {
	t.Parallel()

	original := make([]byte, 32)
	for i := range original {
		original[i] = byte(i)
	}

	encoded := crypt.EncodeKey(original)

	decoded, err := crypt.DecodeKey(encoded)
	if err != nil {
		t.Fatalf("DecodeKey() error = %v", err)
	}

	if string(decoded) != string(original) {
		t.Error("round-trip encode/decode should preserve key")
	}
}

func TestFullCryptoFlow(t *testing.T) {
	t.Parallel()

	addr := "vigo@me.com"
	secretText := "DB_PASSWORD=super-secret-123"

	randomKey, err := crypt.GenerateRandomKey()
	if err != nil {
		t.Fatalf("GenerateRandomKey() error = %v", err)
	}

	finalKey, err := crypt.DeriveKey(randomKey, addr)
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	ciphertext, nonce, err := crypt.Encrypt(finalKey, []byte(secretText))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encoded := crypt.EncodeKey(randomKey)

	decodedKey, err := crypt.DecodeKey(encoded)
	if err != nil {
		t.Fatalf("DecodeKey() error = %v", err)
	}

	recoveredKey, err := crypt.DeriveKey(decodedKey, addr)
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	decrypted, err := crypt.Decrypt(recoveredKey, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != secretText {
		t.Errorf("full flow: got %q; want %q", decrypted, secretText)
	}

	wrongKey, err := crypt.DeriveKey(decodedKey, "wrong@email.com")
	if err != nil {
		t.Fatalf("DeriveKey() error = %v", err)
	}

	_, err = crypt.Decrypt(wrongKey, ciphertext, nonce)
	if err == nil {
		t.Error("decrypt with wrong email-derived key should fail")
	}
}
