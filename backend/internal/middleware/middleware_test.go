package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// --- RateLimiter ---

func TestNewRateLimiterDefaults(t *testing.T) {
	t.Parallel()

	rl, err := middleware.NewRateLimiter()
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}
}

func TestRateLimiterAllowsUpToRate(t *testing.T) {
	t.Parallel()

	rl, err := middleware.NewRateLimiter(middleware.WithRate(3))
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request should pass; got status %d", rec.Code)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	t.Parallel()

	rl, err := middleware.NewRateLimiter(middleware.WithRate(1))
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First IP
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "1.1.1.1:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first IP status = %d; want %d", rec1.Code, http.StatusOK)
	}

	// Second IP should also pass
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "2.2.2.2:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("second IP status = %d; want %d", rec2.Code, http.StatusOK)
	}
}

func TestRateLimiterUsesXForwardedFor(t *testing.T) {
	t.Parallel()

	rl, err := middleware.NewRateLimiter(middleware.WithRate(1))
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	handler := rl.Limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("X-Forwarded-For", "10.0.0.1")
	req1.RemoteAddr = "9.9.9.9:1234"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Errorf("first request status = %d; want %d", rec1.Code, http.StatusOK)
	}

	// Second request from same X-Forwarded-For should be limited
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Forwarded-For", "10.0.0.1")
	req2.RemoteAddr = "9.9.9.9:1234"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d; want %d", rec2.Code, http.StatusTooManyRequests)
	}
}

func TestWithRateInvalid(t *testing.T) {
	t.Parallel()

	_, err := middleware.NewRateLimiter(middleware.WithRate(0))
	if err == nil {
		t.Fatal("WithRate(0) should fail")
	}

	_, err = middleware.NewRateLimiter(middleware.WithRate(-1))
	if err == nil {
		t.Fatal("WithRate(-1) should fail")
	}
}

func TestWithWindowValid(t *testing.T) {
	t.Parallel()

	rl, err := middleware.NewRateLimiter(middleware.WithWindow(5 * time.Minute))
	if err != nil {
		t.Fatalf("NewRateLimiter() error = %v", err)
	}

	if rl == nil {
		t.Fatal("NewRateLimiter() returned nil")
	}
}

func TestWithWindowInvalid(t *testing.T) {
	t.Parallel()

	_, err := middleware.NewRateLimiter(middleware.WithWindow(0))
	if err == nil {
		t.Fatal("WithWindow(0) should fail")
	}

	_, err = middleware.NewRateLimiter(middleware.WithWindow(-1))
	if err == nil {
		t.Fatal("WithWindow(-1) should fail")
	}
}
