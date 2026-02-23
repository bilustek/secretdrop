# Apple Sign-In Design

## Overview

Add Apple Sign-In as a third OAuth provider alongside Google and GitHub.
Web-only flow using `oauth2.Config` with dynamically generated ES256 JWT
client secrets. No new dependencies required.

## Decisions

- **Platform:** Web only (no mobile/iOS token exchange)
- **Private relay emails:** Accept as-is; separate account if user hides email
- **Private key delivery:** Base64-encoded .p8 content via `APPLE_PRIVATE_KEY` env var
- **Approach:** `golang.org/x/oauth2` + manual client_secret JWT generation + Apple JWKS validation
- **No new dependencies:** `golang-jwt/jwt/v5` (existing) for ES256 signing, `crypto/ecdsa` (stdlib) for key parsing

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `APPLE_CLIENT_ID` | Yes (prod) | Apple Services ID (`com.bilustek.secretdrop.web`) |
| `APPLE_TEAM_ID` | Yes (prod) | Apple Developer Team ID (10 chars) |
| `APPLE_KEY_ID` | Yes (prod) | Key ID for the .p8 file |
| `APPLE_PRIVATE_KEY` | Yes (prod) | Base64-encoded .p8 private key content |
| `VITE_ENABLE_APPLE_SIGNIN` | No | Set to `"false"` to hide Apple button |

## Backend Architecture

### Config (`config/config.go`)

Four new unexported fields with getter methods. Required in production, empty-allowed in development.

### Auth Service (`auth/auth.go`)

New option: `WithAppleCredentials(clientID, teamID, keyID, privateKey string)` storing credentials on the Service struct.

### Apple OAuth Flow (`auth/apple.go`)

#### Login: `GET /auth/apple`

1. Generate state token (`generateState()` reuse)
2. Set `oauth_state` cookie (existing pattern)
3. Redirect to Apple authorize URL:
   ```
   https://appleid.apple.com/auth/authorize?
     client_id=com.bilustek.secretdrop.web
     redirect_uri=https://api.secretdrop.us/auth/apple/callback
     response_type=code
     scope=name email
     response_mode=form_post
     state=<state>
   ```

#### Callback: `POST /auth/apple/callback`

Apple sends a form POST (not GET) with fields: `code`, `state`, `user` (JSON, first login only).

1. Parse form body
2. Validate state (cookie vs form field, `subtle.ConstantTimeCompare`)
3. Generate client_secret JWT:
   - Algorithm: ES256
   - Header: `kid` = APPLE_KEY_ID
   - Claims: `iss` = APPLE_TEAM_ID, `aud` = `https://appleid.apple.com`, `sub` = APPLE_CLIENT_ID
   - Expiry: 6 months
   - Signed with .p8 private key (base64 decoded, PEM parsed)
4. Exchange authorization code for tokens via `oauth2.Config.Exchange()` with generated client_secret
5. Validate `id_token` JWT from token response:
   - Fetch Apple JWKS from `https://appleid.apple.com/auth/keys`
   - Verify signature with matching public key
   - Validate: `aud` == APPLE_CLIENT_ID, `iss` == `https://appleid.apple.com`
   - Extract: `sub` (Apple user ID), `email`
6. Extract name from `user` JSON if present (first login only)
7. `userRepo.Upsert()` with `Provider: "apple"`, `ProviderID: sub`
8. `GenerateTokenPair()` and redirect with tokens (existing `redirectWithTokens` pattern)

### Route Registration (`cmd/secretdrop/main.go`)

```go
appleCfg := auth.AppleConfig(
    cfg.AppleClientID(),
    cfg.APIBaseURL()+"/auth/apple/callback",
)
mux.HandleFunc("GET /auth/apple", authSvc.HandleAppleLogin(appleCfg))
mux.HandleFunc("POST /auth/apple/callback", authSvc.HandleAppleCallback(appleCfg, userRepo))
```

## Frontend

### Landing Page (`Landing.tsx`)

- New feature flag: `VITE_ENABLE_APPLE_SIGNIN`
- "Sign in with Apple" button following Apple HIG (black button, Apple logo)
- Same `<a href>` pattern as Google/GitHub buttons

### No Changes Needed

- `AuthCallback.tsx` — already generic (reads token params)
- `AuthContext.tsx` — provider-agnostic
- `api/client.ts` — no changes

## Key Apple-Specific Differences from Google/GitHub

1. **Callback is POST** not GET (form_post response mode)
2. **client_secret is a JWT** regenerated per request, not a static string
3. **User info (name) only sent on first consent** — must capture on first login
4. **Email comes from id_token** not a separate userinfo endpoint
5. **ID token validated via JWKS** not a simple tokeninfo HTTP call

## Documentation Updates

- `CLAUDE.md`: Add new env vars and endpoints
- `docs/openapi.yaml`: Add `GET /auth/apple` and `POST /auth/apple/callback`
