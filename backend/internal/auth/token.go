package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/bilustek/secretdrop/internal/model"
	"github.com/bilustek/secretdrop/internal/user"
)

const googleTokenInfoURL = "https://oauth2.googleapis.com/tokeninfo" //nolint:gosec // URL, not a credential

// tokenExchangeRequest is the request body for POST /auth/token.
type tokenExchangeRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
}

// googleTokenInfo represents the response from Google's tokeninfo endpoint.
type googleTokenInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	Aud     string `json:"aud"`
}

// HandleTokenExchange verifies a provider token and returns a JWT pair.
// For Google: verifies the ID token via Google's tokeninfo endpoint.
// For GitHub: uses the token as an access token to fetch user info from GitHub API.
func (s *Service) HandleTokenExchange(userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req tokenExchangeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "Invalid JSON body"},
			})

			return
		}

		if req.Provider == "" || req.IDToken == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "provider and id_token are required"},
			})

			return
		}

		switch req.Provider {
		case "google":
			s.handleGoogleTokenExchange(w, r, req.IDToken, userRepo)
		case "github":
			s.handleGithubTokenExchange(w, r, req.IDToken, userRepo)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{
					"type":    "validation_error",
					"message": "Unsupported provider. Use 'google' or 'github'",
				},
			})
		}
	}
}

func (s *Service) handleGoogleTokenExchange(
	w http.ResponseWriter, r *http.Request, idToken string, userRepo user.Repository,
) {
	info, err := verifyGoogleIDToken(r.Context(), idToken, s.googleClientID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]string{"type": "oauth_failed", "message": "Failed to verify Google ID token"},
		})

		return
	}

	u, err := userRepo.Upsert(r.Context(), &model.User{
		Provider:   "google",
		ProviderID: info.Sub,
		Email:      info.Email,
		Name:       info.Name,
		AvatarURL:  info.Picture,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]string{"type": "internal_error", "message": "Failed to create user"},
		})

		return
	}

	pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
		})

		return
	}

	writeJSON(w, http.StatusOK, pair)
}

func (s *Service) handleGithubTokenExchange(
	w http.ResponseWriter, r *http.Request, accessToken string, userRepo user.Repository,
) {
	// Create an HTTP client with the access token in the Authorization header.
	client := &http.Client{
		Transport: &bearerTransport{
			token: accessToken,
			base:  http.DefaultTransport,
		},
	}

	userInfo, err := fetchGithubUserInfo(r.Context(), client)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{
			"error": map[string]string{"type": "oauth_failed", "message": "Failed to fetch GitHub user info"},
		})

		return
	}

	// If email is empty, fetch from /user/emails endpoint.
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

	// Use Name, fall back to Login if Name is empty.
	name := userInfo.Name
	if name == "" {
		name = userInfo.Login
	}

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

	pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
		})

		return
	}

	writeJSON(w, http.StatusOK, pair)
}

// bearerTransport is an http.RoundTripper that adds an Authorization header.
type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.token)

	resp, err := t.base.RoundTrip(req2)
	if err != nil {
		return nil, fmt.Errorf("bearer transport: %w", err)
	}

	return resp, nil
}

// refreshRequest is the request body for POST /auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// HandleRefresh validates a refresh token and returns a new rotated token pair.
// It tries reading the refresh_token from a cookie first (web clients), then
// falls back to the JSON body (mobile clients). If the token came from a cookie,
// the response sets new auth cookies; otherwise a JSON token pair is returned.
func (s *Service) HandleRefresh(userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try cookie first (web clients).
		var refreshTokenStr string

		fromCookie := false

		if cookie, err := r.Cookie(CookieRefreshToken); err == nil && cookie.Value != "" {
			refreshTokenStr = cookie.Value
			fromCookie = true
		}

		// Fall back to JSON body (mobile clients).
		if refreshTokenStr == "" {
			var req refreshRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error": map[string]string{"type": "validation_error", "message": "Invalid JSON body"},
				})

				return
			}

			refreshTokenStr = req.RefreshToken
		}

		if refreshTokenStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "refresh_token is required"},
			})

			return
		}

		claims, err := s.VerifyToken(refreshTokenStr)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{
					"type":    "invalid_refresh_token",
					"message": "Invalid or expired refresh token",
				},
			})

			return
		}

		// Reject access tokens used as refresh tokens.
		// Refresh tokens are generated without Email and Tier claims.
		if claims.Email != "" || claims.Tier != "" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{
					"type":    "invalid_refresh_token",
					"message": "Invalid or expired refresh token",
				},
			})

			return
		}

		u, err := userRepo.FindByID(r.Context(), claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "invalid_refresh_token", "message": "User not found"},
			})

			return
		}

		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		// Cookie-based clients get cookies. Mobile clients get JSON.
		if fromCookie {
			if setErr := s.SetAuthCookies(w, pair); setErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]string{"type": "internal_error", "message": "Failed to set auth cookies"},
				})

				return
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		} else {
			writeJSON(w, http.StatusOK, pair)
		}
	}
}

func verifyGoogleIDToken(ctx context.Context, idToken, expectedAud string) (*googleTokenInfo, error) {
	reqURL := googleTokenInfoURL + "?id_token=" + idToken

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create tokeninfo request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch google tokeninfo: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google tokeninfo returned status %d", resp.StatusCode)
	}

	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode google tokeninfo: %w", err)
	}

	// Validate audience to prevent tokens issued for other applications.
	if expectedAud != "" && info.Aud != expectedAud {
		return nil, fmt.Errorf("audience mismatch: got %q, want %q", info.Aud, expectedAud)
	}

	return &info, nil
}
