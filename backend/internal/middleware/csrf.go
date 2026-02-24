package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"strings"
)

// CSRF returns middleware that validates the Double Submit Cookie pattern.
// Safe methods (GET, HEAD, OPTIONS) are exempt. Paths in exemptPrefixes are
// also exempt — any request path starting with a listed prefix is skipped
// (e.g. "/billing/webhook", "/api/v1/secrets/" for unauthenticated reveal).
// Requests with an Authorization header (Bearer token) skip CSRF validation
// because they are not cookie-authenticated and thus not vulnerable to CSRF.
func CSRF(exemptPrefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Bearer token and Basic auth requests are not cookie-based,
			// so they are not vulnerable to CSRF attacks.
			if r.Header.Get("Authorization") != "" {
				next.ServeHTTP(w, r)
				return
			}

			for _, prefix := range exemptPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				writeCSRFError(w)
				return
			}

			header := r.Header.Get("X-CSRF-Token")
			if header == "" {
				writeCSRFError(w)
				return
			}

			if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
				writeCSRFError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, `{"error":{"type":"csrf_error","message":"CSRF validation failed"}}`)
}
