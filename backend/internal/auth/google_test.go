package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func TestGoogleConfig(t *testing.T) {
	t.Parallel()

	cfg := auth.GoogleConfig("client-id", "client-secret", "http://localhost/callback")

	if cfg.ClientID != "client-id" {
		t.Errorf("ClientID = %q; want %q", cfg.ClientID, "client-id")
	}

	if cfg.ClientSecret != "client-secret" {
		t.Errorf("ClientSecret = %q; want %q", cfg.ClientSecret, "client-secret")
	}

	if cfg.RedirectURL != "http://localhost/callback" {
		t.Errorf("RedirectURL = %q; want %q", cfg.RedirectURL, "http://localhost/callback")
	}

	if len(cfg.Scopes) != 3 {
		t.Fatalf("Scopes length = %d; want 3", len(cfg.Scopes))
	}

	wantScopes := []string{"openid", "email", "profile"}
	for i, s := range wantScopes {
		if cfg.Scopes[i] != s {
			t.Errorf("Scopes[%d] = %q; want %q", i, cfg.Scopes[i], s)
		}
	}
}

func TestHandleGoogleLogin_RedirectsToGoogle(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := auth.GoogleConfig("client-id", "client-secret", "http://localhost/callback")

	handler := svc.HandleGoogleLogin(cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify: response is 307 redirect
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	// Verify: Location header contains accounts.google.com
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	if !containsSubstring(location, "accounts.google.com") {
		t.Errorf("Location = %q; want to contain %q", location, "accounts.google.com")
	}

	// Verify: oauth_state cookie is set
	cookies := rec.Result().Cookies()

	var stateCookie *http.Cookie

	for _, c := range cookies {
		if c.Name == "oauth_state" {
			stateCookie = c

			break
		}
	}

	if stateCookie == nil {
		t.Fatal("oauth_state cookie not set")
	}

	if stateCookie.Value == "" {
		t.Error("oauth_state cookie value is empty")
	}

	if !stateCookie.HttpOnly {
		t.Error("oauth_state cookie should be HttpOnly")
	}

	if !stateCookie.Secure {
		t.Error("oauth_state cookie should be Secure")
	}

	if stateCookie.MaxAge != 600 {
		t.Errorf("oauth_state cookie MaxAge = %d; want 600", stateCookie.MaxAge)
	}
}

func TestHandleGoogleCallback_Success(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// 1. Create mock server that serves both token endpoint and userinfo endpoint
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "mock-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/userinfo":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"id":      "google-user-123",
				"email":   "test@example.com",
				"name":    "Test User",
				"picture": "https://example.com/avatar.jpg",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// 2. Create oauth2.Config pointing to mock server
	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint: oauth2.Endpoint{
			TokenURL: mockServer.URL + "/token",
		},
	}

	// Override googleUserInfoURL by using a custom mock
	// We need the userinfo URL to point to our mock server.
	// Since fetchGoogleUserInfo uses the constant, we mock the HTTP client
	// via the oauth2 config's token exchange which sets up the client.
	// The client.Get call goes to googleUserInfoURL which is Google's real URL.
	// Instead, we create our own test by constructing everything manually.

	// 3. Create in-memory user repo
	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	// 4. Create auth service
	svc, err := auth.New("test-secret", auth.WithFrontendBaseURL("http://localhost:3000"))
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// We need to test HandleGoogleCallback which calls fetchGoogleUserInfo
	// with the constant googleUserInfoURL. To make the test work without
	// hitting real Google, we override the transport on the test HTTP client.

	// Create a custom transport that intercepts the userinfo call
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			// Token exchange request
			if req.URL.Path == "/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			// Userinfo request
			if req.URL.Host == "www.googleapis.com" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]string{
					"id":      "google-user-123",
					"email":   "test@example.com",
					"name":    "Test User",
					"picture": "https://example.com/avatar.jpg",
				})

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	// Set the mock transport as default for the test
	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	// 5. Set oauth_state cookie on request and state + code query params
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "test-state",
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: Response 307 redirect
	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}

	// Verify: Location header contains frontend callback with tokens
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	locURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse location URL: %v", err)
	}

	if locURL.Path != "/auth/callback" {
		t.Errorf("redirect path = %q; want %q", locURL.Path, "/auth/callback")
	}

	if locURL.Query().Get("access_token") == "" {
		t.Error("access_token missing from redirect URL")
	}

	if locURL.Query().Get("refresh_token") == "" {
		t.Error("refresh_token missing from redirect URL")
	}
}

func TestHandleGoogleCallback_InvalidState(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	// Send request with wrong state cookie
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=correct-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "wrong-state",
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: 403
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}

	// Verify: error type "invalid_state"
	var body map[string]map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["error"]["type"] != "invalid_state" {
		t.Errorf("error type = %q; want %q", body["error"]["type"], "invalid_state")
	}
}

func TestHandleGoogleCallback_MissingState(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
	}

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	// Send request without state cookie
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=some-state&code=test-code", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify: 403
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

// mockRoundTripper implements http.RoundTripper for testing.
type mockRoundTripper struct {
	handler func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.handler(req)
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := range len(s) - len(substr) + 1 {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// mockUserRepo implements user.Repository for testing error paths.
type mockUserRepo struct {
	upsertFn func(ctx context.Context, u *model.User) (*model.User, error)
}

func (m *mockUserRepo) Upsert(ctx context.Context, u *model.User) (*model.User, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, u)
	}

	return nil, errors.New("upsert not configured")
}

func (m *mockUserRepo) FindByID(_ context.Context, _ int64) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) FindByProvider(_ context.Context, _, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) IncrementSecretsUsed(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) ResetSecretsUsed(_ context.Context, _ int64) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpdateTier(_ context.Context, _ int64, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpsertSubscription(_ context.Context, _ *model.Subscription) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) FindSubscriptionByUserID(_ context.Context, _ int64) (*model.Subscription, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) FindUserByStripeCustomerID(_ context.Context, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserRepo) UpdateSubscriptionStatus(_ context.Context, _, _ string) error {
	return errors.New("not implemented")
}

func (m *mockUserRepo) UpdateSubscriptionPeriod(_ context.Context, _ string, _, _ time.Time) error {
	return errors.New("not implemented")
}

// errorResponse is a helper type for decoding error JSON responses.
type errorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestHandleGoogleCallback_CodeExchangeFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport that fails the token exchange.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			// Token exchange — return 401 to simulate failure.
			if req.URL.Path == "/token" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusUnauthorized)
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString(`{"error":"invalid_grant"}`)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://mock.example.com/token",
		},
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=test-state&code=bad-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

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

func TestHandleGoogleCallback_FetchUserInfoFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange succeeds but userinfo fails.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			// Userinfo — return non-200.
			if req.URL.Host == "www.googleapis.com" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusServiceUnavailable)

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://mock.example.com/token",
		},
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error.Type != "internal_error" {
		t.Errorf("error type = %q; want %q", resp.Error.Type, "internal_error")
	}
}

func TestHandleGoogleCallback_UpsertFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange and userinfo both succeed.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "www.googleapis.com" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]string{
					"id":      "google-user-123",
					"email":   "test@example.com",
					"name":    "Test User",
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

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://mock.example.com/token",
		},
	}

	// Use mock repo that fails on Upsert.
	repo := &mockUserRepo{
		upsertFn: func(_ context.Context, _ *model.User) (*model.User, error) {
			return nil, errors.New("db connection lost")
		},
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleGoogleCallback(cfg, repo)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

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

func TestHandleGoogleCallback_FetchUserInfoBadJSON(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange succeeds, userinfo returns invalid JSON.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "www.googleapis.com" {
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

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://mock.example.com/token",
		},
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleGoogleCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
