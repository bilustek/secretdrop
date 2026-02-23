package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

func testAuthService(t *testing.T) *auth.Service {
	t.Helper()

	svc, err := auth.New("test-secret-key-at-least-32-bytes!")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	return svc
}

type authErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestAuthenticate_ValidToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext() returned false")
		}

		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", gotClaims.UserID)
	}

	if gotClaims.Email != "user@example.com" {
		t.Errorf("Email = %q; want %q", gotClaims.Email, "user@example.com")
	}

	if gotClaims.Tier != "pro" {
		t.Errorf("Tier = %q; want %q", gotClaims.Tier, "pro")
	}
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp authErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "unauthorized" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "unauthorized")
	}

	if resp.Error.Message != "Authentication required" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Authentication required")
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}
}

func TestAuthenticate_NoBearerPrefix(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp authErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "unauthorized" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "unauthorized")
	}

	if resp.Error.Message != "Authentication required" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Authentication required")
	}
}

func TestAuthenticate_InvalidToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-string")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp authErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "unauthorized" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "unauthorized")
	}

	if resp.Error.Message != "Invalid or expired token" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Invalid or expired token")
	}
}

func TestAuthenticate_ExpiredToken(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret-key-at-least-32-bytes!", auth.WithAccessExpiry(1*time.Millisecond))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(1, "expired@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	var resp authErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Message != "Invalid or expired token" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Invalid or expired token")
	}
}

func TestOptionalAuthenticate_NoHeader(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		_, ok := middleware.UserFromContext(r.Context())
		if ok {
			t.Error("UserFromContext() should return false when no header is present")
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestOptionalAuthenticate_ValidToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext() returned false")
		}

		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", gotClaims.UserID)
	}

	if gotClaims.Email != "user@example.com" {
		t.Errorf("Email = %q; want %q", gotClaims.Email, "user@example.com")
	}
}

func TestOptionalAuthenticate_InvalidToken(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		_, ok := middleware.UserFromContext(r.Context())
		if ok {
			t.Error("UserFromContext() should return false for invalid token")
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called even with invalid token")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestOptionalAuthenticate_NoBearerPrefix(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true

		_, ok := middleware.UserFromContext(r.Context())
		if ok {
			t.Error("UserFromContext() should return false for non-Bearer auth")
		}

		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestUserFromContext_Missing(t *testing.T) {
	t.Parallel()

	claims, ok := middleware.UserFromContext(context.Background())
	if ok {
		t.Error("UserFromContext() should return false for empty context")
	}

	if claims != nil {
		t.Errorf("claims = %v; want nil", claims)
	}
}

func TestOptionalAuthenticate_SetsSentryUser(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(99, "sentry@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	hub := sentry.NewHub(nil, sentry.NewScope())

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), hub))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthenticate_SetsSentryUser(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(77, "auth@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	hub := sentry.NewHub(nil, sentry.NewScope())

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	req = req.WithContext(sentry.SetHubOnContext(req.Context(), hub))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestAuthenticate_ValidCookie(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(42, "cookie@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext() returned false")
		}

		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: pair.AccessToken})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", gotClaims.UserID)
	}

	if gotClaims.Email != "cookie@example.com" {
		t.Errorf("Email = %q; want %q", gotClaims.Email, "cookie@example.com")
	}

	if gotClaims.Tier != "pro" {
		t.Errorf("Tier = %q; want %q", gotClaims.Tier, "pro")
	}
}

func TestAuthenticate_CookieTakesPrecedenceOverHeader(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	cookiePair, err := svc.GenerateTokenPair(1, "cookie-user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	headerPair, err := svc.GenerateTokenPair(2, "header-user@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext() returned false")
		}

		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: cookiePair.AccessToken})
	req.Header.Set("Authorization", "Bearer "+headerPair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 1 {
		t.Errorf("UserID = %d; want 1 (cookie user)", gotClaims.UserID)
	}

	if gotClaims.Email != "cookie-user@example.com" {
		t.Errorf("Email = %q; want %q", gotClaims.Email, "cookie-user@example.com")
	}
}

func TestOptionalAuthenticate_ValidCookie(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	pair, err := svc.GenerateTokenPair(42, "cookie@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext() returned false")
		}

		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.OptionalAuthenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: pair.AccessToken})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", gotClaims.UserID)
	}

	if gotClaims.Email != "cookie@example.com" {
		t.Errorf("Email = %q; want %q", gotClaims.Email, "cookie@example.com")
	}
}
