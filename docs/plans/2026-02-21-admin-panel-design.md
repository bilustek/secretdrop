# Admin Panel & API Docs Protection Design

## Goal

Add Basic Auth-protected admin API endpoints for managing users and subscriptions,
and protect the `/docs` endpoint behind the same authentication.

## Decisions

- **Auth:** Basic Auth via `ADMIN_USERNAME` + `ADMIN_PASSWORD` environment variables
- **UI:** Frontend (React) consumes admin JSON API — no server-side HTML
- **Scope:** Read + limited write (tier change, subscription cancel)
- **Path prefix:** `/api/v1/admin/*`

## Architecture

### Authentication & Middleware

- New `middleware.BasicAuth(username, password)` middleware
- Checks `Authorization: Basic ...` header
- Returns 401 + `WWW-Authenticate: Basic realm="admin"` on failure
- Applied to `/api/v1/admin/*` and `/docs`, `/docs/openapi.yaml`

### Config

- `ADMIN_USERNAME` and `ADMIN_PASSWORD` env vars (optional)
- Both must be set to enable admin routes and docs protection
- If unset: admin routes disabled, docs remain public, info logged

### API Endpoints

**Users:**
- `GET /api/v1/admin/users` — List with search, filter, sort, pagination
  - Query: `?q=`, `?tier=`, `?sort=`, `?order=`, `?page=`, `?per_page=`
  - Response: `{ "users": [...], "total": N, "page": N, "per_page": N }`
- `PATCH /api/v1/admin/users/{id}` — Update tier
  - Body: `{ "tier": "pro" | "free" }`

**Subscriptions:**
- `GET /api/v1/admin/subscriptions` — List with filter, sort, pagination
  - Query: `?status=`, `?sort=`, `?order=`, `?page=`, `?per_page=`
  - Response: `{ "subscriptions": [...], "total": N, "page": N, "per_page": N }`
  - Includes joined user email/name
- `DELETE /api/v1/admin/subscriptions/{id}` — Cancel subscription (Stripe + DB)

**Docs:**
- `GET /docs` and `GET /docs/openapi.yaml` — protected by same Basic Auth

### Repository Layer

New methods on user repository using functional options for query building:

```go
type ListOption func(*listQuery)

func WithSearch(q string) ListOption
func WithTier(tier string) ListOption
func WithSort(field, order string) ListOption
func WithPage(page, perPage int) ListOption
```

Methods:
- `ListUsers(ctx, ...ListOption) ([]User, error)`
- `CountUsers(ctx, ...ListOption) (int64, error)`
- `ListSubscriptions(ctx, ...ListOption) ([]SubscriptionWithUser, error)`
- `CountSubscriptions(ctx, ...ListOption) (int64, error)`

### Handler & Wiring

- `internal/handler/admin.go` — Admin handler with `Register(mux)` method
- `NewAdminHandler(userRepo, canceller, opts...)` constructor
- main.go: if both admin env vars set, create BasicAuth middleware, wrap admin routes and docs

### Testing

- BasicAuth middleware: valid/invalid/missing credentials
- Admin handler: list, search, filter, sort, pagination, tier update, subscription cancel
- Repository: ListUsers, CountUsers, ListSubscriptions, CountSubscriptions
