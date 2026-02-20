package middleware

import (
	"net/http"

	"github.com/google/uuid"
)

const requestIDHeader = "X-Request-ID"

// RequestID adds a unique request ID to each request via the X-Request-ID header.
// If the incoming request already has the header, it is preserved.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}

		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}
