package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/golang-jwt/jwt/v5"
)

func TestNew_EmptySecret(t *testing.T) {
	t.Parallel()

	_, err := auth.New("")
	if err == nil {
		t.Fatal("New() should fail with empty secret")
	}

	if !strings.Contains(err.Error(), "jwt secret cannot be empty") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "jwt secret cannot be empty")
	}
}

func TestNew_ValidSecret(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc == nil {
		t.Fatal("New() returned nil service")
	}
}

func TestNew_WithAccessExpiry(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithAccessExpiry(30*time.Minute))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(1, "test@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.VerifyToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}

	expiry := time.Until(claims.ExpiresAt.Time)
	if expiry < 29*time.Minute || expiry > 31*time.Minute {
		t.Errorf("access token expiry = %v; want ~30m", expiry)
	}
}

func TestNew_WithRefreshExpiry(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithRefreshExpiry(7*24*time.Hour))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(1, "test@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.VerifyToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}

	expiry := time.Until(claims.ExpiresAt.Time)
	if expiry < 6*24*time.Hour || expiry > 8*24*time.Hour {
		t.Errorf("refresh token expiry = %v; want ~7 days", expiry)
	}
}

func TestNew_WithAccessExpiryInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expiry time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.New("test-secret", auth.WithAccessExpiry(tt.expiry))
			if err == nil {
				t.Fatalf("WithAccessExpiry(%v) should fail", tt.expiry)
			}

			if !strings.Contains(err.Error(), "access expiry must be positive") {
				t.Errorf("error = %q; want to contain %q", err.Error(), "access expiry must be positive")
			}
		})
	}
}

func TestNew_WithRefreshExpiryInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expiry time.Duration
	}{
		{"zero", 0},
		{"negative", -1 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := auth.New("test-secret", auth.WithRefreshExpiry(tt.expiry))
			if err == nil {
				t.Fatalf("WithRefreshExpiry(%v) should fail", tt.expiry)
			}

			if !strings.Contains(err.Error(), "refresh expiry must be positive") {
				t.Errorf("error = %q; want to contain %q", err.Error(), "refresh expiry must be positive")
			}
		})
	}
}

func TestGenerateTokenPair(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("AccessToken is empty")
	}

	if pair.RefreshToken == "" {
		t.Error("RefreshToken is empty")
	}

	if pair.AccessToken == pair.RefreshToken {
		t.Error("AccessToken and RefreshToken should be different")
	}
}

func TestGenerateTokenPair_AccessClaims(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.VerifyToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("VerifyToken(access) error = %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", claims.UserID)
	}

	if claims.Email != "user@example.com" {
		t.Errorf("Email = %q; want %q", claims.Email, "user@example.com")
	}

	if claims.Tier != "pro" {
		t.Errorf("Tier = %q; want %q", claims.Tier, "pro")
	}

	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}

	expiry := time.Until(claims.ExpiresAt.Time)
	if expiry < 14*time.Minute || expiry > 16*time.Minute {
		t.Errorf("access token expiry = %v; want ~15m", expiry)
	}
}

func TestGenerateTokenPair_RefreshClaims(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.VerifyToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("VerifyToken(refresh) error = %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", claims.UserID)
	}

	// Refresh token should not contain email or tier.
	if claims.Email != "" {
		t.Errorf("Email = %q; want empty for refresh token", claims.Email)
	}

	if claims.Tier != "" {
		t.Errorf("Tier = %q; want empty for refresh token", claims.Tier)
	}

	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}

	expiry := time.Until(claims.ExpiresAt.Time)
	if expiry < 29*24*time.Hour || expiry > 31*24*time.Hour {
		t.Errorf("refresh token expiry = %v; want ~30 days", expiry)
	}
}

func TestVerifyToken_Valid(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(99, "valid@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	claims, err := svc.VerifyToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}

	if claims.UserID != 99 {
		t.Errorf("UserID = %d; want 99", claims.UserID)
	}

	if claims.Email != "valid@example.com" {
		t.Errorf("Email = %q; want %q", claims.Email, "valid@example.com")
	}

	if claims.Tier != "free" {
		t.Errorf("Tier = %q; want %q", claims.Tier, "free")
	}
}

func TestVerifyToken_Expired(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithAccessExpiry(1*time.Millisecond))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(1, "expired@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	_, err = svc.VerifyToken(pair.AccessToken)
	if err == nil {
		t.Fatal("VerifyToken() should fail for expired token")
	}

	if !strings.Contains(err.Error(), "token is expired") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "token is expired")
	}
}

func TestVerifyToken_InvalidSignature(t *testing.T) {
	t.Parallel()

	svc1, err := auth.New("secret-one")
	if err != nil {
		t.Fatalf("New(secret-one) error = %v", err)
	}

	svc2, err := auth.New("secret-two")
	if err != nil {
		t.Fatalf("New(secret-two) error = %v", err)
	}

	pair, err := svc1.GenerateTokenPair(1, "test@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	_, err = svc2.VerifyToken(pair.AccessToken)
	if err == nil {
		t.Fatal("VerifyToken() should fail with wrong secret")
	}
}

func TestVerifyToken_WrongSigningMethod(t *testing.T) {
	t.Parallel()

	// Create a token signed with "none" method.
	claims := &auth.Claims{
		UserID: 1,
		Email:  "test@example.com",
		Tier:   "free",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)

	tokenStr, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none token: %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.VerifyToken(tokenStr)
	if err == nil {
		t.Fatal("VerifyToken() should fail for none signing method")
	}

	if !strings.Contains(err.Error(), "unexpected signing method") {
		t.Errorf("error = %q; want to contain %q", err.Error(), "unexpected signing method")
	}
}

func TestVerifyToken_MalformedToken(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.VerifyToken("not-a-valid-jwt")
	if err == nil {
		t.Fatal("VerifyToken() should fail for malformed token")
	}
}

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

func TestNew_WithGoogleClientID(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithGoogleClientID("my-google-client-id"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc == nil {
		t.Fatal("New() returned nil service")
	}
}

func TestNew_WithSecureCookies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secure bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, err := auth.New("test-secret", auth.WithSecureCookies(tt.secure))
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			if svc.SecureCookies() != tt.secure {
				t.Errorf("SecureCookies() = %v; want %v", svc.SecureCookies(), tt.secure)
			}
		})
	}
}

func TestService_SecureCookies_Default(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.SecureCookies() {
		t.Error("SecureCookies() default should be false")
	}
}

func TestService_AccessExpiry(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithAccessExpiry(5*time.Minute))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.AccessExpiry() != 5*time.Minute {
		t.Errorf("AccessExpiry() = %v; want %v", svc.AccessExpiry(), 5*time.Minute)
	}
}

func TestService_RefreshExpiry(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithRefreshExpiry(7*24*time.Hour))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc.RefreshExpiry() != 7*24*time.Hour {
		t.Errorf("RefreshExpiry() = %v; want %v", svc.RefreshExpiry(), 7*24*time.Hour)
	}
}

func TestService_SetAuthCookies(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret",
		auth.WithSecureCookies(true),
		auth.WithAccessExpiry(15*time.Minute),
		auth.WithRefreshExpiry(30*24*time.Hour),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair := &auth.TokenPair{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
	}

	rec := httptest.NewRecorder()

	if err := svc.SetAuthCookies(rec, pair); err != nil {
		t.Fatalf("SetAuthCookies() error = %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookie count = %d; want 3", len(cookies))
	}

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	// Verify access_token cookie.
	ac, ok := cookieMap[auth.CookieAccessToken]
	if !ok {
		t.Fatal("access_token cookie not found")
	}

	if ac.Value != "test-access" {
		t.Errorf("access_token value = %q; want %q", ac.Value, "test-access")
	}

	if !ac.HttpOnly {
		t.Error("access_token HttpOnly = false; want true")
	}

	if !ac.Secure {
		t.Error("access_token Secure = false; want true")
	}

	// Verify refresh_token cookie.
	rc, ok := cookieMap[auth.CookieRefreshToken]
	if !ok {
		t.Fatal("refresh_token cookie not found")
	}

	if rc.Value != "test-refresh" {
		t.Errorf("refresh_token value = %q; want %q", rc.Value, "test-refresh")
	}

	// Verify csrf_token cookie exists and is non-empty.
	cc, ok := cookieMap[auth.CookieCSRFToken]
	if !ok {
		t.Fatal("csrf_token cookie not found")
	}

	if cc.Value == "" {
		t.Error("csrf_token value is empty")
	}

	if cc.HttpOnly {
		t.Error("csrf_token HttpOnly = true; want false (readable by JS)")
	}
}

func TestHandleLogout(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handler := svc.HandleLogout()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify: 200 status.
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	// Verify: JSON body with status "ok".
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status = %q; want %q", body["status"], "ok")
	}

	// Verify: all three cookies are cleared (MaxAge = -1).
	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookie count = %d; want 3", len(cookies))
	}

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	for _, name := range []string{auth.CookieAccessToken, auth.CookieRefreshToken, auth.CookieCSRFToken} {
		c, ok := cookieMap[name]
		if !ok {
			t.Errorf("%s cookie not found", name)

			continue
		}

		if c.MaxAge != -1 {
			t.Errorf("%s MaxAge = %d; want -1", name, c.MaxAge)
		}

		if c.Value != "" {
			t.Errorf("%s Value = %q; want empty", name, c.Value)
		}
	}
}
