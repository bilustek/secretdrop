# User Timezone Support — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Store each user's IANA timezone, auto-detect from browser on login, and format email expiry times in the user's local timezone alongside UTC.

**Architecture:** Add `timezone` column to users table, new `UpdateTimezone` method on user repository, new `PUT /api/v1/me/timezone` handler, update email formatting to show dual timezone, and update frontend auth callback to auto-sync browser timezone.

**Tech Stack:** Go stdlib `time.LoadLocation`, `Intl.DateTimeFormat` browser API, SQLite `ALTER TABLE`.

---

### Task 1: Add Timezone Field to User Model

**Files:**
- Modify: `backend/internal/model/user.go:18-41`

**Step 1: Add Timezone field to User struct**

Add `Timezone` field after `UpdatedAt` (line 28):

```go
type User struct {
	ID          int64
	Provider    string
	ProviderID  string
	Email       string
	Name        string
	AvatarURL   string
	Tier        string
	SecretsUsed int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Timezone    string

	// SecretsLimitOverride is an optional per-user override for secrets limit.
	// When nil, the tier default from the limits table is used.
	SecretsLimitOverride *int

	// TierSecretsLimit is the secrets limit from the limits table for this user's tier.
	// Set at query time; zero means not loaded.
	TierSecretsLimit int

	// TierRecipientsLimit is the recipients limit from the limits table for this user's tier.
	// Set at query time; zero means not loaded.
	TierRecipientsLimit int
}
```

**Step 2: Run tests to verify nothing breaks**

Run: `cd backend && go test ./internal/model/...`
Expected: PASS (no model tests depend on struct field count)

**Step 3: Commit**

```bash
git add backend/internal/model/user.go
git commit -m "feat: add Timezone field to User model"
```

---

### Task 2: Add UpdateTimezone to User Repository Interface

**Files:**
- Modify: `backend/internal/user/user.go:11-27`

**Step 1: Add UpdateTimezone method to Repository interface**

Add after line 17 (`UpdateTier`):

```go
type Repository interface {
	Upsert(ctx context.Context, u *model.User) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	IncrementSecretsUsed(ctx context.Context, id int64) error
	ResetSecretsUsed(ctx context.Context, id int64) error
	UpdateTier(ctx context.Context, id int64, tier string) error
	UpdateTimezone(ctx context.Context, id int64, timezone string) error
	DeleteUser(ctx context.Context, id int64) error

	UpsertSubscription(ctx context.Context, sub *model.Subscription) error
	FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error)
	FindUserByStripeCustomerID(ctx context.Context, customerID string) (*model.User, error)
	UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error
	UpdateSubscriptionPeriod(ctx context.Context, stripeSubID string, start, end time.Time) error

	GetLimits(ctx context.Context, tier string) (*TierLimits, error)
}
```

**Step 2: Verify build fails (compile-time check)**

Run: `cd backend && go build ./...`
Expected: FAIL — `sqlite.Repository` does not implement `user.Repository` (missing `UpdateTimezone`)

**Step 3: Commit**

```bash
git add backend/internal/user/user.go
git commit -m "feat: add UpdateTimezone to user.Repository interface"
```

---

### Task 3: Implement SQLite Migration and UpdateTimezone

**Files:**
- Modify: `backend/internal/user/sqlite/sqlite.go:71-89` (New function)
- Modify: `backend/internal/user/sqlite/sqlite.go:94-134` (Upsert — add timezone to RETURNING + Scan)
- Modify: `backend/internal/user/sqlite/sqlite.go:137-169` (FindByID — add timezone to SELECT + Scan)
- Modify: `backend/internal/user/sqlite/sqlite.go:173-205` (FindByProvider — add timezone to SELECT + Scan)
- Modify: `backend/internal/user/sqlite/sqlite.go:370-405` (FindUserByStripeCustomerID — add timezone to SELECT + Scan)
- Modify: `backend/internal/user/sqlite/sqlite.go:469-536` (ListUsers — add timezone to SELECT + Scan)
- Test: `backend/internal/user/sqlite/sqlite_test.go`

**Step 1: Write failing tests for UpdateTimezone**

Add to `backend/internal/user/sqlite/sqlite_test.go`:

```go
func TestUpdateTimezone(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, err := repo.Upsert(ctx, &model.User{
		Provider:   "google",
		ProviderID: "g-tz",
		Email:      "tz@example.com",
		Name:       "TZ User",
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	// Default should be UTC.
	if u.Timezone != "UTC" {
		t.Errorf("Timezone = %q; want %q", u.Timezone, "UTC")
	}

	if err := repo.UpdateTimezone(ctx, u.ID, "Europe/Istanbul"); err != nil {
		t.Fatalf("UpdateTimezone() error = %v", err)
	}

	found, err := repo.FindByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.Timezone != "Europe/Istanbul" {
		t.Errorf("Timezone = %q; want %q", found.Timezone, "Europe/Istanbul")
	}
}

func TestUpdateTimezone_NotFound(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	err := repo.UpdateTimezone(ctx, 99999, "Europe/Istanbul")
	if !errors.Is(err, model.ErrNotFound) {
		t.Errorf("UpdateTimezone() error = %v; want model.ErrNotFound", err)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/user/sqlite/... -run TestUpdateTimezone -v`
Expected: FAIL — compile error (UpdateTimezone not defined)

**Step 3: Add migration and implement UpdateTimezone**

In `backend/internal/user/sqlite/sqlite.go`, add after line 86 (the existing `secrets_limit` ALTER TABLE):

```go
_, _ = db.Exec("ALTER TABLE users ADD COLUMN timezone TEXT NOT NULL DEFAULT 'UTC'")
```

Update all SQL queries that SELECT from users to include `timezone` column and all Scan calls to include `&u.Timezone`. The affected queries are:

**Upsert** (line 95-128): Add `timezone` to RETURNING clause and Scan:
```go
const query = `
    INSERT INTO users (provider, provider_id, email, name, avatar_url)
    VALUES (?, ?, ?, ?, ?)
    ON CONFLICT(email) DO UPDATE SET
        provider    = excluded.provider,
        provider_id = excluded.provider_id,
        name        = CASE WHEN excluded.name = '' THEN users.name ELSE excluded.name END,
        avatar_url  = CASE WHEN excluded.avatar_url = '' THEN users.avatar_url ELSE excluded.avatar_url END,
        updated_at  = CURRENT_TIMESTAMP
    RETURNING id, provider, provider_id, email, name, avatar_url,
        tier, secrets_used, created_at, updated_at, secrets_limit, timezone
`
// ... Scan:
).Scan(
    &result.ID,
    &result.Provider,
    &result.ProviderID,
    &result.Email,
    &result.Name,
    &result.AvatarURL,
    &result.Tier,
    &result.SecretsUsed,
    &result.CreatedAt,
    &result.UpdatedAt,
    &result.SecretsLimitOverride,
    &result.Timezone,
)
```

**FindByID** (line 138-158): Add `timezone` to SELECT and Scan:
```go
const query = `
    SELECT id, provider, provider_id, email, name, avatar_url,
        tier, secrets_used, created_at, updated_at, secrets_limit, timezone
    FROM users
    WHERE id = ?
`
// ... Scan adds: &u.Timezone at end
```

**FindByProvider** (line 174-194): Same pattern — add `timezone` to SELECT and Scan.

**FindUserByStripeCustomerID** (line 371-394): Same pattern — add `u.timezone` to SELECT and `&u.Timezone` to Scan.

**ListUsers** (line 473-523): Add `timezone` to SELECT column list (line 473) and `&u.Timezone` to Scan (line 521-523).

**Add UpdateTimezone method** after `UpdateTier` (line 268):

```go
// UpdateTimezone updates the IANA timezone for the given user.
func (r *Repository) UpdateTimezone(ctx context.Context, id int64, timezone string) error {
	const query = `UPDATE users SET timezone = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, timezone, id)
	if err != nil {
		return fmt.Errorf("update timezone: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/user/sqlite/... -v`
Expected: ALL PASS (including new UpdateTimezone tests and existing tests)

**Step 5: Run full backend test suite**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add backend/internal/user/sqlite/sqlite.go backend/internal/user/sqlite/sqlite_test.go
git commit -m "feat: add timezone column migration and UpdateTimezone implementation"
```

---

### Task 4: Add Timezone to MeResponse and PUT /api/v1/me/timezone Handler

**Files:**
- Modify: `backend/internal/model/response.go:24-34` (add Timezone to MeResponse)
- Modify: `backend/internal/model/request.go` (add TimezoneRequest)
- Modify: `backend/internal/handler/secret.go:26-31` (Register new route)
- Modify: `backend/internal/handler/secret.go:60-91` (Me handler — add Timezone)
- Add new handler method: `UpdateTimezone` on SecretHandler
- Test: `backend/internal/handler/secret_test.go`

**Step 1: Write failing tests for PUT /api/v1/me/timezone**

Add to `backend/internal/handler/secret_test.go`:

```go
func TestUpdateTimezone_Valid(t *testing.T) {
	t.Parallel()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}
	t.Cleanup(func() { userRepo.Close() })

	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "tz-valid",
		Email:      "tz@example.com",
		Name:       "TZ User",
	})
	if err != nil {
		t.Fatalf("upsert user error = %v", err)
	}

	h := handler.NewSecretHandler(svc, userRepo)
	mux := http.NewServeMux()
	h.Register(mux)

	claims := &auth.Claims{UserID: u.ID, Email: u.Email, Tier: u.Tier}
	body := `{"timezone":"Europe/Istanbul"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/timezone", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, claims)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d; want %d, body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	// Verify timezone was saved
	found, err := userRepo.FindByID(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found.Timezone != "Europe/Istanbul" {
		t.Errorf("Timezone = %q; want %q", found.Timezone, "Europe/Istanbul")
	}
}

func TestUpdateTimezone_Invalid(t *testing.T) {
	t.Parallel()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}
	t.Cleanup(func() { userRepo.Close() })

	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "tz-invalid",
		Email:      "tz-invalid@example.com",
		Name:       "TZ Invalid",
	})
	if err != nil {
		t.Fatalf("upsert user error = %v", err)
	}

	h := handler.NewSecretHandler(svc, userRepo)
	mux := http.NewServeMux()
	h.Register(mux)

	claims := &auth.Claims{UserID: u.ID, Email: u.Email, Tier: u.Tier}
	body := `{"timezone":"Mars/Olympus"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/timezone", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withAuth(req, claims)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d, body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdateTimezone_Unauthenticated(t *testing.T) {
	t.Parallel()

	_, mux := newTestHandler(t)

	body := `{"timezone":"Europe/Istanbul"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/timezone", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMe_IncludesTimezone(t *testing.T) {
	t.Parallel()

	repo, err := sqlite.New(":memory:")
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	sender := noop.New()

	svc, err := service.New(
		repo, sender,
		service.WithBaseURL("http://localhost:3000"),
		service.WithFromEmail("noreply@test.com"),
		service.WithExpiry(10*time.Minute),
	)
	if err != nil {
		t.Fatalf("service.New() error = %v", err)
	}

	userRepo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}
	t.Cleanup(func() { userRepo.Close() })

	u, err := userRepo.Upsert(context.Background(), &model.User{
		Provider:   "google",
		ProviderID: "tz-me",
		Email:      "tz-me@example.com",
		Name:       "TZ Me",
	})
	if err != nil {
		t.Fatalf("upsert user error = %v", err)
	}

	h := handler.NewSecretHandler(svc, userRepo)
	mux := http.NewServeMux()
	h.Register(mux)

	claims := &auth.Claims{UserID: u.ID, Email: u.Email, Tier: u.Tier}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req = withAuth(req, claims)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}

	tz, ok := resp["timezone"].(string)
	if !ok {
		t.Fatal("response should include timezone field")
	}

	if tz != "UTC" {
		t.Errorf("timezone = %q; want %q", tz, "UTC")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/handler/... -run "TestUpdateTimezone|TestMe_IncludesTimezone" -v`
Expected: FAIL — compile error or 404

**Step 3: Add TimezoneRequest to model/request.go**

```go
// TimezoneRequest is the incoming JSON body for updating user timezone.
type TimezoneRequest struct {
	Timezone string `json:"timezone"`
}
```

**Step 4: Add Timezone to MeResponse in model/response.go**

Add `Timezone` field after `DefaultExpiry`:

```go
type MeResponse struct {
	Email           string `json:"email"`
	Name            string `json:"name"`
	AvatarURL       string `json:"avatar_url"`
	Tier            string `json:"tier"`
	SecretsUsed     int    `json:"secrets_used"`
	SecretsLimit    int    `json:"secrets_limit"`
	RecipientsLimit int    `json:"recipients_limit"`
	MaxTextLength   int    `json:"max_text_length"`
	DefaultExpiry   string `json:"default_expiry"`
	Timezone        string `json:"timezone"`
}
```

**Step 5: Register route and implement handler in handler/secret.go**

Add to `Register` method (after line 30):
```go
mux.HandleFunc("PUT /api/v1/me/timezone", h.UpdateTimezone)
```

Add `Timezone` to `Me` handler response (after `DefaultExpiry` in the MeResponse literal):
```go
Timezone:        u.Timezone,
```

Add `UpdateTimezone` handler method:

```go
// UpdateTimezone handles PUT /api/v1/me/timezone.
func (h *SecretHandler) UpdateTimezone(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)

		return
	}

	var req model.TimezoneRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)

		return
	}

	if _, err := time.LoadLocation(req.Timezone); err != nil {
		writeError(w, "validation_error", "Invalid timezone", http.StatusBadRequest)

		return
	}

	if err := h.userRepo.UpdateTimezone(r.Context(), claims.UserID, req.Timezone); err != nil {
		writeError(w, "internal_error", "Failed to update timezone", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

Note: `UpdateTimezone` is on `h.userRepo` which is `user.Repository`. The handler already stores this as `userRepo user.Repository` (line 17).

**Step 6: Run tests to verify they pass**

Run: `cd backend && go test ./internal/handler/... -v`
Expected: ALL PASS

**Step 7: Commit**

```bash
git add backend/internal/model/request.go backend/internal/model/response.go \
       backend/internal/handler/secret.go backend/internal/handler/secret_test.go
git commit -m "feat: add PUT /api/v1/me/timezone endpoint and timezone in /me response"
```

---

### Task 5: Update Email Formatting with Dual Timezone

**Files:**
- Modify: `backend/internal/service/secret.go:230-236` (pass sender timezone to createForRecipient)
- Modify: `backend/internal/service/secret.go:326-383` (createForRecipient — accept senderTimezone)
- Modify: `backend/internal/service/secret.go:441-494` (buildNotificationEmail — accept senderTimezone)
- Test: `backend/internal/service/secret_test.go`

**Step 1: Write failing test for dual timezone email formatting**

Add to `backend/internal/service/secret_test.go`:

```go
func TestBuildNotificationEmail_WithTimezone(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 3, 2, 15, 4, 0, 0, time.UTC)

	tests := []struct {
		name     string
		timezone string
		wantHas  string
		wantNot  string
	}{
		{
			name:     "UTC timezone shows single time",
			timezone: "UTC",
			wantHas:  "Mar 2, 2026 at 3:04 PM UTC",
			wantNot:  "(3:04 PM UTC)",
		},
		{
			name:     "non-UTC timezone shows dual format",
			timezone: "America/New_York",
			wantHas:  "(3:04 PM UTC)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			html := service.BuildNotificationEmail("Test User", "https://example.com/s/token#key", expiresAt, tt.timezone)

			if !strings.Contains(html, tt.wantHas) {
				t.Errorf("email should contain %q, got:\n%s", tt.wantHas, html)
			}

			if tt.wantNot != "" && strings.Contains(html, tt.wantNot) {
				t.Errorf("email should NOT contain %q for %s timezone", tt.wantNot, tt.timezone)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/service/... -run TestBuildNotificationEmail -v`
Expected: FAIL — `service.BuildNotificationEmail` not exported or wrong signature

**Step 3: Update email formatting**

In `backend/internal/service/secret.go`:

1. Update `Create` method to pass sender timezone. Around line 231, where `senderName` is fetched, also capture `senderTimezone`:

```go
var senderName string
var senderTimezone string
if s.userRepo != nil {
    if u, err := s.userRepo.FindByID(ctx, userID); err == nil {
        senderName = u.Name
        senderTimezone = u.Timezone
    }
}
```

2. Update `createForRecipient` signature to accept `senderTimezone string`:

```go
func (s *SecretService) createForRecipient(
    ctx context.Context,
    text, recipientEmail string,
    expiresAt time.Time,
    senderName string,
    senderTimezone string,
) (*model.RecipientLink, error) {
```

3. Update the call in `Create` (around line 239):

```go
link, err := s.createForRecipient(ctx, req.Text, recipientEmail, expiresAt, senderName, senderTimezone)
```

4. Update `createForRecipient` to pass timezone to email builder (around line 373):

```go
html := BuildNotificationEmail(fromLine, link, expiresAt, senderTimezone)
```

5. Export and update `buildNotificationEmail` → `BuildNotificationEmail` with timezone parameter:

```go
// BuildNotificationEmail creates the HTML email body for secret notifications.
// When senderTimezone is non-empty and not "UTC", the expiry is shown in the
// sender's local time with UTC in parentheses. Otherwise only UTC is shown.
func BuildNotificationEmail(senderName, link string, expiresAt time.Time, senderTimezone string) string {
    var expiry string

    if senderTimezone != "" && senderTimezone != "UTC" {
        loc, err := time.LoadLocation(senderTimezone)
        if err == nil {
            local := expiresAt.In(loc).Format("Jan 2, 2006 at 3:04 PM MST")
            utc := expiresAt.UTC().Format("3:04 PM UTC")
            expiry = local + " (" + utc + ")"
        }
    }

    if expiry == "" {
        expiry = expiresAt.UTC().Format("Jan 2, 2006 at 3:04 PM UTC")
    }

    // ... rest of HTML template unchanged ...
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/service/... -v`
Expected: ALL PASS

**Step 5: Run full backend test suite**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add backend/internal/service/secret.go backend/internal/service/secret_test.go
git commit -m "feat: format email expiry in sender's timezone with UTC fallback"
```

---

### Task 6: Update Frontend — MeResponse Type and Timezone Sync

**Files:**
- Modify: `frontend/src/api/client.ts:109-118` (add timezone to MeResponse)
- Modify: `frontend/src/api/client.ts:174-212` (add updateTimezone to api object)
- Modify: `frontend/src/pages/AuthCallback.tsx` (sync timezone after login)

**Step 1: Add timezone to MeResponse type**

In `frontend/src/api/client.ts`, add `timezone` to the `MeResponse` interface:

```typescript
export interface MeResponse {
  email: string
  name: string
  avatar_url: string
  tier: string
  secrets_used: number
  secrets_limit: number
  max_text_length: number
  default_expiry: string
  timezone: string
}
```

**Step 2: Add updateTimezone API method**

Add to the `api` object in `frontend/src/api/client.ts`:

```typescript
export const api = {
  me: () => softAuthFetch<MeResponse>("/me"),

  updateTimezone: (timezone: string) =>
    authenticatedFetch(`${API_BASE}/me/timezone`, {
      method: "PUT",
      body: JSON.stringify({ timezone }),
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to update timezone")
    }),

  // ... rest unchanged
}
```

**Step 3: Update AuthCallback to sync timezone**

```typescript
import { useEffect } from "react"
import { useNavigate } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"
import { api } from "../api/client"

export default function AuthCallback() {
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    if (!auth) {
      navigate("/", { replace: true })
      return
    }

    auth.refreshUser().then((user) => {
      if (user) {
        const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone
        if (browserTz && browserTz !== user.timezone) {
          api.updateTimezone(browserTz).catch(() => {
            // Timezone sync is best-effort — don't block login
          })
        }
      }

      navigate("/dashboard", { replace: true })
    })
  }, [auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Signing in...</p>
    </div>
  )
}
```

Note: This requires `refreshUser()` to return the user object. Check the AuthContext to see if it does — if not, we can use the `auth.user` property after the `refreshUser()` call resolves. Adjust as needed based on the actual return type.

**Step 4: Run frontend type check and lint**

Run: `cd frontend && npx tsc --noEmit && npx eslint .`
Expected: PASS

**Step 5: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/pages/AuthCallback.tsx
git commit -m "feat: auto-sync browser timezone on login"
```

---

### Task 7: Lint, Full Test Suite, and Final Verification

**Files:** None (verification only)

**Step 1: Run backend linter**

Run: `cd backend && golangci-lint run ./...`
Expected: PASS (no lint errors)

**Step 2: Run backend tests with race detector**

Run: `cd backend && go test -race ./...`
Expected: ALL PASS

**Step 3: Run frontend build**

Run: `cd frontend && npm run build`
Expected: PASS

**Step 4: Run frontend lint**

Run: `cd frontend && npx eslint .`
Expected: PASS

**Step 5: Commit any lint fixes if needed**

If any lint issues are found, fix them and commit:
```bash
git commit -m "fix: resolve lint issues"
```

---

### Task 8: Update OpenAPI Spec

**Files:**
- Modify: `backend/docs/openapi.yaml`

**Step 1: Add timezone field to MeResponse schema**

Add `timezone` property to the `MeResponse` schema component:

```yaml
timezone:
  type: string
  description: User's IANA timezone (e.g. "Europe/Istanbul")
  example: "Europe/Istanbul"
```

**Step 2: Add PUT /api/v1/me/timezone endpoint**

Add the new endpoint to the paths section:

```yaml
/api/v1/me/timezone:
  put:
    summary: Update user timezone
    operationId: updateTimezone
    security:
      - bearerAuth: []
    requestBody:
      required: true
      content:
        application/json:
          schema:
            type: object
            required:
              - timezone
            properties:
              timezone:
                type: string
                description: IANA timezone name
                example: "Europe/Istanbul"
    responses:
      "204":
        description: Timezone updated successfully
      "400":
        description: Invalid timezone
      "401":
        description: Authentication required
```

**Step 3: Commit**

```bash
git add backend/docs/openapi.yaml
git commit -m "docs: add timezone endpoint and field to OpenAPI spec"
```
