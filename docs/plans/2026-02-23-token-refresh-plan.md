# Token Refresh Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable automatic token refresh so users stay logged in for 30 days without re-authenticating.

**Architecture:** Add `POST /auth/refresh` backend endpoint that validates a refresh token, looks up the user, and returns a rotated token pair. Frontend API client intercepts 401 responses, attempts refresh, and retries the original request transparently.

**Tech Stack:** Go (backend), React/TypeScript (frontend), JWT (github.com/golang-jwt/jwt/v5)

---

### Task 1: Backend — `HandleRefresh` handler + tests

**Files:**
- Modify: `backend/internal/auth/token.go` (add `HandleRefresh` method)
- Modify: `backend/internal/auth/token_test.go` (add refresh tests)

**Step 1: Write the failing tests**

Add to `backend/internal/auth/token_test.go`:

```go
func TestHandleRefresh_Success(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Create a user first.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "refresh-test-sub",
		Email:      "refresh@example.com",
		Name:       "Refresh User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Generate initial token pair.
	pair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var newPair auth.TokenPair
	if err := json.NewDecoder(rec.Body).Decode(&newPair); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if newPair.AccessToken == "" {
		t.Error("access_token is empty")
	}

	if newPair.RefreshToken == "" {
		t.Error("refresh_token is empty")
	}

	// New tokens should differ from old ones (rotation).
	if newPair.AccessToken == pair.AccessToken {
		t.Error("new access_token should differ from old")
	}

	if newPair.RefreshToken == pair.RefreshToken {
		t.Error("new refresh_token should differ from old")
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := `{"refresh_token":"invalid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestHandleRefresh_MissingBody(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandleRefresh_DeletedUser(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	// Create and then delete user.
	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "deleted-user-sub",
		Email:      "deleted@example.com",
		Name:       "Deleted User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	pair, err := svc.GenerateTokenPair(u.ID, u.Email, u.Tier)
	if err != nil {
		t.Fatalf("GenerateTokenPair() error = %v", err)
	}

	if err := userRepo.DeleteUser(context.Background(), u.ID); err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	body := fmt.Sprintf(`{"refresh_token":%q}`, pair.RefreshToken)
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestHandleRefresh_InvalidJSON(t *testing.T) {
	t.Parallel()

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	svc, err := auth.New("test-secret")
	if err != nil {
		t.Fatalf("auth.New() error = %v", err)
	}

	handler := svc.HandleRefresh(userRepo)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test -run "TestHandleRefresh" ./internal/auth/ -v`
Expected: Compilation error — `HandleRefresh` method does not exist.

**Step 3: Implement `HandleRefresh`**

Add to `backend/internal/auth/token.go`:

```go
// refreshRequest is the request body for POST /auth/refresh.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// HandleRefresh validates a refresh token and returns a new rotated token pair.
func (s *Service) HandleRefresh(userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "Invalid JSON body"},
			})

			return
		}

		if req.RefreshToken == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": map[string]string{"type": "validation_error", "message": "refresh_token is required"},
			})

			return
		}

		claims, err := s.VerifyToken(req.RefreshToken)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "invalid_refresh_token", "message": "Invalid or expired refresh token"},
			})

			return
		}

		u, err := userRepo.FindByID(r.Context(), claims.UserID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{
				"error": map[string]string{"type": "invalid_refresh_token", "message": "User not found"},
			})

			return
		}

		pair, err := s.GenerateTokenPair(u.ID, u.Email, u.Tier)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": map[string]string{"type": "internal_error", "message": "Failed to generate token"},
			})

			return
		}

		writeJSON(w, http.StatusOK, pair)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test -run "TestHandleRefresh" ./internal/auth/ -v`
Expected: All 5 tests PASS.

**Step 5: Lint**

Run: `cd backend && golangci-lint run ./internal/auth/...`
Expected: No issues.

**Step 6: Commit**

```bash
cd backend && git add internal/auth/token.go internal/auth/token_test.go
git commit -m "feat: add POST /auth/refresh endpoint for token rotation"
```

---

### Task 2: Backend — Register route in main.go + OpenAPI spec

**Files:**
- Modify: `backend/cmd/secretdrop/main.go:191` (add route after `POST /auth/token`)
- Modify: `backend/docs/openapi.yaml` (add `/auth/refresh` path)

**Step 1: Register route**

In `backend/cmd/secretdrop/main.go`, add after line 191 (`mux.HandleFunc("POST /auth/token", ...)`):

```go
mux.HandleFunc("POST /auth/refresh", authSvc.HandleRefresh(userRepo))
```

**Step 2: Update OpenAPI spec**

Add after the `/auth/token` block in `backend/docs/openapi.yaml`:

```yaml
  /auth/refresh:
    post:
      operationId: authRefresh
      summary: Refresh access token
      description: |
        Accepts a refresh token and returns a new rotated token pair (access + refresh).
        Use this when the access token expires (401) to obtain new tokens without
        re-authenticating via OAuth.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [refresh_token]
              properties:
                refresh_token:
                  type: string
                  description: The refresh token from a previous login or refresh
            example:
              refresh_token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
      responses:
        "200":
          description: New rotated token pair
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/TokenPair"
        "400":
          description: Invalid request (missing or malformed body)
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"
        "401":
          description: Invalid or expired refresh token
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/ErrorResponse"
```

**Step 3: Build and verify**

Run: `cd backend && go build ./...`
Expected: Compiles successfully.

**Step 4: Commit**

```bash
git add backend/cmd/secretdrop/main.go backend/docs/openapi.yaml
git commit -m "feat: register /auth/refresh route and update OpenAPI spec"
```

---

### Task 3: Frontend — Automatic token refresh in API client

**Files:**
- Modify: `frontend/src/api/client.ts` (add refresh logic + refactor direct fetch calls)

**Step 1: Implement refresh logic**

Replace `frontend/src/api/client.ts` with the updated version that:
- Adds `refreshTokens()` function calling `POST /auth/refresh`
- Adds promise-based mutex to prevent concurrent refresh attempts
- Modifies `request()` to try refresh on 401 before logging out
- Refactors `checkout()`, `portal()`, `deleteAccount()` to use `request()` or at minimum share the refresh logic

```typescript
import { API_URL } from "./config"

const API_BASE = `${API_URL}/api/v1`

interface ApiError {
  error: {
    type: string
    message: string
  }
}

export class AppError extends Error {
  type: string
  status: number

  constructor(type: string, message: string, status: number) {
    super(message)
    this.name = "AppError"
    this.type = type
    this.status = status
  }
}

// Mutex to prevent concurrent refresh attempts.
let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token")
  if (!refreshToken) return false

  try {
    const res = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })

    if (!res.ok) return false

    const pair = (await res.json()) as { access_token: string; refresh_token: string }
    localStorage.setItem("access_token", pair.access_token)
    localStorage.setItem("refresh_token", pair.refresh_token)

    return true
  } catch {
    return false
  }
}

async function tryRefresh(): Promise<boolean> {
  if (refreshPromise) return refreshPromise

  refreshPromise = refreshTokens().finally(() => {
    refreshPromise = null
  })

  return refreshPromise
}

function forceLogout(): never {
  localStorage.removeItem("access_token")
  localStorage.removeItem("refresh_token")
  window.location.href = "/"
  throw new AppError("unauthorized", "Session expired", 401)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem("access_token")

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      // Retry with new access token.
      const newToken = localStorage.getItem("access_token")
      headers["Authorization"] = `Bearer ${newToken}`

      const retry = await fetch(`${API_BASE}${path}`, { ...options, headers })
      if (!retry.ok) {
        if (retry.status === 401) forceLogout()

        const body: ApiError = await retry.json()
        throw new AppError(body.error.type, body.error.message, retry.status)
      }

      return retry.json() as Promise<T>
    }

    forceLogout()
  }

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

// authenticatedFetch wraps fetch with token injection and refresh logic.
// Used by endpoints that bypass the request() helper (billing, delete).
async function authenticatedFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const token = localStorage.getItem("access_token")

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(url, { ...options, headers })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      const newToken = localStorage.getItem("access_token")
      headers["Authorization"] = `Bearer ${newToken}`

      return fetch(url, { ...options, headers })
    }

    forceLogout()
  }

  return res
}

// ... (existing type exports remain unchanged)

export const api = {
  me: () => request<MeResponse>("/me"),

  createSecret: (data: CreateSecretRequest) =>
    request<CreateSecretResponse>("/secrets", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  revealSecret: (token: string, data: RevealRequest) =>
    request<RevealResponse>(`/secrets/${token}/reveal`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  checkout: () =>
    authenticatedFetch(`${API_URL}/billing/checkout`, {
      method: "POST",
    }).then((r) => r.json() as Promise<CheckoutResponse>),

  portal: () =>
    authenticatedFetch(`${API_URL}/billing/portal`, {
      method: "POST",
    }).then((r) => r.json() as Promise<{ url: string }>),

  deleteAccount: () =>
    authenticatedFetch(`${API_BASE}/me`, {
      method: "DELETE",
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to delete account")
    }),
}
```

**Step 2: Build to verify**

Run: `cd frontend && npm run build`
Expected: Compiles successfully.

**Step 3: Lint**

Run: `cd frontend && npx eslint .`
Expected: No errors.

**Step 4: Commit**

```bash
git add frontend/src/api/client.ts
git commit -m "feat: add automatic token refresh with retry on 401"
```

---

### Task 4: Update CLAUDE.md and project docs

**Files:**
- Modify: `CLAUDE.md` (add `/auth/refresh` to API endpoints list)

**Step 1: Add endpoint to CLAUDE.md**

Add after the `POST /auth/token` line:

```markdown
- `POST /auth/refresh` — Refresh access token (returns rotated pair)
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add /auth/refresh endpoint to CLAUDE.md"
```

---

### Task 5: Full test suite verification

**Step 1: Run all backend tests**

Run: `cd backend && go test -race ./...`
Expected: All tests PASS.

**Step 2: Run backend lint**

Run: `cd backend && golangci-lint run ./...`
Expected: No issues.

**Step 3: Run frontend build + lint**

Run: `cd frontend && npm run build && npx eslint .`
Expected: Builds and lints cleanly.
