package middleware

import (
	"fmt"
	"net/http"
	"strings"
)

// RequireJSON rejects non-JSON POST/PUT/PATCH requests with 415 Unsupported Media Type.
func RequireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnsupportedMediaType)
				fmt.Fprint(w,
					`{"error":{"type":"validation_error",`+
						`"message":"Content-Type must be application/json"}}`,
				)

				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
