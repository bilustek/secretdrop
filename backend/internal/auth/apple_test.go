package auth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
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

func TestVerifyAppleIDToken(t *testing.T) { //nolint:paralleltest // uses httptest server
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a mock JWKS endpoint
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck // test helper
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

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck // test helper
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
