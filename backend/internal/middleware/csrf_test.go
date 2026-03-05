package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilustek/secretdrop/internal/middleware"
)

func TestCSRF_SafeMethodsPass(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			called := false

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.CSRF()(inner)

			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if !called {
				t.Errorf("%s: next handler should be called", method)
			}

			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d; want %d", method, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestCSRF_PostWithMatchingTokens(t *testing.T) {
	t.Parallel()

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf-token-value"})
	req.Header.Set("X-CSRF-Token", "test-csrf-token-value")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called with matching CSRF tokens")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestCSRF_PostWithMismatchedTokens(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called with mismatched CSRF tokens")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "cookie-token"})
	req.Header.Set("X-CSRF-Token", "different-header-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRF_PostWithMissingHeader(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called without CSRF header")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRF_PostWithMissingCookie(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called without CSRF cookie")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("X-CSRF-Token", "test-csrf-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRF_ExemptPaths(t *testing.T) {
	t.Parallel()

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF("/billing/webhook", "/auth/apple/callback")(inner)

	req := httptest.NewRequest(http.MethodPost, "/billing/webhook", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called for exempt path")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestCSRF_SkipsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{"bearer token", "Bearer eyJhbGciOiJIUzI1NiJ9.test"},
		{"basic auth", "Basic dXNlcjpwYXNz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			called := false

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.CSRF()(inner)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
			req.Header.Set("Authorization", tt.value)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if !called {
				t.Errorf("%s: next handler should be called when Authorization header present", tt.name)
			}

			if rec.Code != http.StatusOK {
				t.Errorf("%s: status = %d; want %d", tt.name, rec.Code, http.StatusOK)
			}
		})
	}
}

func TestCSRF_ExemptPrefixMatch(t *testing.T) {
	t.Parallel()

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF("/api/v1/secrets/")(inner)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets/abc123/reveal", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called for prefix-matched exempt path")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestCSRF_PutAndDeleteRequireCSRF(t *testing.T) {
	t.Parallel()

	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				t.Errorf("%s: next handler should not be called without CSRF tokens", method)
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.CSRF()(inner)

			req := httptest.NewRequest(method, "/test", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("%s: status = %d; want %d", method, rec.Code, http.StatusForbidden)
			}
		})
	}
}

func TestCSRF_ErrorResponseFormat(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("next handler should not be called")
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.CSRF()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q; want %q", ct, "application/json")
	}

	var resp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "csrf_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "csrf_error")
	}

	if resp.Error.Message != "CSRF validation failed" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "CSRF validation failed")
	}
}
