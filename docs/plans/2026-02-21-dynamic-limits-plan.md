# Dynamic Tier Limits Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace hardcoded tier limits with a database-driven `limits` table and add per-user override support.

**Architecture:** New `limits` table stores secrets_limit and recipients_limit per tier (free, pro, future tiers). Users table gets an optional `secrets_limit` column for per-user override. Priority: user override > limits table > hardcoded fallback. Admin panel gets a Limits page for CRUD.

**Tech Stack:** Go (backend), React/TypeScript (frontend), SQLite

---

### Task 1: Database — Add `limits` table and `users.secrets_limit` column

**Files:**
- Modify: `backend/internal/user/sqlite/sqlite.go` (migration section, lines 17-55)

**Step 1: Add limits table and alter users table in migration**

Add after existing CREATE TABLE statements:

```go
CREATE TABLE IF NOT EXISTS limits (
    tier             TEXT PRIMARY KEY,
    secrets_limit    INTEGER NOT NULL DEFAULT 5,
    recipients_limit INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO limits (tier, secrets_limit, recipients_limit) VALUES ('free', 5, 1);
INSERT OR IGNORE INTO limits (tier, secrets_limit, recipients_limit) VALUES ('pro', 100, 5);
```

Add `secrets_limit` nullable column to users (use a separate pragma + ALTER approach since SQLite doesn't support IF NOT EXISTS for ALTER):

```go
// Check if column exists before adding
PRAGMA table_info(users);
// If secrets_limit not present:
ALTER TABLE users ADD COLUMN secrets_limit INTEGER;
```

**Step 2: Run backend tests**

Run: `cd backend && go test -race ./internal/user/...`
Expected: PASS

**Step 3: Commit**

```bash
git add backend/internal/user/sqlite/sqlite.go
git commit -m "feat: add limits table and users.secrets_limit column"
```

---

### Task 2: Repository — Add limits CRUD and update user queries

**Files:**
- Create: `backend/internal/user/limits.go` (TierLimits struct + LimitsRepository interface)
- Modify: `backend/internal/user/user.go` (extend AdminRepository)
- Modify: `backend/internal/user/sqlite/sqlite.go` (implement limits queries, update user scan)

**Step 1: Create limits domain types**

In `backend/internal/user/limits.go`:

```go
package user

// TierLimits holds the limit configuration for a tier.
type TierLimits struct {
    Tier            string
    SecretsLimit    int
    RecipientsLimit int
}
```

**Step 2: Extend AdminRepository interface**

In `backend/internal/user/user.go`, add to AdminRepository:

```go
type AdminRepository interface {
    Repository
    // existing...
    ListUsers(ctx context.Context, opts ...ListOption) ([]*model.User, error)
    CountUsers(ctx context.Context, opts ...ListOption) (int64, error)
    ListSubscriptions(ctx context.Context, opts ...ListOption) ([]*SubscriptionWithUser, error)
    CountSubscriptions(ctx context.Context, opts ...ListOption) (int64, error)
    // new
    ListLimits(ctx context.Context) ([]*TierLimits, error)
    GetLimits(ctx context.Context, tier string) (*TierLimits, error)
    UpsertLimits(ctx context.Context, tl *TierLimits) error
    DeleteLimits(ctx context.Context, tier string) error
}
```

**Step 3: Implement SQLite limits queries**

In `backend/internal/user/sqlite/sqlite.go`:

- `ListLimits` — `SELECT tier, secrets_limit, recipients_limit FROM limits ORDER BY tier`
- `GetLimits` — `SELECT ... FROM limits WHERE tier = ?`
- `UpsertLimits` — `INSERT INTO limits ... ON CONFLICT(tier) DO UPDATE SET ...`
- `DeleteLimits` — `DELETE FROM limits WHERE tier = ?`

**Step 4: Update User scan to include secrets_limit**

Update ListUsers and FindByID to scan the new nullable `secrets_limit` column into the User struct. Use `sql.NullInt64` for scanning.

**Step 5: Run tests**

Run: `cd backend && go test -race ./internal/user/...`
Expected: PASS (may need test updates for new column)

**Step 6: Commit**

```bash
git add backend/internal/user/
git commit -m "feat: add limits repository with CRUD operations"
```

---

### Task 3: Model — Replace hardcoded constants with dynamic lookup

**Files:**
- Modify: `backend/internal/model/user.go` (add SecretsLimitOverride field, update methods)
- Modify: `backend/internal/model/secret.go` (remove FreeMaxRecipients/ProMaxRecipients or keep as fallback)
- Modify: `backend/internal/model/admin.go` (add secrets_limit + secrets_limit_override to admin responses)
- Modify: `backend/internal/model/response.go` (add recipients_limit to MeResponse)
- Modify: `backend/internal/model/user_test.go` (update tests)

**Step 1: Update User struct and methods**

```go
type User struct {
    // existing fields...
    SecretsLimitOverride *int  // nullable, from users.secrets_limit
}

// SecretsLimit now accepts tier limits from the limits table.
// Priority: user override > tier limits > hardcoded fallback.
func (u *User) SecretsLimit(tierLimits *TierLimitsConfig) int {
    if u.SecretsLimitOverride != nil {
        return *u.SecretsLimitOverride
    }
    if tierLimits != nil {
        return tierLimits.SecretsLimit
    }
    // hardcoded fallback
    if u.Tier == TierPro {
        return ProTierLimit
    }
    return FreeTierLimit
}
```

Wait — this changes the method signature which breaks callers. Better approach: add a `TierLimitsConfig` struct that gets set on the User before calling SecretsLimit.

Actually simplest: keep the signature, store tier limits on the User:

```go
type User struct {
    // existing fields...
    SecretsLimitOverride *int // from users.secrets_limit column (nullable)
    TierSecretsLimit     int  // from limits table (loaded at query time)
    TierRecipientsLimit  int  // from limits table (loaded at query time)
}

func (u *User) SecretsLimit() int {
    if u.SecretsLimitOverride != nil {
        return *u.SecretsLimitOverride
    }
    if u.TierSecretsLimit > 0 {
        return u.TierSecretsLimit
    }
    // hardcoded fallback
    if u.Tier == TierPro {
        return ProTierLimit
    }
    return FreeTierLimit
}

func (u *User) RecipientsLimit() int {
    if u.TierRecipientsLimit > 0 {
        return u.TierRecipientsLimit
    }
    if u.Tier == TierPro {
        return ProMaxRecipients
    }
    return FreeMaxRecipients
}
```

**Step 2: Update AdminUserResponse**

Add `secrets_limit` and `secrets_limit_override` fields:

```go
type AdminUserResponse struct {
    // existing...
    SecretsUsed          int    `json:"secrets_used"`
    SecretsLimit         int    `json:"secrets_limit"`          // effective limit
    SecretsLimitOverride *int   `json:"secrets_limit_override"` // per-user override (null = use tier default)
}
```

**Step 3: Update MeResponse**

Add `recipients_limit`:

```go
type MeResponse struct {
    // existing...
    SecretsLimit    int `json:"secrets_limit"`
    RecipientsLimit int `json:"recipients_limit"`
}
```

**Step 4: Add AdminUpdateUserRequest** (replaces AdminUpdateTierRequest)

```go
type AdminUpdateUserRequest struct {
    Tier                 *string `json:"tier,omitempty"`
    SecretsLimitOverride *int    `json:"secrets_limit_override,omitempty"`
}
```

Actually we should keep it backward compatible — tier update stays as is, secrets_limit_override is added to the PATCH body.

**Step 5: Update tests**

**Step 6: Commit**

```bash
git add backend/internal/model/
git commit -m "feat: add dynamic limits fields to User model and admin responses"
```

---

### Task 4: Service — Load tier limits when checking CanCreateSecret

**Files:**
- Modify: `backend/internal/service/secret.go` (load limits from repo before CanCreateSecret check)
- Modify: `backend/internal/service/secret_test.go` (update tests)
- Modify: `backend/internal/handler/secret.go` (load limits for /me endpoint)

**Step 1: Update SecretService to use limits**

The service already has `userRepo`. Add a method or use the repo to get limits:

In `Create()`, after fetching user with `FindByID`:

```go
u, err := s.userRepo.FindByID(ctx, userID)
// ... error handling ...

// Load tier limits
tl, err := s.limitsRepo.GetLimits(ctx, u.Tier)
if err == nil {
    u.TierSecretsLimit = tl.SecretsLimit
    u.TierRecipientsLimit = tl.RecipientsLimit
}

if !u.CanCreateSecret() {
    // ...
}
maxRecipients = u.RecipientsLimit()
```

The SecretService needs access to limits. Options:
1. Add a `limitsRepo` field — but that would require AdminRepository
2. Use the existing `userRepo` — but Repository doesn't have GetLimits
3. Add `GetLimits(ctx, tier)` to the base `Repository` interface — cleaner, since any code checking limits needs it

Best approach: Add `GetLimits(ctx, tier) (*TierLimits, error)` to the base `user.Repository` interface since the service layer needs it for every secret creation.

Wait, but we don't want to expose all admin operations. A `LimitsReader` interface is cleanest:

Actually simplest: just add to base Repository since it's a read operation any authenticated flow needs.

**Step 2: Update handler/secret.go Me endpoint**

Load tier limits and set on user before building MeResponse:

```go
u, _ := h.userRepo.FindByID(...)
// load limits from admin repo or a separate limits reader
```

Hmm, the `SecretHandler` only has `user.Repository`, not `AdminRepository`. We need GetLimits accessible from the base interface. So add it there.

**Step 3: Update tests**

**Step 4: Commit**

```bash
git add backend/internal/service/ backend/internal/handler/secret.go backend/internal/user/
git commit -m "feat: load tier limits dynamically in service and handler"
```

---

### Task 5: Admin Handler — Update user endpoint + add limits CRUD endpoints

**Files:**
- Modify: `backend/internal/handler/admin.go` (update UpdateTier to support secrets_limit_override, add limits endpoints, validate tier against limits table)

**Step 1: Add limits endpoints to Register**

```go
mux.HandleFunc("GET /api/v1/admin/limits", h.ListLimits)
mux.HandleFunc("PUT /api/v1/admin/limits/{tier}", h.UpsertLimits)
mux.HandleFunc("DELETE /api/v1/admin/limits/{tier}", h.DeleteLimits)
```

**Step 2: Implement ListLimits handler**

Returns all rows from limits table.

**Step 3: Implement UpsertLimits handler**

Accepts `{"secrets_limit": 100, "recipients_limit": 5}`, tier from path.

**Step 4: Implement DeleteLimits handler**

Deletes a tier from limits table. Should reject deleting "free" or tiers that have users assigned.

**Step 5: Update UpdateTier / UpdateUser**

- Validate tier exists in limits table (not hardcoded check)
- Accept optional `secrets_limit_override` in PATCH body
- Add `UpdateSecretsLimitOverride` to repository

**Step 6: Update ListUsers response**

Include `secrets_limit` (effective) and `secrets_limit_override` (per-user) in response. Load tier limits to compute effective limit.

**Step 7: Run tests**

**Step 8: Commit**

```bash
git add backend/internal/handler/admin.go
git commit -m "feat: add limits CRUD endpoints and user override support"
```

---

### Task 6: Admin Handler + Repository Tests

**Files:**
- Modify: `backend/internal/handler/admin_test.go`
- Modify: `backend/internal/user/sqlite/admin_test.go`
- Create: `backend/internal/user/limits_test.go` (if needed)

**Step 1: Write repository tests for limits CRUD**

- TestListLimits (returns seeded free + pro)
- TestGetLimits (found + not found)
- TestUpsertLimits (insert new tier + update existing)
- TestDeleteLimits (success + not found)

**Step 2: Write handler tests for limits endpoints**

- TestListLimits
- TestUpsertLimits (valid + invalid body)
- TestDeleteLimits (success + protected tier)
- TestUpdateTier_DynamicValidation (accept tier from limits table)
- TestUpdateUser_SecretsLimitOverride

**Step 3: Run full test suite**

Run: `cd backend && go test -race ./...`

**Step 4: Commit**

```bash
git add backend/internal/
git commit -m "test: add limits CRUD and user override tests"
```

---

### Task 7: Frontend — Admin API client + Limits page

**Files:**
- Modify: `frontend/src/api/admin.ts` (add TierLimits interface, limits API methods, update AdminUser)
- Create: `frontend/src/pages/admin/Limits.tsx`
- Modify: `frontend/src/components/AdminLayout.tsx` (add Limits nav link)
- Modify: `frontend/src/App.tsx` (add limits route)

**Step 1: Update admin API client**

```typescript
export interface TierLimits {
  tier: string
  secrets_limit: number
  recipients_limit: number
}

export interface AdminUser {
  // existing...
  secrets_limit: number           // effective limit
  secrets_limit_override: number | null  // per-user override
}

// Add to adminApi:
fetchLimits: () => adminRequest<TierLimits[]>("/limits"),
upsertLimits: (tier: string, data: { secrets_limit: number; recipients_limit: number }) =>
  adminRequest<TierLimits>(`/limits/${tier}`, { method: "PUT", body: JSON.stringify(data) }),
deleteLimits: (tier: string) =>
  adminRequest<void>(`/limits/${tier}`, { method: "DELETE" }),
```

**Step 2: Create Limits page**

Table showing all tiers with secrets_limit and recipients_limit. Inline edit for each row. "Add Tier" button to add new tier (e.g. vip). Delete button for non-default tiers. ConfirmModal for delete.

**Step 3: Add route and nav link**

- AdminLayout navLinks: add `{ to: "/admin/limits", label: "Limits", icon: Gauge }` (lucide-react Gauge icon)
- App.tsx: add `<Route path="limits" element={<AdminLimits />} />`

**Step 4: Build and lint**

Run: `cd frontend && npm run build && npx eslint .`

**Step 5: Commit**

```bash
git add frontend/
git commit -m "feat(admin): add limits management page"
```

---

### Task 8: Frontend — Update Users page with per-user override

**Files:**
- Modify: `frontend/src/pages/admin/Users.tsx` (show effective limit, add override edit)
- Modify: `frontend/src/api/admin.ts` (updateUser method)

**Step 1: Update Users table**

Add "Limit" column showing effective `secrets_limit`. If `secrets_limit_override` is set, show it with a badge indicating "custom". Add a small edit button to set/clear override.

**Step 2: Add updateUser API method**

```typescript
updateUser: (id: number, data: { tier?: string; secrets_limit_override?: number | null }) =>
  adminRequest<{ status: string }>(`/users/${id}`, {
    method: "PATCH",
    body: JSON.stringify(data),
  }),
```

This replaces the current `updateTier` method. Update Users.tsx to use `updateUser` with `{ tier: newTier }` for tier changes.

**Step 3: Build and lint**

**Step 4: Commit**

```bash
git add frontend/
git commit -m "feat(admin): add per-user secrets limit override to users page"
```

---

### Task 9: OpenAPI spec update

**Files:**
- Modify: `backend/docs/openapi.yaml`

**Step 1: Add limits endpoints**

- `GET /api/v1/admin/limits` — List all tier limits
- `PUT /api/v1/admin/limits/{tier}` — Create/update tier limits
- `DELETE /api/v1/admin/limits/{tier}` — Delete tier limits

**Step 2: Update existing admin schemas**

- AdminUserResponse: add `secrets_limit`, `secrets_limit_override`
- AdminUpdateUserRequest: add `secrets_limit_override`
- MeResponse: add `recipients_limit`

**Step 3: Commit**

```bash
git add backend/docs/openapi.yaml
git commit -m "docs: add limits endpoints and update schemas in OpenAPI spec"
```

---

### Task 10: Documentation update (README, CLAUDE.md)

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

**Step 1: Update README**

- Add limits endpoints to API Endpoints table
- Add Limits page to Admin Panel section
- Update Pricing section to mention configurable limits

**Step 2: Update CLAUDE.md**

- Add limits endpoints to API Endpoints list
- Update Frontend Routes to include /admin/limits
- Mention limits table in project description

**Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "docs: update README and CLAUDE.md with dynamic limits feature"
```

---

### Task 11: Build verification and cleanup

**Step 1: Run full backend tests**

Run: `cd backend && go test -race ./...`

**Step 2: Run linter**

Run: `cd backend && golangci-lint run ./...`

**Step 3: Build frontend**

Run: `cd frontend && npm run build`

**Step 4: Lint frontend**

Run: `cd frontend && npx eslint .`
