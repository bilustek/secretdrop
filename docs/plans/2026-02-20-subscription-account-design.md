# Subscription & Account Management Design

**Date:** 2026-02-20

## Context

Dashboard, auth, and billing flows are complete. Need to fix subscription lifecycle issues, update free tier limits, add account deletion, and update legal pages.

## Decisions

### 1. Cancel Policy: Graceful (period end)

When a user cancels, they keep Pro access until the current billing period ends.

- Use Stripe `cancel_at_period_end` (already handled by Stripe Customer Portal)
- `customer.subscription.updated` webhook with `cancel_at_period_end=true` → keep tier as "pro"
- `customer.subscription.deleted` fires at actual period end → downgrade to "free"
- Frontend shows "Cancels on {date}" if subscription is pending cancellation

### 2. Fix Period Tracking in Webhooks

Current bug: `current_period_start` and `current_period_end` are stored as zero values.

- `handleCheckoutCompleted`: fetch subscription from Stripe API to get period dates
- `handleSubscriptionUpdated`: extract period from subscription event data
- `handleInvoicePaid`: extract period from invoice's subscription data

### 3. Free Tier: 5 secrets / lifetime

- Change `FreeTierLimit` from 1 to 5
- No reset mechanism needed (free users don't receive `invoice.paid` webhooks)
- Update landing page pricing section to reflect "5 secrets (lifetime)"

### 4. Delete Account: `DELETE /api/v1/me`

**Backend:**
- Auth required
- If active Stripe subscription exists: cancel it via Stripe API
- Delete user's secrets from repository
- Delete subscription record
- Delete user record
- Returns 204 No Content

**Frontend confirmation modal (avatar dropdown → "Delete Account"):**
- Title: "Delete your account?"
- Remaining credits: "You have {remaining} of {limit} secrets remaining"
- If Pro: "Your Pro subscription will be cancelled"
- Warning: "This action is permanent and cannot be undone."
- Thank you: "Thank you for using SecretDrop."
- Buttons: "Keep My Account" (secondary) | "Delete My Account" (red/danger)

### 5. Refund Policy: No refunds

All payments are non-refundable. Cancellation retains access until billing period ends.

### 6. Terms & Privacy Updates

**Terms additions:**
- Section on subscription management and cancellation
- Refund policy (no refunds, access until period end)
- Account deletion (immediate, permanent, data removed)
- Free tier description (5 lifetime secrets)

**Privacy additions:**
- Account deletion data handling
