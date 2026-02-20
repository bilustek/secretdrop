# Auth & Billing Design

## Context

SecretDrop currently operates anonymously with no user accounts. This design
adds Google + GitHub OAuth authentication, a freemium model with Stripe
billing, and usage limits.

## Decisions

| Topic | Decision |
|-------|----------|
| Auth providers | Google + GitHub OAuth |
| Auth implementation | Custom JWT (Go backend) |
| Free tier | Lifetime 1 secret |
| Paid tier (Pro) | $2.99/month, 100 secrets/month |
| Stripe integration | Checkout Session (hosted) |
| Anonymous access | Disabled for secret creation; reveal remains public |
| Email budget | 10K emails/month allocated to SecretDrop (from Resend 50K pool) |
| Architecture | Monolith (extend existing Go backend) |
| Mobile readiness | Token exchange endpoint (`POST /auth/token`) from day one |

## Database Schema

Two new tables added to SQLite:

```sql
CREATE TABLE users (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    provider     TEXT    NOT NULL,  -- "google" | "github"
    provider_id  TEXT    NOT NULL,  -- unique ID from OAuth provider
    email        TEXT    NOT NULL,
    name         TEXT    NOT NULL DEFAULT '',
    avatar_url   TEXT    NOT NULL DEFAULT '',
    tier         TEXT    NOT NULL DEFAULT 'free',  -- "free" | "pro"
    secrets_used INTEGER NOT NULL DEFAULT 0,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, provider_id)
);

CREATE TABLE subscriptions (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id                INTEGER NOT NULL REFERENCES users(id),
    stripe_customer_id     TEXT    NOT NULL,
    stripe_subscription_id TEXT    NOT NULL UNIQUE,
    status                 TEXT    NOT NULL DEFAULT 'active',  -- "active" | "canceled" | "past_due"
    current_period_start   DATETIME NOT NULL,
    current_period_end     DATETIME NOT NULL,
    created_at             DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_provider ON users(provider, provider_id);
CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);
CREATE INDEX idx_subscriptions_stripe_id ON subscriptions(stripe_subscription_id);
```

### Limit mechanism

- **Free:** `secrets_used` is a lifetime counter. Limit reached at 1.
- **Pro:** `secrets_used` resets to 0 monthly via Stripe `invoice.paid` webhook. Limit reached at 100.

## Auth Flow

### Web (redirect-based)

```
GET  /auth/google           -> redirect to Google
GET  /auth/google/callback  -> code exchange -> upsert user -> JWT
GET  /auth/github           -> redirect to GitHub
GET  /auth/github/callback  -> code exchange -> upsert user -> JWT
```

### Mobile (token exchange)

```
POST /auth/token  -> { "provider": "google", "id_token": "xxx" }
                  -> verify with provider -> upsert user -> JWT
```

### JWT structure

```json
{
  "sub": 42,
  "email": "user@example.com",
  "tier": "free",
  "exp": 1708473600
}
```

- Access token: 15-minute expiry
- Refresh token: 30-day expiry, stored in DB, rotated on use
- Web: both tokens as `httpOnly` cookies
- Mobile: both tokens in JSON response body

## Billing Flow (Stripe)

### Endpoints

| Endpoint | Auth | Description |
|----------|------|-------------|
| `POST /billing/checkout` | Yes | Creates Stripe Checkout Session, returns URL |
| `POST /billing/webhook` | No (Stripe signature) | Handles Stripe webhook events |
| `POST /billing/portal` | Yes | Returns Stripe Customer Portal URL |

### Webhook events

| Event | Action |
|-------|--------|
| `checkout.session.completed` | Create subscription row, set `tier=pro` |
| `invoice.paid` | Reset `secrets_used=0` (monthly reset) |
| `customer.subscription.deleted` | Set `tier=free`, subscription `status=canceled` |
| `customer.subscription.updated` | Update subscription `status` |

### Stripe resources

- 1 Product: "SecretDrop Pro"
- 1 Price: $2.99/month recurring

## Limit Enforcement

```
POST /api/v1/secrets (authenticated)
  |
  +- free tier:  secrets_used >= 1   -> 403 limit_reached
  +- pro tier:   secrets_used >= 100 -> 403 limit_reached
  +- pro tier:   subscription past_due/canceled -> apply free limits
```

### Endpoint auth requirements

| Endpoint | Auth required |
|----------|---------------|
| `POST /api/v1/secrets` | Yes |
| `POST /api/v1/secrets/{token}/reveal` | No (recipient may not have account) |
| `GET /api/v1/me` | Yes |
| `GET /healthz` | No |

### New endpoint

`GET /api/v1/me` returns user info and remaining quota:

```json
{
  "email": "user@example.com",
  "tier": "pro",
  "secrets_used": 23,
  "secrets_limit": 100
}
```

## Package Structure

```
backend/internal/
  auth/          # NEW - OAuth flows + JWT generation/validation
    auth.go      # JWT logic, functional options
    google.go    # Google OAuth handlers
    github.go    # GitHub OAuth handlers
    token.go     # POST /auth/token (mobile exchange)
  user/          # NEW - User repository interface
    user.go      # Repository interface + model
    sqlite/      # SQLite implementation
      sqlite.go
  billing/       # NEW - Stripe integration
    billing.go   # Checkout session, portal
    webhook.go   # Stripe webhook handler
  config/        # UPDATE - new env vars
  model/         # UPDATE - User, Subscription models
  service/       # UPDATE - limit enforcement
  handler/       # UPDATE - /me endpoint
  middleware/    # UPDATE - Authenticate middleware
```

## New Environment Variables

| Variable | Required | Default |
|----------|----------|---------|
| `GOOGLE_CLIENT_ID` | Yes | -- |
| `GOOGLE_CLIENT_SECRET` | Yes | -- |
| `GITHUB_CLIENT_ID` | Yes | -- |
| `GITHUB_CLIENT_SECRET` | Yes | -- |
| `JWT_SECRET` | Yes | -- |
| `STRIPE_SECRET_KEY` | Yes (prod only) | -- |
| `STRIPE_WEBHOOK_SECRET` | Yes (prod only) | -- |
| `STRIPE_PRICE_ID` | Yes (prod only) | -- |
