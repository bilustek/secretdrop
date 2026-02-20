package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const (
	githubUserURL   = "https://api.github.com/user"
	githubEmailsURL = "https://api.github.com/user/emails"
)

// githubUserInfo represents the response from GitHub's /user endpoint.
type githubUserInfo struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// githubEmail represents a single entry from GitHub's /user/emails endpoint.
type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// GithubConfig creates an OAuth2 config for GitHub.
func GithubConfig(clientID, clientSecret, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"user:email", "read:user"},
		Endpoint:     github.Endpoint,
	}
}

// HandleGithubLogin redirects the user to GitHub's OAuth consent page.
//
//nolint:revive // receiver unused but method needed for API consistency
func (s *Service) HandleGithubLogin(cfg *oauth2.Config) http.HandlerFunc {
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

// HandleGithubCallback exchanges the auth code for tokens, fetches user info,
// upserts the user, and returns a JWT token pair.
func (s *Service) HandleGithubCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
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

		userInfo, err := fetchGithubUserInfo(r.Context(), client)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to fetch user info"},
			})

			return
		}

		// 4. If email is empty, fetch from /user/emails endpoint
		if userInfo.Email == "" {
			email, emailErr := fetchGithubPrimaryEmail(r.Context(), client)
			if emailErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]string{"type": "internal_error", "message": "Failed to fetch user email"},
				})

				return
			}

			userInfo.Email = email
		}

		// 5. Use Name, fall back to Login if Name is empty
		name := userInfo.Name
		if name == "" {
			name = userInfo.Login
		}

		// 6. Upsert user
		u, err := userRepo.Upsert(r.Context(), &model.User{
			Provider:   "github",
			ProviderID: strconv.FormatInt(userInfo.ID, 10),
			Email:      userInfo.Email,
			Name:       name,
			AvatarURL:  userInfo.AvatarURL,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to create user"},
			})

			return
		}

		// 7. Generate JWT pair
		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		writeJSON(w, http.StatusOK, pair)
	}
}

func fetchGithubUserInfo(ctx context.Context, client *http.Client) (*githubUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create github user request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch github user: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user returned status %d", resp.StatusCode)
	}

	var info githubUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode github user: %w", err)
	}

	return &info, nil
}

func fetchGithubPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEmailsURL, nil)
	if err != nil {
		return "", fmt.Errorf("create github emails request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch github emails: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github emails returned status %d", resp.StatusCode)
	}

	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decode github emails: %w", err)
	}

	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}

	return "", errors.New("no primary verified email found")
}
