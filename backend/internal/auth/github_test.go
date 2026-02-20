package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func TestGithubConfig(t *testing.T) {
	t.Parallel()

	cfg := auth.GithubConfig("client-id", "client-secret", "http://localhost/callback")

	if cfg.ClientID != "client-id" {
		t.Errorf("ClientID = %q; want %q", cfg.ClientID, "client-id")
	}

	if cfg.ClientSecret != "client-secret" {
		t.Errorf("ClientSecret = %q; want %q", cfg.ClientSecret, "client-secret")
	}

	if cfg.RedirectURL != "http://localhost/callback" {
		t.Errorf("RedirectURL = %q; want %q", cfg.RedirectURL, "http://localhost/callback")
	}

	if len(cfg.Scopes) != 2 {
		t.Fatalf("Scopes length = %d; want 2", len(cfg.Scopes))
	}

	wantScopes := []string{"user:email", "read:user"}
	for i, s := range wantScopes {
		if cfg.Scopes[i] != s {
			t.Errorf("Scopes[%d] = %q; want %q", i, cfg.Scopes[i], s)
		}
	}

	// Verify GitHub endpoint
	if cfg.Endpoint.AuthURL != "https://github.com/login/oauth/authorize" {
		t.Errorf("Endpoint.AuthURL = %q; want GitHub auth URL", cfg.Endpoint.AuthURL)
	}

	if cfg.Endpoint.TokenURL != "https://github.com/login/oauth/access_token" {
		t.Errorf("Endpoint.TokenURL = %q; want GitHub token URL", cfg.Endpoint.TokenURL)
	}
}

func TestHandleGithubLogin_RedirectsToGithub(t *testing.T) {
	t.Parallel()

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	cfg := auth.GithubConfig("client-id", "client-secret", "http://localhost/callback")

	handler := svc.HandleGithubLogin(cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Verify: response is 307 redirect
	if rec.Code != http.StatusTemporaryRedirect {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusTemporaryRedirect)
	}

	// Verify: Location header contains github.com
	location := rec.Header().Get("Location")
	if location == "" {
		t.Fatal("Location header is empty")
	}

	if !containsSubstring(location, "github.com") {
		t.Errorf("Location = %q; want to contain %q", location, "github.com")
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

func TestHandleGithubCallback_Success(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Create oauth2.Config with mock token URL
	// The mock transport will intercept all HTTP calls
	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       []string{"user:email", "read:user"},
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}

	// Create a custom transport that intercepts all calls
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			// Token exchange request
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			// GitHub /user request
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         12345,
					"login":      "testuser",
					"name":       "Test User",
					"email":      "test@example.com",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345",
				})

				return rec.Result(), nil
			}

			return nil, http.ErrNotSupported
		},
	}

	// Set mock transport as default
	origTransport := http.DefaultTransport
	http.DefaultTransport = mockTransport

	defer func() { http.DefaultTransport = origTransport }()

	// Create in-memory user repo
	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	// Create auth service
	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleGithubCallback(cfg, userRepo)

	// Set oauth_state cookie and state + code query params
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "test-state",
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: Response 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: Body contains access_token and refresh_token
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

func TestHandleGithubCallback_EmailFromEmailsAPI(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURL:  "http://localhost/callback",
		Scopes:       []string{"user:email", "read:user"},
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}

	// Create a mock transport where /user returns null email
	// and /user/emails returns the primary verified email
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			// Token exchange
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			// GitHub /user — email is empty
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         67890,
					"login":      "privateuser",
					"name":       "",
					"email":      nil,
					"avatar_url": "https://avatars.githubusercontent.com/u/67890",
				})

				return rec.Result(), nil
			}

			// GitHub /user/emails — return primary verified email
			if req.URL.Host == "api.github.com" && req.URL.Path == "/user/emails" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode([]map[string]any{
					{"email": "secondary@example.com", "primary": false, "verified": true},
					{"email": "primary@example.com", "primary": true, "verified": true},
					{"email": "unverified@example.com", "primary": false, "verified": false},
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{
		Name:  "oauth_state",
		Value: "test-state",
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify: Response 200
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify: Body contains access_token and refresh_token
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

func TestHandleGithubCallback_InvalidState(t *testing.T) {
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	// Send request with wrong state cookie
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=correct-state&code=test-code", nil)
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

func TestHandleGithubCallback_MissingState(t *testing.T) {
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	// Send request without state cookie.
	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=some-state&code=test-code", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandleGithubCallback_CodeExchangeFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport that fails the token exchange.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.WriteHeader(http.StatusUnauthorized)
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString(`{"error":"bad_verification_code"}`)

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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=bad-code", nil)
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

func TestHandleGithubCallback_FetchUserInfoFailure(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange succeeds but /user returns non-200.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
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

	if resp.Error.Message != "Failed to fetch user info" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to fetch user info")
	}
}

func TestHandleGithubCallback_FetchUserInfoBadJSON(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange succeeds, /user returns invalid JSON.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString("invalid-json{{{")

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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestHandleGithubCallback_EmailFetchFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty email, /user/emails returns non-200.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

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

	cfg := &oauth2.Config{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
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

	if resp.Error.Message != "Failed to fetch user email" {
		t.Errorf("error message = %q; want %q", resp.Error.Message, "Failed to fetch user email")
	}
}

func TestHandleGithubCallback_EmailBadJSON(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty email, /user/emails returns invalid JSON.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

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
				rec.Header().Set("Content-Type", "application/json")
				rec.Body.WriteString("not-valid-json[[[")

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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestHandleGithubCallback_NoPrimaryVerifiedEmail(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty email, /user/emails returns no primary verified email.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

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
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode([]map[string]any{
					{"email": "unverified@example.com", "primary": true, "verified": false},
					{"email": "secondary@example.com", "primary": false, "verified": true},
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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want %d; body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}

func TestHandleGithubCallback_UpsertFailure(t *testing.T) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: token exchange and /user both succeed.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         12345,
					"login":      "testuser",
					"name":       "Test User",
					"email":      "test@example.com",
					"avatar_url": "https://avatars.githubusercontent.com/u/12345",
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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, repo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
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

func TestHandleGithubCallback_NameFallbackToLogin(
	t *testing.T,
) { //nolint:paralleltest // modifies http.DefaultTransport
	// Mock transport: /user returns empty name, Login should be used.
	mockTransport := &mockRoundTripper{
		handler: func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "github.com" && req.URL.Path == "/login/oauth/access_token" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"access_token": "mock-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				})

				return rec.Result(), nil
			}

			if req.URL.Host == "api.github.com" && req.URL.Path == "/user" {
				rec := httptest.NewRecorder()
				rec.Header().Set("Content-Type", "application/json")
				json.NewEncoder(rec).Encode(map[string]any{
					"id":         22222,
					"login":      "loginonly",
					"name":       "",
					"email":      "login@example.com",
					"avatar_url": "https://avatars.githubusercontent.com/u/22222",
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
			TokenURL: "https://github.com/login/oauth/access_token",
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

	handler := svc.HandleGithubCallback(cfg, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?state=test-state&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "test-state"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var pair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&pair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access_token is empty")
	}
}
