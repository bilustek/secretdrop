package auth

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleAudience           = "https://appleid.apple.com"
	appleClientSecretExpiry = 180 * 24 * time.Hour // 6 months
)

// GenerateAppleClientSecret creates an ES256 JWT used as the client_secret
// for Apple's token endpoint. Apple requires this instead of a static secret.
func (s *Service) GenerateAppleClientSecret() (string, error) {
	// Decode base64-encoded PEM key
	pemBytes, err := base64.StdEncoding.DecodeString(s.applePrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode apple private key: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errors.New("decode apple PEM block: no valid PEM data found")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse apple private key: %w", err)
	}

	now := time.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    s.appleTeamID,
		Subject:   s.appleClientID,
		Audience:  jwt.ClaimStrings{appleAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(appleClientSecretExpiry)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.appleKeyID

	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign apple client secret: %w", err)
	}

	return signed, nil
}
