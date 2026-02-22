# Open Source Migration Design

**Date:** 2026-02-22
**Status:** Approved

## Goal

Migrate SecretDrop from `github.com/bilusteknoloji/secretdrop` (private) to
`github.com/bilustek/secretdrop` (public) with a clean git history — no leaked
secrets, no stale org references.

## Approach: Fresh Init

Create a new repo at `github.com/bilustek/secretdrop`. Copy the current codebase
(latest state only) with all changes applied. Single initial commit. Old repo gets
a deprecation notice pointing to the new location.

**Rationale:** `.envrc` with real API keys, Stripe secrets, OAuth credentials, Slack
webhooks, and admin passwords exists in git history. BFG cleanup is fragile and the
keys must be rotated regardless. Fresh init is the simplest guaranteed-clean path.

## Changes Required

### 1. Go Module Path

- `go.mod`: `github.com/bilusteknoloji/secretdrop` → `github.com/bilustek/secretdrop`
- ~60 Go source files: all import paths updated accordingly
- `go.sum`: regenerated via `go mod tidy`

### 2. Hardcoded References (bilusteknoloji → bilustek)

| File | What changes |
|------|-------------|
| `README.md` | Badge URLs, clone URL, codecov badge |
| `CLAUDE.md` | No org references (already generic) |
| `backend/docs/openapi.yaml` | Repository URL |
| `.github/workflows/test.yml` | Codecov slug |
| `docs/plans/*.md` | Import paths in code examples (bulk replace) |

### 3. Contact Email

`support@bilusteknoloji.com` → `support@bilustek.com` in:
- `frontend/src/pages/Terms.tsx`
- `frontend/src/pages/Privacy.tsx`
- `backend/internal/handler/contact.go` (`contactRecipient` const)

### 4. Production Config Cleanup

- `docker-stack.yml` → rename to `docker-stack.example.yml`
  - Replace `ghcr.io/bilusteknoloji/secretdrop/*` → `ghcr.io/bilustek/secretdrop/*`
  - Replace `Host(\`api.secretdrop.us\`)` → `Host(\`api.example.com\`)`
  - Replace `Host(\`secretdrop.us\`)` → `Host(\`example.com\`)`
  - Replace `node.hostname == srv1391785` → `node.hostname == your-node`
- Create `.envrc.example` with placeholder values
- `.envrc` is already in `.gitignore` — will NOT be copied to new repo

### 5. Licence & Copyright

- `LICENCE`: "Bilus Teknoloji A.S." → "Bilustek, LLC"
- Year: "2025" → "2025-2026"

### 6. Docker Image References

- `ghcr.io/bilusteknoloji/secretdrop/*` → `ghcr.io/bilustek/secretdrop/*`
- Only in `docker-stack.example.yml` (build workflows use `${{ github.repository }}`)

### 7. CI/CD

- `.github/workflows/test.yml`: codecov slug `bilusteknoloji/secretdrop` → `bilustek/secretdrop`
- Build workflows (`build-backend.yml`, `build-frontend.yml`) use `${{ github.repository }}` — auto-correct

### 8. Frontend Meta Tags

- `frontend/index.html`: OG/Twitter meta tags reference `secretdrop.us` — keep as-is (product domain, not org)

### 9. Secret Rotation (Manual, post-migration)

All secrets in the old `.envrc` are compromised and must be rotated:
- Resend API key
- Google OAuth client ID + secret
- GitHub OAuth client ID + secret
- Stripe test + prod keys (secret, webhook secret, publishable)
- Slack webhook URLs (subscriptions + notifications)
- JWT secret
- Admin username + password
- Sentry DSN

## Out of Scope

- Renaming the product ("SecretDrop" stays)
- Changing the `secretdrop.us` domain
- Modifying application behavior or features
- Adding CONTRIBUTING.md, CODE_OF_CONDUCT.md etc. (can be done later)

## Migration Steps (High Level)

1. Apply all code changes in current repo on `make-it-opensource` branch
2. Verify: `go build`, `go test`, `golangci-lint`, frontend build all pass
3. Create new repo `github.com/bilustek/secretdrop`
4. Copy cleaned codebase to new repo, commit, push
5. Rotate all compromised secrets
6. Add deprecation notice to old repo
7. Update Codecov, CI secrets in new repo
