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
│       ├── config/            # Config with functional options (env vars)
│       ├── model/             # Domain models, request/response types, errors
│       ├── crypt/             # HKDF + AES-256-GCM encrypt/decrypt
│       ├── repository/        # Repository interface
│       │   └── sqlite/        # SQLite implementation
│       ├── email/             # Sender interface
│       │   ├── resend/        # Resend API implementation
│       │   ├── console/       # Console logger (development)
│       │   └── noop/          # No-op sender (testing)
│       ├── service/           # Business logic: create + reveal
│       ├── handler/           # HTTP handlers + JSON helpers
│       ├── middleware/        # RequestID, logging, content-type, rate limit
│       └── cleanup/           # Ticker-based expired secret deletion
├── frontend/                  # React/TypeScript (TBD)
├── .gitignore
├── .pre-commit-config.yaml
└── CLAUDE.md
```

## API Endpoints

- `POST /api/v1/secrets` — Create encrypted secret (201)
- `POST /api/v1/secrets/{token}/reveal` — Reveal and burn secret (200)
- `GET /healthz` — Health check (200)

## Environment Variables

| Variable | Required | Default |
|----------|----------|---------|
| `GOLANG_ENV` | No | `production` |
| `RESEND_API_KEY` | Yes (prod only) | — |
| `PORT` | No | `8080` |
| `DATABASE_URL` | No | `file:secretdrop.db?_journal_mode=WAL` |
| `BASE_URL` | No | `http://localhost:3000` |
| `FROM_EMAIL` | No | `SecretDrop <noreply@secretdrop.app>` |
| `SECRET_EXPIRY` | No | `10m` |
| `CLEANUP_INTERVAL` | No | `1m` |

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
