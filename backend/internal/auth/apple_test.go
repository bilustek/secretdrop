package auth_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
)

// generateTestP8Key creates a base64-encoded ECDSA P-256 private key in PEM format for testing.
func generateTestP8Key(t *testing.T) (string, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	})

	return base64.StdEncoding.EncodeToString(pemBlock), key
}

func TestGenerateAppleClientSecret(t *testing.T) {
	t.Parallel()

	b64Key, privKey := generateTestP8Key(t)

	svc, err := auth.New("test-secret", auth.WithAppleCredentials(
		"com.bilustek.secretdrop.web",
		"ABCDE12345",
		"KEY123",
		b64Key,
	))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	secret, err := svc.GenerateAppleClientSecret()
	if err != nil {
		t.Fatalf("GenerateAppleClientSecret() error = %v", err)
	}

	if secret == "" {
		t.Fatal("GenerateAppleClientSecret() returned empty string")
	}

	// Parse the JWT and verify claims using the public key
	token, err := jwt.Parse(secret, func(token *jwt.Token) (any, error) {
		return &privKey.PublicKey, nil
	})
	if err != nil {
		t.Fatalf("parse client secret JWT: %v", err)
	}

	if !token.Valid {
		t.Fatal("client secret JWT is not valid")
	}

	// Verify algorithm
	if token.Method.Alg() != "ES256" {
		t.Errorf("algorithm = %q; want %q", token.Method.Alg(), "ES256")
	}

	// Verify kid header
	kid, ok := token.Header["kid"].(string)
	if !ok || kid != "KEY123" {
		t.Errorf("kid = %q; want %q", kid, "KEY123")
	}

	// Verify claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("failed to cast claims")
	}

	if claims["iss"] != "ABCDE12345" {
		t.Errorf("iss = %q; want %q", claims["iss"], "ABCDE12345")
	}

	// Use GetAudience() for reliable verification since jwt.ClaimStrings serializes as an array
	aud, err := claims.GetAudience()
	if err != nil {
		t.Fatalf("get audience: %v", err)
	}

	if len(aud) != 1 || aud[0] != "https://appleid.apple.com" {
		t.Errorf("aud = %v; want %v", aud, []string{"https://appleid.apple.com"})
	}

	if claims["sub"] != "com.bilustek.secretdrop.web" {
		t.Errorf("sub = %q; want %q", claims["sub"], "com.bilustek.secretdrop.web")
	}

	// Verify expiry is approximately 6 months
	exp, err := claims.GetExpirationTime()
	if err != nil {
		t.Fatalf("get expiration: %v", err)
	}

	expiry := time.Until(exp.Time)
	if expiry < 179*24*time.Hour || expiry > 181*24*time.Hour {
		t.Errorf("expiry = %v; want ~180 days", expiry)
	}
}

func TestGenerateAppleClientSecret_InvalidKey(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithAppleCredentials(
		"com.bilustek.secretdrop.web",
		"ABCDE12345",
		"KEY123",
		base64.StdEncoding.EncodeToString([]byte("not-a-pem-key")),
	))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	_, err = svc.GenerateAppleClientSecret()
	if err == nil {
		t.Fatal("GenerateAppleClientSecret() should fail with invalid key")
	}
}
