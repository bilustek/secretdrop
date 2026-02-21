## Overview

This project implements a secure text sharing API for web, mobile and desktop apps.
Secrets are encrypted with AES-256-GCM (HKDF-derived keys), stored in SQLite,
and auto-deleted after one-time reveal or expiry.

## Technologies Used

- React 19, TypeScript 5.9, Vite 7, Tailwind CSS 4 for web frontend
- Go for backend API server (go1.26.0)
- golangci-lint v2 for Go linting
- SQLite via modernc.org/sqlite (pure Go, no CGO)
- Resend for transactional email (github.com/resend/resend-go/v2)
- Google + GitHub OAuth via golang.org/x/oauth2
- JWT auth via github.com/golang-jwt/jwt/v5
- Stripe billing via github.com/stripe/stripe-go/v82
- Sentry error tracking via github.com/getsentry/sentry-go

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
│       ├── sentry/            # Sentry init + slog handler
│       │   └── sloghandler/   # Custom slog.Handler for Sentry error forwarding
│       ├── slack/             # Slack notifier interface
│       │   ├── webhook/       # Slack webhook implementation
│       │   ├── console/       # Console logger (development)
│       │   ├── noop/          # No-op notifier (testing)
│       │   └── sloghandler/   # Custom slog.Handler for error notifications
│       ├── middleware/        # RequestID, logging, auth, content-type, CORS
│       ├── model/             # Domain models, request/response types, errors
│       ├── repository/        # Secret repository interface
│       │   └── sqlite/        # SQLite implementation
│       ├── service/           # Business logic: create + reveal + limits
│       └── user/              # User repository interface
│           └── sqlite/        # SQLite implementation (users + subscriptions)
├── frontend/                  # React/TypeScript SPA
│   └── src/
│       ├── api/               # API clients (app + admin) + shared config
│       ├── components/        # Shared components (Layout, AdminLayout, ConfirmModal, ThemeToggle)
│       ├── context/           # Auth + Theme context providers
│       └── pages/             # Route pages
│           └── admin/         # Admin panel (Login, Users, Subscriptions, Limits)
├── .gitignore
├── .pre-commit-config.yaml
└── CLAUDE.md
```

## API Endpoints

- `POST /api/v1/secrets` — Create encrypted secret (201, auth required)
- `POST /api/v1/secrets/{token}/reveal` — Reveal and burn secret (200)
- `GET /api/v1/me` — Authenticated user profile (200, auth required)
- `DELETE /api/v1/me` — Delete user account (204, auth required)
- `POST /api/v1/contact` — Send contact form message (200)
- `GET /auth/google` — Google OAuth login redirect
- `GET /auth/google/callback` — Google OAuth callback
- `GET /auth/github` — GitHub OAuth login redirect
- `GET /auth/github/callback` — GitHub OAuth callback
- `POST /auth/token` — Mobile token exchange
- `POST /billing/checkout` — Stripe checkout session (auth required)
- `POST /billing/portal` — Stripe customer portal (auth required)
- `POST /billing/webhook` — Stripe webhook handler
- `GET /api/v1/admin/users` — List users with search/filter/sort/pagination (admin auth)
- `PATCH /api/v1/admin/users/{id}` — Update user tier or secrets limit override (admin auth)
- `GET /api/v1/admin/subscriptions` — List subscriptions with filter/sort/pagination (admin auth)
- `DELETE /api/v1/admin/subscriptions/{id}` — Cancel subscription (admin auth)
- `GET /api/v1/admin/limits` — List all tier limits (admin auth)
- `PUT /api/v1/admin/limits/{tier}` — Create or update tier limits (admin auth)
- `DELETE /api/v1/admin/limits/{tier}` — Delete tier limits (admin auth)
- `GET /healthz` — Health check (200)

## Environment Variables

| Variable | Required | Default |
|----------|----------|---------|
| `GOLANG_ENV` | No | `production` |
| `RESEND_API_KEY` | Yes (prod only) | — |
| `PORT` | No | `8080` |
| `DATABASE_URL` | No | `file:db/secretdrop.db?_journal_mode=WAL` |
| `API_BASE_URL` | No | `http://localhost:8080` |
| `FRONTEND_BASE_URL` | No | `http://localhost:3000` |
| `FROM_EMAIL` | No | `SecretDrop <noreply@secretdrop.us>` |
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
| `SLACK_WEBHOOK_SUBSCRIPTIONS` | No | — |
| `SLACK_WEBHOOK_NOTIFICATIONS` | No | — |
| `ADMIN_USERNAME` | No | — |
| `ADMIN_PASSWORD` | No | — |
| `SENTRY_DSN` | No | — |
| `SENTRY_TRACES_SAMPLE_RATE` | No | `1.0` |
| `VITE_API_BASE_URL` | No (frontend) | `""` (same origin) |

## Frontend Routes

- `/` — Landing page
- `/dashboard` — User dashboard (OAuth required)
- `/s/:token` — Reveal secret
- `/admin/login` — Admin login (Basic Auth, sessionStorage)
- `/admin/users` — Admin: manage users (search, filter, sort, tier change, per-user limit override)
- `/admin/subscriptions` — Admin: manage subscriptions (search, filter, sort, cancel)
- `/admin/limits` — Admin: manage tier limits (CRUD for secrets/recipients limits per tier)

## Running

```bash
# Backend (production)
cd backend
RESEND_API_KEY=re_xxx go run ./cmd/secretdrop/    # starts server on :8080

# Backend (development — emails logged to console)
cd backend
GOLANG_ENV=development go run ./cmd/secretdrop/

# Frontend
cd frontend
npm install
npm run dev       # development server at http://localhost:3000
```

## Development Commands

```bash
# Backend
cd backend && go build ./...              # build
cd backend && golangci-lint run ./...     # lint
cd backend && golangci-lint fmt ./...     # format
cd backend && go test -race ./...        # test

# Frontend
cd frontend && npm run build             # production build
cd frontend && npx eslint .             # lint
```

## Development Process

1. Before implementing, use DeepWiki's Ask Question tool to get latest
   information so that you write the code as per the latest library updates.
