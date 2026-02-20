# Auth & Billing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Google + GitHub OAuth, JWT auth, Stripe billing with freemium model to SecretDrop.

**Architecture:** Extend the existing Go monolith. New packages: `auth/`, `user/`, `billing/`. New middleware for JWT verification. Stripe Checkout (hosted) for payments. SQLite tables for users and subscriptions.

**Tech Stack:** Go 1.26, `golang.org/x/oauth2`, `github.com/golang-jwt/jwt/v5`, `github.com/stripe/stripe-go/v82`, SQLite (modernc.org/sqlite)

**Design doc:** `docs/plans/2026-02-20-auth-billing-design.md`

---

## Phase 1: User Foundation

### Task 1: User model

**Files:**
- Create: `backend/internal/model/user.go`

**Step 1: Create user model**

```go
package model

import "time"

const (
	TierFree = "free"
	TierPro  = "pro"

	FreeTierLimit = 1
	ProTierLimit  = 100
)

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
}

func (u *User) SecretsLimit() int {
	if u.Tier == TierPro {
		return ProTierLimit
	}
	return FreeTierLimit
}

func (u *User) CanCreateSecret() bool {
	return u.SecretsUsed < u.SecretsLimit()
}
```

**Step 2: Add limit_reached error to model/errors.go**

Add to existing sentinel errors:

```go
var ErrLimitReached = errors.New("secret creation limit reached")
```

Add status constant:

```go
StatusForbidden = http.StatusForbidden // 403 (already exists, verify)
```

**Step 3: Add MeResponse to model/response.go**

```go
type MeResponse struct {
	Email        string `json:"email"`
	Name         string `json:"name"`
	AvatarURL    string `json:"avatar_url"`
	Tier         string `json:"tier"`
	SecretsUsed  int    `json:"secrets_used"`
	SecretsLimit int    `json:"secrets_limit"`
}
```

**Step 4: Write tests for User methods**

Create `backend/internal/model/user_test.go`:

```go
func TestUser_SecretsLimit(t *testing.T) {
	tests := []struct {name string; tier string; want int}{
		{"free tier", model.TierFree, model.FreeTierLimit},
		{"pro tier", model.TierPro, model.ProTierLimit},
	}
	// table-driven test
}

func TestUser_CanCreateSecret(t *testing.T) {
	tests := []struct {name string; tier string; used int; want bool}{
		{"free unused", model.TierFree, 0, true},
		{"free exhausted", model.TierFree, 1, false},
		{"pro under limit", model.TierPro, 50, true},
		{"pro at limit", model.TierPro, 100, false},
	}
	// table-driven test
}
```

**Step 5: Run tests**

Run: `cd backend && go test -race ./internal/model/...`
Expected: PASS

**Step 6: Lint + commit**

```bash
cd backend && golangci-lint run ./...
git add backend/internal/model/
git commit -m "add user model with tier limits"
```

---

### Task 2: User repository interface

**Files:**
- Create: `backend/internal/user/user.go`

**Step 1: Define interface**

```go
package user

import (
	"context"
	"github.com/bilusteknoloji/secretdrop/internal/model"
)

type Repository interface {
	Upsert(ctx context.Context, u *model.User) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	FindByProvider(ctx context.Context, provider, providerID string) (*model.User, error)
	IncrementSecretsUsed(ctx context.Context, id int64) error
	ResetSecretsUsed(ctx context.Context, id int64) error
	UpdateTier(ctx context.Context, id int64, tier string) error
}
```

**Step 2: Commit**

```bash
git add backend/internal/user/user.go
git commit -m "add user repository interface"
```

---

### Task 3: User SQLite implementation

**Files:**
- Create: `backend/internal/user/sqlite/sqlite.go`
- Create: `backend/internal/user/sqlite/sqlite_test.go`

**Step 1: Write failing tests**

Test all 6 interface methods:
- `TestUpsert_NewUser` — insert, verify fields
- `TestUpsert_ExistingUser` — insert same provider+providerID, verify update (name, avatar, email)
- `TestFindByID` — insert then find
- `TestFindByID_NotFound` — find non-existent, expect `model.ErrNotFound`
- `TestFindByProvider` — insert then find by provider+providerID
- `TestFindByProvider_NotFound` — expect `model.ErrNotFound`
- `TestIncrementSecretsUsed` — insert, increment, verify count
- `TestResetSecretsUsed` — insert, increment, reset, verify 0
- `TestUpdateTier` — insert (free), update to pro, verify

Pattern: each test opens `sqlite.New(":memory:")`, defers `Close()`.

**Step 2: Run tests to verify they fail**

Run: `cd backend && go test -race ./internal/user/sqlite/...`
Expected: FAIL (compilation errors)

**Step 3: Implement SQLite repository**

```go
package sqlite

import (
	"database/sql"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

var _ user.Repository = (*Repository)(nil)

const migration = `
CREATE TABLE IF NOT EXISTS users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    provider     TEXT    NOT NULL,
    provider_id  TEXT    NOT NULL,
    email        TEXT    NOT NULL,
    name         TEXT    NOT NULL DEFAULT '',
    avatar_url   TEXT    NOT NULL DEFAULT '',
    tier         TEXT    NOT NULL DEFAULT 'free',
    secrets_used INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_id)
);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);
`

type Repository struct {
    db *sql.DB
}

func New(dsn string) (*Repository, error) {
    db, err := sql.Open("sqlite", dsn)
    // same pattern as repository/sqlite: SetMaxOpenConns(1), exec migration
    // ...
    return &Repository{db: db}, nil
}

func (r *Repository) Close() error { return r.db.Close() }
```

Implement all 6 methods following existing sqlite patterns:
- `Upsert`: `INSERT ... ON CONFLICT(provider, provider_id) DO UPDATE SET ...`
- `FindByID`: `SELECT ... WHERE id = ?` — return `model.ErrNotFound` if no rows
- `FindByProvider`: `SELECT ... WHERE provider = ? AND provider_id = ?`
- `IncrementSecretsUsed`: `UPDATE users SET secrets_used = secrets_used + 1, updated_at = ... WHERE id = ?`
- `ResetSecretsUsed`: `UPDATE users SET secrets_used = 0, updated_at = ... WHERE id = ?`
- `UpdateTier`: `UPDATE users SET tier = ?, updated_at = ... WHERE id = ?`

**Step 4: Run tests**

Run: `cd backend && go test -race ./internal/user/sqlite/...`
Expected: PASS

**Step 5: Lint + commit**

```bash
cd backend && golangci-lint run ./...
git add backend/internal/user/
git commit -m "add user SQLite repository implementation"
```

---

### Task 4: Subscription model + repository

**Files:**
- Create: `backend/internal/model/subscription.go`
- Modify: `backend/internal/user/user.go` — add subscription methods to interface
- Modify: `backend/internal/user/sqlite/sqlite.go` — add migration + implementation
- Modify: `backend/internal/user/sqlite/sqlite_test.go` — add tests

**Step 1: Create subscription model**

```go
package model

import "time"

const (
	SubscriptionActive   = "active"
	SubscriptionCanceled = "canceled"
	SubscriptionPastDue  = "past_due"
)

type Subscription struct {
	ID                   int64
	UserID               int64
	StripeCustomerID     string
	StripeSubscriptionID string
	Status               string
	CurrentPeriodStart   time.Time
	CurrentPeriodEnd     time.Time
	CreatedAt            time.Time
}
```

**Step 2: Add subscription methods to user.Repository**

```go
type Repository interface {
	// ... existing user methods ...
	UpsertSubscription(ctx context.Context, sub *model.Subscription) error
	FindSubscriptionByUserID(ctx context.Context, userID int64) (*model.Subscription, error)
	FindUserByStripeCustomerID(ctx context.Context, customerID string) (*model.User, error)
	UpdateSubscriptionStatus(ctx context.Context, stripeSubID, status string) error
}
```

**Step 3: Add subscriptions migration to sqlite.go**

Append to migration const:

```sql
CREATE TABLE IF NOT EXISTS subscriptions (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                INTEGER NOT NULL REFERENCES users(id),
    stripe_customer_id     TEXT    NOT NULL,
    stripe_subscription_id TEXT    NOT NULL UNIQUE,
    status                 TEXT    NOT NULL DEFAULT 'active',
    current_period_start   DATETIME NOT NULL,
    current_period_end     DATETIME NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_id ON subscriptions(stripe_subscription_id);
```

**Step 4: Write tests for subscription methods**

- `TestUpsertSubscription` — create user, upsert subscription, verify
- `TestFindSubscriptionByUserID` — create + find
- `TestFindSubscriptionByUserID_NotFound` — expect `model.ErrNotFound`
- `TestFindUserByStripeCustomerID` — create user + subscription, find user
- `TestUpdateSubscriptionStatus` — create, update to canceled, verify

**Step 5: Implement subscription methods**

**Step 6: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/user/sqlite/... ./internal/model/...
cd backend && golangci-lint run ./...
git add backend/internal/model/subscription.go backend/internal/user/
git commit -m "add subscription model and repository methods"
```

---

## Phase 2: Config Updates

### Task 5: Add new environment variables to config

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

**Step 1: Add fields to Config struct**

```go
type Config struct {
	// ... existing fields ...
	googleClientID     string
	googleClientSecret string
	githubClientID     string
	githubClientSecret string
	jwtSecret          string
	stripeSecretKey    string
	stripeWebhookSecret string
	stripePriceID      string
}
```

**Step 2: Add getters**

```go
func (c *Config) GoogleClientID() string     { return c.googleClientID }
func (c *Config) GoogleClientSecret() string { return c.googleClientSecret }
func (c *Config) GithubClientID() string     { return c.githubClientID }
func (c *Config) GithubClientSecret() string { return c.githubClientSecret }
func (c *Config) JWTSecret() string          { return c.jwtSecret }
func (c *Config) StripeSecretKey() string    { return c.stripeSecretKey }
func (c *Config) StripeWebhookSecret() string { return c.stripeWebhookSecret }
func (c *Config) StripePriceID() string      { return c.stripePriceID }
```

**Step 3: Load from env in Load()**

```go
c.googleClientID = os.Getenv("GOOGLE_CLIENT_ID")
c.googleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
c.githubClientID = os.Getenv("GITHUB_CLIENT_ID")
c.githubClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
c.jwtSecret = os.Getenv("JWT_SECRET")
c.stripeSecretKey = os.Getenv("STRIPE_SECRET_KEY")
c.stripeWebhookSecret = os.Getenv("STRIPE_WEBHOOK_SECRET")
c.stripePriceID = os.Getenv("STRIPE_PRICE_ID")
```

**Step 4: Add production validation**

After existing `RESEND_API_KEY` check:

```go
if !c.IsDev() {
	for _, kv := range []struct{ name, val string }{
		{"GOOGLE_CLIENT_ID", c.googleClientID},
		{"GOOGLE_CLIENT_SECRET", c.googleClientSecret},
		{"GITHUB_CLIENT_ID", c.githubClientID},
		{"GITHUB_CLIENT_SECRET", c.githubClientSecret},
		{"JWT_SECRET", c.jwtSecret},
		{"STRIPE_SECRET_KEY", c.stripeSecretKey},
		{"STRIPE_WEBHOOK_SECRET", c.stripeWebhookSecret},
		{"STRIPE_PRICE_ID", c.stripePriceID},
	} {
		if kv.val == "" {
			return nil, fmt.Errorf("%s environment variable is required", kv.name)
		}
	}
}
```

**Step 5: Add functional option WithJWTSecret for testing**

```go
func WithJWTSecret(secret string) Option {
	return func(c *Config) error {
		if secret == "" {
			return errors.New("jwt secret cannot be empty")
		}
		c.jwtSecret = secret
		return nil
	}
}
```

Add similar `With*` options for other new fields as needed.

**Step 6: Update tests**

- Test Load() with new env vars set (use `t.Setenv()`)
- Test production validation rejects empty required vars
- Test development mode skips validation
- Test getter methods

**Step 7: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/config/...
cd backend && golangci-lint run ./...
git add backend/internal/config/
git commit -m "add auth and billing environment variables to config"
```

---

## Phase 3: Auth — JWT

### Task 6: JWT token generation and verification

**Files:**
- Create: `backend/internal/auth/auth.go`
- Create: `backend/internal/auth/auth_test.go`

**Step 1: Install dependency**

```bash
cd backend && go get github.com/golang-jwt/jwt/v5
```

**Step 2: Write failing tests**

```go
func TestGenerateAccessToken(t *testing.T)       // returns signed JWT, verify claims
func TestGenerateRefreshToken(t *testing.T)      // returns signed JWT with longer exp
func TestVerifyToken_Valid(t *testing.T)          // parse and return claims
func TestVerifyToken_Expired(t *testing.T)        // reject expired token
func TestVerifyToken_InvalidSignature(t *testing.T) // reject tampered token
```

**Step 3: Implement auth package**

```go
package auth

import (
	"fmt"
	"time"
	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultAccessExpiry  = 15 * time.Minute
	defaultRefreshExpiry = 30 * 24 * time.Hour
)

type Option func(*Service) error

type Claims struct {
	UserID int64  `json:"sub"`
	Email  string `json:"email"`
	Tier   string `json:"tier"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Service struct {
	secret       []byte
	accessExpiry time.Duration
	refreshExpiry time.Duration
}

func New(secret string, opts ...Option) (*Service, error) {
	if secret == "" {
		return nil, errors.New("jwt secret cannot be empty")
	}
	s := &Service{
		secret:        []byte(secret),
		accessExpiry:  defaultAccessExpiry,
		refreshExpiry: defaultRefreshExpiry,
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}
	return s, nil
}

func (s *Service) GenerateTokenPair(userID int64, email, tier string) (*TokenPair, error)
func (s *Service) VerifyToken(tokenStr string) (*Claims, error)
```

**Step 4: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/auth/...
cd backend && golangci-lint run ./...
git add backend/internal/auth/ backend/go.mod backend/go.sum
git commit -m "add JWT token generation and verification"
```

---

### Task 7: Auth middleware

**Files:**
- Create: `backend/internal/middleware/auth.go`
- Create: `backend/internal/middleware/auth_test.go`

**Step 1: Write failing tests**

```go
func TestAuthenticate_ValidToken(t *testing.T)    // sets user in context, calls next
func TestAuthenticate_MissingHeader(t *testing.T) // 401 unauthorized
func TestAuthenticate_InvalidToken(t *testing.T)  // 401 unauthorized
func TestAuthenticate_ExpiredToken(t *testing.T)  // 401 unauthorized
func TestUserFromContext(t *testing.T)            // retrieve user claims from context
func TestUserFromContext_Missing(t *testing.T)    // returns nil, false
```

**Step 2: Implement**

```go
package middleware

import (
	"context"
	"net/http"
	"strings"
	"github.com/bilusteknoloji/secretdrop/internal/auth"
)

type contextKey string

const userContextKey contextKey = "user"

func Authenticate(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeAuthError(w, "Authorization header required")
				return
			}
			token := strings.TrimPrefix(header, "Bearer ")
			if token == header {
				writeAuthError(w, "Bearer token required")
				return
			}
			claims, err := authSvc.VerifyToken(token)
			if err != nil {
				writeAuthError(w, "Invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), userContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (*auth.Claims, bool) {
	claims, ok := ctx.Value(userContextKey).(*auth.Claims)
	return claims, ok
}

func writeAuthError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":{"type":"unauthorized","message":"%s"}}`, msg)
}
```

**Step 3: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/middleware/...
cd backend && golangci-lint run ./...
git add backend/internal/middleware/auth.go backend/internal/middleware/auth_test.go
git commit -m "add JWT authentication middleware"
```

---

### Task 8: Google OAuth handler

**Files:**
- Create: `backend/internal/auth/google.go`
- Create: `backend/internal/auth/google_test.go`

**Step 1: Install dependency**

```bash
cd backend && go get golang.org/x/oauth2
```

**Step 2: Implement Google OAuth handler**

Two handlers: `HandleGoogleLogin` (redirect) and `HandleGoogleCallback` (code exchange).

```go
package auth

import (
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

func (s *Service) GoogleConfig(clientID, clientSecret, callbackURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (s *Service) HandleGoogleLogin(cfg *oauth2.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := generateState() // random string, store in cookie
		http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, ...})
		http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
	}
}

func (s *Service) HandleGoogleCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. verify state from cookie
		// 2. exchange code for token: cfg.Exchange(ctx, code)
		// 3. fetch user info from https://www.googleapis.com/oauth2/v2/userinfo
		// 4. upsert user: userRepo.Upsert(ctx, &model.User{Provider: "google", ...})
		// 5. generate JWT pair: s.GenerateTokenPair(user.ID, user.Email, user.Tier)
		// 6. set cookies (web) or return JSON
	}
}
```

**Step 3: Write tests**

Test with httptest server mocking Google's token + userinfo endpoints.
- `TestHandleGoogleLogin_RedirectsToGoogle`
- `TestHandleGoogleCallback_Success`
- `TestHandleGoogleCallback_InvalidState`

**Step 4: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/auth/...
cd backend && golangci-lint run ./...
git add backend/internal/auth/google.go backend/internal/auth/google_test.go backend/go.mod backend/go.sum
git commit -m "add Google OAuth login and callback handlers"
```

---

### Task 9: GitHub OAuth handler

**Files:**
- Create: `backend/internal/auth/github.go`
- Create: `backend/internal/auth/github_test.go`

**Step 1: Implement GitHub OAuth handler**

Same pattern as Google but with GitHub endpoints:
- Auth endpoint: `https://github.com/login/oauth/authorize`
- Token endpoint: `https://github.com/login/oauth/access_token`
- User info: `https://api.github.com/user` (with Authorization header)
- Email: `https://api.github.com/user/emails` (primary verified email)

```go
func (s *Service) GithubConfig(clientID, clientSecret, callbackURL string) *oauth2.Config
func (s *Service) HandleGithubLogin(cfg *oauth2.Config) http.HandlerFunc
func (s *Service) HandleGithubCallback(cfg *oauth2.Config, userRepo user.Repository) http.HandlerFunc
```

**Step 2: Write tests**

Same pattern as Google tests with mock GitHub endpoints.

**Step 3: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/auth/...
cd backend && golangci-lint run ./...
git add backend/internal/auth/github.go backend/internal/auth/github_test.go
git commit -m "add GitHub OAuth login and callback handlers"
```

---

### Task 10: Token exchange endpoint (mobile)

**Files:**
- Create: `backend/internal/auth/token.go`
- Create: `backend/internal/auth/token_test.go`

**Step 1: Implement token exchange**

```go
// POST /auth/token
// Body: {"provider": "google", "id_token": "xxx"}

type TokenExchangeRequest struct {
	Provider string `json:"provider"`
	IDToken  string `json:"id_token"`
}

func (s *Service) HandleTokenExchange(userRepo user.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. decode request
		// 2. switch provider:
		//    "google" -> verify id_token with Google's tokeninfo endpoint
		//    "github" -> use token as access_token, fetch user info from GitHub API
		// 3. upsert user
		// 4. generate JWT pair
		// 5. return JSON: {"access_token": "...", "refresh_token": "..."}
	}
}
```

**Step 2: Write tests**

- `TestHandleTokenExchange_Google_Success`
- `TestHandleTokenExchange_GitHub_Success`
- `TestHandleTokenExchange_InvalidProvider`
- `TestHandleTokenExchange_InvalidToken`

**Step 3: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/auth/...
cd backend && golangci-lint run ./...
git add backend/internal/auth/token.go backend/internal/auth/token_test.go
git commit -m "add token exchange endpoint for mobile auth"
```

---

## Phase 4: Billing — Stripe

### Task 11: Stripe billing package

**Files:**
- Create: `backend/internal/billing/billing.go`
- Create: `backend/internal/billing/billing_test.go`

**Step 1: Install dependency**

```bash
cd backend && go get github.com/stripe/stripe-go/v82
```

Check DeepWiki for latest stripe-go version before installing.

**Step 2: Implement billing service**

```go
package billing

import (
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

type Option func(*Service) error

type Service struct {
	secretKey      string
	webhookSecret  string
	priceID        string
	userRepo       user.Repository
	successURL     string
	cancelURL      string
}

func New(secretKey, webhookSecret, priceID string, userRepo user.Repository, opts ...Option) (*Service, error)

// POST /billing/checkout — creates Stripe Checkout Session
func (s *Service) HandleCheckout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. get user from context (middleware.UserFromContext)
		// 2. find or create Stripe customer
		// 3. create checkout session with priceID, customer, success/cancel URLs
		// 4. return JSON: {"url": session.URL}
	}
}

// POST /billing/portal — returns Stripe Customer Portal URL
func (s *Service) HandlePortal() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. get user from context
		// 2. find subscription, get stripe_customer_id
		// 3. create portal session
		// 4. return JSON: {"url": portalSession.URL}
	}
}
```

**Step 3: Write tests**

Use Stripe test mode / mock HTTP client.
- `TestHandleCheckout_CreatesSession`
- `TestHandleCheckout_Unauthenticated` — 401
- `TestHandlePortal_NoSubscription` — 404

**Step 4: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/billing/...
cd backend && golangci-lint run ./...
git add backend/internal/billing/billing.go backend/internal/billing/billing_test.go backend/go.mod backend/go.sum
git commit -m "add Stripe checkout and portal handlers"
```

---

### Task 12: Stripe webhook handler

**Files:**
- Create: `backend/internal/billing/webhook.go`
- Create: `backend/internal/billing/webhook_test.go`

**Step 1: Implement webhook handler**

```go
// POST /billing/webhook — Stripe webhook (no auth, signature verification)
func (s *Service) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. read body (max 65536 bytes)
		// 2. verify signature: webhook.ConstructEvent(body, sig, webhookSecret)
		// 3. switch event.Type:
		//    "checkout.session.completed":
		//        extract customer ID, subscription ID, user email
		//        find user by email or stripe customer ID
		//        upsert subscription row
		//        update user tier to "pro"
		//    "invoice.paid":
		//        find user by stripe customer ID
		//        reset secrets_used to 0
		//    "customer.subscription.deleted":
		//        update subscription status to "canceled"
		//        update user tier to "free"
		//    "customer.subscription.updated":
		//        update subscription status (past_due, active, etc.)
		//        if past_due -> apply free limits
		// 4. return 200 OK
	}
}
```

**Step 2: Write tests**

Build valid Stripe webhook payloads with test signing key.
- `TestWebhook_CheckoutCompleted` — user tier becomes pro
- `TestWebhook_InvoicePaid` — secrets_used resets to 0
- `TestWebhook_SubscriptionDeleted` — user tier becomes free
- `TestWebhook_SubscriptionUpdated_PastDue`
- `TestWebhook_InvalidSignature` — 400

**Step 3: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/billing/...
cd backend && golangci-lint run ./...
git add backend/internal/billing/webhook.go backend/internal/billing/webhook_test.go
git commit -m "add Stripe webhook handler for subscription lifecycle"
```

---

## Phase 5: Limit Enforcement

### Task 13: Add limit check to secret service

**Files:**
- Modify: `backend/internal/service/secret.go`
- Modify: `backend/internal/service/secret_test.go`

**Step 1: Add user.Repository dependency to SecretService**

```go
type SecretService struct {
	repo     repository.Repository
	sender   email.Sender
	userRepo user.Repository  // NEW
	expiry   time.Duration
	// ...
}

func WithUserRepo(r user.Repository) Option {
	return func(s *SecretService) error {
		s.userRepo = r
		return nil
	}
}
```

**Step 2: Add userID parameter to Create method**

Change signature:
```go
func (s *SecretService) Create(ctx context.Context, userID int64, req *model.CreateRequest) (*model.CreateResponse, error)
```

Add at the beginning of Create:
```go
u, err := s.userRepo.FindByID(ctx, userID)
if err != nil {
    return nil, &model.AppError{Type: "internal_error", Message: "Failed to verify user", StatusCode: 500}
}
if !u.CanCreateSecret() {
    return nil, &model.AppError{Type: "limit_reached", Message: "Secret creation limit reached", StatusCode: http.StatusForbidden}
}
// ... existing logic ...
// after successful store + email, increment usage:
if err := s.userRepo.IncrementSecretsUsed(ctx, userID); err != nil {
    slog.Error("increment secrets used", "error", err)
}
```

**Step 3: Update tests**

- Add `TestCreate_FreeTierLimitReached` — user with 1 secret used, expect 403
- Add `TestCreate_ProTierLimitReached` — user with 100 secrets used, expect 403
- Add `TestCreate_IncrementsUsage` — verify secrets_used increases after create
- Update existing Create tests to pass userID and provide user.Repository mock/noop

**Step 4: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/service/...
cd backend && golangci-lint run ./...
git add backend/internal/service/
git commit -m "add usage limit enforcement to secret creation"
```

---

### Task 14: Update handler to pass user context

**Files:**
- Modify: `backend/internal/handler/secret.go`
- Modify: `backend/internal/handler/secret_test.go`

**Step 1: Update Create handler to extract user from context**

```go
func (h *SecretHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}

	var req model.CreateRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, "validation_error", "Invalid JSON body", http.StatusBadRequest)
		return
	}

	resp, err := h.svc.Create(r.Context(), claims.UserID, &req)
	// ... rest same ...
}
```

**Step 2: Add /me endpoint**

```go
func (h *SecretHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
		return
	}
	// fetch full user from DB, return MeResponse
}
```

Update Register:
```go
func (h *SecretHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/secrets", h.Create)
	mux.HandleFunc("POST /api/v1/secrets/{token}/reveal", h.Reveal)
	mux.HandleFunc("GET /api/v1/me", h.Me)
	mux.HandleFunc("GET /healthz", handleHealthz)
}
```

Note: `Reveal` remains unchanged — no auth required.

**Step 3: Update tests + write new tests**

- `TestMe_ReturnsUserInfo`
- `TestMe_Unauthenticated` — 401
- `TestCreate_Unauthenticated` — 401
- Update existing Create tests to set user in context

**Step 4: Run tests + lint + commit**

```bash
cd backend && go test -race ./internal/handler/...
cd backend && golangci-lint run ./...
git add backend/internal/handler/
git commit -m "add /me endpoint and auth requirement to secret creation"
```

---

## Phase 6: Integration

### Task 15: Wire everything in main.go

**Files:**
- Modify: `backend/cmd/secretdrop/main.go`

**Step 1: Update Run() to create new dependencies**

```go
func Run() error {
	// ... existing config, repo, sender, svc setup ...

	// User repository
	userRepo, err := usersqlite.New(cfg.DatabaseURL())
	if err != nil {
		return fmt.Errorf("open user database: %w", err)
	}
	defer func() { ... }()

	// Auth service
	authSvc, err := auth.New(cfg.JWTSecret())
	if err != nil {
		return fmt.Errorf("create auth service: %w", err)
	}

	// OAuth configs
	googleCfg := authSvc.GoogleConfig(cfg.GoogleClientID(), cfg.GoogleClientSecret(), cfg.BaseURL()+"/auth/google/callback")
	githubCfg := authSvc.GithubConfig(cfg.GithubClientID(), cfg.GithubClientSecret(), cfg.BaseURL()+"/auth/github/callback")

	// Update service with user repo
	svc, err := service.New(repo, sender,
		service.WithBaseURL(cfg.BaseURL()),
		service.WithFromEmail(cfg.FromEmail()),
		service.WithExpiry(cfg.SecretExpiry()),
		service.WithUserRepo(userRepo),
	)

	// Register routes
	mux := http.NewServeMux()
	h.Register(mux)  // existing: POST /api/v1/secrets, reveal, healthz, /me

	// Auth routes (no auth middleware)
	mux.HandleFunc("GET /auth/google", authSvc.HandleGoogleLogin(googleCfg))
	mux.HandleFunc("GET /auth/google/callback", authSvc.HandleGoogleCallback(googleCfg, userRepo))
	mux.HandleFunc("GET /auth/github", authSvc.HandleGithubLogin(githubCfg))
	mux.HandleFunc("GET /auth/github/callback", authSvc.HandleGithubCallback(githubCfg, userRepo))
	mux.HandleFunc("POST /auth/token", authSvc.HandleTokenExchange(userRepo))

	// Billing (needs auth except webhook)
	if !cfg.IsDev() {
		billingSvc, err := billing.New(cfg.StripeSecretKey(), cfg.StripeWebhookSecret(), cfg.StripePriceID(), userRepo)
		if err != nil { return ... }
		mux.HandleFunc("POST /billing/webhook", billingSvc.HandleWebhook())
		// checkout + portal need auth — registered below with auth middleware
	}

	// Docs
	handler.SetOpenAPISpec(docs.OpenAPISpec)
	handler.RegisterDocs(mux)

	// Middleware chain
	authMw := middleware.Authenticate(authSvc)

	// Routes that need auth get wrapped individually:
	// POST /api/v1/secrets, GET /api/v1/me, POST /billing/checkout, POST /billing/portal
	// Routes without auth: reveal, healthz, /docs, /auth/*, /billing/webhook

	// ... rest of server setup unchanged ...
}
```

Note: The middleware chain architecture needs careful routing — auth middleware
should only apply to routes that require it. Two approaches:
- (A) Separate muxes for auth/no-auth routes
- (B) Auth middleware checks per-route in handler

Recommended: (A) Use two handler groups, compose into main mux.

**Step 2: Build + smoke test**

```bash
cd backend && go build ./cmd/secretdrop/
cd backend && GOLANG_ENV=development go run ./cmd/secretdrop/  # ctrl-c after startup
```

**Step 3: Lint + commit**

```bash
cd backend && golangci-lint run ./...
git add backend/cmd/secretdrop/main.go
git commit -m "wire auth, billing and user repository in main"
```

---

### Task 16: Update OpenAPI spec

**Files:**
- Modify: `backend/docs/openapi.yaml`

Add new endpoints to OpenAPI spec:
- `GET /auth/google` — redirect to Google
- `GET /auth/github` — redirect to GitHub
- `POST /auth/token` — token exchange
- `GET /api/v1/me` — user info
- `POST /billing/checkout` — create checkout session
- `POST /billing/portal` — customer portal
- `POST /billing/webhook` — Stripe webhook

Add Bearer auth security scheme.

**Step 1: Update spec**
**Step 2: Verify embed still works**

```bash
cd backend && go build ./cmd/secretdrop/
```

**Step 3: Commit**

```bash
git add backend/docs/openapi.yaml
git commit -m "add auth, billing and me endpoints to OpenAPI spec"
```

---

### Task 17: Update documentation

**Files:**
- Modify: `CLAUDE.md` — update project structure, env vars, endpoints
- Modify: `README.md` — update project structure, env vars, endpoints, quick start

**Step 1: Update both files**

Add new packages to project structure trees.
Add new environment variables to tables.
Add new endpoints to API endpoint tables.
Update quick start with OAuth setup instructions.

**Step 2: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "update documentation for auth and billing features"
```

---

## Verification

After all tasks complete:

```bash
cd backend
go build ./cmd/secretdrop/
go test -race ./...
golangci-lint run ./...
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
```

Target: all tests pass, no lint errors, coverage stays above 70%.

## Task Summary

| # | Task | Phase |
|---|------|-------|
| 1 | User model | Foundation |
| 2 | User repository interface | Foundation |
| 3 | User SQLite implementation | Foundation |
| 4 | Subscription model + repository | Foundation |
| 5 | Config env vars | Config |
| 6 | JWT generation/verification | Auth |
| 7 | Auth middleware | Auth |
| 8 | Google OAuth handler | Auth |
| 9 | GitHub OAuth handler | Auth |
| 10 | Token exchange (mobile) | Auth |
| 11 | Stripe checkout + portal | Billing |
| 12 | Stripe webhook handler | Billing |
| 13 | Limit check in service | Limits |
| 14 | Handler updates + /me | Limits |
| 15 | Wire in main.go | Integration |
| 16 | Update OpenAPI spec | Docs |
| 17 | Update documentation | Docs |
