package middleware

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultRate   = 10 // requests per window
	defaultWindow = 1 * time.Minute
)

type visitor struct {
	count   int
	resetAt time.Time
}

// RateLimiter implements IP-based in-memory rate limiting.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     int
	window   time.Duration
}

// RateLimitOption configures a RateLimiter value.
type RateLimitOption func(*RateLimiter) error

// WithRate sets the maximum number of requests per window.
func WithRate(n int) RateLimitOption {
	return func(rl *RateLimiter) error {
		if n <= 0 {
			return errors.New("rate must be positive")
		}

		rl.rate = n

		return nil
	}
}

// WithWindow sets the rate limiting window duration.
func WithWindow(d time.Duration) RateLimitOption {
	return func(rl *RateLimiter) error {
		if d <= 0 {
			return errors.New("window must be positive")
		}

		rl.window = d

		return nil
	}
}

// NewRateLimiter creates a new RateLimiter with the given options.
func NewRateLimiter(opts ...RateLimitOption) (*RateLimiter, error) {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     defaultRate,
		window:   defaultWindow,
	}

	for _, opt := range opts {
		if err := opt(rl); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	return rl, nil
}

// Limit is an HTTP middleware that rate-limits requests by client IP.
func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		if !rl.allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"type":"rate_limited","message":"Too many requests"}}`)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	v, exists := rl.visitors[ip]
	if !exists || now.After(v.resetAt) {
		rl.visitors[ip] = &visitor{count: 1, resetAt: now.Add(rl.window)}

		return true
	}

	v.count++

	return v.count <= rl.rate
}

func extractIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
