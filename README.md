![Version](https://img.shields.io/badge/version-0.0.0-orange.svg)
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
| Stripe account | — | [stripe.com](https://stripe.com) → grab API keys |

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
export BASE_URL=http://localhost:3000    # frontend URL (used in links)
export FROM_EMAIL="SecretDrop <noreply@secretdrop.app>"     # sender address
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
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/api/v1/secrets` | Yes | Create an encrypted secret |
| `POST` | `/api/v1/secrets/{token}/reveal` | No | Reveal and burn a secret |
| `GET`  | `/api/v1/me` | Yes | Authenticated user profile |
| `GET`  | `/auth/google` | No | Google OAuth login redirect |
| `GET`  | `/auth/google/callback` | No | Google OAuth callback |
| `GET`  | `/auth/github` | No | GitHub OAuth login redirect |
| `GET`  | `/auth/github/callback` | No | GitHub OAuth callback |
| `POST` | `/auth/token` | No | Mobile token exchange |
| `POST` | `/billing/checkout` | Yes | Create Stripe checkout session |
| `POST` | `/billing/portal` | Yes | Stripe customer portal |
| `POST` | `/billing/webhook` | No | Stripe webhook handler |
| `GET`  | `/healthz` | No | Health check |
| `GET`  | `/docs` | No | API documentation (Scalar UI) |
| `GET`  | `/docs/openapi.yaml` | No | OpenAPI 3.1 spec |

### Example: Create a Secret

```bash
curl -s -X POST http://localhost:8080/api/v1/secrets \
  -H "Content-Type: application/json" \
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

## Development

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
│       ├── auth/               # OAuth flows (Google, GitHub) + JWT
│       ├── billing/            # Stripe checkout, portal, webhooks
│       ├── cleanup/            # Expired record cleanup worker
│       ├── config/             # Env vars → Config struct
│       ├── crypt/              # AES-256-GCM + HKDF encryption
│       ├── email/              # Resend email delivery
│       ├── handler/            # HTTP handlers + docs
│       ├── middleware/         # Request ID, logging, auth, content-type, rate limit
│       ├── model/              # Domain models, request/response, errors
│       ├── repository/         # Secret repository (SQLite)
│       ├── service/            # Business logic (create/reveal + limits)
│       └── user/               # User repository (SQLite, users + subscriptions)
├── frontend/                   # React/TypeScript (TBD)
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
- **OAuth:** Google + GitHub sign-in with CSRF state cookies

## Pricing

| Tier | Price | Secrets |
|------|-------|---------|
| Free | $0 | 1 secret (lifetime) |
| Pro | $2.99/month | 100 secrets/month |

## License

MIT — see [LICENCE](LICENCE) for details.
