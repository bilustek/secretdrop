# Token Refresh Design

## Problem

Access tokens expire after 15 minutes. When the frontend receives a 401 response,
it clears both tokens from localStorage and redirects to the home page, forcing the
user to re-authenticate via Google/GitHub OAuth. The refresh token is generated and
stored but never used.

## Solution

### Backend: `POST /auth/refresh`

New endpoint in the auth package that accepts a refresh token and returns a new
token pair (rotation).

**Request:**
```json
{ "refresh_token": "eyJhbGci..." }
```

**Success response (200):**
```json
{ "access_token": "eyJhbGci...", "refresh_token": "eyJhbGci..." }
```

**Error response (401):**
```json
{ "error": { "type": "invalid_refresh_token", "message": "..." } }
```

**Flow:**
1. Decode and verify the refresh token JWT
2. Fetch user from DB by `claims.UserID` (ensures user still exists, gets current tier)
3. Generate new token pair with current user data
4. Return new pair

### Frontend: Automatic retry in API client

Modify `request()` in `client.ts` to intercept 401 responses:

1. On 401, check if `refresh_token` exists in localStorage
2. If yes, call `POST /auth/refresh` with the refresh token
3. On success, store new token pair and retry the original request
4. On failure (refresh also 401), clear tokens and redirect to home
5. Use a promise-based mutex to prevent concurrent refresh calls when
   multiple requests fail simultaneously

### Scope

- Token durations unchanged: access 15min, refresh 30 days
- No refresh token blacklist/revocation (JWT remains stateless)
- Rotation: every refresh returns a new pair, old refresh token naturally expires
- `checkout()`, `portal()`, `deleteAccount()` in `client.ts` bypass the `request()`
  helper — these also need 401 handling or should be refactored to use `request()`
