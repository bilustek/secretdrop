package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

// --- RequireJSON ---

func TestRequireJSON_PostWithoutJSON(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestRequireJSON_PostWithJSON(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireJSON_GetWithoutJSON(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireJSON_PutWithoutJSON(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPut, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestRequireJSON_PatchWithoutJSON(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPatch, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestRequireJSON_SkippedPath(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "/auth/apple/callback")

	req := httptest.NewRequest(http.MethodPost, "/auth/apple/callback", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestRequireJSON_NonSkippedPathStillBlocked(t *testing.T) {
	t.Parallel()

	handler := middleware.RequireJSON(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "/auth/apple/callback")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/secrets", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

// --- RequestID ---

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	t.Parallel()

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID should be set")
	}

	if len(id) != 36 {
		t.Errorf("X-Request-ID length = %d; want 36 (UUID format)", len(id))
	}
}

func TestRequestID_PreservesExisting(t *testing.T) {
	t.Parallel()

	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "custom-id-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	id := rec.Header().Get("X-Request-ID")
	if id != "custom-id-123" {
		t.Errorf("X-Request-ID = %q; want %q", id, "custom-id-123")
	}
}

// --- Logging ---

func TestLogging_SetsStatusCode(t *testing.T) {
	t.Parallel()

	handler := middleware.Logging(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusCreated)
	}
}

func TestLogging_DefaultsToOK(t *testing.T) {
	t.Parallel()

	handler := middleware.Logging(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// no explicit WriteHeader
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}
