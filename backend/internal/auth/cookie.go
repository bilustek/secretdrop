package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

const (
	// CookieAccessToken is the cookie name for the JWT access token.
	CookieAccessToken = "access_token"

	// CookieRefreshToken is the cookie name for the JWT refresh token.
	CookieRefreshToken = "refresh_token"

	// CookieCSRFToken is the cookie name for the CSRF protection token.
	CookieCSRFToken = "csrf_token"

	csrfTokenBytes = 32
)

// SetTokenCookies writes access, refresh, and CSRF cookies to the response.
// The access and refresh cookies are HttpOnly; the CSRF cookie is readable by JavaScript.
func SetTokenCookies(
	w http.ResponseWriter,
	pair *TokenPair,
	csrfToken string,
	secure bool,
	accessExpiry, refreshExpiry time.Duration,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    pair.AccessToken,
		Path:     "/",
		MaxAge:   int(accessExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    pair.RefreshToken,
		Path:     "/auth/refresh",
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieCSRFToken,
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearTokenCookies removes the access, refresh, and CSRF cookies by setting MaxAge to -1.
func ClearTokenCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    "",
		Path:     "/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:   CookieCSRFToken,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// GenerateCSRFToken returns a cryptographically random base64 RawURL-encoded token.
func GenerateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenBytes)

	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate csrf token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}
