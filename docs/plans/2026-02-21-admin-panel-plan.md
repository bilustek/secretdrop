# Admin Panel & API Docs Protection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Basic Auth-protected admin API endpoints for managing users and subscriptions, and protect the `/docs` endpoint behind the same authentication.

**Architecture:** New BasicAuth middleware protects `/api/v1/admin/*` and `/docs` routes. Admin handler provides list/search/filter/sort/pagination for users and subscriptions, plus tier update and subscription cancel. User repository gets new list/count query methods with functional options.

**Tech Stack:** Go 1.26, net/http, SQLite (modernc.org/sqlite), existing handler/middleware patterns

---

### Task 1: Config — Add Admin Credentials

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go` (if exists, otherwise tests are in Load tests)

**Step 1: Add fields and getters to config**

Add to the `Config` struct (after `slackWebhookNotifications`):

```go
adminUsername string
adminPassword string
```

Add getters:

```go
// AdminUsername returns the admin Basic Auth username.
func (c *Config) AdminUsername() string { return c.adminUsername }

// AdminPassword returns the admin Basic Auth password.
func (c *Config) AdminPassword() string { return c.adminPassword }
```

Add env var reading in `Load()` (after slack lines, before `for _, opt := range opts`):

```go
c.adminUsername = os.Getenv("ADMIN_USERNAME")
c.adminPassword = os.Getenv("ADMIN_PASSWORD")
```

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat: add admin credentials to config"
```

---

### Task 2: BasicAuth Middleware

**Files:**
- Create: `backend/internal/middleware/basicauth.go`
- Create: `backend/internal/middleware/basicauth_test.go`

**Step 1: Write the failing tests**

Create `backend/internal/middleware/basicauth_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
)

func TestBasicAuth_ValidCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "secret")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}
}

func TestBasicAuth_InvalidCredentials(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth("admin", "wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}

	if got := rec.Header().Get("WWW-Authenticate"); got != `Basic realm="admin"` {
		t.Errorf("WWW-Authenticate = %q; want %q", got, `Basic realm="admin"`)
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	t.Parallel()

	handler := middleware.BasicAuth("admin", "secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusUnauthorized)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/middleware/ -run TestBasicAuth -v`
Expected: FAIL (BasicAuth not defined)

**Step 3: Write the implementation**

Create `backend/internal/middleware/basicauth.go`:

```go
package middleware

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth returns middleware that requires HTTP Basic Authentication
// with the given username and password.
func BasicAuth(username, password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			if !ok ||
				subtle.ConstantTimeCompare([]byte(u), []byte(username)) != 1 ||
				subtle.ConstantTimeCompare([]byte(p), []byte(password)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/middleware/ -run TestBasicAuth -v`
Expected: PASS (3/3)

**Step 5: Run linter**

Run: `cd backend && golangci-lint run ./internal/middleware/...`
Expected: No issues

**Step 6: Commit**

```bash
git add backend/internal/middleware/basicauth.go backend/internal/middleware/basicauth_test.go
git commit -m "feat: add BasicAuth middleware"
```

---

### Task 3: User Repository — Admin List Interface

**Files:**
- Modify: `backend/internal/user/user.go`

**Step 1: Add admin query types and interface methods**

Add to `backend/internal/user/user.go` (after the existing `Repository` interface):

```go
// AdminRepository extends Repository with admin query operations.
type AdminRepository interface {
	Repository

	ListUsers(ctx context.Context, opts ...ListOption) ([]*model.User, error)
	CountUsers(ctx context.Context, opts ...ListOption) (int64, error)
	ListSubscriptions(ctx context.Context, opts ...ListOption) ([]*SubscriptionWithUser, error)
	CountSubscriptions(ctx context.Context, opts ...ListOption) (int64, error)
}

// SubscriptionWithUser holds a subscription joined with its user's email and name.
type SubscriptionWithUser struct {
	model.Subscription
	UserEmail string
	UserName  string
}

// ListOption configures a list query.
type ListOption func(*ListQuery)

// ListQuery holds the parameters for list queries.
type ListQuery struct {
	Search  string
	Tier    string
	Status  string
	Sort    string
	Order   string
	Page    int
	PerPage int
}

// DefaultListQuery returns a ListQuery with sensible defaults.
func DefaultListQuery() *ListQuery {
	return &ListQuery{
		Sort:    "created_at",
		Order:   "desc",
		Page:    1,
		PerPage: 20,
	}
}

// ApplyOptions applies the given options to a default ListQuery.
func ApplyOptions(opts ...ListOption) *ListQuery {
	q := DefaultListQuery()
	for _, opt := range opts {
		opt(q)
	}

	return q
}

// WithSearch filters results by email (LIKE match).
func WithSearch(search string) ListOption {
	return func(q *ListQuery) {
		q.Search = search
	}
}

// WithTier filters users by tier.
func WithTier(tier string) ListOption {
	return func(q *ListQuery) {
		q.Tier = tier
	}
}

// WithStatus filters subscriptions by status.
func WithStatus(status string) ListOption {
	return func(q *ListQuery) {
		q.Status = status
	}
}

// WithSort sets the sort field and order.
func WithSort(field, order string) ListOption {
	return func(q *ListQuery) {
		q.Sort = field
		q.Order = order
	}
}

// WithPage sets the page number and page size.
func WithPage(page, perPage int) ListOption {
	return func(q *ListQuery) {
		q.Page = page
		q.PerPage = perPage
	}
}
```

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add backend/internal/user/user.go
git commit -m "feat: add AdminRepository interface and list options"
```

---

### Task 4: User Repository — SQLite Admin Implementation

**Files:**
- Modify: `backend/internal/user/sqlite/sqlite.go`
- Create: `backend/internal/user/sqlite/admin_test.go`

**Step 1: Write the failing tests**

Create `backend/internal/user/sqlite/admin_test.go`:

```go
package sqlite_test

import (
	"context"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func newTestRepo(t *testing.T) *usersqlite.Repository {
	t.Helper()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	t.Cleanup(func() { _ = repo.Close() })

	return repo
}

func seedUsers(t *testing.T, repo *usersqlite.Repository, count int) {
	t.Helper()

	ctx := context.Background()

	for i := range count {
		tier := model.TierFree
		if i%3 == 0 {
			tier = model.TierPro
		}

		u, err := repo.Upsert(ctx, &model.User{
			Provider:   "google",
			ProviderID: "gid-" + string(rune('a'+i)),
			Email:      "user" + string(rune('a'+i)) + "@example.com",
			Name:       "User " + string(rune('A'+i)),
		})
		if err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}

		if tier == model.TierPro {
			if err := repo.UpdateTier(ctx, u.ID, model.TierPro); err != nil {
				t.Fatalf("UpdateTier() error = %v", err)
			}
		}
	}
}

func TestListUsers_Pagination(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	seedUsers(t, repo, 5)

	ctx := context.Background()

	users, err := repo.ListUsers(ctx, user.WithPage(1, 2))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 2 {
		t.Errorf("len(users) = %d; want 2", len(users))
	}

	count, err := repo.CountUsers(ctx)
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 5 {
		t.Errorf("count = %d; want 5", count)
	}
}

func TestListUsers_SearchByEmail(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "bob@example.com", Name: "Bob",
	})

	users, err := repo.ListUsers(ctx, user.WithSearch("alice"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}
}

func TestListUsers_FilterByTier(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u1, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "free@example.com", Name: "Free",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "pro@example.com", Name: "Pro",
	})
	_ = repo.UpdateTier(ctx, u1.ID+1, model.TierPro)

	users, err := repo.ListUsers(ctx, user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(users))
	}
}

func TestListUsers_Sort(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "b@example.com", Name: "Bravo",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "a@example.com", Name: "Alpha",
	})

	users, err := repo.ListUsers(ctx, user.WithSort("email", "asc"))
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if len(users) < 2 {
		t.Fatalf("len(users) = %d; want >= 2", len(users))
	}

	if users[0].Email != "a@example.com" {
		t.Errorf("first user email = %q; want %q", users[0].Email, "a@example.com")
	}
}

func TestCountUsers_WithFilter(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "free@example.com", Name: "Free",
	})
	u2, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g2",
		Email: "pro@example.com", Name: "Pro",
	})
	_ = repo.UpdateTier(ctx, u2.ID, model.TierPro)

	count, err := repo.CountUsers(ctx, user.WithTier(model.TierPro))
	if err != nil {
		t.Fatalf("CountUsers() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}

func TestListSubscriptions_Basic(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	subs, err := repo.ListSubscriptions(ctx)
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d; want 1", len(subs))
	}

	if subs[0].UserEmail != "sub@example.com" {
		t.Errorf("UserEmail = %q; want %q", subs[0].UserEmail, "sub@example.com")
	}

	if subs[0].UserName != "Sub User" {
		t.Errorf("UserName = %q; want %q", subs[0].UserName, "Sub User")
	}
}

func TestListSubscriptions_FilterByStatus(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_2",
		Status:               model.SubscriptionCanceled,
	})

	subs, err := repo.ListSubscriptions(ctx, user.WithStatus(model.SubscriptionActive))
	if err != nil {
		t.Fatalf("ListSubscriptions() error = %v", err)
	}

	if len(subs) != 1 {
		t.Errorf("len(subs) = %d; want 1", len(subs))
	}
}

func TestCountSubscriptions(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})

	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	count, err := repo.CountSubscriptions(ctx)
	if err != nil {
		t.Fatalf("CountSubscriptions() error = %v", err)
	}

	if count != 1 {
		t.Errorf("count = %d; want 1", count)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/user/sqlite/ -run "TestList|TestCount" -v`
Expected: FAIL (methods not defined)

**Step 3: Write the implementation**

Add to `backend/internal/user/sqlite/sqlite.go` (before the `Close` method):

```go
// compile-time admin interface check.
var _ user.AdminRepository = (*Repository)(nil)

// Allowed sort columns for users (whitelist to prevent SQL injection).
var userSortColumns = map[string]string{
	"created_at":   "created_at",
	"email":        "email",
	"name":         "name",
	"tier":         "tier",
	"secrets_used": "secrets_used",
}

// Allowed sort columns for subscriptions.
var subscriptionSortColumns = map[string]string{
	"created_at": "s.created_at",
	"status":     "s.status",
}

// ListUsers returns a paginated list of users with optional search, filter, and sort.
func (r *Repository) ListUsers(ctx context.Context, opts ...user.ListOption) ([]*model.User, error) {
	q := user.ApplyOptions(opts...)

	query := "SELECT id, provider, provider_id, email, name, avatar_url, tier, secrets_used, created_at, updated_at FROM users"
	args := []any{}
	clauses := []string{}

	if q.Search != "" {
		clauses = append(clauses, "email LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if q.Tier != "" {
		clauses = append(clauses, "tier = ?")
		args = append(args, q.Tier)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	col := userSortColumns[q.Sort]
	if col == "" {
		col = "created_at"
	}

	order := "DESC"
	if strings.EqualFold(q.Order, "asc") {
		order = "ASC"
	}

	query += " ORDER BY " + col + " " + order

	if q.PerPage > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, q.PerPage, (q.Page-1)*q.PerPage)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []*model.User

	for rows.Next() {
		u := &model.User{}
		if err := rows.Scan(
			&u.ID, &u.Provider, &u.ProviderID, &u.Email,
			&u.Name, &u.AvatarURL, &u.Tier, &u.SecretsUsed,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

// CountUsers returns the total count of users matching the given options.
func (r *Repository) CountUsers(ctx context.Context, opts ...user.ListOption) (int64, error) {
	q := user.ApplyOptions(opts...)

	query := "SELECT COUNT(*) FROM users"
	args := []any{}
	clauses := []string{}

	if q.Search != "" {
		clauses = append(clauses, "email LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if q.Tier != "" {
		clauses = append(clauses, "tier = ?")
		args = append(args, q.Tier)
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}

	return count, nil
}

// ListSubscriptions returns a paginated list of subscriptions with user info.
func (r *Repository) ListSubscriptions(ctx context.Context, opts ...user.ListOption) ([]*user.SubscriptionWithUser, error) {
	q := user.ApplyOptions(opts...)

	query := `SELECT s.id, s.user_id, s.stripe_customer_id,
		s.stripe_subscription_id, s.status,
		s.current_period_start, s.current_period_end,
		s.created_at, u.email, u.name
		FROM subscriptions s
		JOIN users u ON s.user_id = u.id`
	args := []any{}
	clauses := []string{}

	if q.Status != "" {
		clauses = append(clauses, "s.status = ?")
		args = append(args, q.Status)
	}

	if q.Search != "" {
		clauses = append(clauses, "u.email LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	col := subscriptionSortColumns[q.Sort]
	if col == "" {
		col = "s.created_at"
	}

	order := "DESC"
	if strings.EqualFold(q.Order, "asc") {
		order = "ASC"
	}

	query += " ORDER BY " + col + " " + order

	if q.PerPage > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, q.PerPage, (q.Page-1)*q.PerPage)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []*user.SubscriptionWithUser

	for rows.Next() {
		s := &user.SubscriptionWithUser{}
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.StripeCustomerID,
			&s.StripeSubscriptionID, &s.Status,
			&s.CurrentPeriodStart, &s.CurrentPeriodEnd,
			&s.CreatedAt, &s.UserEmail, &s.UserName,
		); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}

		subs = append(subs, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return subs, nil
}

// CountSubscriptions returns the total count of subscriptions matching the given options.
func (r *Repository) CountSubscriptions(ctx context.Context, opts ...user.ListOption) (int64, error) {
	q := user.ApplyOptions(opts...)

	query := `SELECT COUNT(*) FROM subscriptions s JOIN users u ON s.user_id = u.id`
	args := []any{}
	clauses := []string{}

	if q.Status != "" {
		clauses = append(clauses, "s.status = ?")
		args = append(args, q.Status)
	}

	if q.Search != "" {
		clauses = append(clauses, "u.email LIKE ?")
		args = append(args, "%"+q.Search+"%")
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}

	var count int64
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count subscriptions: %w", err)
	}

	return count, nil
}
```

Note: Add `"strings"` to the import block in `sqlite.go`.

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/user/sqlite/ -run "TestList|TestCount" -v`
Expected: PASS (all tests)

**Step 5: Run all tests to check nothing broke**

Run: `cd backend && go test -race ./...`
Expected: PASS

**Step 6: Run linter**

Run: `cd backend && golangci-lint run ./internal/user/...`
Expected: No issues

**Step 7: Commit**

```bash
git add backend/internal/user/sqlite/sqlite.go backend/internal/user/sqlite/admin_test.go
git commit -m "feat: add admin list/count methods to user repository"
```

---

### Task 5: Admin Response Models

**Files:**
- Modify: `backend/internal/model/response.go`

**Step 1: Add admin response types**

Add to `backend/internal/model/response.go`:

```go
// AdminUserResponse represents a user in admin list responses.
type AdminUserResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Tier        string `json:"tier"`
	SecretsUsed int    `json:"secrets_used"`
	CreatedAt   string `json:"created_at"`
}

// AdminUsersListResponse is the paginated list of users.
type AdminUsersListResponse struct {
	Users   []AdminUserResponse `json:"users"`
	Total   int64               `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"per_page"`
}

// AdminSubscriptionResponse represents a subscription in admin list responses.
type AdminSubscriptionResponse struct {
	ID                   int64  `json:"id"`
	UserID               int64  `json:"user_id"`
	UserEmail            string `json:"user_email"`
	UserName             string `json:"user_name"`
	StripeCustomerID     string `json:"stripe_customer_id"`
	StripeSubscriptionID string `json:"stripe_subscription_id"`
	Status               string `json:"status"`
	CurrentPeriodStart   string `json:"current_period_start"`
	CurrentPeriodEnd     string `json:"current_period_end"`
	CreatedAt            string `json:"created_at"`
}

// AdminSubscriptionsListResponse is the paginated list of subscriptions.
type AdminSubscriptionsListResponse struct {
	Subscriptions []AdminSubscriptionResponse `json:"subscriptions"`
	Total         int64                       `json:"total"`
	Page          int                         `json:"page"`
	PerPage       int                         `json:"per_page"`
}

// AdminUpdateTierRequest is the body for PATCH /api/v1/admin/users/{id}.
type AdminUpdateTierRequest struct {
	Tier string `json:"tier"`
}
```

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add backend/internal/model/response.go
git commit -m "feat: add admin response and request models"
```

---

### Task 6: Admin Handler

**Files:**
- Create: `backend/internal/handler/admin.go`
- Create: `backend/internal/handler/admin_test.go`

**Step 1: Write failing tests**

Create `backend/internal/handler/admin_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bilusteknoloji/secretdrop/internal/handler"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	usersqlite "github.com/bilusteknoloji/secretdrop/internal/user/sqlite"
)

func newAdminTestRepo(t *testing.T) *usersqlite.Repository {
	t.Helper()

	repo, err := usersqlite.New(":memory:")
	if err != nil {
		t.Fatalf("usersqlite.New() error = %v", err)
	}

	t.Cleanup(func() { _ = repo.Close() })

	return repo
}

func TestAdminListUsers_Empty(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("total = %d; want 0", resp.Total)
	}
}

func TestAdminListUsers_WithData(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "github", ProviderID: "gh1",
		Email: "bob@example.com", Name: "Bob",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?per_page=1&page=1", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminUsersListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("total = %d; want 2", resp.Total)
	}

	if len(resp.Users) != 1 {
		t.Errorf("len(users) = %d; want 1", len(resp.Users))
	}
}

func TestAdminListUsers_Search(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})
	_, _ = repo.Upsert(ctx, &model.User{
		Provider: "github", ProviderID: "gh1",
		Email: "bob@example.com", Name: "Bob",
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?q=alice", nil)
	rec := httptest.NewRecorder()

	h.ListUsers(rec, req)

	var resp model.AdminUsersListResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}
}

func TestAdminUpdateTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "alice@example.com", Name: "Alice",
	})

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"pro"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/"+strings.TrimSpace(
		func() string { s := ""; s = strings.TrimSpace(func() string { return fmt.Sprintf("%d", u.ID) }()); return s }(),
	), body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	updated, _ := repo.FindByID(ctx, u.ID)
	if updated.Tier != model.TierPro {
		t.Errorf("tier = %q; want %q", updated.Tier, model.TierPro)
	}
}

func TestAdminUpdateTier_InvalidTier(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)

	h := handler.NewAdminHandler(repo, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)

	body := strings.NewReader(`{"tier":"enterprise"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/1", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestAdminListSubscriptions(t *testing.T) {
	t.Parallel()

	repo := newAdminTestRepo(t)
	ctx := context.Background()

	u, _ := repo.Upsert(ctx, &model.User{
		Provider: "google", ProviderID: "g1",
		Email: "sub@example.com", Name: "Sub User",
	})
	_ = repo.UpsertSubscription(ctx, &model.Subscription{
		UserID:               u.ID,
		StripeCustomerID:     "cus_1",
		StripeSubscriptionID: "sub_1",
		Status:               model.SubscriptionActive,
	})

	h := handler.NewAdminHandler(repo, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil)
	rec := httptest.NewRecorder()

	h.ListSubscriptions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d; want %d", rec.Code, http.StatusOK)
	}

	var resp model.AdminSubscriptionsListResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("total = %d; want 1", resp.Total)
	}

	if resp.Subscriptions[0].UserEmail != "sub@example.com" {
		t.Errorf("user_email = %q; want %q", resp.Subscriptions[0].UserEmail, "sub@example.com")
	}
}
```

Note: The `TestAdminUpdateTier` test has a complex ID formatting — the implementer should simplify it using `strconv.FormatInt(u.ID, 10)` and a direct `fmt.Sprintf` for the URL path.

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/handler/ -run TestAdmin -v`
Expected: FAIL (NewAdminHandler not defined)

**Step 3: Write the implementation**

Create `backend/internal/handler/admin.go`:

```go
package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

const (
	defaultPerPage = 20
	maxPerPage     = 100
	timeFormat     = time.RFC3339
)

// AdminHandler handles admin API requests.
type AdminHandler struct {
	repo      user.AdminRepository
	canceller SubscriptionCanceller
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(repo user.AdminRepository, canceller SubscriptionCanceller) *AdminHandler {
	return &AdminHandler{repo: repo, canceller: canceller}
}

// Register registers admin routes on the given mux.
// The caller is responsible for wrapping with BasicAuth middleware.
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/admin/users", h.ListUsers)
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", h.UpdateTier)
	mux.HandleFunc("GET /api/v1/admin/subscriptions", h.ListSubscriptions)
	mux.HandleFunc("DELETE /api/v1/admin/subscriptions/{id}", h.CancelSubscription)
}

// ListUsers handles GET /api/v1/admin/users.
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	opts := parseUserListOptions(r)

	users, err := h.repo.ListUsers(r.Context(), opts...)
	if err != nil {
		slog.Error("admin list users", "error", err)
		writeError(w, "internal_error", "Failed to list users", http.StatusInternalServerError)

		return
	}

	count, err := h.repo.CountUsers(r.Context(), opts...)
	if err != nil {
		slog.Error("admin count users", "error", err)
		writeError(w, "internal_error", "Failed to count users", http.StatusInternalServerError)

		return
	}

	q := user.ApplyOptions(opts...)
	resp := model.AdminUsersListResponse{
		Users:   make([]model.AdminUserResponse, 0, len(users)),
		Total:   count,
		Page:    q.Page,
		PerPage: q.PerPage,
	}

	for _, u := range users {
		resp.Users = append(resp.Users, model.AdminUserResponse{
			ID:          u.ID,
			Email:       u.Email,
			Name:        u.Name,
			Provider:    u.Provider,
			Tier:        u.Tier,
			SecretsUsed: u.SecretsUsed,
			CreatedAt:   u.CreatedAt.Format(timeFormat),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpdateTier handles PATCH /api/v1/admin/users/{id}.
func (h *AdminHandler) UpdateTier(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, "validation_error", "Invalid user ID", http.StatusBadRequest)

		return
	}

	var req model.AdminUpdateTierRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)

		return
	}

	if req.Tier != model.TierFree && req.Tier != model.TierPro {
		writeError(w, "validation_error", "Tier must be 'free' or 'pro'", http.StatusBadRequest)

		return
	}

	if err := h.repo.UpdateTier(r.Context(), id, req.Tier); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, "not_found", "User not found", http.StatusNotFound)
		} else {
			slog.Error("admin update tier", "error", err)
			writeError(w, "internal_error", "Failed to update tier", http.StatusInternalServerError)
		}

		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListSubscriptions handles GET /api/v1/admin/subscriptions.
func (h *AdminHandler) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	opts := parseSubscriptionListOptions(r)

	subs, err := h.repo.ListSubscriptions(r.Context(), opts...)
	if err != nil {
		slog.Error("admin list subscriptions", "error", err)
		writeError(w, "internal_error", "Failed to list subscriptions", http.StatusInternalServerError)

		return
	}

	count, err := h.repo.CountSubscriptions(r.Context(), opts...)
	if err != nil {
		slog.Error("admin count subscriptions", "error", err)
		writeError(w, "internal_error", "Failed to count subscriptions", http.StatusInternalServerError)

		return
	}

	q := user.ApplyOptions(opts...)
	resp := model.AdminSubscriptionsListResponse{
		Subscriptions: make([]model.AdminSubscriptionResponse, 0, len(subs)),
		Total:         count,
		Page:          q.Page,
		PerPage:       q.PerPage,
	}

	for _, s := range subs {
		resp.Subscriptions = append(resp.Subscriptions, model.AdminSubscriptionResponse{
			ID:                   s.ID,
			UserID:               s.UserID,
			UserEmail:            s.UserEmail,
			UserName:             s.UserName,
			StripeCustomerID:     s.StripeCustomerID,
			StripeSubscriptionID: s.StripeSubscriptionID,
			Status:               s.Status,
			CurrentPeriodStart:   s.CurrentPeriodStart.Format(timeFormat),
			CurrentPeriodEnd:     s.CurrentPeriodEnd.Format(timeFormat),
			CreatedAt:            s.CreatedAt.Format(timeFormat),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// CancelSubscription handles DELETE /api/v1/admin/subscriptions/{id}.
func (h *AdminHandler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, "validation_error", "Invalid subscription ID", http.StatusBadRequest)

		return
	}

	sub, err := h.repo.FindSubscriptionByUserID(r.Context(), id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			writeError(w, "not_found", "Subscription not found", http.StatusNotFound)
		} else {
			slog.Error("admin find subscription", "error", err)
			writeError(w, "internal_error", "Failed to find subscription", http.StatusInternalServerError)
		}

		return
	}

	if h.canceller != nil {
		if cancelErr := h.canceller.CancelSubscription(r.Context(), sub.StripeSubscriptionID); cancelErr != nil {
			slog.Error("admin cancel stripe subscription", "error", cancelErr)
			writeError(w, "internal_error", "Failed to cancel subscription", http.StatusInternalServerError)

			return
		}
	}

	if err := h.repo.UpdateSubscriptionStatus(r.Context(), sub.StripeSubscriptionID, model.SubscriptionCanceled); err != nil {
		slog.Error("admin update subscription status", "error", err)
		writeError(w, "internal_error", "Failed to update subscription status", http.StatusInternalServerError)

		return
	}

	if err := h.repo.UpdateTier(r.Context(), sub.UserID, model.TierFree); err != nil {
		slog.Error("admin downgrade tier", "error", err)
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseUserListOptions(r *http.Request) []user.ListOption {
	var opts []user.ListOption

	if q := r.URL.Query().Get("q"); q != "" {
		opts = append(opts, user.WithSearch(q))
	}

	if tier := r.URL.Query().Get("tier"); tier != "" {
		opts = append(opts, user.WithTier(tier))
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		order := r.URL.Query().Get("order")
		opts = append(opts, user.WithSort(sort, order))
	}

	page, perPage := parsePagination(r)
	opts = append(opts, user.WithPage(page, perPage))

	return opts
}

func parseSubscriptionListOptions(r *http.Request) []user.ListOption {
	var opts []user.ListOption

	if q := r.URL.Query().Get("q"); q != "" {
		opts = append(opts, user.WithSearch(q))
	}

	if status := r.URL.Query().Get("status"); status != "" {
		opts = append(opts, user.WithStatus(status))
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		order := r.URL.Query().Get("order")
		opts = append(opts, user.WithSort(sort, order))
	}

	page, perPage := parsePagination(r)
	opts = append(opts, user.WithPage(page, perPage))

	return opts
}

func parsePagination(r *http.Request) (page, perPage int) {
	page = 1
	perPage = defaultPerPage

	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	if v, err := strconv.Atoi(r.URL.Query().Get("per_page")); err == nil && v > 0 {
		perPage = v
		if perPage > maxPerPage {
			perPage = maxPerPage
		}
	}

	return page, perPage
}
```

**Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/handler/ -run TestAdmin -v`
Expected: PASS

**Step 5: Run all tests**

Run: `cd backend && go test -race ./...`
Expected: PASS

**Step 6: Run linter**

Run: `cd backend && golangci-lint run ./internal/handler/...`
Expected: No issues

**Step 7: Commit**

```bash
git add backend/internal/handler/admin.go backend/internal/handler/admin_test.go
git commit -m "feat: add admin handler with list, update tier, cancel subscription"
```

---

### Task 7: Protect Docs Endpoint

**Files:**
- Modify: `backend/internal/handler/docs.go`

**Step 1: Modify RegisterDocs to accept optional middleware**

Replace `RegisterDocs` in `backend/internal/handler/docs.go`:

```go
// RegisterDocs registers the API documentation routes on the given mux.
// If protect is not nil, it wraps the handlers with that middleware (e.g. BasicAuth).
func RegisterDocs(mux *http.ServeMux, protect func(http.Handler) http.Handler) {
	spec := http.HandlerFunc(handleOpenAPISpec)
	ui := http.HandlerFunc(handleDocsUI)

	if protect != nil {
		mux.Handle("GET /docs/openapi.yaml", protect(spec))
		mux.Handle("GET /docs", protect(ui))
	} else {
		mux.HandleFunc("GET /docs/openapi.yaml", handleOpenAPISpec)
		mux.HandleFunc("GET /docs", handleDocsUI)
	}
}
```

**Step 2: Update the caller in main.go**

In `backend/cmd/secretdrop/main.go`, find:

```go
handler.RegisterDocs(mux)
```

And change it to (temporarily, will be properly wired in Task 8):

```go
handler.RegisterDocs(mux, nil)
```

**Step 3: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 4: Run all tests**

Run: `cd backend && go test -race ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add backend/internal/handler/docs.go backend/cmd/secretdrop/main.go
git commit -m "feat: add optional protection middleware to docs endpoint"
```

---

### Task 8: Wire Everything in main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go`

**Step 1: Add admin wiring**

In `main.go`, after the delete account route block and before `var chain http.Handler = mux`, add:

```go
// Admin routes (conditional — only when ADMIN_USERNAME and ADMIN_PASSWORD are set)
if cfg.AdminUsername() != "" && cfg.AdminPassword() != "" {
	adminAuth := middleware.BasicAuth(cfg.AdminUsername(), cfg.AdminPassword())
	adminHandler := handler.NewAdminHandler(userRepo, billingSvc)
	adminMux := http.NewServeMux()
	adminHandler.Register(adminMux)

	mux.Handle("/api/v1/admin/", adminAuth(adminMux))

	handler.RegisterDocs(mux, adminAuth)

	slog.Info("admin routes enabled")
} else {
	handler.RegisterDocs(mux, nil)

	slog.Info("admin routes disabled (ADMIN_USERNAME or ADMIN_PASSWORD not set)")
}
```

Also remove the existing `handler.RegisterDocs(mux, nil)` line that was added in Task 7.

**Step 2: Verify it compiles**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 3: Run all tests**

Run: `cd backend && go test -race ./...`
Expected: PASS

**Step 4: Run linter**

Run: `cd backend && golangci-lint run ./...`
Expected: No issues

**Step 5: Commit**

```bash
git add backend/cmd/secretdrop/main.go
git commit -m "feat: wire admin routes and docs protection in main"
```

---

### Task 9: Update Docs & CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update CLAUDE.md**

Add new endpoints to the API Endpoints section:

```
- `GET /api/v1/admin/users` — List users with search/filter/sort/pagination (admin auth)
- `PATCH /api/v1/admin/users/{id}` — Update user tier (admin auth)
- `GET /api/v1/admin/subscriptions` — List subscriptions with filter/sort/pagination (admin auth)
- `DELETE /api/v1/admin/subscriptions/{id}` — Cancel subscription (admin auth)
```

Add new env vars to the Environment Variables table:

```
| `ADMIN_USERNAME` | No | — |
| `ADMIN_PASSWORD` | No | — |
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add admin endpoints and env vars to CLAUDE.md"
```

---

### Task 10: Final Verification

**Step 1: Run full test suite**

Run: `cd backend && go test -race -count=1 ./...`
Expected: PASS

**Step 2: Run linter**

Run: `cd backend && golangci-lint run ./...`
Expected: No issues

**Step 3: Build**

Run: `cd backend && go build ./...`
Expected: SUCCESS

**Step 4: Manual smoke test (optional)**

```bash
cd backend
GOLANG_ENV=development ADMIN_USERNAME=admin ADMIN_PASSWORD=secret go run ./cmd/secretdrop/
```

In another terminal:
```bash
# Without auth — should 401
curl http://localhost:8080/api/v1/admin/users

# With auth — should 200
curl -u admin:secret http://localhost:8080/api/v1/admin/users

# Docs without auth — should 401
curl http://localhost:8080/docs

# Docs with auth — should 200
curl -u admin:secret http://localhost:8080/docs
```
