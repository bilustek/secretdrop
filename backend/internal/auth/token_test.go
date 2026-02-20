package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func TestHandleTokenExchange_Google_Success(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock Google tokeninfo endpoint via DefaultTransport replacement.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "oauth2.googleapis.com" && req.URL.Path == "/tokeninfo" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]string{
					"sub":     "google-sub-123",
					"email":   "google@example.com",
					"name":    "Google User",
					"picture": "https://example.com/avatar.jpg",
				})

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	body := `{"provider":"google","id_token":"valid-google-id-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: access_token and refresh_token in response
	var pair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&pair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access_token is empty")
	}

	if pair.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
}

func TestHandleTokenExchange_Github_Success(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock GitHub /user endpoint via DefaultTransport replacement.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				// Verify the Authorization header is set correctly.
				authHeader := req.Header.Get("Authorization")
				if authHeader != "Bearer github-access-token" {
					rec := httptest.NewRecorder()
					rec.WriteHeader(http.StatusUnauthorized)

					return rec.Result(), nil
				}

				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         54321,
					"login":      "ghuser",
					"name":       "GitHub User",
					"email":      "github@example.com",
					"avatar_url": "https://avatars.githubusercontent.com/u/54321",
				})

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	body := `{"provider":"github","id_token":"github-access-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: access_token and refresh_token in response
	var pair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&pair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access_token is empty")
	}

	if pair.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}
}

func TestHandleTokenExchange_InvalidProvider(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	body := `{"provider":"facebook","id_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	// Verify: error message about unsupported provider
	var resp map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["error"]["type"] != "validation_error" {
		t.Errorf("error type = %q; want %q", resp["error"]["type"], "validation_error")
	}

	wantMsg := "Unsupported provider"
	if !containsSubstring(resp["error"]["message"], wantMsg) {
		t.Errorf("error message = %q; want to contain %q", resp["error"]["message"], wantMsg)
	}
}

func TestHandleTokenExchange_MissingFields(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	tests := []struct {
		name string
		body string
	}{
		{"missing id_token", `{"provider":"google"}`},
		{"missing provider", `{"id_token":"some-token"}`},
		{"both empty", `{"provider":"","id_token":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// Verify: 400
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}

			var resp map[string]map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp["error"]["type"] != "validation_error" {
				t.Errorf("error type = %q; want %q", resp["error"]["type"], "validation_error")
			}
		})
	}
}

func TestHandleTokenExchange_InvalidBody(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader("not-valid-json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp["error"]["type"] != "validation_error" {
		t.Errorf("error type = %q; want %q", resp["error"]["type"], "validation_error")
	}

	wantMsg := "Invalid JSON body"
	if resp["error"]["message"] != wantMsg {
		t.Errorf("error message = %q; want %q", resp["error"]["message"], wantMsg)
	}
}
