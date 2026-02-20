package auth_test

import (
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
