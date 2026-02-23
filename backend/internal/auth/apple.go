package auth

import (
	"context"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const (
	appleAudience           = "https://appleid.apple.com"
	appleAuthURL            = "https://appleid.apple.com/auth/authorize"
	appleTokenURL           = "https://appleid.apple.com/auth/token" //nolint:gosec // URL, not a credential
	appleClientSecretExpiry = 180 * 24 * time.Hour                   // 6 months
	appleJWKSURL            = "https://appleid.apple.com/auth/keys"
)

// AppleIDTokenInfo holds the verified claims from an Apple ID token.
type AppleIDTokenInfo struct {
	Sub   string
	Email string
}

// appleJWKS represents Apple's JSON Web Key Set response.
type appleJWKS struct {
	Keys []appleJWK `json:"keys"`
}

// appleJWK represents a single key in Apple's JWKS response.
// Apple uses RSA keys (RS256) for id_token signing.
type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// appleUserName holds the first and last name from Apple's user JSON.
type appleUserName struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// appleUser represents the user JSON Apple sends on first consent.
type appleUser struct {
	Name  appleUserName `json:"name"`
	Email string        `json:"email"`
}

// AppleConfig creates an OAuth2 config for Apple.
// Note: ClientSecret is left empty — it's dynamically generated per request.
func AppleConfig(clientID, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:    clientID,
		RedirectURL: callbackURL,
		Scopes:      []string{"name", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  appleAuthURL,
			TokenURL: appleTokenURL,
		},
	}
}

// GenerateAppleClientSecret creates an ES256 JWT used as the client_secret
// for Apple's token endpoint. Apple requires this instead of a static secret.
func (s *Service) GenerateAppleClientSecret() (string, error) {
	// Decode base64-encoded PEM key
	pemBytes, err := base64.StdEncoding.DecodeString(s.applePrivateKey)
	if err != nil {
		return "", fmt.Errorf("decode apple private key: %w", err)
	}

	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errors.New("decode apple PEM block: no valid PEM data found")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse apple private key: %w", err)
	}

	now := time.Now()

	claims := jwt.RegisteredClaims{
		Issuer:    s.appleTeamID,
		Subject:   s.appleClientID,
		Audience:  jwt.ClaimStrings{appleAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(appleClientSecretExpiry)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = s.appleKeyID

	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign apple client secret: %w", err)
	}

	return signed, nil
}

// VerifyAppleIDToken verifies an Apple ID token JWT using Apple's JWKS endpoint.
// The jwksURL parameter allows overriding the JWKS URL for testing.
func VerifyAppleIDToken(ctx context.Context, idToken, expectedAud, jwksURL string) (*AppleIDTokenInfo, error) {
	if jwksURL == "" {
		jwksURL = appleJWKSURL
	}

	keys, err := fetchAppleJWKS(ctx, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetch apple JWKS: %w", err)
	}

	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid header")
		}

		pubKey, findErr := findApplePublicKey(keys, kid)
		if findErr != nil {
			return nil, findErr
		}

		return pubKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("verify apple id token: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("apple id token is not valid")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("unexpected claims type")
	}

	// Validate audience
	aud, err := claims.GetAudience()
	if err != nil || len(aud) == 0 || aud[0] != expectedAud {
		return nil, fmt.Errorf("audience mismatch: got %v, want %q", aud, expectedAud)
	}

	// Validate issuer
	iss, err := claims.GetIssuer()
	if err != nil || iss != appleAudience {
		return nil, fmt.Errorf("issuer mismatch: got %q, want %q", iss, appleAudience)
	}

	var sub string
	if v, ok := claims["sub"].(string); ok {
		sub = v
	}

	var email string
	if v, ok := claims["email"].(string); ok {
		email = v
	}

	return &AppleIDTokenInfo{
		Sub:   sub,
		Email: email,
	}, nil
}

func fetchAppleJWKS(ctx context.Context, jwksURL string) (*appleJWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS returned status %d", resp.StatusCode)
	}

	var jwks appleJWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	return &jwks, nil
}

func findApplePublicKey(jwks *appleJWKS, kid string) (*rsa.PublicKey, error) {
	for _, key := range jwks.Keys {
		if key.Kid == kid && key.Kty == "RSA" {
			nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
			if err != nil {
				return nil, fmt.Errorf("decode JWK n: %w", err)
			}

			eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
			if err != nil {
				return nil, fmt.Errorf("decode JWK e: %w", err)
			}

			// Convert e bytes to int (typically 65537)
			eInt := new(big.Int).SetBytes(eBytes).Int64()

			return &rsa.PublicKey{
				N: new(big.Int).SetBytes(nBytes),
				E: int(eInt),
			}, nil
		}
	}

	return nil, fmt.Errorf("no matching key found for kid %q", kid)
}

// HandleAppleLogin redirects the user to Apple's Sign In page.
//
//nolint:revive // receiver unused but method needed for API consistency
func (s *Service) HandleAppleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, err := generateState()
		if err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)

			return
		}

		// SameSite=None is required because Apple's form_post callback is a
		// cross-site POST from appleid.apple.com — Lax cookies are not sent
		// on cross-site POST requests.
		http.SetCookie(w, &http.Cookie{
			Name:     oauthStateCookieName,
			Value:    state,
			MaxAge:   oauthStateCookieMaxAge,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
			Path:     "/",
		})

		// Apple requires response_mode=form_post
		authURL := cfg.AuthCodeURL(state, oauth2.SetAuthURLParam("response_mode", "form_post"))
		http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
	}
}

// HandleAppleCallback handles the POST callback from Apple after user consent.
// Apple sends: code, state, user (JSON, first login only) as form POST.
func (s *Service) HandleAppleCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Parse form body
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "invalid_request", "message": "Invalid form data"},
			})

			return
		}

		// 2. Verify state
		stateCookie, err := r.Cookie(oauthStateCookieName)
		formState := r.FormValue("state")

		if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(formState)) != 1 {
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

		// 3. Generate client_secret JWT
		clientSecret, err := s.GenerateAppleClientSecret()
		if err != nil {
			slog.Error("apple client secret generation failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate client secret"},
			})

			return
		}

		// 4. Exchange code for tokens (with dynamic client_secret)
		cfgWithSecret := *cfg
		cfgWithSecret.ClientSecret = clientSecret

		code := r.FormValue("code")

		token, err := cfgWithSecret.Exchange(r.Context(), code)
		if err != nil {
			slog.Error("apple code exchange failed", "error", err)
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "oauth_failed", "message": "Failed to exchange authorization code"},
			})

			return
		}

		// 5. Verify id_token from token response
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok || rawIDToken == "" {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Missing id_token in response"},
			})

			return
		}

		idInfo, err := VerifyAppleIDToken(r.Context(), rawIDToken, s.appleClientID, "")
		if err != nil {
			slog.Error("apple id token verification failed", "error", err)
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "oauth_failed", "message": "Failed to verify Apple ID token"},
			})

			return
		}

		// 6. Extract name from user JSON (first login only)
		name := ""
		if userJSON := r.FormValue("user"); userJSON != "" {
			var appleUsr appleUser
			if jsonErr := json.Unmarshal([]byte(userJSON), &appleUsr); jsonErr == nil {
				parts := []string{}
				if appleUsr.Name.FirstName != "" {
					parts = append(parts, appleUsr.Name.FirstName)
				}
				if appleUsr.Name.LastName != "" {
					parts = append(parts, appleUsr.Name.LastName)
				}
				name = strings.Join(parts, " ")
			}
		}

		// 7. Upsert user
		u, err := userRepo.Upsert(r.Context(), &model.User{
			Provider:   "apple",
			ProviderID: idInfo.Sub,
			Email:      idInfo.Email,
			Name:       name,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to create user"},
			})

			return
		}

		// 8. Generate JWT pair and redirect
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
