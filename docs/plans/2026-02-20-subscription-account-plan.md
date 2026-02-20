# Subscription & Account Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix subscription lifecycle (period tracking, graceful cancel), bump free tier to 5, add account deletion with confirmation modal, and update legal pages.

**Architecture:** Changes span model constants, webhook handlers, user repository (new delete methods), a new handler endpoint, and frontend (modal + terms/privacy updates). Secrets table has no user_id so account deletion only removes user + subscription rows; secrets expire naturally via cleanup worker.

**Tech Stack:** Go (backend), React/TypeScript (frontend), Stripe API (cancel subscription), SQLite

---

### Task 1: Bump Free Tier Limit to 5

**Files:**
- Modify: `backend/internal/model/user.go:12` (change constant)
- Modify: `backend/internal/model/user_test.go` (update test expectations)
- Modify: `frontend/src/pages/Landing.tsx:83` (update pricing card text)

**Step 1: Update the constant**

In `backend/internal/model/user.go`, change:
```go
FreeTierLimit = 1
```
to:
```go
FreeTierLimit = 5
```

**Step 2: Update tests**

Check `backend/internal/model/user_test.go` for any test asserting `SecretsLimit() == 1` for free users, update to `5`.

**Step 3: Update Landing page pricing**

In `frontend/src/pages/Landing.tsx`, change the free tier feature text from:
```
1 secret (lifetime)
```
to:
```
5 secrets (lifetime)
```

**Step 4: Run tests and lint**

```bash
cd backend && go test ./internal/model/... -v
cd frontend && npx tsc --noEmit
```

**Step 5: Commit**

```bash
git add backend/internal/model/user.go backend/internal/model/user_test.go frontend/src/pages/Landing.tsx
git commit -m "bump free tier limit from 1 to 5 secrets"
```

---

### Task 2: Fix Webhook Period Tracking

**Files:**
- Modify: `backend/internal/billing/webhook.go` (all 4 handlers)
- Modify: `backend/internal/user/sqlite/sqlite.go` (add UpdateSubscriptionPeriod method)
- Modify: `backend/internal/user/user.go` (add interface method)

**Step 1: Add `UpdateSubscriptionPeriod` to repository interface**

In `backend/internal/user/user.go`, add to the Repository interface:
```go
UpdateSubscriptionPeriod(ctx context.Context, stripeSubID string, start, end time.Time) error
```

Also add `"time"` to the imports.

**Step 2: Implement in SQLite repository**

In `backend/internal/user/sqlite/sqlite.go`, add:
```go
func (r *Repository) UpdateSubscriptionPeriod(ctx context.Context, stripeSubID string, start, end time.Time) error {
	const query = `UPDATE subscriptions SET current_period_start = ?, current_period_end = ? WHERE stripe_subscription_id = ?`

	result, err := r.db.ExecContext(ctx, query, start, end, stripeSubID)
	if err != nil {
		return fmt.Errorf("update subscription period: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return nil
}
```

**Step 3: Fix `handleCheckoutCompleted`**

In `backend/internal/billing/webhook.go`, the checkout session doesn't carry period info directly. Extract the subscription ID and use it to set period from the `subscription.updated` event that Stripe sends right after. For now, set period to zero (will be updated by `subscription.updated`). No change needed here — the `handleSubscriptionUpdated` fix below will catch it.

**Step 4: Fix `handleSubscriptionUpdated`**

Add period extraction and update call. After the existing `UpdateSubscriptionStatus` call, add:
```go
periodStart := time.Unix(sub.CurrentPeriodStart, 0)
periodEnd := time.Unix(sub.CurrentPeriodEnd, 0)

if err := s.userRepo.UpdateSubscriptionPeriod(ctx, sub.ID, periodStart, periodEnd); err != nil {
	slog.Error("update subscription period", slogKeyError, err, slogKeySubscriptionID, sub.ID)
}
```

Add `"time"` to imports.

**Step 5: Fix `handleInvoicePaid`**

After `ResetSecretsUsed`, add period update from invoice lines. The invoice carries `Lines.Data[0].Period` with start/end:
```go
if invoice.Lines != nil && len(invoice.Lines.Data) > 0 {
	line := invoice.Lines.Data[0]
	periodStart := time.Unix(line.Period.Start, 0)
	periodEnd := time.Unix(line.Period.End, 0)

	sub, subErr := s.userRepo.FindSubscriptionByUserID(ctx, u.ID)
	if subErr == nil {
		if periodErr := s.userRepo.UpdateSubscriptionPeriod(ctx, sub.StripeSubscriptionID, periodStart, periodEnd); periodErr != nil {
			slog.Error("update subscription period from invoice", slogKeyError, periodErr)
		}
	}
}
```

Note: `FindSubscriptionByUserID` is on the `user.Repository` interface but we need it here. The `s.userRepo` already has this method.

**Step 6: Fix `tierForStatus` for cancel_at_period_end**

When user cancels via portal, Stripe sends `subscription.updated` with `status=active` but `cancel_at_period_end=true`. The current `tierForStatus` already returns "pro" for active status, which is correct — user stays pro until period ends. When period actually ends, `subscription.deleted` fires and downgrades. **No change needed here.**

**Step 7: Run tests and lint**

```bash
cd backend && go test -race ./... && golangci-lint run ./...
```

**Step 8: Commit**

```bash
git add backend/internal/user/user.go backend/internal/user/sqlite/sqlite.go backend/internal/billing/webhook.go
git commit -m "fix webhook period tracking and store subscription dates"
```

---

### Task 3: Add Delete Account Backend

**Files:**
- Modify: `backend/internal/user/user.go` (add DeleteUser to interface)
- Modify: `backend/internal/user/sqlite/sqlite.go` (implement DeleteUser)
- Create: `backend/internal/handler/account.go` (DELETE /api/v1/me handler)
- Modify: `backend/cmd/secretdrop/main.go` (register route)
- Modify: `backend/internal/billing/billing.go` (add CancelSubscription to StripeClient interface)

**Step 1: Add `DeleteUser` to repository interface**

In `backend/internal/user/user.go`:
```go
DeleteUser(ctx context.Context, id int64) error
```

**Step 2: Implement DeleteUser in SQLite**

In `backend/internal/user/sqlite/sqlite.go`:
```go
func (r *Repository) DeleteUser(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM subscriptions WHERE user_id = ?", id); err != nil {
		return fmt.Errorf("delete subscriptions: %w", err)
	}

	result, err := tx.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errFmtRowsAffected, err)
	}

	if n == 0 {
		return model.ErrNotFound
	}

	return tx.Commit()
}
```

**Step 3: Add CancelSubscription to StripeClient interface**

In `backend/internal/billing/billing.go`, add to `StripeClient` interface:
```go
CancelSubscription(ctx context.Context, id string) error
```

Add implementation to `stripeClientAdapter`:
```go
func (a *stripeClientAdapter) CancelSubscription(ctx context.Context, id string) error {
	_, err := a.client.V1Subscriptions.Cancel(ctx, id, nil)
	if err != nil {
		return fmt.Errorf("cancel subscription: %w", err)
	}

	return nil
}
```

Also expose a public method on Service:
```go
func (s *Service) CancelSubscription(ctx context.Context, stripeSubID string) error {
	return s.stripeClient.CancelSubscription(ctx, stripeSubID)
}
```

**Step 4: Create account handler**

Create `backend/internal/handler/account.go`:
```go
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/bilusteknoloji/secretdrop/internal/middleware"
	"github.com/bilusteknoloji/secretdrop/internal/model"
	"github.com/bilusteknoloji/secretdrop/internal/user"
)

// SubscriptionCanceller cancels a Stripe subscription.
type SubscriptionCanceller interface {
	CancelSubscription(ctx context.Context, stripeSubID string) error
}

// NewDeleteAccountHandler returns a handler for DELETE /api/v1/me.
func NewDeleteAccountHandler(userRepo user.Repository, canceller SubscriptionCanceller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.UserFromContext(r.Context())
		if !ok {
			writeError(w, "unauthorized", "Authentication required", http.StatusUnauthorized)
			return
		}

		// Cancel Stripe subscription if exists
		sub, err := userRepo.FindSubscriptionByUserID(r.Context(), claims.UserID)
		if err == nil && sub.Status == model.SubscriptionActive {
			if cancelErr := canceller.CancelSubscription(r.Context(), sub.StripeSubscriptionID); cancelErr != nil {
				slog.Error("cancel stripe subscription during account deletion", "error", cancelErr)
			}
		} else if err != nil && !errors.Is(err, model.ErrNotFound) {
			slog.Error("find subscription for deletion", "error", err)
		}

		// Delete user and subscription records
		if err := userRepo.DeleteUser(r.Context(), claims.UserID); err != nil {
			if errors.Is(err, model.ErrNotFound) {
				writeError(w, "not_found", "User not found", http.StatusNotFound)
			} else {
				slog.Error("delete user account", "error", err)
				writeError(w, "internal_error", "Failed to delete account", http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

Note: `SubscriptionCanceller` is nil-safe — if billing is disabled (dev mode), pass `nil` and skip cancellation. Update the handler to accept `SubscriptionCanceller` as a nilable parameter.

Actually, better approach: make the parameter optional by checking for nil:
```go
if canceller != nil {
    // cancel logic
}
```

**Step 5: Register route in main.go**

In `backend/cmd/secretdrop/main.go`, after the contact handler registration:
```go
// Account deletion (auth required)
mux.HandleFunc("DELETE /api/v1/me", handler.NewDeleteAccountHandler(userRepo, nil))
```

Then update `registerBillingRoutes` to replace the nil canceller when billing is available. Simpler approach: register it once with nil, and when billing is enabled, register it again (last registration wins in Go's ServeMux).

Even simpler: declare a `var canceller handler.SubscriptionCanceller` before billing routes, set it in `registerBillingRoutes` if billing is enabled, then register the delete handler after billing setup.

**Step 6: Run tests and lint**

```bash
cd backend && go build ./... && go test -race ./... && golangci-lint run ./...
```

**Step 7: Commit**

```bash
git add backend/internal/user/user.go backend/internal/user/sqlite/sqlite.go backend/internal/billing/billing.go backend/internal/handler/account.go backend/cmd/secretdrop/main.go
git commit -m "add DELETE /api/v1/me endpoint for account deletion"
```

---

### Task 4: Add Delete Account Frontend (Modal + API)

**Files:**
- Modify: `frontend/src/api/client.ts` (add deleteAccount method)
- Modify: `frontend/src/components/Layout.tsx` (add Delete Account button + confirmation modal)

**Step 1: Add API method**

In `frontend/src/api/client.ts`, add to the `api` object:
```typescript
deleteAccount: () =>
  request<void>("/me", { method: "DELETE" }),
```

**Step 2: Add delete modal to Layout.tsx**

In the avatar dropdown menu (Layout.tsx), add a "Delete Account" button after "Sign Out". When clicked, it opens a confirmation modal showing:
- Title: "Delete your account?"
- Info: "You have {remaining} of {limit} secrets remaining"
- If tier is "pro": "Your Pro subscription will be cancelled."
- Warning: "This action is permanent and cannot be undone."
- Thanks: "Thank you for using SecretDrop."
- Buttons: "Keep My Account" (secondary) | "Delete My Account" (red)

On confirm: call `api.deleteAccount()`, then `auth.logout()`.

**Step 3: Run checks**

```bash
cd frontend && npx tsc --noEmit && npx eslint .
```

**Step 4: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/components/Layout.tsx
git commit -m "add delete account modal with confirmation dialog"
```

---

### Task 5: Update Terms & Privacy Pages

**Files:**
- Modify: `frontend/src/pages/Terms.tsx` (add subscription, refund, deletion sections)
- Modify: `frontend/src/pages/Privacy.tsx` (add deletion data handling)

**Step 1: Update Terms.tsx**

Add new sections (renumber existing sections accordingly):

- **Subscription & Billing**: Free tier (5 lifetime secrets), Pro tier ($2.99/month, 100 secrets/month). Managed through Stripe.
- **Cancellation**: Cancel anytime via billing portal. Access continues until end of billing period.
- **Refund Policy**: All payments are non-refundable. Cancellation retains access until period end.
- **Account Deletion**: You may delete your account at any time. This permanently removes your account data and cancels any active subscription. Existing shared secrets will expire on their original schedule.

**Step 2: Update Privacy.tsx**

Add to "Data Retention" and "Your Rights" sections:
- Account deletion removes all user data and subscription records immediately
- Shared secrets are not linked to your account and expire independently

**Step 3: Run checks**

```bash
cd frontend && npx tsc --noEmit
```

**Step 4: Commit**

```bash
git add frontend/src/pages/Terms.tsx frontend/src/pages/Privacy.tsx
git commit -m "update terms with subscription, refund, and deletion policies"
```

---

### Task 6: Verify Everything

**Step 1: Full backend test suite**

```bash
cd backend && go test -race ./... && golangci-lint run ./...
```

**Step 2: Full frontend build**

```bash
cd frontend && npx tsc --noEmit && npx eslint . && npx vite build
```

**Step 3: Run pre-commit hooks**

```bash
pre-commit run --all-files
```
