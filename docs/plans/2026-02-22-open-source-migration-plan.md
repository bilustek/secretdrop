# Open Source Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate all `bilusteknoloji` references to `bilustek`, clean production secrets, and prepare the codebase for a fresh public repo at `github.com/bilustek/secretdrop`.

**Architecture:** Pure find-and-replace migration — no behavioral changes. Go module path changes cascade through all imports. Production config becomes example files with placeholders.

**Tech Stack:** Go 1.26, React 19/TypeScript, GitHub Actions, Docker

---

### Task 1: Update Go Module Path

**Files:**
- Modify: `backend/go.mod:1`

**Step 1: Update module declaration**

In `backend/go.mod`, change line 1:
```
module github.com/bilusteknoloji/secretdrop
```
to:
```
module github.com/bilustek/secretdrop
```

**Step 2: Verify the file**

Run: `head -1 backend/go.mod`
Expected: `module github.com/bilustek/secretdrop`

---

### Task 2: Update All Go Import Paths

**Files (60 files):**
- Modify: All `.go` files under `backend/` that import `github.com/bilusteknoloji/secretdrop`

**Step 1: Bulk replace import paths**

In every `.go` file under `backend/`, replace all occurrences of:
```
github.com/bilusteknoloji/secretdrop
```
with:
```
github.com/bilustek/secretdrop
```

This affects 59 Go source files (excluding `go.mod` already done in Task 1):
- `backend/cmd/secretdrop/main.go`
- `backend/internal/auth/*.go`
- `backend/internal/billing/*.go`
- `backend/internal/cleanup/*.go`
- `backend/internal/config/*_test.go`
- `backend/internal/crypt/*_test.go`
- `backend/internal/email/**/*.go`
- `backend/internal/handler/*.go`
- `backend/internal/middleware/*.go`
- `backend/internal/model/*_test.go`
- `backend/internal/repository/**/*.go`
- `backend/internal/sentry/**/*.go`
- `backend/internal/service/*.go`
- `backend/internal/slack/**/*.go`
- `backend/internal/user/**/*.go`

**Step 2: Regenerate go.sum**

Run:
```bash
cd backend && go mod tidy
```
Expected: `go.sum` updates cleanly, no errors.

**Step 3: Verify build**

Run:
```bash
cd backend && go build ./...
```
Expected: Clean build, exit code 0.

**Step 4: Run tests**

Run:
```bash
cd backend && go test -race -count=1 ./...
```
Expected: All tests pass.

**Step 5: Run linter**

Run:
```bash
cd backend && golangci-lint run ./...
```
Expected: No lint errors.

**Step 6: Commit**

```bash
git add backend/
git commit -m "refactor: migrate Go module path from bilusteknoloji to bilustek"
```

---

### Task 3: Update Contact Email

**Files:**
- Modify: `frontend/src/pages/Terms.tsx:105`
- Modify: `frontend/src/pages/Privacy.tsx:84`
- Modify: `backend/internal/handler/contact.go:12`

**Step 1: Update Terms.tsx**

Replace:
```
support@bilusteknoloji.com
```
with:
```
support@bilustek.com
```

**Step 2: Update Privacy.tsx**

Same replacement as Step 1.

**Step 3: Update contact handler**

In `backend/internal/handler/contact.go`, replace:
```go
const contactRecipient = "support@bilusteknoloji.com"
```
with:
```go
const contactRecipient = "support@bilustek.com"
```

**Step 4: Verify backend still builds**

Run:
```bash
cd backend && go build ./...
```
Expected: Clean build.

**Step 5: Verify frontend builds**

Run:
```bash
cd frontend && npm run build
```
Expected: Clean build.

**Step 6: Commit**

```bash
git add frontend/src/pages/Terms.tsx frontend/src/pages/Privacy.tsx backend/internal/handler/contact.go
git commit -m "chore: update contact email to support@bilustek.com"
```

---

### Task 4: Update LICENCE

**Files:**
- Modify: `LICENCE`

**Step 1: Update copyright holder and year**

Replace:
```
Copyright (c) 2025 Bilus Teknoloji A.Ş.
```
with:
```
Copyright (c) 2025-2026 Bilustek, LLC
```

**Step 2: Commit**

```bash
git add LICENCE
git commit -m "chore: update copyright to Bilustek, LLC"
```

---

### Task 5: Update README.md

**Files:**
- Modify: `README.md`

**Step 1: Update all org references**

Replace all occurrences of `bilusteknoloji` with `bilustek` in `README.md`. This covers:
- Badge URLs (lines 3-5): GitHub Actions badges + Codecov badge
- Clone URL (line 35): `git clone https://github.com/bilustek/secretdrop.git`

Note: The codecov badge token (`?token=96Z01WRM6E`) will need to be updated after setting up Codecov on the new repo. For now, update the org name.

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: update README references to bilustek org"
```

---

### Task 6: Update OpenAPI Spec

**Files:**
- Modify: `backend/docs/openapi.yaml:25`

**Step 1: Update repository URL**

Replace:
```yaml
url: https://github.com/bilusteknoloji/secretdrop
```
with:
```yaml
url: https://github.com/bilustek/secretdrop
```

**Step 2: Commit**

```bash
git add backend/docs/openapi.yaml
git commit -m "docs: update OpenAPI spec repository URL"
```

---

### Task 7: Update CI Workflows

**Files:**
- Modify: `.github/workflows/test.yml:42`

**Step 1: Update Codecov slug**

Replace:
```yaml
slug: bilusteknoloji/secretdrop
```
with:
```yaml
slug: bilustek/secretdrop
```

**Step 2: Commit**

```bash
git add .github/workflows/test.yml
git commit -m "ci: update Codecov slug to bilustek org"
```

---

### Task 8: Convert docker-stack.yml to Example

**Files:**
- Rename: `docker-stack.yml` → `docker-stack.example.yml`

**Step 1: Rename and update content**

Rename `docker-stack.yml` to `docker-stack.example.yml` and make these replacements:

1. `ghcr.io/bilusteknoloji/secretdrop/secretdrop-backend:latest` → `ghcr.io/bilustek/secretdrop/secretdrop-backend:latest`
2. `ghcr.io/bilusteknoloji/secretdrop/secretdrop-frontend:latest` → `ghcr.io/bilustek/secretdrop/secretdrop-frontend:latest`
3. `node.hostname == srv1391785` → `node.hostname == your-node-hostname`
4. `` Host(`api.secretdrop.us`) `` → `` Host(`api.example.com`) ``
5. `` Host(`secretdrop.us`) `` → `` Host(`example.com`) ``

**Step 2: Add docker-stack.yml to .gitignore**

Append to `.gitignore`:
```
# Docker stack (use docker-stack.example.yml as template)
docker-stack.yml
```

**Step 3: Commit**

```bash
git add docker-stack.example.yml .gitignore
git rm docker-stack.yml
git commit -m "chore: convert docker-stack.yml to example with placeholders"
```

---

### Task 9: Create .envrc.example

**Files:**
- Create: `.envrc.example`

**Step 1: Create example env file**

Create `.envrc.example` with placeholder values:

```bash
export GOLANG_ENV="development"
export RESEND_API_KEY="re_your_api_key_here"
export JWT_SECRET="change-me-to-a-strong-random-string"
export FROM_EMAIL="SecretDrop <noreply@yourdomain.com>"

export GOOGLE_CLIENT_ID="your-google-client-id.apps.googleusercontent.com"
export GOOGLE_CLIENT_SECRET="your-google-client-secret"

export GITHUB_CLIENT_ID="your-github-client-id"
export GITHUB_CLIENT_SECRET="your-github-client-secret"

# Stripe (Test)
export STRIPE_PUBLISHABLE_KEY="pk_test_your_key_here"
export STRIPE_SECRET_KEY="sk_test_your_key_here"
export STRIPE_WEBHOOK_SECRET="whsec_your_webhook_secret_here"
export STRIPE_PRICE_ID="price_your_price_id_here"

export SLACK_WEBHOOK_SUBSCRIPTIONS=""
export SLACK_WEBHOOK_NOTIFICATIONS=""

# Admin panel (optional)
export ADMIN_USERNAME="admin"
export ADMIN_PASSWORD="change-me-to-a-strong-password"

# Feature flags
export VITE_ENABLE_GOOGLE_SIGNIN="false"

# Sentry (optional)
# export SENTRY_DSN="https://your-key@your-org.ingest.sentry.io/your-project-id"
```

**Step 2: Commit**

```bash
git add .envrc.example
git commit -m "chore: add .envrc.example with placeholder values"
```

---

### Task 10: Update docs/plans References

**Files:**
- Modify: `docs/plans/2026-02-20-auth-billing-plan.md`
- Modify: `docs/plans/2026-02-20-slack-integration-plan.md`
- Modify: `docs/plans/2026-02-20-subscription-account-plan.md`
- Modify: `docs/plans/2026-02-21-admin-panel-plan.md`
- Modify: `docs/plans/2026-02-21-sentry-integration-plan.md`

**Step 1: Bulk replace in all plan docs**

Replace all occurrences of `github.com/bilusteknoloji/secretdrop` with `github.com/bilustek/secretdrop` in all `docs/plans/*.md` files.

**Step 2: Commit**

```bash
git add docs/plans/
git commit -m "docs: update import paths in plan documents"
```

---

### Task 11: Final Verification

**Step 1: Verify no bilusteknoloji references remain (excluding git history)**

Run:
```bash
grep -r "bilusteknoloji" --include="*.go" --include="*.yml" --include="*.yaml" --include="*.md" --include="*.tsx" --include="*.ts" --include="*.html" --include="*.json" . | grep -v node_modules | grep -v .git/
```
Expected: No output (zero matches).

**Step 2: Full backend verification**

Run:
```bash
cd backend && go build ./... && go test -race -count=1 ./... && golangci-lint run ./...
```
Expected: All pass.

**Step 3: Full frontend verification**

Run:
```bash
cd frontend && npm run build && npx tsc --noEmit && npx eslint .
```
Expected: All pass.

---

### Task 12: Post-Migration (Manual Steps — Not Code)

These steps happen after code changes are committed:

1. **Create new repo** at `github.com/bilustek/secretdrop` (public)
2. **Copy codebase** (excluding `.git/`, `.envrc`, `node_modules/`, `*.db`):
   ```bash
   mkdir /tmp/secretdrop-clean
   rsync -av --exclude='.git' --exclude='.envrc' --exclude='node_modules' --exclude='*.db' . /tmp/secretdrop-clean/
   cd /tmp/secretdrop-clean
   git init
   git add -A
   git commit -m "Initial open source release"
   git remote add origin git@github.com:bilustek/secretdrop.git
   git branch -M main
   git push -u origin main
   ```
3. **Set up new repo secrets** in GitHub Actions settings:
   - `CODECOV_TOKEN` (new token from Codecov for new repo)
4. **Rotate all compromised secrets** (see design doc Section 9)
5. **Add deprecation notice** to old `bilusteknoloji/secretdrop` repo README
6. **Set up Codecov** for new repo, update badge token in README if needed
