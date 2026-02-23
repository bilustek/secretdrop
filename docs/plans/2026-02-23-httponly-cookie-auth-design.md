# Design: httpOnly Cookie Auth + CSP Security Hardening

## Problem

JWT tokens (access_token, refresh_token) are stored in localStorage, making them
vulnerable to exfiltration via XSS attacks. An attacker who steals the refresh
token gains persistent access even after the XSS vulnerability is patched.
Additionally, no Content-Security-Policy or security headers are set.

## Solution

Migrate token storage from localStorage to httpOnly cookies and add CSP + security
headers for defense-in-depth.

## Cookie Structure

| Cookie          | HttpOnly | Secure | SameSite | Path            | MaxAge  |
|-----------------|----------|--------|----------|-----------------|---------|
| `access_token`  | Yes      | Yes    | Lax      | `/`             | 15m     |
| `refresh_token` | Yes      | Yes    | Lax      | `/auth/refresh` | 30d     |
| `csrf_token`    | No       | Yes    | Lax      | `/`             | 30d     |

- `refresh_token` scoped to `/auth/refresh` — only sent to the refresh endpoint.
- `csrf_token` readable by JS (not httpOnly) for Double Submit Cookie pattern.
- `SameSite=Lax` allows OAuth redirect flows while blocking cross-site POSTs.

## CSRF Protection

Double Submit Cookie pattern:
1. Backend sets `csrf_token` cookie (non-httpOnly) on login/refresh.
2. Frontend reads cookie, sends value as `X-CSRF-Token` header on mutating requests.
3. Backend middleware compares cookie value with header (constant-time comparison).

Exempt endpoints:
- `POST /billing/webhook` — Stripe signature verification
- `POST /auth/apple/callback` — Apple form_post with state cookie verification
- `POST /auth/token` — Mobile token exchange (no cookies involved)
- `POST /auth/refresh` — Uses httpOnly refresh_token cookie as proof of possession
- Safe methods (GET, HEAD, OPTIONS)

## Backend Changes

### New files
- `internal/auth/cookie.go` — Cookie helper functions:
  - `SetTokenCookies(w, pair, csrfToken, secure bool)` — sets 3 cookies
  - `ClearTokenCookies(w)` — clears cookies on logout (MaxAge=-1)
  - `GenerateCSRFToken()` — crypto/rand 32-byte base64url token
- `internal/middleware/csrf.go` — CSRF middleware
- `internal/middleware/security.go` — Security headers middleware

### Modified files
- `internal/auth/google.go` — `redirectWithTokens()` sets cookies instead of URL params
- `internal/auth/github.go` — Same change via shared `redirectWithTokens()`
- `internal/auth/apple.go` — Same change via shared `redirectWithTokens()`
- `internal/auth/token.go` — `HandleRefresh()` reads refresh_token from cookie (web)
  or request body (mobile fallback); response sets new cookies (web) or returns JSON (mobile)
- `internal/auth/auth.go` — Add `WithSecureCookies(bool)` option
- `internal/middleware/auth.go` — Read token from cookie first, then Authorization header
- `internal/middleware/cors.go` — Add `Access-Control-Allow-Credentials: true`,
  add `X-CSRF-Token` to allowed headers
- `cmd/secretdrop/main.go` — Wire new middleware, add `POST /auth/logout` route

### New endpoint
- `POST /auth/logout` — Clears all auth cookies

### Mobile compatibility
- `POST /auth/token` remains unchanged — returns JSON, no cookies
- Middleware checks cookie first, falls back to Bearer header

## Security Headers

New middleware sets on every response:

```
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Referrer-Policy: strict-origin-when-cross-origin
```

## Content-Security-Policy

```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' https://*.googleusercontent.com https://*.gravatar.com https://avatars.githubusercontent.com;
connect-src 'self';
font-src 'self';
frame-ancestors 'none';
form-action 'self' https://appleid.apple.com;
base-uri 'self'
```

- `script-src 'self'` — blocks inline script injection (primary XSS defense)
- `connect-src 'self'` — blocks data exfiltration via fetch/XHR to external origins
- `frame-ancestors 'none'` — prevents clickjacking
- `style-src 'unsafe-inline'` — required for Tailwind runtime styles
- `img-src` whitelist — Google, GitHub, Gravatar avatar URLs

## Frontend Changes

### Modified files
- `api/client.ts` — Remove all localStorage token management; add `credentials: "include"`
  to every fetch; add `X-CSRF-Token` header to mutating requests; read csrf_token from
  `document.cookie`; simplify refresh logic (just call `/auth/refresh` with credentials)
- `context/AuthContext.tsx` — Remove `login(accessToken, refreshToken)`; auth state
  determined by `GET /api/v1/me` result; `logout()` calls `POST /auth/logout`
- `pages/AuthCallback.tsx` — Remove URL param token reading; just call `/api/v1/me`
  then redirect to `/dashboard`

## Defense Layers Summary

| Layer                     | Protects against                              |
|---------------------------|-----------------------------------------------|
| httpOnly cookie           | XSS token theft                               |
| CSRF Double Submit        | Cross-site request forgery                     |
| CSP `script-src 'self'`   | Inline script injection                       |
| CSP `connect-src 'self'`  | Data exfiltration to external origins          |
| X-Frame-Options DENY      | Clickjacking                                  |
| X-Content-Type-Options     | MIME sniffing attacks                         |
| Referrer-Policy            | Referrer header information leak              |
