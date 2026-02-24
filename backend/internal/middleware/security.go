package middleware

import (
	"net/http"
	"strings"
)

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' https://*.googleusercontent.com https://*.gravatar.com https://avatars.githubusercontent.com; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self' https://appleid.apple.com; " +
	"base-uri 'self'"

const docsContentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
	"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
	"connect-src 'self'; " +
	"font-src 'self' https://cdn.jsdelivr.net; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'"

// SecurityHeaders returns middleware that sets security response headers.
func SecurityHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

			if strings.HasPrefix(r.URL.Path, "/docs") {
				w.Header().Set("Content-Security-Policy", docsContentSecurityPolicy)
			} else {
				w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
			}

			next.ServeHTTP(w, r)
		})
	}
}
