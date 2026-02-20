package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const (
	googleUserInfoURL      = "https://www.googleapis.com/oauth2/v2/userinfo"
	oauthStateCookieName   = "oauth_state"
	oauthStateCookieMaxAge = 600 // 10 minutes
	stateBytes             = 32
)

// googleUserInfo represents the response from Google's userinfo endpoint.
type googleUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// GoogleConfig creates an OAuth2 config for Google.
func GoogleConfig(clientID, clientSecret, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// HandleGoogleLogin redirects the user to Google's OAuth consent page.
//
//nolint:revive // receiver unused but method needed for API consistency
func (s *Service) HandleGoogleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := generateState()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)

			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			MaxAge:   oauthStateCookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   true,
			Path:     "/",
		})

		http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}
}

// HandleGoogleCallback exchanges the auth code for tokens, fetches user info,
// upserts the user, and returns a JWT token pair.
func (s *Service) HandleGoogleCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		queryState := r.URL.Query().Get("state")

		if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(queryState)) != 1 {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": map[string]string{"type": "invalid_state", "message": "Invalid OAuth state"},
			})

			return
		}

		// Clear state cookie
		http.SetCookie(w, &http.Cookie{
			Name:   oauthStateCookieName,
			MaxAge: -1,
			Path:   "/",
		})

		// 2. Exchange code for token
		code := r.URL.Query().Get("code")

		token, err := cfg.Exchange(r.Context(), code)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "oauth_failed", "message": "Failed to exchange authorization code"},
			})

			return
		}

		// 3. Fetch user info
		client := cfg.Client(r.Context(), token)

		userInfo, err := fetchGoogleUserInfo(r.Context(), client)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to fetch user info"},
			})

			return
		}

		// 4. Upsert user
		u, err := userRepo.Upsert(r.Context(), &model.User{
			Provider:   "google",
			ProviderID: userInfo.ID,
			Email:      userInfo.Email,
			Name:       userInfo.Name,
			AvatarURL:  userInfo.Picture,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to create user"},
			})

			return
		}

		// 5. Generate JWT pair
		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		s.redirectWithTokens(w, r, pair)
	}
}

func fetchGoogleUserInfo(ctx context.Context, client *http.Client) (*googleUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create userinfo request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch google userinfo: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo returned status %d", resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode google userinfo: %w", err)
	}

	return &info, nil
}

func generateState() (string, error) {
	b := make([]byte, stateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}

	return base64.URLEncoding.EncodeToString(b), nil
}

// redirectWithTokens redirects to the frontend callback with tokens in query params.
func (s *Service) redirectWithTokens(w http.ResponseWriter, r *http.Request, pair *TokenPair) {
	u, _ := url.Parse(s.frontendBaseURL)
	u.Path = "/auth/callback"

	q := u.Query()
	q.Set("access_token", pair.AccessToken)
	q.Set("refresh_token", pair.RefreshToken)
	u.RawQuery = q.Encode()

	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}

// writeJSON is a small helper for auth handlers (not exported, auth-package only).
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
