![Version](https://img.shields.io/badge/version-0.3.11-orange.svg)
![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg?logo=go&logoColor=white)
[![Run golangci-lint](https://github.com/bilusteknoloji/secretdrop/actions/workflows/lint.yml/badge.svg)](https://github.com/bilusteknoloji/secretdrop/actions/workflows/lint.yml)
[![Run go tests](https://github.com/bilusteknoloji/secretdrop/actions/workflows/test.yml/badge.svg)](https://github.com/bilusteknoloji/secretdrop/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/bilusteknoloji/secretdrop/graph/badge.svg?token=96Z01WRM6E)](https://codecov.io/gh/bilusteknoloji/secretdrop)

# SecretDrop

Secure, one-time secret sharing API for developers.

Instead of pasting API keys in Slack, share them through SecretDrop: text is
encrypted with AES-256-GCM, a one-time link is sent to the recipient via
email. Once the link is opened or expires, the record is **permanently
deleted** from the database — no trace left.

---

## Requirements

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.26+ | [go.dev/dl](https://go.dev/dl/) |
| golangci-lint | v2 | `brew install golangci-lint` |
| pre-commit | any | `brew install pre-commit` |
| Resend account | — | [resend.com](https://resend.com) → grab an API key |
| Google OAuth app | — | [console.cloud.google.com](https://console.cloud.google.com) |
| GitHub OAuth app | — | [github.com/settings/developers](https://github.com/settings/developers) |
| Apple Developer account | — | [developer.apple.com](https://developer.apple.com) → Sign in with Apple |
| Stripe account | — | [stripe.com](https://stripe.com) → grab API keys |
| Sentry account | — (optional) | [sentry.io](https://sentry.io) → create a Go project |

## Quick Start

```bash
# Clone the repo
git clone https://github.com/bilusteknoloji/secretdrop.git
cd secretdrop

# Install pre-commit hooks
pre-commit install

# Run the backend
cd backend
RESEND_API_KEY=re_xxxxx go run ./cmd/secretdrop/
```

The server starts at `http://localhost:8080`.

## Environment Variables

Create a `.env` file and `source` it, or export directly:

```bash
# backend/.env (git-ignored)
export RESEND_API_KEY=re_xxxxx          # (required) Resend API key
export PORT=8080                         # server port
export DATABASE_URL="file:db/secretdrop.db?_journal_mode=WAL"  # SQLite path
export API_BASE_URL=http://localhost:8080      # backend URL (OAuth callbacks)
export FRONTEND_BASE_URL=http://localhost:3000 # frontend URL (secret links, billing redirects)
export FROM_EMAIL="SecretDrop <hello@secretdrop.us>"       # sender address
export REPLY_TO_EMAIL="support@bilustek.com"               # reply-to address
export SECRET_EXPIRY=10m                 # secret TTL
export CLEANUP_INTERVAL=1m              # expired record cleanup frequency

# Auth (required in production)
export GOOGLE_CLIENT_ID=xxx
export GOOGLE_CLIENT_SECRET=xxx
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export JWT_SECRET=change-me-to-a-strong-random-string

# Billing (required in production)
export STRIPE_SECRET_KEY=sk_xxx
export STRIPE_WEBHOOK_SECRET=whsec_xxx
export STRIPE_PRICE_ID=price_xxx

export SLACK_WEBHOOK_SUBSCRIPTIONS="https://hooks.slack.com/services/xxx"
export SLACK_WEBHOOK_NOTIFICATIONS="https://hooks.slack.com/services/xxx"

# Admin (optional — enables admin panel and protects /docs)
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD=change-me-to-a-strong-password

# Apple Sign-In (optional — enables Sign in with Apple)
export APPLE_CLIENT_ID=com.bilustek.secretdrop.web
export APPLE_TEAM_ID=XXXXXXXXXX
export APPLE_KEY_ID=XXXXXXXXXX
export APPLE_PRIVATE_KEY=base64-encoded-p8-private-key

# Sentry (optional — error tracking and performance monitoring)
export SENTRY_DSN=https://xxx@xxx.ingest.sentry.io/xxx
export SENTRY_TRACES_SAMPLE_RATE=1.0

# Stripe project filtering (optional — multi-project Stripe account)
export STRIPE_PROJECT_METAKEY=project
export STRIPE_PROJECT_METADATA=secretdrop

# Frontend (build-time, passed as Docker build arg)
# VITE_API_BASE_URL=https://api.secretdrop.us  # empty = same origin
# VITE_ENABLE_APPLE_SIGNIN=false               # set "false" to hide Apple button
```

## API Documentation

https://api.secretdrop.us/docs

Interactive API docs (Scalar UI) are available at [`/docs`](https://api.secretdrop.us/docs)
(protected by Basic Auth when `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set).

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/secrets` | Bearer | Create an encrypted secret |
| `POST` | `/api/v1/secrets/{token}/reveal` | No | Reveal and burn a secret |
| `GET`  | `/api/v1/me` | Bearer | Authenticated user profile |
| `GET`  | `/auth/google` | No | Google OAuth login redirect |
| `GET`  | `/auth/google/callback` | No | Google OAuth callback |
| `GET`  | `/auth/github` | No | GitHub OAuth login redirect |
| `GET`  | `/auth/github/callback` | No | GitHub OAuth callback |
| `GET`  | `/auth/apple` | No | Apple OAuth login redirect |
| `POST` | `/auth/apple/callback` | No | Apple OAuth callback (form POST) |
| `POST` | `/auth/token` | No | Mobile token exchange |
| `POST` | `/auth/refresh` | No | Refresh access token (rotated pair) |
| `POST` | `/billing/checkout` | Bearer | Create Stripe checkout session |
| `POST` | `/billing/portal` | Bearer | Stripe customer portal |
| `POST` | `/billing/webhook` | No | Stripe webhook handler |
| `GET`  | `/healthz` | No | Health check |
| `DELETE` | `/api/v1/me` | Bearer | Delete user account |
| `POST` | `/api/v1/contact` | No | Send contact form message |
| `GET`  | `/api/v1/admin/users` | Basic | List users (search/filter/sort/pagination) |
| `PATCH` | `/api/v1/admin/users/{id}` | Basic | Update user tier or secrets limit override |
| `GET`  | `/api/v1/admin/subscriptions` | Basic | List subscriptions (search/filter/sort/pagination) |
| `DELETE` | `/api/v1/admin/subscriptions/{id}` | Basic | Cancel subscription |
| `GET`  | `/api/v1/admin/limits` | Basic | List tier limits |
| `PUT`  | `/api/v1/admin/limits/{tier}` | Basic | Create or update tier limits |
| `DELETE` | `/api/v1/admin/limits/{tier}` | Basic | Delete tier limits |
| `GET`  | `/docs` | Basic* | API documentation (Scalar UI) |
| `GET`  | `/docs/openapi.yaml` | Basic* | OpenAPI 3.1 spec |

**Auth types:**
- **Bearer** — JWT access token via `Authorization: Bearer <token>` header (obtained through OAuth)
- **Basic** — HTTP Basic Auth via `Authorization: Basic <base64>` header (admin credentials from env vars)
- **Basic\*** — Protected only when `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set; public otherwise

### Authentication Flow

1. User visits `/auth/google`, `/auth/github`, or `/auth/apple` to start OAuth
2. After consent, the callback returns a JWT token pair (access + refresh)
3. Use the access token in subsequent API requests:

> **Apple Sign-In notes:** Apple uses `response_mode=form_post` — the callback
> receives a POST with `application/x-www-form-urlencoded` (not a GET redirect
> like Google/GitHub). The server generates a short-lived ES256 client_secret JWT
> per request and verifies the returned id_token via Apple's JWKS endpoint (RS256).

```bash
# Get token via OAuth callback (browser flow) or mobile token exchange:
curl -s -X POST http://localhost:8080/auth/token \
  -H "Content-Type: application/json" \
  -d '{"provider": "google", "id_token": "..."}' | jq .

# Response:
# { "access_token": "eyJ...", "refresh_token": "eyJ..." }
```

### Example: Create a Secret

```bash
curl -s -X POST http://localhost:8080/api/v1/secrets \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJ..." \
  -d '{
    "text": "DB_PASSWORD=super-secret",
    "to": ["alice@example.com"]
  }' | jq .
```

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "expires_at": "2026-02-20T23:10:00Z",
  "recipients": [
    {
      "email": "alice@example.com",
      "link": "http://localhost:3000/s/abc123#base64urlkey"
    }
  ]
}
```

### Example: Reveal a Secret

```bash
# Parse token and key from the link:
# http://localhost:3000/s/{token}#{key}

curl -s -X POST http://localhost:8080/api/v1/secrets/{token}/reveal \
  -H "Content-Type: application/json" \
  -d '{
    "email": "alice@example.com",
    "key": "base64urlkey"
  }' | jq .
```

```json
{
  "text": "DB_PASSWORD=super-secret"
}
```

A second request to the same endpoint returns `404` — the secret has been deleted.

### Example: Admin — List Users

```bash
curl -s -u admin:secret http://localhost:8080/api/v1/admin/users?tier=pro | jq .
```

```json
{
  "users": [
    {
      "id": 1,
      "email": "jane@example.com",
      "name": "Jane Doe",
      "provider": "google",
      "tier": "pro",
      "secrets_used": 23,
      "secrets_limit": 100,
      "secrets_limit_override": null,
      "created_at": "2026-02-20T10:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "per_page": 20
}
```

### Example: Admin — Update User Tier

```bash
curl -s -u admin:secret -X PATCH http://localhost:8080/api/v1/admin/users/1 \
  -H "Content-Type: application/json" \
  -d '{"tier": "pro"}' | jq .
```

### Example: Admin — List Subscriptions

```bash
curl -s -u admin:secret http://localhost:8080/api/v1/admin/subscriptions?status=active | jq .
```

### Example: Admin — Cancel Subscription

```bash
curl -s -u admin:secret -X DELETE http://localhost:8080/api/v1/admin/subscriptions/1
# Returns 204 No Content
```

## Admin Panel

The admin panel is a separate section of the frontend at `/admin`. It uses HTTP
Basic Auth (independent from the main app's OAuth/JWT flow).

1. Set `ADMIN_USERNAME` and `ADMIN_PASSWORD` environment variables for the backend
2. Navigate to `http://localhost:3000/admin/login`
3. Sign in with the admin credentials

**Pages:**
- `/admin/users` — Search, filter by tier, sort, change user tiers, set per-user secrets limit override
- `/admin/subscriptions` — Search, filter by status, sort, cancel subscriptions
- `/admin/limits` — Configure secrets and recipients limits per tier (add/edit/delete tiers)

Credentials are stored in `sessionStorage` and cleared when the tab is closed.

## Development

### Frontend

```bash
cd frontend
npm install        # install dependencies
npm run dev        # development server at http://localhost:3000
npm run build      # production build
npx eslint .       # lint
```

### Backend

```bash
cd backend

# Build
go build ./...

# Run tests
go test -race ./...

# Lint
golangci-lint run ./...

# Format
golangci-lint fmt ./...

# Tidy modules
go mod tidy
```

## Project Structure

```
secretdrop/
├── backend/
│   ├── cmd/secretdrop/
│   │   └── main.go             # Application entrypoint
│   ├── docs/
│   │   ├── embed.go            # Embeds OpenAPI spec
│   │   └── openapi.yaml        # OpenAPI 3.1 spec
│   └── internal/
│       ├── appinfo/            # Version metadata
│       ├── auth/               # OAuth flows (Google, GitHub, Apple) + JWT
│       ├── billing/            # Stripe checkout, portal, webhooks
│       ├── cleanup/            # Expired record cleanup worker
│       ├── config/             # Env vars → Config struct
│       ├── email/              # Resend email delivery
│       ├── handler/            # HTTP handlers + docs
│       ├── middleware/         # Request ID, logging, auth, content-type, CORS
│       ├── model/              # Domain models, request/response, errors
│       ├── repository/         # Secret repository (SQLite)
│       ├── sentry/            # Sentry init + slog handler
│       ├── service/            # Business logic (create/reveal + limits)
│       └── user/               # User repository (SQLite, users + subscriptions)
├── frontend/                   # React/TypeScript SPA
│   └── src/
│       ├── api/                # API clients (app + admin)
│       ├── components/         # Shared components (Layout, AdminLayout, ConfirmModal)
│       ├── context/            # Auth + Theme context providers
│       └── pages/              # Route pages
│           └── admin/          # Admin panel pages (Login, Users, Subscriptions, Limits)
├── .pre-commit-config.yaml
├── .gitignore
└── CLAUDE.md
```

## Security Model

- **Encryption:** AES-256-GCM with HKDF-SHA256 key derivation bound to the recipient's email
- **Key never stored in DB** — only carried in the URL fragment (`#` portion)
- **URL fragments** are not sent to the server or logged (RFC 3986)
- **Email hashing:** SHA-256 — raw email is never persisted
- **One-time use:** record is permanently deleted from DB after reveal
- **Auto-cleanup:** expired records are periodically purged
- **Authentication:** JWT Bearer tokens (15-min access, 30-day refresh)
- **OAuth:** Google + GitHub + Apple sign-in with CSRF state cookies

Admin IP list

```bash
ADMIN_ALLOWED_IPS=78.190.91.131/32,92.45.192.128/32,78.189.35.197/32
```

## Pricing

| Tier | Price | Secrets |
|------|-------|---------|
| Free | $0 | 1 secret (lifetime) |
| Pro | $2.99/month | 100 secrets/month |

## License

MIT — see [LICENCE](LICENCE) for details.

## Rake Tasks

`rake` runs default task: `run:backend`

```bash
rake -T

rake run:backend  # run backend
```

