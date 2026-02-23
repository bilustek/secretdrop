package auth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

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
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a mock JWKS endpoint
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pubKey := privKey.PublicKey
		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": "test-kid",
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
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

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
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
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pubKey := privKey.PublicKey
		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": "test-kid",
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
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

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
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

// appleCallbackTestEnv holds all fixtures for Apple callback handler tests.
type appleCallbackTestEnv struct {
	ecPrivKey  *ecdsa.PrivateKey // ECDSA key for client_secret (ES256)
	rsaPrivKey *rsa.PrivateKey   // RSA key for id_token signing (RS256)
	b64PEMKey  string
	jwksJSON   []byte
	clientID   string
	tokenURL   string
	jwksURL    string
	svc        *auth.Service
	cfg        *oauth2.Config
}

// newAppleCallbackTestEnv creates a complete test environment for Apple callback tests.
func newAppleCallbackTestEnv(t *testing.T) *appleCallbackTestEnv {
	t.Helper()

	// ECDSA key for client_secret generation (ES256)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdsa key: %v", err)
	}

	// RSA key for id_token signing (RS256)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	// Build JWKS JSON from the RSA public key
	rsaPubKey := rsaKey.PublicKey
	jwksJSON, err := json.Marshal(map[string]any{
		"keys": []map[string]any{
			{
				"kty": "RSA",
				"kid": "test-kid",
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(rsaPubKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaPubKey.E)).Bytes()),
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal JWKS: %v", err)
	}

	// PEM-encode the ECDSA key for Apple client_secret generation
	der, err := x509.MarshalPKCS8PrivateKey(ecKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	b64Key := base64.StdEncoding.EncodeToString(pemBlock)

	clientID := "com.bilustek.secretdrop.web"
	tokenURL := "https://appleid.apple.com/auth/token"
	jwksURL := "https://appleid.apple.com/auth/keys"

	svc, err := auth.New("test-secret",
		auth.WithAppleCredentials(clientID, "ABCDE12345", "test-kid", b64Key),
		auth.WithFrontendBaseURL("http://localhost:3000"),
	)
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: "http://localhost/callback",
		Scopes:      []string{"name", "email"},
		Endpoint: oauth2.Endpoint{
			TokenURL: tokenURL,
		},
	}

	return &appleCallbackTestEnv{
		ecPrivKey:  ecKey,
		rsaPrivKey: rsaKey,
		b64PEMKey:  b64Key,
		jwksJSON:   jwksJSON,
		clientID:   clientID,
		tokenURL:   tokenURL,
		jwksURL:    jwksURL,
		svc:        svc,
		cfg:        cfg,
	}
}

// signAppleIDToken creates a signed Apple ID token JWT for testing (RS256).
func (env *appleCallbackTestEnv) signAppleIDToken(t *testing.T) string {
	t.Helper()

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://appleid.apple.com",
		"aud":   env.clientID,
		"sub":   "apple-user-001",
		"email": "user@privaterelay.appleid.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(env.rsaPrivKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	return idToken
}

// tokenExchangeResponse builds the JSON body for a successful token exchange.
func (env *appleCallbackTestEnv) tokenExchangeResponse(t *testing.T, idToken string) []byte {
	t.Helper()

	resp, err := json.Marshal(map[string]any{
		"access_token":  "mock-access-token",
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "mock-refresh-token",
		"id_token":      idToken,
	})
	if err != nil {
		t.Fatalf("marshal token response: %v", err)
	}

	return resp
}

// appleFormRequest builds a POST form request with state cookie for Apple callback.
func appleFormRequest(state, code, userJSON string) *http.Request {
	form := url.Values{}
	form.Set("code", code)
	form.Set("state", state)

	if userJSON != "" {
		form.Set("user", userJSON)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/apple/callback", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})

	return req
}

func TestHandleAppleCallback_Success(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)
	idToken := env.signAppleIDToken(t)
	tokenResp := env.tokenExchangeResponse(t, idToken)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(tokenResp)

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(env.jwksJSON)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)
	req := appleFormRequest("test-state", "test-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}

	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	locURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location URL: %v", err)
	}

	if locURL.Path != "/auth/callback" {
		t.Errorf("redirect path = %q; want %q", locURL.Path, "/auth/callback")
	}

	if locURL.Query().Get("access_token") == "" {
		t.Error("access_token missing from redirect URL")
	}

	if locURL.Query().Get("refresh_token") == "" {
		t.Error("refresh_token missing from redirect URL")
	}
}

func TestHandleAppleCallback_SuccessWithUserJSON(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)
	idToken := env.signAppleIDToken(t)
	tokenResp := env.tokenExchangeResponse(t, idToken)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(tokenResp)

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(env.jwksJSON)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)

	userJSON := `{"name":{"firstName":"John","lastName":"Doe"},"email":"john@example.com"}`
	req := appleFormRequest("test-state", "test-code", userJSON)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
}

func TestHandleAppleCallback_SuccessWithFirstNameOnly(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)
	idToken := env.signAppleIDToken(t)
	tokenResp := env.tokenExchangeResponse(t, idToken)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(tokenResp)

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(env.jwksJSON)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)

	userJSON := `{"name":{"firstName":"Jane","lastName":""},"email":"jane@example.com"}`
	req := appleFormRequest("test-state", "test-code", userJSON)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
}

func TestHandleAppleCallback_CodeExchangeFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusUnauthorized)
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString(`{"error":"invalid_grant"}`)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)
	req := appleFormRequest("test-state", "bad-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "oauth_failed" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "oauth_failed")
	}
}

func TestHandleAppleCallback_MissingIDToken(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)

	// Token exchange returns no id_token
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{ //nolint:errcheck // test helper
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)
	req := appleFormRequest("test-state", "test-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}
}

func TestHandleAppleCallback_IDTokenVerificationFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)

	// Return a valid token exchange with a garbage id_token
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{ //nolint:errcheck // test helper
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
					"id_token":     "invalid.jwt.token",
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(env.jwksJSON)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)
	req := appleFormRequest("test-state", "test-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "oauth_failed" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "oauth_failed")
	}
}

func TestHandleAppleCallback_InvalidClientSecret(t *testing.T) {
	t.Parallel()

	// Service with invalid key — GenerateAppleClientSecret will fail
	svc, err := auth.New("test-secret",
		auth.WithAppleCredentials("com.test", "TEAM", "KEY", base64.StdEncoding.EncodeToString([]byte("not-a-pem"))),
		auth.WithFrontendBaseURL("http://localhost:3000"),
	)
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := &oauth2.Config{
		ClientID: "com.test",
		Endpoint: oauth2.Endpoint{TokenURL: "https://appleid.apple.com/auth/token"},
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := svc.HandleAppleCallback(cfg, userRepo)
	req := appleFormRequest("test-state", "test-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}
}

func TestHandleAppleCallback_InvalidUserJSON(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)
	idToken := env.signAppleIDToken(t)
	tokenResp := env.tokenExchangeResponse(t, idToken)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(tokenResp)

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(env.jwksJSON)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)

	// Send invalid user JSON — should still succeed (name will be empty)
	req := appleFormRequest("test-state", "test-code", "{invalid json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Should succeed — invalid user JSON is non-fatal (name just stays empty)
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
}

func TestHandleAppleCallback_JWKSFetchFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	env := newAppleCallbackTestEnv(t)

	// Token exchange returns id_token, but JWKS endpoint fails
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://appleid.apple.com",
		"aud":   env.clientID,
		"sub":   "apple-user-001",
		"email": "user@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(env.rsaPrivKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	tokenResp := env.tokenExchangeResponse(t, idToken)

	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.Write(tokenResp)

				return rec.Result(), nil
			}

			if req.URL.Host == "appleid.apple.com" && req.URL.Path == "/auth/keys" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusInternalServerError)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	handler := env.svc.HandleAppleCallback(env.cfg, userRepo)
	req := appleFormRequest("test-state", "test-code", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestVerifyAppleIDToken_JWKSFetchError(t *testing.T) { //nolint:paralleltest // uses httptest server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := auth.VerifyAppleIDToken(context.Background(), "dummy-token", "com.test", server.URL)
	if err == nil {
		t.Fatal("VerifyAppleIDToken() should fail when JWKS fetch fails")
	}
}

func TestVerifyAppleIDToken_InvalidIssuer(t *testing.T) { //nolint:paralleltest // uses httptest server
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pubKey := privKey.PublicKey
		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": "test-kid",
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck // test helper
	}))
	defer jwksServer.Close()

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   "https://evil.example.com",
		"aud":   "com.bilustek.secretdrop.web",
		"sub":   "user-001",
		"email": "user@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	_, err = auth.VerifyAppleIDToken(context.Background(), idToken, "com.bilustek.secretdrop.web", jwksServer.URL)
	if err == nil {
		t.Fatal("VerifyAppleIDToken() should fail with wrong issuer")
	}
}

func TestVerifyAppleIDToken_NoMatchingKid(t *testing.T) { //nolint:paralleltest // uses httptest server
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// JWKS with different kid
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pubKey := privKey.PublicKey
		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"kid": "different-kid",
					"use": "sig",
					"alg": "RS256",
					"n":   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
					"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
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
		"aud":   "com.test",
		"sub":   "user-001",
		"email": "user@example.com",
		"iat":   now.Unix(),
		"exp":   now.Add(10 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-kid"

	idToken, err := token.SignedString(privKey)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}

	_, err = auth.VerifyAppleIDToken(context.Background(), idToken, "com.test", jwksServer.URL)
	if err == nil {
		t.Fatal("VerifyAppleIDToken() should fail when kid not found in JWKS")
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
