package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	keySize   = 32 // AES-256
	tokenSize = 16 // 16 bytes → base64url encoded token
)

// GenerateRandomKey creates a cryptographically secure random key of keySize bytes.
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate random key: %w", err)
	}

	return key, nil
}

// GenerateToken creates a random token encoded as base64url (no padding).
func GenerateToken() (string, error) {
	b := make([]byte, tokenSize)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DeriveKey uses HKDF-SHA256 to derive a 32-byte key from the input key material
// and email as info parameter.
func DeriveKey(randomKey []byte, email string) ([]byte, error) {
	derived, err := hkdf.Key(sha256.New, randomKey, nil, email, keySize)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	return derived, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
// Returns ciphertext (with appended tag) and nonce.
func Encrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the given key and nonce.
func Decrypt(key, ciphertext, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

// HashEmail returns the hex-encoded SHA-256 hash of an email address.
func HashEmail(email string) string {
	h := sha256.Sum256([]byte(email))

	return hex.EncodeToString(h[:])
}

// EncodeKey encodes a key as base64url (no padding) for URL fragment usage.
func EncodeKey(key []byte) string {
	return base64.RawURLEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64url-encoded key.
func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	return key, nil
}
