package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

func TestBasicAuth_ValidCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestBasicAuth_InvalidCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	if got := rec.Header().Get("WWW-Authenticate"); got != `Basic realm="admin"` {
		t.Errorf("WWW-Authenticate = %q; want %q", got, `Basic realm="admin"`)
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}
