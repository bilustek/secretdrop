package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleAudience           = "https://appleid.apple.com"
	appleClientSecretExpiry = 180 * 24 * time.Hour // 6 months
	appleJWKSURL            = "https://appleid.apple.com/auth/keys"
)

// AppleIDTokenInfo holds the verified claims from an Apple ID token.
type AppleIDTokenInfo struct {
	Sub   string
	Email string
}

// appleJWKS represents Apple's JSON Web Key Set response.
type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

// appleJWK represents a single key in Apple's JWKS response.
type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

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

// VerifyAppleIDToken verifies an Apple ID token JWT using Apple's JWKS endpoint.
// The jwksURL parameter allows overriding the JWKS URL for testing.
func VerifyAppleIDToken(ctx context.Context, idToken, expectedAud, jwksURL string) (*AppleIDTokenInfo, error) {
	if jwksURL == "" {
		jwksURL = appleJWKSURL
	}

	keys, err := fetchAppleJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetch apple JWKS: %w", err)
	}

	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid header")
		}

		pubKey, findErr := findApplePublicKey(keys, kid)
		if findErr != nil {
			return nil, findErr
		}

		return pubKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify apple id token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("apple id token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unexpected claims type")
	}

	// Validate audience
	aud, err := claims.GetAudience()
	if err != nil || len(aud) == 0 || aud[0] != expectedAud {
		return nil, fmt.Errorf("audience mismatch: got %v, want %q", aud, expectedAud)
	}

	// Validate issuer
	iss, err := claims.GetIssuer()
	if err != nil || iss != appleAudience {
		return nil, fmt.Errorf("issuer mismatch: got %q, want %q", iss, appleAudience)
	}

	var sub string
	if v, ok := claims["sub"].(string); ok {
		sub = v
	}

	var email string
	if v, ok := claims["email"].(string); ok {
		email = v
	}

	return &AppleIDTokenInfo{
		Sub:   sub,
		Email: email,
	}, nil
}

func fetchAppleJWKS(ctx context.Context, jwksURL string) (*appleJWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS returned status %d", resp.StatusCode)
	}

	var jwks appleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	return &jwks, nil
}

func findApplePublicKey(jwks *appleJWKS, kid string) (*ecdsa.PublicKey, error) {
	for _, key := range jwks.Keys {
		if key.Kid == kid && key.Kty == "EC" && key.Crv == "P-256" {
			xBytes, err := base64.RawURLEncoding.DecodeString(key.X)
			if err != nil {
				return nil, fmt.Errorf("decode JWK x: %w", err)
			}

			yBytes, err := base64.RawURLEncoding.DecodeString(key.Y)
			if err != nil {
				return nil, fmt.Errorf("decode JWK y: %w", err)
			}

			return &ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     new(big.Int).SetBytes(xBytes),
				Y:     new(big.Int).SetBytes(yBytes),
			}, nil
		}
	}

	return nil, fmt.Errorf("no matching key found for kid %q", kid)
}
