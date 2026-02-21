package middleware

import "net/http"

// CORS returns middleware that sets CORS headers when the request Origin
// matches allowedOrigin.  Preflight (OPTIONS) requests receive a 204
// response immediately; all other matching requests get the headers added
// and are forwarded to next.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" || origin != allowedOrigin {
				next.ServeHTTP(w, r)

				return
			}

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
