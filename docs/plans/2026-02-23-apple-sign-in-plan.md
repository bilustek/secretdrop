# Apple Sign-In Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Apple as a third OAuth provider (web-only) alongside Google and GitHub.

**Architecture:** Apple Sign-In uses `oauth2.Config` with a dynamically generated ES256 JWT as client_secret. The callback is POST (form_post), not GET. The id_token JWT is validated via Apple's JWKS endpoint. No new dependencies needed — `golang-jwt/jwt/v5` (existing) handles ES256 signing, and `crypto/ecdsa` (stdlib) handles key parsing.

**Tech Stack:** Go 1.26, golang-jwt/jwt/v5, golang.org/x/oauth2, crypto/ecdsa, React 19, TypeScript

---

### Task 1: Config — Add Apple environment variables

**Files:**
- Modify: `backend/internal/config/config.go:25-54` (Config struct)
- Modify: `backend/internal/config/config.go:198-268` (getters)
- Modify: `backend/internal/config/config.go:274-358` (Load function)
- Test: `backend/internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/config/config_test.go`:

```go
func TestAppleConfigDefaults(t *testing.T) {
	clearAllEnvVars(t)
	t.Setenv("GOLANG_ENV", "development")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppleClientID() != "" {
		t.Errorf("AppleClientID() = %q; want empty", cfg.AppleClientID())
	}

	if cfg.AppleTeamID() != "" {
		t.Errorf("AppleTeamID() = %q; want empty", cfg.AppleTeamID())
	}

	if cfg.AppleKeyID() != "" {
		t.Errorf("AppleKeyID() = %q; want empty", cfg.AppleKeyID())
	}

	if cfg.ApplePrivateKey() != "" {
		t.Errorf("ApplePrivateKey() = %q; want empty", cfg.ApplePrivateKey())
	}
}

func TestAppleConfigFromEnvVars(t *testing.T) {
	clearAllEnvVars(t)
	t.Setenv("GOLANG_ENV", "development")
	t.Setenv("APPLE_CLIENT_ID", "com.bilustek.secretdrop.web")
	t.Setenv("APPLE_TEAM_ID", "ABCDE12345")
	t.Setenv("APPLE_KEY_ID", "KEY123")
	t.Setenv("APPLE_PRIVATE_KEY", "dGVzdC1rZXk=")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppleClientID() != "com.bilustek.secretdrop.web" {
		t.Errorf("AppleClientID() = %q; want %q", cfg.AppleClientID(), "com.bilustek.secretdrop.web")
	}

	if cfg.AppleTeamID() != "ABCDE12345" {
		t.Errorf("AppleTeamID() = %q; want %q", cfg.AppleTeamID(), "ABCDE12345")
	}

	if cfg.AppleKeyID() != "KEY123" {
		t.Errorf("AppleKeyID() = %q; want %q", cfg.AppleKeyID(), "KEY123")
	}

	if cfg.ApplePrivateKey() != "dGVzdC1rZXk=" {
		t.Errorf("ApplePrivateKey() = %q; want %q", cfg.ApplePrivateKey(), "dGVzdC1rZXk=")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/config/ -run TestAppleConfig -v`
Expected: FAIL — `cfg.AppleClientID undefined`

**Step 3: Write minimal implementation**

In `backend/internal/config/config.go`:

1. Add to Config struct (after line 53, before the `}` closing brace):

```go
appleClientID   string
appleTeamID     string
appleKeyID      string
applePrivateKey string
```

2. Add getters (after `SentryTracesSampleRate()` getter, line 268):

```go
// AppleClientID returns the Apple Services ID for Sign in with Apple.
func (c *Config) AppleClientID() string { return c.appleClientID }

// AppleTeamID returns the Apple Developer Team ID.
func (c *Config) AppleTeamID() string { return c.appleTeamID }

// AppleKeyID returns the Key ID for the Apple .p8 private key.
func (c *Config) AppleKeyID() string { return c.appleKeyID }

// ApplePrivateKey returns the base64-encoded Apple .p8 private key content.
func (c *Config) ApplePrivateKey() string { return c.applePrivateKey }
```

3. In `Load()`, after the Sentry env var reading (after line 331), add:

```go
c.appleClientID = os.Getenv("APPLE_CLIENT_ID")
c.appleTeamID = os.Getenv("APPLE_TEAM_ID")
c.appleKeyID = os.Getenv("APPLE_KEY_ID")
c.applePrivateKey = os.Getenv("APPLE_PRIVATE_KEY")
```

4. Also add `"APPLE_CLIENT_ID", "APPLE_TEAM_ID", "APPLE_KEY_ID", "APPLE_PRIVATE_KEY"` to the `clearAllEnvVars` helper in the test file.

**Note:** Apple env vars are NOT added to the production-required list — they're optional (app works without Apple Sign-In).

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/config/ -run TestAppleConfig -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/config/...
git add backend/internal/config/config.go backend/internal/config/config_test.go
git commit -m "feat: add Apple Sign-In config fields (APPLE_CLIENT_ID, APPLE_TEAM_ID, APPLE_KEY_ID, APPLE_PRIVATE_KEY)"
```

---

### Task 2: Auth Service — Add Apple credentials option

**Files:**
- Modify: `backend/internal/auth/auth.go:34-40` (Service struct)
- Modify: `backend/internal/auth/auth.go:98-105` (options)
- Test: `backend/internal/auth/auth_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/auth/auth_test.go`:

```go
func TestNew_WithAppleCredentials(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithAppleCredentials(
		"com.bilustek.secretdrop.web",
		"ABCDE12345",
		"KEY123",
		"base64-encoded-key",
	))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc == nil {
		t.Fatal("New() returned nil service")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run TestNew_WithAppleCredentials -v`
Expected: FAIL — `auth.WithAppleCredentials undefined`

**Step 3: Write minimal implementation**

In `backend/internal/auth/auth.go`:

1. Add to Service struct (after `frontendBaseURL string`, line 39):

```go
appleClientID  string
appleTeamID    string
appleKeyID     string
applePrivateKey string
```

2. Add option function (after `WithFrontendBaseURL`, line 105):

```go
// WithAppleCredentials sets the Apple Sign-In credentials.
func WithAppleCredentials(clientID, teamID, keyID, privateKey string) Option {
	return func(s *Service) error {
		s.appleClientID = clientID
		s.appleTeamID = teamID
		s.appleKeyID = keyID
		s.applePrivateKey = privateKey

		return nil
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/auth/ -run TestNew_WithAppleCredentials -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/...
git add backend/internal/auth/auth.go backend/internal/auth/auth_test.go
git commit -m "feat: add WithAppleCredentials option to auth service"
```

---

### Task 3: Apple client_secret JWT generation

**Files:**
- Create: `backend/internal/auth/apple.go`
- Test: `backend/internal/auth/apple_test.go`

This task implements the function that generates the ES256 JWT used as `client_secret` when exchanging the authorization code with Apple.

**Step 1: Write the failing test**

Create `backend/internal/auth/apple_test.go`:

```go
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

	if claims["aud"] != "https://appleid.apple.com" {
		t.Errorf("aud = %q; want %q", claims["aud"], "https://appleid.apple.com")
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
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run TestGenerateAppleClientSecret -v`
Expected: FAIL — `svc.GenerateAppleClientSecret undefined`

**Step 3: Write minimal implementation**

Create `backend/internal/auth/apple.go`:

```go
package auth

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleAudience          = "https://appleid.apple.com"
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
		return "", fmt.Errorf("decode apple PEM block: no valid PEM data found")
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
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/auth/ -run TestGenerateAppleClientSecret -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/...
git add backend/internal/auth/apple.go backend/internal/auth/apple_test.go
git commit -m "feat: add Apple client_secret JWT generation (ES256)"
```

---

### Task 4: Apple JWKS validation for id_token

**Files:**
- Modify: `backend/internal/auth/apple.go`
- Test: `backend/internal/auth/apple_test.go`

This task adds the function to verify Apple's `id_token` JWT using Apple's public JWKS endpoint.

**Step 1: Write the failing test**

Add to `backend/internal/auth/apple_test.go`:

```go
func TestVerifyAppleIDToken(t *testing.T) { //nolint:paralleltest // uses httptest server
	// Generate an ECDSA key pair for signing and verifying
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a mock JWKS endpoint
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Convert public key to JWK format manually
		pubKey := privKey.PublicKey
		xBytes := pubKey.X.Bytes()
		yBytes := pubKey.Y.Bytes()

		// Pad to 32 bytes for P-256
		for len(xBytes) < 32 {
			xBytes = append([]byte{0}, xBytes...)
		}
		for len(yBytes) < 32 {
			yBytes = append([]byte{0}, yBytes...)
		}

		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "EC",
					"kid": "test-kid",
					"use": "sig",
					"alg": "ES256",
					"crv": "P-256",
					"x":   base64.RawURLEncoding.EncodeToString(xBytes),
					"y":   base64.RawURLEncoding.EncodeToString(yBytes),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksServer.Close()

	// Create a signed ID token
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://appleid.apple.com",
		"aud":   "com.bilustek.secretdrop.web",
		"sub":   "apple-user-001",
		"email": "user@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	info, err := auth.VerifyAppleIDToken(context.Background(), idToken, "com.bilustek.secretdrop.web", jwksServer.URL)
	if err != nil {
		t.Fatalf("VerifyAppleIDToken() error = %v", err)
	}

	if info.Sub != "apple-user-001" {
		t.Errorf("Sub = %q; want %q", info.Sub, "apple-user-001")
	}

	if info.Email != "user@example.com" {
		t.Errorf("Email = %q; want %q", info.Email, "user@example.com")
	}
}

func TestVerifyAppleIDToken_WrongAudience(t *testing.T) { //nolint:paralleltest // uses httptest server
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pubKey := privKey.PublicKey
		xBytes := pubKey.X.Bytes()
		yBytes := pubKey.Y.Bytes()

		for len(xBytes) < 32 {
			xBytes = append([]byte{0}, xBytes...)
		}
		for len(yBytes) < 32 {
			yBytes = append([]byte{0}, yBytes...)
		}

		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "EC",
					"kid": "test-kid",
					"use": "sig",
					"alg": "ES256",
					"crv": "P-256",
					"x":   base64.RawURLEncoding.EncodeToString(xBytes),
					"y":   base64.RawURLEncoding.EncodeToString(yBytes),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))
	defer jwksServer.Close()

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://appleid.apple.com",
		"aud":   "com.other.app",
		"sub":   "apple-user-001",
		"email": "user@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	_, err = auth.VerifyAppleIDToken(context.Background(), idToken, "com.bilustek.secretdrop.web", jwksServer.URL)
	if err == nil {
		t.Fatal("VerifyAppleIDToken() should fail with wrong audience")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run TestVerifyAppleIDToken -v`
Expected: FAIL — `auth.VerifyAppleIDToken undefined`

**Step 3: Write minimal implementation**

Add to `backend/internal/auth/apple.go`:

```go
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"math/big"
	"net/http"
)

const appleJWKSURL = "https://appleid.apple.com/auth/keys"

// appleIDTokenInfo holds the verified claims from an Apple ID token.
type appleIDTokenInfo struct {
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

// VerifyAppleIDToken verifies an Apple ID token JWT using Apple's JWKS endpoint.
// The jwksURL parameter allows overriding the JWKS URL for testing.
func VerifyAppleIDToken(ctx context.Context, idToken, expectedAud, jwksURL string) (*appleIDTokenInfo, error) {
	if jwksURL == "" {
		jwksURL = appleJWKSURL
	}

	// Fetch JWKS
	keys, err := fetchAppleJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetch apple JWKS: %w", err)
	}

	// Parse and verify the token
	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid header")
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
		return nil, fmt.Errorf("apple id token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("unexpected claims type")
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

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	return &appleIDTokenInfo{
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
```

**Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/auth/ -run TestVerifyAppleIDToken -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/...
git add backend/internal/auth/apple.go backend/internal/auth/apple_test.go
git commit -m "feat: add Apple ID token JWKS verification"
```

---

### Task 5: Apple OAuth handlers (login + callback)

**Files:**
- Modify: `backend/internal/auth/apple.go`
- Test: `backend/internal/auth/apple_test.go`

**Step 1: Write the failing test for AppleConfig and HandleAppleLogin**

Add to `backend/internal/auth/apple_test.go`:

```go
func TestAppleConfig(t *testing.T) {
	t.Parallel()

	cfg := auth.AppleConfig("com.bilustek.secretdrop.web", "http://localhost/callback")

	if cfg.ClientID != "com.bilustek.secretdrop.web" {
		t.Errorf("ClientID = %q; want %q", cfg.ClientID, "com.bilustek.secretdrop.web")
	}

	if cfg.RedirectURL != "http://localhost/callback" {
		t.Errorf("RedirectURL = %q; want %q", cfg.RedirectURL, "http://localhost/callback")
	}

	if len(cfg.Scopes) != 2 {
		t.Fatalf("Scopes length = %d; want 2", len(cfg.Scopes))
	}

	wantScopes := []string{"name", "email"}
	for i, s := range wantScopes {
		if cfg.Scopes[i] != s {
			t.Errorf("Scopes[%d] = %q; want %q", i, cfg.Scopes[i], s)
		}
	}
}

func TestHandleAppleLogin_Redirect(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := auth.AppleConfig("com.bilustek.secretdrop.web", "http://localhost/callback")
	handler := svc.HandleAppleLogin(cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/apple", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	if !containsSubstring(location, "appleid.apple.com") {
		t.Errorf("Location = %q; want to contain %q", location, "appleid.apple.com")
	}

	if !containsSubstring(location, "response_mode=form_post") {
		t.Errorf("Location = %q; want to contain response_mode=form_post", location)
	}

	// Verify oauth_state cookie
	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c
			break
		}
	}

	if stateCookie == nil {
		t.Fatal("oauth_state cookie not set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run "TestAppleConfig|TestHandleAppleLogin" -v`
Expected: FAIL — `auth.AppleConfig undefined`

**Step 3: Write implementation**

Add to `backend/internal/auth/apple.go`:

```go
import (
	"crypto/subtle"
	"net/url"

	"golang.org/x/oauth2"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const appleAuthURL = "https://appleid.apple.com/auth/authorize"
const appleTokenURL = "https://appleid.apple.com/auth/token" //nolint:gosec // URL, not a credential

// appleUser represents the user JSON Apple sends on first consent.
type appleUser struct {
	Name struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
	Email string `json:"email"`
}

// AppleConfig creates an OAuth2 config for Apple.
// Note: ClientSecret is left empty — it's dynamically generated per request.
func AppleConfig(clientID, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: callbackURL,
		Scopes:      []string{"name", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  appleAuthURL,
			TokenURL: appleTokenURL,
		},
	}
}

// HandleAppleLogin redirects the user to Apple's Sign In page.
//
//nolint:revive // receiver unused but method needed for API consistency
func (s *Service) HandleAppleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := generateState()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)

			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			MaxAge:   oauthStateCookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			Path:     "/",
		})

		// Apple requires response_mode=form_post
		authURL := cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("response_mode", "form_post"))
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleAppleCallback handles the POST callback from Apple after user consent.
// Apple sends: code, state, user (JSON, first login only) as form POST.
func (s *Service) HandleAppleCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Parse form body
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "invalid_request", "message": "Invalid form data"},
			})

			return
		}

		// 2. Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		formState := r.FormValue("state")

		if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(formState)) != 1 {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]string{"type": "invalid_state", "message": "Invalid OAuth state"},
			})

			return
		}

		// Clear state cookie
		http.SetCookie(w, &http.Cookie{
			Name:   oauthStateCookieName,
			MaxAge: -1,
			Path:   "/",
		})

		// 3. Generate client_secret JWT
		clientSecret, err := s.GenerateAppleClientSecret()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate client secret"},
			})

			return
		}

		// 4. Exchange code for tokens (with dynamic client_secret)
		cfgWithSecret := *cfg
		cfgWithSecret.ClientSecret = clientSecret

		code := r.FormValue("code")

		token, err := cfgWithSecret.Exchange(r.Context(), code)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "oauth_failed", "message": "Failed to exchange authorization code"},
			})

			return
		}

		// 5. Verify id_token from token response
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok || rawIDToken == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Missing id_token in response"},
			})

			return
		}

		idInfo, err := VerifyAppleIDToken(r.Context(), rawIDToken, s.appleClientID, "")
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "oauth_failed", "message": "Failed to verify Apple ID token"},
			})

			return
		}

		// 6. Extract name from user JSON (first login only)
		name := ""
		if userJSON := r.FormValue("user"); userJSON != "" {
			var appleUsr appleUser
			if jsonErr := json.Unmarshal([]byte(userJSON), &appleUsr); jsonErr == nil {
				parts := []string{}
				if appleUsr.Name.FirstName != "" {
					parts = append(parts, appleUsr.Name.FirstName)
				}
				if appleUsr.Name.LastName != "" {
					parts = append(parts, appleUsr.Name.LastName)
				}
				name = strings.Join(parts, " ")
			}
		}

		// 7. Upsert user
		u, err := userRepo.Upsert(r.Context(), &model.User{
			Provider:   "apple",
			ProviderID: idInfo.Sub,
			Email:      idInfo.Email,
			Name:       name,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to create user"},
			})

			return
		}

		// 8. Generate JWT pair and redirect
		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		s.redirectWithTokens(w, r, pair)
	}
}
```

Add `"strings"` to the import block.

**Step 4: Write test for HandleAppleCallback**

Add to `backend/internal/auth/apple_test.go`:

```go
func TestHandleAppleCallback_InvalidState(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := auth.AppleConfig("com.bilustek.secretdrop.web", "http://localhost/callback")

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := svc.HandleAppleCallback(cfg, userRepo)

	// POST form with mismatched state
	form := url.Values{}
	form.Set("code", "test-code")
	form.Set("state", "correct-state")

	req := httptest.NewRequest(http.MethodPost, "/auth/apple/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "wrong-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_state" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_state")
	}
}

func TestHandleAppleCallback_MissingStateCookie(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := auth.AppleConfig("com.bilustek.secretdrop.web", "http://localhost/callback")

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := svc.HandleAppleCallback(cfg, userRepo)

	form := url.Values{}
	form.Set("code", "test-code")
	form.Set("state", "some-state")

	req := httptest.NewRequest(http.MethodPost, "/auth/apple/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}
```

**Step 5: Run tests**

Run: `cd backend && go test ./internal/auth/ -run "TestAppleConfig|TestHandleApple" -v`
Expected: PASS

**Step 6: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/...
git add backend/internal/auth/apple.go backend/internal/auth/apple_test.go
git commit -m "feat: add Apple OAuth login and callback handlers"
```

---

### Task 6: Wire Apple routes in main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go:131-134` (auth service creation)
- Modify: `backend/cmd/secretdrop/main.go:175-192` (route registration)

**Step 1: Update auth service creation**

In `backend/cmd/secretdrop/main.go`, modify the auth service creation at line 131:

```go
authSvc, err := auth.New(jwtSecret,
	auth.WithGoogleClientID(cfg.GoogleClientID()),
	auth.WithFrontendBaseURL(cfg.FrontendBaseURL()),
	auth.WithAppleCredentials(
		cfg.AppleClientID(),
		cfg.AppleTeamID(),
		cfg.AppleKeyID(),
		cfg.ApplePrivateKey(),
	),
)
```

**Step 2: Add Apple routes**

After the GitHub callback route registration (after line 190), add:

```go
// Apple Sign-In (conditional — only when credentials are configured)
if cfg.AppleClientID() != "" {
	appleCfg := auth.AppleConfig(
		cfg.AppleClientID(),
		cfg.APIBaseURL()+"/auth/apple/callback",
	)

	mux.HandleFunc("GET /auth/apple", authSvc.HandleAppleLogin(appleCfg))
	mux.HandleFunc("POST /auth/apple/callback", authSvc.HandleAppleCallback(appleCfg, userRepo))

	slog.Info("apple sign-in enabled")
}
```

**Step 3: Build to verify**

Run: `cd backend && go build ./cmd/secretdrop/`
Expected: BUILD SUCCESS

**Step 4: Run all tests**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./...
git add backend/cmd/secretdrop/main.go
git commit -m "feat: wire Apple Sign-In routes in main.go"
```

---

### Task 7: Frontend — Add Sign in with Apple button

**Files:**
- Modify: `frontend/src/pages/Landing.tsx`

**Step 1: Add Apple icon component and button**

In `frontend/src/pages/Landing.tsx`:

1. Add feature flag (after line 7, `const showGoogle = ...`):

```typescript
const showApple = import.meta.env.VITE_ENABLE_APPLE_SIGNIN !== "false"
```

2. Add AppleIcon component (after GitHubIcon, around line 92):

```typescript
function AppleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M17.05 20.28c-.98.95-2.05.88-3.08.4-1.09-.5-2.08-.48-3.24 0-1.44.62-2.2.44-3.06-.4C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z" />
    </svg>
  )
}
```

3. Add Apple button in hero section (after GitHub button, around line 143):

```typescript
{showApple && (
  <a
    href={`${API_URL}/auth/apple`}
    className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-black text-white font-medium hover:opacity-90 transition-opacity dark:bg-white dark:text-black"
  >
    <AppleIcon className="w-5 h-5" />
    Sign in with Apple
  </a>
)}
```

4. Add Apple button in sign-in modal (after GitHub button in modal, around line 332):

```typescript
{showApple && (
  <a
    href={`${API_URL}/auth/apple`}
    className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-black text-white font-medium hover:opacity-90 transition-opacity dark:bg-white dark:text-black"
  >
    <AppleIcon className="w-5 h-5" />
    Sign in with Apple
  </a>
)}
```

**Step 2: Build to verify**

Run: `cd frontend && npm run build`
Expected: BUILD SUCCESS

**Step 3: Lint**

Run: `cd frontend && npx eslint .`
Expected: NO ERRORS

**Step 4: Commit**

```bash
git add frontend/src/pages/Landing.tsx
git commit -m "feat: add Sign in with Apple button to landing page"
```

---

### Task 8: Update documentation

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/openapi.yaml`

**Step 1: Update CLAUDE.md**

1. Add new env vars to the Environment Variables table:

```markdown
| `APPLE_CLIENT_ID` | No | — |
| `APPLE_TEAM_ID` | No | — |
| `APPLE_KEY_ID` | No | — |
| `APPLE_PRIVATE_KEY` | No | — |
| `VITE_ENABLE_APPLE_SIGNIN` | No (frontend) | `""` (enabled by default) |
```

2. Add new endpoints to API Endpoints:

```markdown
- `GET /auth/apple` — Apple OAuth login redirect
- `POST /auth/apple/callback` — Apple OAuth callback (form POST)
```

**Step 2: Update OpenAPI spec**

Add `GET /auth/apple` and `POST /auth/apple/callback` endpoints to `docs/openapi.yaml` following the same pattern as Google/GitHub OAuth endpoints.

**Step 3: Commit**

```bash
git add CLAUDE.md docs/openapi.yaml
git commit -m "docs: add Apple Sign-In endpoints and env vars"
```

---

### Task 9: Final verification

**Step 1: Run all backend tests**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 2: Run backend lint**

Run: `cd backend && golangci-lint run ./...`
Expected: NO ISSUES

**Step 3: Run frontend build**

Run: `cd frontend && npm run build`
Expected: BUILD SUCCESS

**Step 4: Run frontend lint**

Run: `cd frontend && npx eslint .`
Expected: NO ERRORS
