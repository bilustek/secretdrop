package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getsentry/sentry-go"

	"github.com/bilustek/secretdrop/internal/auth"
)

type contextKey string

const userContextKey contextKey = "user"

// extractToken extracts the JWT token from the request.
// Priority: cookie > Authorization Bearer header.
func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie("access_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	token := strings.TrimPrefix(header, "Bearer ")
	if token == header {
		return ""
	}

	return token
}

// OptionalAuthenticate returns middleware that sets user context from a JWT
// token (cookie or Bearer header) if present, but does NOT reject requests
// without tokens. Individual handlers decide whether authentication is required.
func OptionalAuthenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				// Invalid token — pass through without user context
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)

			if hub := sentry.GetHubFromContext(ctx); hub != nil {
				hub.Scope().SetUser(sentry.User{ID: strconv.FormatInt(claims.UserID, 10)})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authenticate returns middleware that validates JWT tokens from cookie or Bearer header.
func Authenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				writeAuthError(w, "Authentication required")
				return
			}

			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				writeAuthError(w, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)

			if hub := sentry.GetHubFromContext(ctx); hub != nil {
				hub.Scope().SetUser(sentry.User{ID: strconv.FormatInt(claims.UserID, 10)})
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves auth claims from the request context.
func UserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userContextKey).(*auth.Claims)
	return claims, ok
}

// ContextWithUser returns a new context with the given auth claims.
// This is intended for use in tests and internal services that need to
// set user identity without going through the HTTP middleware.
func ContextWithUser(ctx context.Context, claims *auth.Claims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":{"type":"unauthorized","message":"%s"}}`, msg)
}
