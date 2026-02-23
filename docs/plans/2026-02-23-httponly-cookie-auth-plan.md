# httpOnly Cookie Auth + CSP Security Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate JWT storage from localStorage to httpOnly cookies and add CSP + security headers to prevent XSS token theft and data exfiltration.

**Architecture:** Backend sets httpOnly cookies on OAuth callback/refresh instead of returning tokens in URL params/JSON. Middleware reads tokens from cookies first, falls back to Bearer header for mobile. CSRF Double Submit Cookie pattern protects mutating endpoints. CSP and security headers provide defense-in-depth.

**Tech Stack:** Go stdlib net/http cookies, crypto/rand for CSRF tokens, React fetch with `credentials: "include"`

---

### Task 1: Cookie Helper Functions

**Files:**
- Create: `backend/internal/auth/cookie.go`
- Test: `backend/internal/auth/cookie_test.go`

**Step 1: Write the failing tests**

```go
package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
)

func TestSetTokenCookies(t *testing.T) {
	t.Parallel()

	pair := &auth.TokenPair{
		AccessToken:  "access-xxx",
		RefreshToken: "refresh-xxx",
	}

	rec := httptest.NewRecorder()
	auth.SetTokenCookies(rec, pair, "csrf-xxx", true, 15*time.Minute, 30*24*time.Hour)

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies; want 3", len(cookies))
	}

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	// access_token cookie
	ac := cookieMap["access_token"]
	if ac == nil {
		t.Fatal("access_token cookie not set")
	}
	if ac.Value != "access-xxx" {
		t.Errorf("access_token value = %q; want %q", ac.Value, "access-xxx")
	}
	if !ac.HttpOnly {
		t.Error("access_token should be HttpOnly")
	}
	if !ac.Secure {
		t.Error("access_token should be Secure")
	}
	if ac.SameSite != http.SameSiteLaxMode {
		t.Errorf("access_token SameSite = %v; want Lax", ac.SameSite)
	}
	if ac.Path != "/" {
		t.Errorf("access_token Path = %q; want %q", ac.Path, "/")
	}

	// refresh_token cookie
	rc := cookieMap["refresh_token"]
	if rc == nil {
		t.Fatal("refresh_token cookie not set")
	}
	if rc.Value != "refresh-xxx" {
		t.Errorf("refresh_token value = %q; want %q", rc.Value, "refresh-xxx")
	}
	if !rc.HttpOnly {
		t.Error("refresh_token should be HttpOnly")
	}
	if rc.Path != "/auth/refresh" {
		t.Errorf("refresh_token Path = %q; want %q", rc.Path, "/auth/refresh")
	}

	// csrf_token cookie
	cc := cookieMap["csrf_token"]
	if cc == nil {
		t.Fatal("csrf_token cookie not set")
	}
	if cc.Value != "csrf-xxx" {
		t.Errorf("csrf_token value = %q; want %q", cc.Value, "csrf-xxx")
	}
	if cc.HttpOnly {
		t.Error("csrf_token should NOT be HttpOnly")
	}
}

func TestSetTokenCookies_InsecureMode(t *testing.T) {
	t.Parallel()

	pair := &auth.TokenPair{
		AccessToken:  "access-xxx",
		RefreshToken: "refresh-xxx",
	}

	rec := httptest.NewRecorder()
	auth.SetTokenCookies(rec, pair, "csrf-xxx", false, 15*time.Minute, 30*24*time.Hour)

	for _, c := range rec.Result().Cookies() {
		if c.Secure {
			t.Errorf("cookie %q should not be Secure in insecure mode", c.Name)
		}
	}
}

func TestClearTokenCookies(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	auth.ClearTokenCookies(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies; want 3", len(cookies))
	}

	for _, c := range cookies {
		if c.MaxAge != -1 {
			t.Errorf("cookie %q MaxAge = %d; want -1", c.Name, c.MaxAge)
		}
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	t.Parallel()

	token1, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken() error = %v", err)
	}

	if token1 == "" {
		t.Fatal("GenerateCSRFToken() returned empty string")
	}

	token2, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken() second call error = %v", err)
	}

	if token1 == token2 {
		t.Error("two generated CSRF tokens should be different")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/auth/ -run "TestSetTokenCookies|TestClearTokenCookies|TestGenerateCSRFToken" -v`
Expected: FAIL — functions not defined

**Step 3: Write minimal implementation**

```go
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

const (
	// CookieAccessToken is the cookie name for the JWT access token.
	CookieAccessToken = "access_token"

	// CookieRefreshToken is the cookie name for the JWT refresh token.
	CookieRefreshToken = "refresh_token"

	// CookieCSRFToken is the cookie name for the CSRF token.
	CookieCSRFToken = "csrf_token"

	csrfTokenBytes = 32
)

// SetTokenCookies writes access, refresh, and CSRF cookies to the response.
func SetTokenCookies(w http.ResponseWriter, pair *TokenPair, csrfToken string, secure bool, accessExpiry, refreshExpiry time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    pair.AccessToken,
		Path:     "/",
		MaxAge:   int(accessExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    pair.RefreshToken,
		Path:     "/auth/refresh",
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieCSRFToken,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearTokenCookies removes all auth cookies from the response.
func ClearTokenCookies(w http.ResponseWriter) {
	for _, name := range []string{CookieAccessToken, CookieRefreshToken, CookieCSRFToken} {
		path := "/"
		if name == CookieRefreshToken {
			path = "/auth/refresh"
		}

		http.SetCookie(w, &http.Cookie{
			Name:   name,
			Value:  "",
			Path:   path,
			MaxAge: -1,
		})
	}
}

// GenerateCSRFToken returns a cryptographically random base64url-encoded token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/auth/ -run "TestSetTokenCookies|TestClearTokenCookies|TestGenerateCSRFToken" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/
git add backend/internal/auth/cookie.go backend/internal/auth/cookie_test.go
git commit -m "feat: add cookie helper functions for httpOnly JWT storage"
```

---

### Task 2: Auth Service — SecureCookies Option + SetAuthCookies Method

**Files:**
- Modify: `backend/internal/auth/auth.go`
- Test: `backend/internal/auth/auth_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/auth/auth_test.go`:

```go
func TestNew_WithSecureCookies(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithSecureCookies(true))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if svc == nil {
		t.Fatal("New() returned nil service")
	}
}

func TestService_SetAuthCookies(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithSecureCookies(false))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(1, "test@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	rec := httptest.NewRecorder()
	if err := svc.SetAuthCookies(rec, pair); err != nil {
		t.Fatalf("SetAuthCookies() error = %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies; want 3", len(cookies))
	}

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	if cookieMap["csrf_token"] == nil {
		t.Fatal("csrf_token cookie not set")
	}
	if cookieMap["csrf_token"].Value == "" {
		t.Error("csrf_token value should not be empty")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run "TestNew_WithSecureCookies|TestService_SetAuthCookies" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `backend/internal/auth/auth.go` — new field and option:

```go
// Add to Service struct:
secureCookies bool

// Add option:
// WithSecureCookies sets whether cookies should have the Secure flag.
// In development mode (HTTP), set to false.
func WithSecureCookies(secure bool) Option {
	return func(s *Service) error {
		s.secureCookies = secure

		return nil
	}
}

// SecureCookies returns whether cookies should have the Secure flag.
func (s *Service) SecureCookies() bool {
	return s.secureCookies
}

// AccessExpiry returns the access token expiry duration.
func (s *Service) AccessExpiry() time.Duration {
	return s.accessExpiry
}

// RefreshExpiry returns the refresh token expiry duration.
func (s *Service) RefreshExpiry() time.Duration {
	return s.refreshExpiry
}

// SetAuthCookies generates a CSRF token and sets all three auth cookies.
func (s *Service) SetAuthCookies(w http.ResponseWriter, pair *TokenPair) error {
	csrfToken, err := GenerateCSRFToken()
	if err != nil {
		return fmt.Errorf("set auth cookies: %w", err)
	}

	SetTokenCookies(w, pair, csrfToken, s.secureCookies, s.accessExpiry, s.refreshExpiry)

	return nil
}
```

Also need to add `"net/http"` to imports in auth.go.

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/auth/ -run "TestNew_WithSecureCookies|TestService_SetAuthCookies" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/
git add backend/internal/auth/auth.go backend/internal/auth/auth_test.go
git commit -m "feat: add SecureCookies option and SetAuthCookies method to auth service"
```

---

### Task 3: Modify OAuth Callbacks — Cookie-Based redirectWithTokens

**Files:**
- Modify: `backend/internal/auth/google.go:182-195` (redirectWithTokens function)
- Test: `backend/internal/auth/google_test.go` (update existing tests)

**Step 1: Write the failing test**

Add to `backend/internal/auth/google_test.go` — a new test for cookie-based redirect:

```go
func TestRedirectWithTokens_SetsCookies(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret",
		auth.WithFrontendBaseURL("https://secretdrop.us"),
		auth.WithSecureCookies(true),
	)
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	pair := &auth.TokenPair{
		AccessToken:  "access-test",
		RefreshToken: "refresh-test",
	}

	req := httptest.NewRequest(http.MethodGet, "/callback", nil)
	rec := httptest.NewRecorder()

	svc.RedirectWithTokens(rec, req, pair)

	// Should redirect to /auth/callback without tokens in URL
	result := rec.Result()
	if result.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d; want %d", result.StatusCode, http.StatusSeeOther)
	}

	location := result.Header.Get("Location")
	if location != "https://secretdrop.us/auth/callback" {
		t.Errorf("Location = %q; want %q", location, "https://secretdrop.us/auth/callback")
	}

	// URL should NOT contain tokens
	if strings.Contains(location, "access_token") {
		t.Error("Location should not contain access_token query param")
	}

	// Cookies should be set
	cookies := result.Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	if cookieMap["access_token"] == nil {
		t.Fatal("access_token cookie not set")
	}
	if cookieMap["access_token"].Value != "access-test" {
		t.Errorf("access_token = %q; want %q", cookieMap["access_token"].Value, "access-test")
	}
	if cookieMap["refresh_token"] == nil {
		t.Fatal("refresh_token cookie not set")
	}
	if cookieMap["csrf_token"] == nil {
		t.Fatal("csrf_token cookie not set")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run "TestRedirectWithTokens_SetsCookies" -v`
Expected: FAIL — `RedirectWithTokens` is unexported / doesn't set cookies

**Step 3: Modify redirectWithTokens**

Replace the `redirectWithTokens` method in `backend/internal/auth/google.go:182-195`:

```go
// RedirectWithTokens sets auth cookies and redirects to the frontend callback.
func (s *Service) RedirectWithTokens(w http.ResponseWriter, r *http.Request, pair *TokenPair) {
	if err := s.SetAuthCookies(w, pair); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]string{"type": "internal_error", "message": "Failed to set auth cookies"},
		})

		return
	}

	u, _ := url.Parse(s.frontendBaseURL)
	u.Path = "/auth/callback"

	// 303 See Other ensures the browser always uses GET for the redirect,
	// even when the original request was POST (e.g. Apple form_post callback).
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}
```

Note: This renames `redirectWithTokens` → `RedirectWithTokens` (exported). Update all callers in google.go, github.go, apple.go: `s.redirectWithTokens(w, r, pair)` → `s.RedirectWithTokens(w, r, pair)`.

**Step 4: Run all auth tests to ensure nothing broke**

Run: `cd backend && go test ./internal/auth/ -v`
Expected: PASS (all existing tests + new test)

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/
git add backend/internal/auth/google.go backend/internal/auth/github.go backend/internal/auth/apple.go backend/internal/auth/google_test.go
git commit -m "feat: set httpOnly cookies in OAuth callbacks instead of URL params"
```

---

### Task 4: Modify Refresh Endpoint — Cookie + Body Dual Support

**Files:**
- Modify: `backend/internal/auth/token.go:198-262` (HandleRefresh function)
- Test: `backend/internal/auth/token_test.go` (update existing tests)

**Step 1: Write the failing test**

Add to `backend/internal/auth/token_test.go`:

```go
func TestHandleRefresh_FromCookie(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret", auth.WithSecureCookies(false))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Create initial token pair
	pair, err := svc.GenerateTokenPair(42, "user@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	userRepo := &mockUserRepo{user: &model.User{
		ID: 42, Email: "user@example.com", Tier: "pro",
	}}

	handler := svc.HandleRefresh(userRepo)

	// Send refresh token as cookie, empty JSON body
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: pair.RefreshToken})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Should set new cookies
	cookies := rec.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	if cookieMap["access_token"] == nil {
		t.Fatal("response should set access_token cookie")
	}
	if cookieMap["csrf_token"] == nil {
		t.Fatal("response should set csrf_token cookie")
	}
}
```

Note: A `mockUserRepo` may already exist in token_test.go. If not, create one that implements `user.Repository` with a `FindByID` that returns the preset user.

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run "TestHandleRefresh_FromCookie" -v`
Expected: FAIL

**Step 3: Modify HandleRefresh**

Update `HandleRefresh` in `backend/internal/auth/token.go` to read from cookie first, then body:

```go
// HandleRefresh validates a refresh token and returns a new rotated token pair.
// Web clients send the refresh token via httpOnly cookie.
// Mobile clients send it in the JSON body (fallback).
func (s *Service) HandleRefresh(userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try cookie first (web clients)
		var refreshTokenStr string
		if cookie, err := r.Cookie(CookieRefreshToken); err == nil && cookie.Value != "" {
			refreshTokenStr = cookie.Value
		}

		// Fall back to JSON body (mobile clients)
		if refreshTokenStr == "" {
			var req refreshRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error": map[string]string{"type": "validation_error", "message": "Invalid JSON body"},
				})

				return
			}

			refreshTokenStr = req.RefreshToken
		}

		if refreshTokenStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "refresh_token is required"},
			})

			return
		}

		claims, err := s.VerifyToken(refreshTokenStr)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{
					"type":    "invalid_refresh_token",
					"message": "Invalid or expired refresh token",
				},
			})

			return
		}

		// Reject access tokens used as refresh tokens.
		if claims.Email != "" || claims.Tier != "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{
					"type":    "invalid_refresh_token",
					"message": "Invalid or expired refresh token",
				},
			})

			return
		}

		u, err := userRepo.FindByID(r.Context(), claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "invalid_refresh_token", "message": "User not found"},
			})

			return
		}

		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		// If request came via cookie, set cookies. Otherwise JSON response.
		if _, cookieErr := r.Cookie(CookieRefreshToken); cookieErr == nil {
			if setErr := s.SetAuthCookies(w, pair); setErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]string{"type": "internal_error", "message": "Failed to set auth cookies"},
				})

				return
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		} else {
			writeJSON(w, http.StatusOK, pair)
		}
	}
}
```

**Step 4: Run all refresh tests**

Run: `cd backend && go test ./internal/auth/ -run "TestHandleRefresh" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/auth/
git add backend/internal/auth/token.go backend/internal/auth/token_test.go
git commit -m "feat: support refresh token via httpOnly cookie with JSON body fallback"
```

---

### Task 5: Logout Endpoint

**Files:**
- Modify: `backend/internal/auth/auth.go` (add HandleLogout method)
- Test: `backend/internal/auth/auth_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/auth/auth_test.go`:

```go
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

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("got %d cookies; want 3", len(cookies))
	}

	for _, c := range cookies {
		if c.MaxAge != -1 {
			t.Errorf("cookie %q MaxAge = %d; want -1", c.Name, c.MaxAge)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/auth/ -run "TestHandleLogout" -v`
Expected: FAIL

**Step 3: Write implementation**

Add to `backend/internal/auth/auth.go`:

```go
// HandleLogout clears all auth cookies.
func (s *Service) HandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		ClearTokenCookies(w)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
```

Note: `writeJSON` is in google.go (unexported, auth-package only). It's accessible since it's in the same package.

**Step 4: Run test**

Run: `cd backend && go test ./internal/auth/ -run "TestHandleLogout" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd backend && golangci-lint run ./internal/auth/
git add backend/internal/auth/auth.go backend/internal/auth/auth_test.go
git commit -m "feat: add POST /auth/logout endpoint to clear auth cookies"
```

---

### Task 6: Auth Middleware — Cookie + Bearer Dual Support

**Files:**
- Modify: `backend/internal/middleware/auth.go`
- Test: `backend/internal/middleware/auth_test.go`

**Step 1: Write the failing tests**

Add to `backend/internal/middleware/auth_test.go`:

```go
func TestAuthenticate_ValidCookie(t *testing.T) {
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
}

func TestAuthenticate_CookieTakesPrecedenceOverHeader(t *testing.T) {
	t.Parallel()

	svc := testAuthService(t)

	cookiePair, err := svc.GenerateTokenPair(42, "cookie@example.com", "pro")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	headerPair, err := svc.GenerateTokenPair(99, "header@example.com", "free")
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	var gotClaims *auth.Claims

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, _ := middleware.UserFromContext(r.Context())
		gotClaims = claims
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Authenticate(svc)(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: cookiePair.AccessToken})
	req.Header.Set("Authorization", "Bearer "+headerPair.AccessToken)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42 (from cookie)", gotClaims.UserID)
	}
}

func TestOptionalAuthenticate_ValidCookie(t *testing.T) {
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
	req.AddCookie(&http.Cookie{Name: "access_token", Value: pair.AccessToken})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotClaims == nil {
		t.Fatal("claims should not be nil")
	}

	if gotClaims.UserID != 42 {
		t.Errorf("UserID = %d; want 42", gotClaims.UserID)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/middleware/ -run "TestAuthenticate_ValidCookie|TestAuthenticate_CookieTakesPrecedenceOverHeader|TestOptionalAuthenticate_ValidCookie" -v`
Expected: FAIL

**Step 3: Modify auth middleware**

Add a helper function `extractToken` and update both middleware functions:

```go
// extractToken extracts the JWT token from the request.
// Priority: cookie > Authorization Bearer header.
func extractToken(r *http.Request) string {
	// 1. Try cookie
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// 2. Fall back to Authorization header
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	token := strings.TrimPrefix(header, "Bearer ")
	if token == header {
		return "" // no "Bearer " prefix
	}

	return token
}
```

Then update `OptionalAuthenticate`:

```go
func OptionalAuthenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)

			if hub := sentry.GetHubFromContext(ctx); hub != nil {
				hub.Scope().SetUser(sentry.User{ID: strconv.FormatInt(claims.UserID, 10)})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

And update `Authenticate`:

```go
func Authenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				writeAuthError(w, "Authentication required")
				return
			}

			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				writeAuthError(w, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)

			if hub := sentry.GetHubFromContext(ctx); hub != nil {
				hub.Scope().SetUser(sentry.User{ID: strconv.FormatInt(claims.UserID, 10)})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

Note: The error message for missing auth changed from "Authorization header required" / "Bearer token required" to "Authentication required". Update existing tests accordingly.

**Step 4: Run ALL auth middleware tests**

Run: `cd backend && go test ./internal/middleware/ -v`
Expected: PASS (update existing test expectations where error messages changed)

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/middleware/
git add backend/internal/middleware/auth.go backend/internal/middleware/auth_test.go
git commit -m "feat: auth middleware reads JWT from cookie first, Bearer header fallback"
```

---

### Task 7: CSRF Middleware

**Files:**
- Create: `backend/internal/middleware/csrf.go`
- Test: `backend/internal/middleware/csrf_test.go`

**Step 1: Write the failing tests**

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

func TestCSRF_SafeMethodsPass(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		called := false
		handler := middleware.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(method, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if !called {
			t.Errorf("%s: next handler should be called", method)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d; want %d", method, rec.Code, http.StatusOK)
		}
	}
}

func TestCSRF_PostWithMatchingTokens(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf-token"})
	req.Header.Set("X-CSRF-Token", "test-csrf-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called with matching CSRF tokens")
	}
}

func TestCSRF_PostWithMismatchedTokens(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "real-token"})
	req.Header.Set("X-CSRF-Token", "fake-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should NOT be called with mismatched CSRF tokens")
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRF_PostWithMissingHeader(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf-token"})
	// No X-CSRF-Token header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should NOT be called without CSRF header")
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRF_ExemptPaths(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CSRF("/billing/webhook", "/auth/apple/callback", "/auth/token", "/auth/refresh")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", nil)
	// No CSRF cookie or header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("exempt path should pass without CSRF")
	}
}

func TestCSRF_NoCookieNoProblemForUnauthenticated(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CSRF()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// POST with no csrf cookie at all — should fail
	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should NOT be called without CSRF cookie")
	}

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/middleware/ -run "TestCSRF" -v`
Expected: FAIL

**Step 3: Write implementation**

```go
package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
)

// CSRF returns middleware that validates the Double Submit Cookie pattern.
// Safe methods (GET, HEAD, OPTIONS) are exempt. Paths in exemptPaths are
// also exempt (e.g. webhook endpoints that use their own verification).
func CSRF(exemptPaths ...string) func(http.Handler) http.Handler {
	exempt := make(map[string]struct{}, len(exemptPaths))
	for _, p := range exemptPaths {
		exempt[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Safe methods are exempt.
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)

				return
			}

			// Exempt paths bypass CSRF.
			if _, ok := exempt[r.URL.Path]; ok {
				next.ServeHTTP(w, r)

				return
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				writeCSRFError(w)

				return
			}

			header := r.Header.Get("X-CSRF-Token")
			if header == "" {
				writeCSRFError(w)

				return
			}

			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				writeCSRFError(w)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, `{"error":{"type":"csrf_error","message":"CSRF validation failed"}}`)
}
```

**Step 4: Run tests**

Run: `cd backend && go test ./internal/middleware/ -run "TestCSRF" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/middleware/
git add backend/internal/middleware/csrf.go backend/internal/middleware/csrf_test.go
git commit -m "feat: add CSRF Double Submit Cookie middleware"
```

---

### Task 8: Security Headers Middleware

**Files:**
- Create: `backend/internal/middleware/security.go`
- Test: `backend/internal/middleware/security_test.go`

**Step 1: Write the failing tests**

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
	}

	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("%s = %q; want %q", tt.header, got, tt.want)
		}
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	t.Parallel()

	handler := middleware.SecurityHeaders()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header not set")
	}

	requiredDirectives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"connect-src 'self'",
		"frame-ancestors 'none'",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive: %s", directive)
		}
	}
}
```

**Step 2: Run tests**

Run: `cd backend && go test ./internal/middleware/ -run "TestSecurityHeaders" -v`
Expected: FAIL

**Step 3: Write implementation**

```go
package middleware

import "net/http"

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' https://*.googleusercontent.com https://*.gravatar.com https://avatars.githubusercontent.com; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self' https://appleid.apple.com; " +
	"base-uri 'self'"

// SecurityHeaders returns middleware that sets security response headers
// including Content-Security-Policy, X-Content-Type-Options, X-Frame-Options,
// and Referrer-Policy.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)

			next.ServeHTTP(w, r)
		})
	}
}
```

**Step 4: Run tests**

Run: `cd backend && go test ./internal/middleware/ -run "TestSecurityHeaders" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/middleware/
git add backend/internal/middleware/security.go backend/internal/middleware/security_test.go
git commit -m "feat: add security headers middleware with CSP"
```

---

### Task 9: Update CORS Middleware

**Files:**
- Modify: `backend/internal/middleware/cors.go`
- Modify: `backend/internal/middleware/cors_test.go`

**Step 1: Write the failing test**

Add to `backend/internal/middleware/cors_test.go`:

```go
func TestCORS_AllowCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://secretdrop.us")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q; want %q", got, "true")
	}

	headers := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(headers, "X-CSRF-Token") {
		t.Errorf("Access-Control-Allow-Headers = %q; should contain X-CSRF-Token", headers)
	}
}
```

Note: Add `"strings"` to imports if not present.

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/middleware/ -run "TestCORS_AllowCredentials" -v`
Expected: FAIL

**Step 3: Modify CORS middleware**

In `backend/internal/middleware/cors.go`, add two lines:

```go
w.Header().Set("Access-Control-Allow-Credentials", "true")
```

And update the allowed headers:

```go
w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token")
```

**Step 4: Run ALL CORS tests**

Run: `cd backend && go test ./internal/middleware/ -run "TestCORS" -v`
Expected: PASS

**Step 5: Lint and commit**

```bash
cd backend && golangci-lint run ./internal/middleware/
git add backend/internal/middleware/cors.go backend/internal/middleware/cors_test.go
git commit -m "feat: add CORS credentials support and X-CSRF-Token allowed header"
```

---

### Task 10: Wire Everything in main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go`

**Step 1: Add WithSecureCookies to auth service creation**

At line 131 in main.go, add `auth.WithSecureCookies(!cfg.IsDev())`:

```go
authSvc, err := auth.New(jwtSecret,
    auth.WithGoogleClientID(cfg.GoogleClientID()),
    auth.WithFrontendBaseURL(cfg.FrontendBaseURL()),
    auth.WithSecureCookies(!cfg.IsDev()),
    auth.WithAppleCredentials(
        cfg.AppleClientID(),
        cfg.AppleTeamID(),
        cfg.AppleKeyID(),
        cfg.ApplePrivateKey(),
    ),
)
```

**Step 2: Add logout route**

After line 221 (`mux.HandleFunc("POST /auth/refresh", ...)`), add:

```go
mux.HandleFunc("POST /auth/logout", authSvc.HandleLogout())
```

**Step 3: Add CSRF and SecurityHeaders middleware to the chain**

Update the middleware chain (lines 255-261):

```go
var chain http.Handler = mux
chain = middleware.RequireJSON(chain, "/auth/apple/callback", "/billing/webhook")
chain = middleware.CSRF("/billing/webhook", "/auth/apple/callback", "/auth/token", "/auth/refresh", "/auth/logout")(chain)
chain = middleware.OptionalAuthenticate(authSvc)(chain)
chain = middleware.SecurityHeaders()(chain)
chain = middleware.Logging(chain)
chain = middleware.RequestID(chain)

chain = middleware.CORS(cfg.FrontendBaseURL())(chain)
```

Note: CSRF must be AFTER OptionalAuthenticate so it runs on authenticated requests. SecurityHeaders should be early (outer) so headers are set on all responses. Order from outer to inner: CORS → RequestID → Logging → SecurityHeaders → OptionalAuthenticate → CSRF → RequireJSON → mux.

**Step 4: Build and test**

Run: `cd backend && go build ./cmd/secretdrop/`
Expected: Build succeeds

Run: `cd backend && go test -race ./...`
Expected: PASS

**Step 5: Commit**

```bash
cd backend && golangci-lint run ./...
git add backend/cmd/secretdrop/main.go
git commit -m "feat: wire httpOnly cookie auth, CSRF, and security headers middleware"
```

---

### Task 11: Frontend — Rewrite API Client

**Files:**
- Modify: `frontend/src/api/client.ts`

**Step 1: Rewrite client.ts**

Replace the entire file:

```typescript
import { API_URL } from "./config"

const API_BASE = `${API_URL}/api/v1`

interface ApiError {
  error: {
    type: string
    message: string
  }
}

export class AppError extends Error {
  type: string
  status: number

  constructor(type: string, message: string, status: number) {
    super(message)
    this.name = "AppError"
    this.type = type
    this.status = status
  }
}

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : ""
}

// Mutex to prevent concurrent refresh attempts.
let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  try {
    const res = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({}),
    })

    return res.ok
  } catch {
    return false
  }
}

async function tryRefresh(): Promise<boolean> {
  if (refreshPromise) return refreshPromise

  refreshPromise = refreshTokens().finally(() => {
    refreshPromise = null
  })

  return refreshPromise
}

function forceLogout(): never {
  window.location.href = "/"
  throw new AppError("unauthorized", "Session expired", 401)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await authenticatedFetch(`${API_BASE}${path}`, options)

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

async function authenticatedFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  // Add CSRF token for mutating requests
  const method = (options.method ?? "GET").toUpperCase()
  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    const csrf = getCSRFToken()
    if (csrf) {
      headers["X-CSRF-Token"] = csrf
    }
  }

  const res = await fetch(url, { ...options, headers, credentials: "include" })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      return fetch(url, { ...options, headers, credentials: "include" })
    }

    forceLogout()
  }

  return res
}

export interface MeResponse {
  email: string
  name: string
  avatar_url: string
  tier: string
  secrets_used: number
  secrets_limit: number
  max_text_length: number
}

export interface CreateSecretRequest {
  text: string
  to: string[]
}

export interface RecipientLink {
  email: string
  link: string
}

export interface CreateSecretResponse {
  id: string
  expires_at: string
  recipients: RecipientLink[]
}

export interface RevealRequest {
  email: string
  key: string
}

export interface RevealResponse {
  text: string
}

export interface CheckoutResponse {
  url: string
}

export const api = {
  me: () => request<MeResponse>("/me"),

  createSecret: (data: CreateSecretRequest) =>
    request<CreateSecretResponse>("/secrets", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  revealSecret: (token: string, data: RevealRequest) =>
    request<RevealResponse>(`/secrets/${token}/reveal`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  checkout: () =>
    authenticatedFetch(`${API_URL}/billing/checkout`, {
      method: "POST",
    }).then((r) => r.json() as Promise<CheckoutResponse>),

  portal: () =>
    authenticatedFetch(`${API_URL}/billing/portal`, {
      method: "POST",
    }).then((r) => r.json() as Promise<{ url: string }>),

  deleteAccount: () =>
    authenticatedFetch(`${API_BASE}/me`, {
      method: "DELETE",
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to delete account")
    }),

  logout: () =>
    fetch(`${API_URL}/auth/logout`, {
      method: "POST",
      credentials: "include",
    }),
}
```

**Step 2: Verify build**

Run: `cd frontend && npx tsc --noEmit`
Expected: No type errors

**Step 3: Commit**

```bash
git add frontend/src/api/client.ts
git commit -m "feat: migrate API client from localStorage to httpOnly cookie auth"
```

---

### Task 12: Frontend — Rewrite AuthContext

**Files:**
- Modify: `frontend/src/context/AuthContext.tsx`

**Step 1: Rewrite AuthContext.tsx**

```tsx
import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"
import { api, type MeResponse } from "../api/client"

interface AuthContextValue {
  user: MeResponse | null
  isAuthenticated: boolean
  isLoading: boolean
  logout: () => void
  refreshUser: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const fetchUser = useCallback(async () => {
    try {
      const me = await api.me()
      setUser(me)
    } catch {
      setUser(null)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  const logout = useCallback(async () => {
    await api.logout()
    setUser(null)
  }, [])

  return (
    <AuthContext value={{ user, isAuthenticated: !!user, isLoading, logout, refreshUser: fetchUser }}>
      {children}
    </AuthContext>
  )
}
```

**Step 2: Check for compile errors caused by removed `login`**

The `login` method was removed from `AuthContextValue`. Find all usages:

Run: `cd frontend && grep -rn "auth.login\|\.login(" src/ --include="*.tsx" --include="*.ts"`

The only consumer is `AuthCallback.tsx` which is updated in the next task.

**Step 3: Commit**

```bash
git add frontend/src/context/AuthContext.tsx
git commit -m "feat: remove localStorage from AuthContext, use cookie-based auth"
```

---

### Task 13: Frontend — Simplify AuthCallback

**Files:**
- Modify: `frontend/src/pages/AuthCallback.tsx`

**Step 1: Rewrite AuthCallback.tsx**

```tsx
import { useEffect } from "react"
import { useNavigate } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"

export default function AuthCallback() {
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    if (!auth) {
      navigate("/", { replace: true })
      return
    }

    // Cookies are already set by the backend redirect.
    // Just fetch the user profile and navigate to dashboard.
    auth.refreshUser().then(() => navigate("/dashboard", { replace: true }))
  }, [auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Signing in...</p>
    </div>
  )
}
```

**Step 2: Build frontend**

Run: `cd frontend && npm run build`
Expected: Build succeeds

**Step 3: Lint**

Run: `cd frontend && npx eslint .`
Expected: No errors

**Step 4: Commit**

```bash
git add frontend/src/pages/AuthCallback.tsx
git commit -m "feat: simplify AuthCallback to use cookie-based auth"
```

---

### Task 14: Full Integration Test

**Step 1: Run all backend tests**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 2: Run backend linter**

Run: `cd backend && golangci-lint run ./...`
Expected: No issues

**Step 3: Run frontend build**

Run: `cd frontend && npm run build`
Expected: Build succeeds

**Step 4: Run frontend lint**

Run: `cd frontend && npx eslint .`
Expected: No issues

**Step 5: Final commit if any fixes needed**

If any test/lint issues were found and fixed in previous steps:
```bash
git add -A
git commit -m "fix: resolve test/lint issues from httpOnly cookie migration"
```

---

### Task 15: Update OpenAPI Spec and CLAUDE.md

**Files:**
- Modify: `backend/docs/openapi.yaml` — add `POST /auth/logout` endpoint, update auth descriptions
- Modify: `CLAUDE.md` — add `POST /auth/logout` to API endpoints list

**Step 1: Add logout endpoint to openapi.yaml**

Add a new path entry for `POST /auth/logout` with 200 response.

**Step 2: Add to CLAUDE.md API endpoints**

Add line after the refresh endpoint:
```
- `POST /auth/logout` — Clear auth cookies (200)
```

**Step 3: Commit**

```bash
git add backend/docs/openapi.yaml CLAUDE.md
git commit -m "docs: add POST /auth/logout endpoint to OpenAPI spec and CLAUDE.md"
```
