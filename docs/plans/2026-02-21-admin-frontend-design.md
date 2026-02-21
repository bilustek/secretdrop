# Admin Panel Frontend Design

## Overview

Add an admin panel to the React frontend for managing users and subscriptions.
The admin panel uses HTTP Basic Auth (separate from the main OAuth flow) and
lives under `/admin/*` routes with its own layout.

## File Structure

```
src/
├── api/
│   └── admin.ts               # Admin API client (Basic Auth)
├── components/
│   ├── AdminLayout.tsx         # Admin header + nav + outlet
│   └── ConfirmModal.tsx        # Reusable confirmation dialog
├── pages/
│   └── admin/
│       ├── Login.tsx           # Username/password login form
│       ├── Users.tsx           # User list with search/filter/tier update
│       └── Subscriptions.tsx   # Subscription list with filter/cancel
```

## Routing

```
/admin/login            → AdminLogin (public)
/admin                  → AdminLayout (redirects to login if no credentials)
  /admin/users          → AdminUsers
  /admin/subscriptions  → AdminSubscriptions
```

`/admin` root redirects to `/admin/users`.

## Authentication

- Separate login page at `/admin/login` with username/password form.
- Credentials validated by calling `GET /api/v1/admin/users?per_page=1`.
  If 200 OK, credentials are valid; if 401, they are rejected.
- Credentials stored in `sessionStorage` under `admin_credentials` key
  (cleared when browser tab closes).
- All admin API calls include `Authorization: Basic base64(user:pass)` header.
- AdminLayout checks for credentials on mount; redirects to login if missing.
- Logout clears sessionStorage and redirects to `/admin/login`.

## API Client (`api/admin.ts`)

Functions:
- `adminLogin(username, password)` — validate credentials
- `adminFetchUsers(params)` — GET /api/v1/admin/users
- `adminUpdateTier(id, tier)` — PATCH /api/v1/admin/users/{id}
- `adminFetchSubscriptions(params)` — GET /api/v1/admin/subscriptions
- `adminCancelSubscription(id)` — DELETE /api/v1/admin/subscriptions/{id}

All functions read credentials from sessionStorage and add Basic Auth header.

## Pages

### Users Page

**Top bar:**
- Search input (debounced 300ms, searches by email/name)
- Tier filter dropdown: All / Free / Pro
- Result count badge

**Table columns:** Email | Name | Provider | Tier | Secrets Used | Created At | Actions

- Provider shown as Google/GitHub icon
- Tier shown as badge (free=gray, pro=blue)
- Email and Created At columns are sortable (click to toggle asc/desc)
- Actions: button to toggle tier (free↔pro) with confirmation modal

**Bottom:** Prev/Next pagination with page info ("Page 1 of 3")

### Subscriptions Page

**Top bar:**
- Status filter dropdown: All / Active / Canceled
- Result count badge

**Table columns:** User Email | User Name | Stripe Sub ID | Status | Period Start | Period End | Created At | Actions

- Status shown as badge (active=green, canceled=red)
- Created At column is sortable
- Actions: Cancel button (only for active subscriptions) with confirmation modal

**Bottom:** Prev/Next pagination with page info

## Shared Components

### ConfirmModal
- Title, message, confirm/cancel buttons
- Used by both Users (tier change) and Subscriptions (cancel) pages

### AdminLayout
- Header: "SecretDrop Admin" title
- Navigation: Users | Subscriptions links (active state)
- Logout button
- Dark mode toggle (uses existing ThemeContext)
- Content area with `<Outlet />`

## Design Decisions

- **Dark mode:** Inherits from main app's ThemeContext
- **Error handling:** Inline alert messages (no toast library)
- **Loading states:** Spinner while data loads
- **Responsive:** Tables use `overflow-x-auto` for mobile horizontal scroll
- **No extra dependencies:** Uses existing stack (React, Tailwind, Lucide, React Router)
- **Debounce:** Search input debounced at 300ms before firing API call
- **Per page:** Fixed at 20 items per page
