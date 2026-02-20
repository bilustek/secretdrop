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

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":{"type":"unauthorized","message":"%s"}}`, msg)
}
