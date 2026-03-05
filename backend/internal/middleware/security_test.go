package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilustek/secretdrop/internal/middleware"
)

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeaders()(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for name, want := range headers {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q; want %q", name, got, want)
		}
	}

	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("Content-Security-Policy should be set")
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeaders()(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")

	requiredDirectives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self'",
		"connect-src 'self'",
		"font-src 'self'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"base-uri 'self'",
	}

	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive %q; got %q", directive, csp)
		}
	}
}

func TestSecurityHeaders_DocsCSP(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.SecurityHeaders()(inner)

	tests := []struct {
		name           string
		path           string
		wantCDN        bool
		wantObjectNone bool
	}{
		{
			name:           "docs path gets CDN CSP",
			path:           "/docs",
			wantCDN:        true,
			wantObjectNone: true,
		},
		{
			name:           "docs subpath gets CDN CSP",
			path:           "/docs/index.html",
			wantCDN:        true,
			wantObjectNone: true,
		},
		{
			name:           "api path gets default CSP",
			path:           "/api/test",
			wantCDN:        false,
			wantObjectNone: true,
		},
		{
			name:           "root path gets default CSP",
			path:           "/",
			wantCDN:        false,
			wantObjectNone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			csp := rec.Header().Get("Content-Security-Policy")

			hasCDN := strings.Contains(csp, "cdn.jsdelivr.net")
			if hasCDN != tt.wantCDN {
				t.Errorf(
					"path %q: CSP contains cdn.jsdelivr.net = %v; want %v\nCSP: %s",
					tt.path,
					hasCDN,
					tt.wantCDN,
					csp,
				)
			}

			if tt.wantObjectNone && !strings.Contains(csp, "object-src 'none'") {
				t.Errorf("path %q: CSP missing object-src 'none'\nCSP: %s", tt.path, csp)
			}
		})
	}
}

func TestSecurityHeaders_PassesThrough(t *testing.T) {
	t.Parallel()

	called := false

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	handler := middleware.SecurityHeaders()(inner)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("next handler should be called")
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
}
