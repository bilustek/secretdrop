package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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
// The cookieDomain parameter sets the Domain attribute on cookies so they are accessible
// across subdomains (e.g. ".secretdrop.us" for api.secretdrop.us + secretdrop.us).
func SetTokenCookies(
	w http.ResponseWriter,
	pair *TokenPair,
	csrfToken string,
	secure bool,
	cookieDomain string,
	accessExpiry, refreshExpiry time.Duration,
) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    pair.AccessToken,
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   int(accessExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    pair.RefreshToken,
		Path:     "/auth/refresh",
		Domain:   cookieDomain,
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieCSRFToken,
		Value:    csrfToken,
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   int(refreshExpiry.Seconds()),
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// CookieDomain extracts the root domain from a URL for cross-subdomain cookie sharing.
// For "https://secretdrop.us" or "https://api.secretdrop.us" it returns "secretdrop.us".
// For localhost URLs it returns "" (no domain attribute needed).
func CookieDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return ""
	}

	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return ""
	}

	return strings.Join(parts[len(parts)-2:], ".")
}

// ClearTokenCookies removes the access, refresh, and CSRF cookies by setting MaxAge to -1.
func ClearTokenCookies(w http.ResponseWriter, cookieDomain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    "",
		Path:     "/",
		Domain:   cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    "",
		Path:     "/auth/refresh",
		Domain:   cookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:   CookieCSRFToken,
		Value:  "",
		Path:   "/",
		Domain: cookieDomain,
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
