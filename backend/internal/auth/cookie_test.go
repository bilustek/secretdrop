package auth_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
)

func TestSetTokenCookies(t *testing.T) {
	t.Parallel()

	pair := &auth.TokenPair{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
	}
	csrfToken := "test-csrf-token"
	accessExpiry := 15 * time.Minute
	refreshExpiry := 30 * 24 * time.Hour

	tests := []struct {
		name   string
		secure bool
	}{
		{"secure mode", true},
		{"insecure mode", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			auth.SetTokenCookies(rec, pair, csrfToken, tt.secure, accessExpiry, refreshExpiry)

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

			if ac.Value != "test-access-token" {
				t.Errorf("access_token value = %q; want %q", ac.Value, "test-access-token")
			}

			if !ac.HttpOnly {
				t.Error("access_token HttpOnly = false; want true")
			}

			if ac.Secure != tt.secure {
				t.Errorf("access_token Secure = %v; want %v", ac.Secure, tt.secure)
			}

			if ac.SameSite != http.SameSiteLaxMode {
				t.Errorf("access_token SameSite = %v; want Lax", ac.SameSite)
			}

			if ac.Path != "/" {
				t.Errorf("access_token Path = %q; want %q", ac.Path, "/")
			}

			if ac.MaxAge != int(accessExpiry.Seconds()) {
				t.Errorf("access_token MaxAge = %d; want %d", ac.MaxAge, int(accessExpiry.Seconds()))
			}

			// Verify refresh_token cookie.
			rc, ok := cookieMap[auth.CookieRefreshToken]
			if !ok {
				t.Fatal("refresh_token cookie not found")
			}

			if rc.Value != "test-refresh-token" {
				t.Errorf("refresh_token value = %q; want %q", rc.Value, "test-refresh-token")
			}

			if !rc.HttpOnly {
				t.Error("refresh_token HttpOnly = false; want true")
			}

			if rc.Secure != tt.secure {
				t.Errorf("refresh_token Secure = %v; want %v", rc.Secure, tt.secure)
			}

			if rc.SameSite != http.SameSiteLaxMode {
				t.Errorf("refresh_token SameSite = %v; want Lax", rc.SameSite)
			}

			if rc.Path != "/auth/refresh" {
				t.Errorf("refresh_token Path = %q; want %q", rc.Path, "/auth/refresh")
			}

			if rc.MaxAge != int(refreshExpiry.Seconds()) {
				t.Errorf("refresh_token MaxAge = %d; want %d", rc.MaxAge, int(refreshExpiry.Seconds()))
			}

			// Verify csrf_token cookie.
			cc, ok := cookieMap[auth.CookieCSRFToken]
			if !ok {
				t.Fatal("csrf_token cookie not found")
			}

			if cc.Value != "test-csrf-token" {
				t.Errorf("csrf_token value = %q; want %q", cc.Value, "test-csrf-token")
			}

			if cc.HttpOnly {
				t.Error("csrf_token HttpOnly = true; want false")
			}

			if cc.Secure != tt.secure {
				t.Errorf("csrf_token Secure = %v; want %v", cc.Secure, tt.secure)
			}

			if cc.SameSite != http.SameSiteLaxMode {
				t.Errorf("csrf_token SameSite = %v; want Lax", cc.SameSite)
			}

			if cc.Path != "/" {
				t.Errorf("csrf_token Path = %q; want %q", cc.Path, "/")
			}

			if cc.MaxAge != int(refreshExpiry.Seconds()) {
				t.Errorf("csrf_token MaxAge = %d; want %d", cc.MaxAge, int(refreshExpiry.Seconds()))
			}
		})
	}
}

func TestClearTokenCookies(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	auth.ClearTokenCookies(rec)

	cookies := rec.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookie count = %d; want 3", len(cookies))
	}

	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	// Verify access_token cookie is cleared.
	ac, ok := cookieMap[auth.CookieAccessToken]
	if !ok {
		t.Fatal("access_token cookie not found")
	}

	if ac.Value != "" {
		t.Errorf("access_token value = %q; want empty", ac.Value)
	}

	if ac.MaxAge != -1 {
		t.Errorf("access_token MaxAge = %d; want -1", ac.MaxAge)
	}

	if ac.Path != "/" {
		t.Errorf("access_token Path = %q; want %q", ac.Path, "/")
	}

	// Verify refresh_token cookie is cleared.
	rc, ok := cookieMap[auth.CookieRefreshToken]
	if !ok {
		t.Fatal("refresh_token cookie not found")
	}

	if rc.Value != "" {
		t.Errorf("refresh_token value = %q; want empty", rc.Value)
	}

	if rc.MaxAge != -1 {
		t.Errorf("refresh_token MaxAge = %d; want -1", rc.MaxAge)
	}

	if rc.Path != "/auth/refresh" {
		t.Errorf("refresh_token Path = %q; want %q", rc.Path, "/auth/refresh")
	}

	// Verify csrf_token cookie is cleared.
	cc, ok := cookieMap[auth.CookieCSRFToken]
	if !ok {
		t.Fatal("csrf_token cookie not found")
	}

	if cc.Value != "" {
		t.Errorf("csrf_token value = %q; want empty", cc.Value)
	}

	if cc.MaxAge != -1 {
		t.Errorf("csrf_token MaxAge = %d; want -1", cc.MaxAge)
	}

	if cc.Path != "/" {
		t.Errorf("csrf_token Path = %q; want %q", cc.Path, "/")
	}
}

func TestClearTokenCookies_HttpOnlyFlags(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	auth.ClearTokenCookies(rec)

	cookies := rec.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)

	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	// access_token and refresh_token should be HttpOnly.
	if ac, ok := cookieMap[auth.CookieAccessToken]; ok && !ac.HttpOnly {
		t.Error("cleared access_token HttpOnly = false; want true")
	}

	if rc, ok := cookieMap[auth.CookieRefreshToken]; ok && !rc.HttpOnly {
		t.Error("cleared refresh_token HttpOnly = false; want true")
	}

	// csrf_token should NOT be HttpOnly.
	if cc, ok := cookieMap[auth.CookieCSRFToken]; ok && cc.HttpOnly {
		t.Error("cleared csrf_token HttpOnly = true; want false")
	}
}

func TestGenerateCSRFToken(t *testing.T) {
	t.Parallel()

	token, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken() error = %v", err)
	}

	if token == "" {
		t.Fatal("GenerateCSRFToken() returned empty token")
	}

	// Verify it is valid base64 RawURL encoding.
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("base64.RawURLEncoding.DecodeString() error = %v", err)
	}

	// Verify the decoded length is 32 bytes.
	if len(decoded) != 32 {
		t.Errorf("decoded token length = %d; want 32", len(decoded))
	}
}

func TestGenerateCSRFToken_Uniqueness(t *testing.T) {
	t.Parallel()

	token1, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken() #1 error = %v", err)
	}

	token2, err := auth.GenerateCSRFToken()
	if err != nil {
		t.Fatalf("GenerateCSRFToken() #2 error = %v", err)
	}

	if token1 == token2 {
		t.Error("two calls to GenerateCSRFToken() returned the same value")
	}
}

func TestCookieConstants(t *testing.T) {
	t.Parallel()

	if auth.CookieAccessToken != "access_token" {
		t.Errorf("CookieAccessToken = %q; want %q", auth.CookieAccessToken, "access_token")
	}

	if auth.CookieRefreshToken != "refresh_token" {
		t.Errorf("CookieRefreshToken = %q; want %q", auth.CookieRefreshToken, "refresh_token")
	}

	if auth.CookieCSRFToken != "csrf_token" {
		t.Errorf("CookieCSRFToken = %q; want %q", auth.CookieCSRFToken, "csrf_token")
	}
}
