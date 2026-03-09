![Version](https://img.shields.io/badge/version-0.6.1-orange.svg)
![Go](https://img.shields.io/badge/go-1.26-00ADD8.svg?logo=go&logoColor=white)
[![Run golangci-lint](https://github.com/bilustek/secretdrop/actions/workflows/lint.yml/badge.svg)](https://github.com/bilustek/secretdrop/actions/workflows/lint.yml)
[![Run go tests](https://github.com/bilustek/secretdrop/actions/workflows/test.yml/badge.svg)](https://github.com/bilustek/secretdrop/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/bilustek/secretdrop/graph/badge.svg?token=H3FHJ3VZRO)](https://codecov.io/gh/bilustek/secretdrop)

# SecretDrop

Secure, one-time secret sharing API for developers.

Instead of pasting API keys in Slack, share them through SecretDrop: text is
encrypted with AES-256-GCM, a one-time link is sent to the recipient via
email. Once the link is opened or expires, the record is **permanently deleted** 
from the database - no trace left.

---

## Development Requirements

- `go 1.26+`
- `golangci-lint v2`
- `pre-commit`

### 3rd Party Providers (API Keys, Credentials)

- [Resend API Key](https://resend.com)
- Google OAuth app
- GitHub OAuth app
- Apple Developer account
- Stripe account
- Sentry account

Quick development setup for macOS and `brew` users:

```bash
brew install go golangci-lint pre-commit
```

Environment variables for local development (backend):

```bash
# optional variables
export PORT="<port-number>"          # default is: 8080
export SENTRY_DSN="<secret>"         # sends to sentry if this variable is set!           
export API_BASE_URL="..."            # default is: http://localhost:8080
export FRONTEND_BASE_URL="..."       # default is: http://localhost:3000
export APPLE_CLIENT_ID="<...>"
export APPLE_TEAM_ID="<...>"
export APPLE_KEY_ID="<...>"
export APPLE_PRIVATE_KEY="<...>"
export DATABASE_URL="..."                # default is: file:db/secretdrop.db?_journal_mode=WAL
export STRIPE_WEBHOOK_FORWARD_TO="..."   # default is: localhost:8080/billing/webhook
export SECRET_EXPIRY="..."               # default is: 10m (10 minutes)
export CLEANUP_INTERVAL="..."            # default is: 1m (1 minute)

# in development mode, fake mail send, slack hook goes to console only!
# just set some random value, real value required in "production" environment
export RESEND_API_KEY="<your-api-key>"
export SLACK_WEBHOOK_SUBSCRIPTIONS="<slack-webhook>"
export SLACK_WEBHOOK_NOTIFICATIONS="<slack-webhook>"

# required
export GOLANG_ENV="development" # default is: production
export JWT_SECRET="<secret>"
export GOOGLE_CLIENT_ID="<secret>"
export GOOGLE_CLIENT_SECRET="<secret>"
export GITHUB_CLIENT_ID="<secret>"
export GITHUB_CLIENT_SECRET="<secret>"
export ADMIN_USERNAME="<username>"
export ADMIN_PASSWORD="<secret>"
export STRIPE_PRICE_ID="<...>"
export STRIPE_SECRET_KEY="<secret>"
export STRIPE_WEBHOOK_SECRET="<secret>"

export FROM_EMAIL="Name <email>"
export REPLY_TO_EMAIL="<email>"

# Set Strip Product (your subscription Product) METADATA as -> project: secretdrop
export STRIPE_PROJECT_METAKEY="project"
export STRIPE_PROJECT_METADATA="secretdrop"
```

Build time environment variables for frontend:

```bash
# remove these variables if you like to enable, "false" hides sign-in button.
export VITE_ENABLE_GOOGLE_SIGNIN="false"
export VITE_ENABLE_APPLE_SIGNIN="false"
```

Run backend and frontend together:

```bash
git clone git@github.com:bilustek/secretdrop.git
cd secretdrop/
pre-commit install
stripe login # for webhooks
cd frontend/
npm install
cd ../
rake
```

---

## API Documentation

Run the local server, hit: http://localhost:8080/docs, (Scalar UI) protected by 
**Basic Auth** when `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set!

**Auth types:**

- **Bearer**: JWT access token via `Authorization: Bearer <token>` header (obtained through OAuth)
- **Basic**: HTTP Basic Auth via `Authorization: Basic <base64>` header (admin credentials from env vars)
- **Basic**: Protected only when `ADMIN_USERNAME` and `ADMIN_PASSWORD` are set; public otherwise

---

### Authentication Flow

1. User visits `/auth/google`, `/auth/github`, or `/auth/apple` to start OAuth
2. After consent, the callback returns a JWT token pair (access + refresh)
3. Use the access token in subsequent API requests:

> **Apple Sign-In notes:** Apple uses `response_mode=form_post` - the callback
> receives a POST with `application/x-www-form-urlencoded` (not a GET redirect
> like Google/GitHub). The server generates a short-lived ES256 client_secret JWT
> per request and verifies the returned id_token via Apple's JWKS endpoint (RS256).

---

## Security Model

- **Encryption:** AES-256-GCM with HKDF-SHA256 key derivation bound to the recipient's email
- **Key never stored in DB** - only carried in the URL fragment (`#` portion)
- **URL fragments** are not sent to the server or logged (RFC 3986)
- **Email hashing:** SHA-256 - raw email is never persisted
- **One-time use:** record is permanently deleted from DB after reveal
- **Auto-cleanup:** expired records are periodically purged
- **Authentication:** JWT Bearer tokens (15-min access, 30-day refresh)
- **OAuth:** Google + GitHub + Apple sign-in with CSRF state cookies

---

## Contributor(s)

* [Uğur Özyılmazel](https://github.com/vigo) - Creator, maintainer

---

## Contribute

All PR’s are welcome!

1. `fork` (https://github.com/bilustek/secretdrop/fork)
1. Create your `branch` (`git checkout -b my-feature`)
1. `commit` yours (`git commit -am 'add some functionality'`)
1. `push` your `branch` (`git push origin my-feature`)
1. Then create a new **Pull Request**!

---

## License

This project is licensed under MIT (MIT)

---

This project is intended to be a safe, welcoming space for collaboration, and
contributors are expected to adhere to the [code of conduct][coc].

[coc]: https://github.com/bilustek/secretdrop/blob/main/CODE_OF_CONDUCT.md
