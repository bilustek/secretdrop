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

## Quick Start

```bash
# Clone the repo
git clone https://github.com/bilusteknoloji/secretdrop.git
cd secretdrop

# Install pre-commit hooks
pre-commit install

# Run the backend
cd backend
RESEND_API_KEY=re_xxxxx go run .
```

The server starts at `http://localhost:8080`.

## Environment Variables

Create a `.env` file and `source` it, or export directly:

```bash
# backend/.env (git-ignored)
export RESEND_API_KEY=re_xxxxx          # (required) Resend API key
export PORT=8080                         # server port
export DATABASE_URL="file:secretdrop.db?_journal_mode=WAL"  # SQLite path
export BASE_URL=http://localhost:3000    # frontend URL (used in links)
export FROM_EMAIL="SecretDrop <noreply@secretdrop.app>"     # sender address
export SECRET_EXPIRY=10m                 # secret TTL
export CLEANUP_INTERVAL=1m              # expired record cleanup frequency
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/secrets` | Create an encrypted secret |
| `POST` | `/api/v1/secrets/{token}/reveal` | Reveal and burn a secret |
| `GET`  | `/healthz` | Health check |
| `GET`  | `/docs` | API documentation (Scalar UI) |
| `GET`  | `/docs/openapi.yaml` | OpenAPI 3.1 spec |

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
│   ├── main.go                 # Application entrypoint
│   ├── docs/
│   │   └── openapi.yaml        # OpenAPI 3.1 spec
│   └── internal/
│       ├── config/             # Env vars → Config struct
│       ├── model/              # Domain models, request/response, errors
│       ├── crypt/              # AES-256-GCM + HKDF encryption
│       ├── repository/         # SQLite repository
│       ├── email/              # Resend email delivery
│       ├── service/            # Business logic (create/reveal)
│       ├── handler/            # HTTP handlers + docs
│       ├── middleware/         # Request ID, logging, content-type, rate limit
│       └── cleanup/            # Expired record cleanup worker
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

## License

MIT — see [LICENCE](LICENCE) for details.
