## Overview

This project implements a secure text sharing API for web, mobile and desktop apps.
Secrets are encrypted with AES-256-GCM (HKDF-derived keys), stored in SQLite,
and auto-deleted after one-time reveal or expiry.

## Technologies Used

- React and TypeScript for web frontend
- Go for backend API server (go1.26.0)
- golangci-lint v2 for Go linting
- SQLite via modernc.org/sqlite (pure Go, no CGO)
- Resend for transactional email (github.com/resend/resend-go/v2)
- Google + GitHub OAuth via golang.org/x/oauth2
- JWT auth via github.com/golang-jwt/jwt/v5
- Stripe billing via github.com/stripe/stripe-go/v82

## Project Structure

```
secretdrop/
├── backend/
│   ├── cmd/secretdrop/
│   │   └── main.go            # Application entrypoint
│   ├── docs/
│   │   ├── embed.go           # Embeds OpenAPI spec
│   │   └── openapi.yaml       # OpenAPI 3.1 spec
│   ├── go.mod / go.sum
│   ├── .golangci.yml
│   └── internal/
│       ├── appinfo/           # Version metadata
│       ├── auth/              # OAuth flows (Google, GitHub) + JWT
│       ├── billing/           # Stripe checkout, portal, webhooks
│       ├── cleanup/           # Ticker-based expired secret deletion
│       ├── config/            # Config with functional options (env vars)
│       ├── crypt/             # HKDF + AES-256-GCM encrypt/decrypt
│       ├── email/             # Sender interface
│       │   ├── resend/        # Resend API implementation
│       │   ├── console/       # Console logger (development)
│       │   └── noop/          # No-op sender (testing)
│       ├── handler/           # HTTP handlers + JSON helpers
│       ├── middleware/        # RequestID, logging, auth, content-type, rate limit
│       ├── model/             # Domain models, request/response types, errors
│       ├── repository/        # Secret repository interface
│       │   └── sqlite/        # SQLite implementation
│       ├── service/           # Business logic: create + reveal + limits
│       └── user/              # User repository interface
│           └── sqlite/        # SQLite implementation (users + subscriptions)
├── frontend/                  # React/TypeScript (TBD)
├── .gitignore
├── .pre-commit-config.yaml
└── CLAUDE.md
```

## API Endpoints

- `POST /api/v1/secrets` — Create encrypted secret (201, auth required)
- `POST /api/v1/secrets/{token}/reveal` — Reveal and burn secret (200)
- `GET /api/v1/me` — Authenticated user profile (200, auth required)
- `GET /auth/google` — Google OAuth login redirect
- `GET /auth/google/callback` — Google OAuth callback
- `GET /auth/github` — GitHub OAuth login redirect
- `GET /auth/github/callback` — GitHub OAuth callback
- `POST /auth/token` — Mobile token exchange
- `POST /billing/checkout` — Stripe checkout session (auth required)
- `POST /billing/portal` — Stripe customer portal (auth required)
- `POST /billing/webhook` — Stripe webhook handler
- `GET /healthz` — Health check (200)

## Environment Variables

| Variable | Required | Default |
|----------|----------|---------|
| `GOLANG_ENV` | No | `production` |
| `RESEND_API_KEY` | Yes (prod only) | — |
| `PORT` | No | `8080` |
| `DATABASE_URL` | No | `file:db/secretdrop.db?_journal_mode=WAL` |
| `BASE_URL` | No | `http://localhost:3000` |
| `FROM_EMAIL` | No | `SecretDrop <noreply@secretdrop.app>` |
| `SECRET_EXPIRY` | No | `10m` |
| `CLEANUP_INTERVAL` | No | `1m` |
| `GOOGLE_CLIENT_ID` | Yes (prod only) | — |
| `GOOGLE_CLIENT_SECRET` | Yes (prod only) | — |
| `GITHUB_CLIENT_ID` | Yes (prod only) | — |
| `GITHUB_CLIENT_SECRET` | Yes (prod only) | — |
| `JWT_SECRET` | Yes (prod only) | — |
| `STRIPE_SECRET_KEY` | Yes (prod only) | — |
| `STRIPE_WEBHOOK_SECRET` | Yes (prod only) | — |
| `STRIPE_PRICE_ID` | Yes (prod only) | — |

## Running the Backend

```bash
# Production
cd backend
RESEND_API_KEY=re_xxx go run ./cmd/secretdrop/    # starts server on :8080

# Development (no API key needed, emails logged to console)
cd backend
GOLANG_ENV=development go run ./cmd/secretdrop/
```

## Development Commands

```bash
# Build
cd backend && go build ./...

# Lint
cd backend && golangci-lint run ./...

# Format
cd backend && golangci-lint fmt ./...

# Test
cd backend && go test -race ./...
```

## Development Process

1. Before implementing, use DeepWiki's Ask Question tool to get latest
   information so that you write the code as per the latest library updates.
