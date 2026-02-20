package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
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
	svc, err := auth.New("test-secret")
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
