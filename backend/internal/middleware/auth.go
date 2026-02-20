package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/bilusteknoloji/secretdrop/internal/auth"
)

type contextKey string

const userContextKey contextKey = "user"

// OptionalAuthenticate returns middleware that sets user context from a JWT
// Bearer token if present, but does NOT reject requests without tokens.
// Individual handlers decide whether authentication is required.
func OptionalAuthenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header { // no "Bearer " prefix found
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
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authenticate returns middleware that validates JWT Bearer tokens.
func Authenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeAuthError(w, "Authorization header required")
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			if token == header { // no "Bearer " prefix found
				writeAuthError(w, "Bearer token required")
				return
			}

			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				writeAuthError(w, "Invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, claims)
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
