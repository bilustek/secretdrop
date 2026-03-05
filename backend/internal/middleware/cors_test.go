package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilustek/secretdrop/internal/middleware"
)

func TestCORS_MatchingOrigin(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://secretdrop.us")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called")
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://secretdrop.us" {
		t.Errorf("Access-Control-Allow-Origin = %q; want %q", got, "https://secretdrop.us")
	}

	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("Access-Control-Allow-Methods should be set")
	}

	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("Access-Control-Allow-Headers should be set")
	}
}

func TestCORS_NonMatchingOrigin(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called even for non-matching origin")
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q; want empty", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "https://secretdrop.us")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("next handler should NOT be called for preflight")
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusNoContent)
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://secretdrop.us" {
		t.Errorf("Access-Control-Allow-Origin = %q; want %q", got, "https://secretdrop.us")
	}
}

func TestCORS_NoOriginHeader(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called when no Origin header")
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q; want empty", got)
	}
}

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
}

func TestCORS_AllowCSRFTokenHeader(t *testing.T) {
	t.Parallel()

	handler := middleware.CORS("https://secretdrop.us")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://secretdrop.us")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	allowHeaders := rec.Header().Get("Access-Control-Allow-Headers")
	if !strings.Contains(allowHeaders, "X-CSRF-Token") {
		t.Errorf("Access-Control-Allow-Headers = %q; want to contain %q", allowHeaders, "X-CSRF-Token")
	}
}
