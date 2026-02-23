package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/model"
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

func TestHandleTokenExchange_Google_TokenVerifyFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock Google tokeninfo endpoint returning non-200.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "oauth2.googleapis.com" && req.URL.Path == "/tokeninfo" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusBadRequest)

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

	body := `{"provider":"google","id_token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "oauth_failed" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "oauth_failed")
	}
}

func TestHandleTokenExchange_Google_TokenInfoBadJSON(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock Google tokeninfo endpoint returning invalid JSON.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "oauth2.googleapis.com" && req.URL.Path == "/tokeninfo" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString("not-valid-json{{{")

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

	body := `{"provider":"google","id_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestHandleTokenExchange_Google_AudienceMismatch(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock Google tokeninfo returning a different audience.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "oauth2.googleapis.com" && req.URL.Path == "/tokeninfo" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]string{
					"sub":   "google-sub-123",
					"email": "google@example.com",
					"name":  "Google User",
					"aud":   "wrong-client-id",
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

	// Create service with a specific Google client ID to trigger audience validation.
	svc, err := auth.New("test-secret", auth.WithGoogleClientID("expected-client-id"))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(userRepo)

	body := `{"provider":"google","id_token":"valid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "oauth_failed" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "oauth_failed")
	}
}

func TestHandleTokenExchange_Google_UpsertFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock Google tokeninfo endpoint returning valid response.
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

	// Use mock repo that fails on Upsert.
	repo := &mockUserRepo{
		upsertFn: func(_ context.Context, _ *model.User) (*model.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(repo)

	body := `{"provider":"google","id_token":"valid-google-id-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Message != "Failed to create user" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to create user")
	}
}

func TestHandleTokenExchange_Github_FetchUserInfoFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: GitHub /user returns non-200.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusUnauthorized)

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

	body := `{"provider":"github","id_token":"bad-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "oauth_failed" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "oauth_failed")
	}
}

func TestHandleTokenExchange_Github_EmailFetchFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty email, /user/emails returns non-200.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         12345,
					"login":      "testuser",
					"name":       "Test User",
					"email":      nil,
					"avatar_url": "https://avatars.githubusercontent.com/u/12345",
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "api.github.com" && req.URL.Path == "/user/emails" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusForbidden)

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

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Message != "Failed to fetch user email" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to fetch user email")
	}
}

func TestHandleTokenExchange_Github_UpsertFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns valid user info.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
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

	// Use mock repo that fails on Upsert.
	repo := &mockUserRepo{
		upsertFn: func(_ context.Context, _ *model.User) (*model.User, error) {
			return nil, errors.New("db error")
		},
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleTokenExchange(repo)

	body := `{"provider":"github","id_token":"github-access-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Message != "Failed to create user" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to create user")
	}
}

func TestHandleTokenExchange_Github_NameFallbackToLogin(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty name, so Login should be used as fallback.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         11111,
					"login":      "loginname",
					"name":       "",
					"email":      "login@example.com",
					"avatar_url": "https://avatars.githubusercontent.com/u/11111",
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

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify we got a valid token pair back.
	var pair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&pair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access_token is empty")
	}
}

func TestHandleTokenExchange_Google_TransportError(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: returns a network error for tokeninfo.
	mockTransport := &mockRoundTripper{
		handler: func(_ *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
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

	body := `{"provider":"google","id_token":"some-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestHandleRefresh_Success(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	// Create a user in the repo.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "refresh-test-123",
		Email:      "refresh@example.com",
		Name:       "Refresh User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Generate initial token pair.
	originalPair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	// Sleep briefly so new tokens get a different iat (JWT uses second precision).
	time.Sleep(1100 * time.Millisecond)

	body := fmt.Sprintf(`{"refresh_token":%q}`, originalPair.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: new token pair returned
	var newPair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&newPair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if newPair.AccessToken == "" {
		t.Error("access_token is empty")
	}

	if newPair.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}

	// Verify: tokens are rotated (different from original)
	if newPair.AccessToken == originalPair.AccessToken {
		t.Error("new access_token should differ from original (rotation)")
	}

	if newPair.RefreshToken == originalPair.RefreshToken {
		t.Error("new refresh_token should differ from original (rotation)")
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := `{"refresh_token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 401
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_refresh_token" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_refresh_token")
	}
}

func TestHandleRefresh_MissingBody(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "validation_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "validation_error")
	}

	wantMsg := "refresh_token is required"
	if resp.Error.Message != wantMsg {
		t.Errorf("error message = %q; want %q", resp.Error.Message, wantMsg)
	}
}

func TestHandleRefresh_DeletedUser(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	// Create a user, generate tokens, then delete user.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "deleted-user-123",
		Email:      "deleted@example.com",
		Name:       "Deleted User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	// Delete the user.
	if err := userRepo.DeleteUser(context.Background(), u.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 401
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "invalid_refresh_token" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_refresh_token")
	}

	wantMsg := "User not found"
	if resp.Error.Message != wantMsg {
		t.Errorf("error message = %q; want %q", resp.Error.Message, wantMsg)
	}
}

func TestHandleRefresh_InvalidJSON(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 400
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "validation_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "validation_error")
	}

	wantMsg := "Invalid JSON body"
	if resp.Error.Message != wantMsg {
		t.Errorf("error message = %q; want %q", resp.Error.Message, wantMsg)
	}
}

func TestHandleRefresh_ViaCookie(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	// Create a user in the repo.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "cookie-refresh-123",
		Email:      "cookie-refresh@example.com",
		Name:       "Cookie Refresh User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	svc, err := auth.New("test-secret", auth.WithSecureCookies(true))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Generate initial token pair.
	originalPair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	// Sleep briefly so new tokens get a different iat (JWT uses second precision).
	time.Sleep(1100 * time.Millisecond)

	// Send request with refresh_token as a cookie (no JSON body).
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  auth.CookieRefreshToken,
		Value: originalPair.RefreshToken,
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: response body is {"status":"ok"} (not a token pair).
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status = %q; want %q", body["status"], "ok")
	}

	// Verify: new auth cookies are set.
	cookies := rec.Result().Cookies()
	cookieMap := make(map[string]*http.Cookie)

	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	ac, ok := cookieMap[auth.CookieAccessToken]
	if !ok {
		t.Fatal("access_token cookie not set in response")
	}

	if ac.Value == "" {
		t.Error("access_token cookie value is empty")
	}

	if ac.Value == originalPair.AccessToken {
		t.Error("new access_token cookie should differ from original (rotation)")
	}

	rc, ok := cookieMap[auth.CookieRefreshToken]
	if !ok {
		t.Fatal("refresh_token cookie not set in response")
	}

	if rc.Value == "" {
		t.Error("refresh_token cookie value is empty")
	}

	if rc.Value == originalPair.RefreshToken {
		t.Error("new refresh_token cookie should differ from original (rotation)")
	}

	if _, ok := cookieMap[auth.CookieCSRFToken]; !ok {
		t.Error("csrf_token cookie not set in response")
	}
}

func TestHandleRefresh_RejectsAccessToken(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Create a user and generate a token pair.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "reject-access-sub",
		Email:      "reject@example.com",
		Name:       "Reject User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	// Use the ACCESS token as refresh_token — should be rejected.
	body := fmt.Sprintf(`{"refresh_token":%q}`, pair.AccessToken)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if resp.Error.Type != "invalid_refresh_token" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "invalid_refresh_token")
	}
}
