# Admin Panel Frontend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an admin panel to the React frontend for managing users and subscriptions via Basic Auth.

**Architecture:** New `/admin/*` routes with a separate AdminLayout (own header + nav, no main app chrome). Admin API client uses Basic Auth with credentials stored in sessionStorage. No new dependencies — uses existing React, Tailwind, Lucide, React Router stack.

**Tech Stack:** React 19, TypeScript, Tailwind CSS 4, React Router 7, Lucide React, Vite

---

### Task 1: Admin API Client

**Files:**
- Create: `frontend/src/api/admin.ts`

**Step 1: Create admin API client with type definitions and all endpoint functions**

```typescript
const API_BASE = "/api/v1/admin"

const CREDENTIALS_KEY = "admin_credentials"

export interface AdminUser {
  id: number
  email: string
  name: string
  provider: string
  tier: string
  secrets_used: number
  created_at: string
}

export interface AdminUsersResponse {
  users: AdminUser[]
  total: number
  page: number
  per_page: number
}

export interface AdminSubscription {
  id: number
  user_id: number
  user_email: string
  user_name: string
  stripe_customer_id: string
  stripe_subscription_id: string
  status: string
  current_period_start: string
  current_period_end: string
  created_at: string
}

export interface AdminSubscriptionsResponse {
  subscriptions: AdminSubscription[]
  total: number
  page: number
  per_page: number
}

export interface UserListParams {
  q?: string
  tier?: string
  sort?: string
  order?: string
  page?: number
  per_page?: number
}

export interface SubscriptionListParams {
  q?: string
  status?: string
  sort?: string
  order?: string
  page?: number
  per_page?: number
}

function getCredentials(): string | null {
  return sessionStorage.getItem(CREDENTIALS_KEY)
}

export function setCredentials(username: string, password: string) {
  sessionStorage.setItem(CREDENTIALS_KEY, btoa(`${username}:${password}`))
}

export function clearCredentials() {
  sessionStorage.removeItem(CREDENTIALS_KEY)
}

export function hasCredentials(): boolean {
  return sessionStorage.getItem(CREDENTIALS_KEY) !== null
}

async function adminRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const encoded = getCredentials()
  if (!encoded) throw new Error("Not authenticated")

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    Authorization: `Basic ${encoded}`,
    ...((options.headers as Record<string, string>) ?? {}),
  }

  const res = await fetch(`${API_BASE}${path}`, { ...options, headers })

  if (res.status === 401) {
    clearCredentials()
    throw new Error("Invalid credentials")
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: { message: "Request failed" } }))
    throw new Error(body.error?.message ?? "Request failed")
  }

  if (res.status === 204) return undefined as T

  return res.json() as Promise<T>
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const entries = Object.entries(params).filter(([, v]) => v !== undefined && v !== "")
  if (entries.length === 0) return ""
  return "?" + new URLSearchParams(entries.map(([k, v]) => [k, String(v)])).toString()
}

export const adminApi = {
  login: (username: string, password: string) => {
    setCredentials(username, password)
    return adminRequest<AdminUsersResponse>(`/users?per_page=1`)
  },

  fetchUsers: (params: UserListParams = {}) =>
    adminRequest<AdminUsersResponse>(`/users${buildQuery(params)}`),

  updateTier: (id: number, tier: string) =>
    adminRequest<{ status: string }>(`/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ tier }),
    }),

  fetchSubscriptions: (params: SubscriptionListParams = {}) =>
    adminRequest<AdminSubscriptionsResponse>(`/subscriptions${buildQuery(params)}`),

  cancelSubscription: (id: number) =>
    adminRequest<void>(`/subscriptions/${id}`, { method: "DELETE" }),
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```
git add frontend/src/api/admin.ts
git commit -m "feat(admin): add admin API client with Basic Auth"
```

---

### Task 2: ConfirmModal Component

**Files:**
- Create: `frontend/src/components/ConfirmModal.tsx`

**Step 1: Create reusable confirmation modal component**

Follow the existing modal pattern from `Layout.tsx` (fixed overlay, backdrop blur, click-outside to close, Escape to close). Props: `title`, `message`, `confirmLabel`, `confirmVariant` (danger/default), `onConfirm`, `onCancel`.

```typescript
import { useEffect } from "react"

interface ConfirmModalProps {
  title: string
  message: string
  confirmLabel?: string
  confirmVariant?: "danger" | "default"
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmModal({
  title,
  message,
  confirmLabel = "Confirm",
  confirmVariant = "default",
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onCancel])

  const confirmClass =
    confirmVariant === "danger"
      ? "bg-red-600 text-white hover:bg-red-700"
      : "bg-gray-900 text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onCancel}
      role="presentation"
    >
      <div
        className="max-w-md w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
        onClick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
        <p className="mt-3 text-sm text-gray-600 dark:text-gray-400">{message}</p>
        <div className="mt-6 flex gap-3 justify-end">
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            Cancel
          </button>
          <button type="button" onClick={onConfirm} className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${confirmClass}`}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```
git add frontend/src/components/ConfirmModal.tsx
git commit -m "feat(admin): add reusable ConfirmModal component"
```

---

### Task 3: AdminLayout Component

**Files:**
- Create: `frontend/src/components/AdminLayout.tsx`

**Step 1: Create admin layout with header, nav, and auth guard**

Admin layout has its own header with "SecretDrop Admin" branding, nav links for Users and Subscriptions (with active state), logout button, and dark mode toggle. If no credentials in sessionStorage, redirects to `/admin/login`. Uses `<Outlet />` for nested routes.

```typescript
import { Link, Navigate, Outlet, useLocation } from "react-router"
import { LogOut, Shield, Users, CreditCard } from "lucide-react"
import { ThemeToggle } from "./ThemeToggle"
import { hasCredentials, clearCredentials } from "../api/admin"

export function AdminLayout() {
  const { pathname } = useLocation()

  if (!hasCredentials()) {
    return <Navigate to="/admin/login" replace />
  }

  const handleLogout = () => {
    clearCredentials()
    window.location.href = "/admin/login"
  }

  const navLinks = [
    { to: "/admin/users", label: "Users", icon: Users },
    { to: "/admin/subscriptions", label: "Subscriptions", icon: CreditCard },
  ]

  return (
    <div className="min-h-screen flex flex-col bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
      <header className="border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/admin/users" className="flex items-center gap-2 font-semibold text-lg">
              <Shield size={20} />
              SecretDrop Admin
            </Link>
            <nav className="hidden sm:flex items-center gap-1">
              {navLinks.map(({ to, label, icon: Icon }) => (
                <Link
                  key={to}
                  to={to}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    pathname === to
                      ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white"
                      : "text-gray-500 hover:text-gray-900 dark:hover:text-white"
                  }`}
                >
                  <Icon size={16} />
                  {label}
                </Link>
              ))}
            </nav>
          </div>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <button
              type="button"
              onClick={handleLogout}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-gray-500 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              <LogOut size={16} />
              <span className="hidden sm:inline">Logout</span>
            </button>
          </div>
        </div>
      </header>

      {/* Mobile nav */}
      <div className="sm:hidden border-b border-gray-200 dark:border-gray-800">
        <div className="flex">
          {navLinks.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
                pathname === to
                  ? "border-b-2 border-gray-900 dark:border-white text-gray-900 dark:text-white"
                  : "text-gray-500 hover:text-gray-900 dark:hover:text-white"
              }`}
            >
              <Icon size={16} />
              {label}
            </Link>
          ))}
        </div>
      </div>

      <main className="flex-1">
        <div className="max-w-6xl mx-auto px-4 py-6">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```
git add frontend/src/components/AdminLayout.tsx
git commit -m "feat(admin): add AdminLayout with header, nav, and auth guard"
```

---

### Task 4: Admin Login Page

**Files:**
- Create: `frontend/src/pages/admin/Login.tsx`

**Step 1: Create admin login page with username/password form**

Simple centered form with username and password fields. On submit, calls `adminApi.login()`. On success, redirects to `/admin/users`. On failure, shows inline error message. If already authenticated, redirects to `/admin/users`.

```typescript
import { useState } from "react"
import { useNavigate, Navigate } from "react-router"
import { Shield } from "lucide-react"
import { adminApi, hasCredentials, clearCredentials } from "../../api/admin"

export default function AdminLogin() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [isLoading, setIsLoading] = useState(false)

  if (hasCredentials()) {
    return <Navigate to="/admin/users" replace />
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setIsLoading(true)

    try {
      await adminApi.login(username, password)
      navigate("/admin/users", { replace: true })
    } catch {
      clearCredentials()
      setError("Invalid username or password")
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-white dark:bg-gray-950 px-4">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <Shield size={32} className="mx-auto mb-3 text-gray-900 dark:text-white" />
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Admin Panel</h1>
          <p className="mt-1 text-sm text-gray-500">Sign in to manage SecretDrop</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-400">
              {error}
            </div>
          )}

          <div>
            <label htmlFor="username" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Username
            </label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              autoFocus
              autoComplete="username"
              className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
            />
          </div>

          <div>
            <label htmlFor="password" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Password
            </label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="current-password"
              className="w-full rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
            />
          </div>

          <button
            type="submit"
            disabled={isLoading}
            className="w-full rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 py-2 text-sm font-medium hover:bg-gray-800 dark:hover:bg-gray-200 transition-colors disabled:opacity-50"
          >
            {isLoading ? "Signing in..." : "Sign In"}
          </button>
        </form>
      </div>
    </div>
  )
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```
git add frontend/src/pages/admin/Login.tsx
git commit -m "feat(admin): add admin login page"
```

---

### Task 5: Admin Users Page

**Files:**
- Create: `frontend/src/pages/admin/Users.tsx`

**Step 1: Create users page with search, filter, sort, pagination, and tier update**

Features:
- Search input with 300ms debounce
- Tier filter dropdown (All/Free/Pro)
- Sortable columns (Email, Created At) with click toggle
- Pagination (Prev/Next + page info)
- Tier change button opens ConfirmModal, calls `adminApi.updateTier()`
- Inline error/loading states
- Provider shown as text badge (Google/GitHub)
- Tier shown as styled badge

Reference `frontend/src/pages/Dashboard.tsx` for existing state management patterns (useState, loading, error states).

The debounce should be done with a simple `useEffect` + `setTimeout` pattern (no external library).

Table component rendered inline (no separate component).

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Verify it renders correctly in the browser**

Run: `cd frontend && npm run dev`
Navigate to: `http://localhost:3000/admin/users` (after login)
Expected: Users table with search, filter, sort, and pagination working

**Step 4: Commit**

```
git add frontend/src/pages/admin/Users.tsx
git commit -m "feat(admin): add users management page"
```

---

### Task 6: Admin Subscriptions Page

**Files:**
- Create: `frontend/src/pages/admin/Subscriptions.tsx`

**Step 1: Create subscriptions page with filter, sort, pagination, and cancel**

Features:
- Status filter dropdown (All/Active/Canceled)
- Sortable Created At column
- Pagination (Prev/Next + page info)
- Cancel button (only for active) opens ConfirmModal, calls `adminApi.cancelSubscription()`
- Inline error/loading states
- Status shown as colored badge (active=green, canceled=red)
- Dates formatted as locale strings

Follow the same patterns as Users page for consistency.

**Step 2: Verify TypeScript compiles**

Run: `cd frontend && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```
git add frontend/src/pages/admin/Subscriptions.tsx
git commit -m "feat(admin): add subscriptions management page"
```

---

### Task 7: Wire Routes in App.tsx

**Files:**
- Modify: `frontend/src/App.tsx`

**Step 1: Add admin routes to App.tsx**

Import the new pages and AdminLayout. Add admin routes outside the main `<Layout>` route group (similar to how AuthCallback is outside Layout). Add a redirect from `/admin` to `/admin/users`.

```typescript
// Add imports at top:
import { AdminLayout } from "./components/AdminLayout"
import AdminLogin from "./pages/admin/Login"
import AdminUsers from "./pages/admin/Users"
import AdminSubscriptions from "./pages/admin/Subscriptions"
import { Navigate } from "react-router"

// Add inside <Routes>, after the existing auth callback route:
<Route path="admin/login" element={<AdminLogin />} />
<Route path="admin" element={<AdminLayout />}>
  <Route index element={<Navigate to="/admin/users" replace />} />
  <Route path="users" element={<AdminUsers />} />
  <Route path="subscriptions" element={<AdminSubscriptions />} />
</Route>
```

**Step 2: Verify TypeScript compiles and lint passes**

Run: `cd frontend && npx tsc --noEmit && npx eslint .`
Expected: No errors

**Step 3: Verify full flow in the browser**

1. Navigate to `http://localhost:3000/admin` — should redirect to `/admin/login`
2. Enter credentials — should redirect to `/admin/users`
3. Click Subscriptions nav link — should navigate to `/admin/subscriptions`
4. Click Logout — should return to `/admin/login`
5. Verify dark mode toggle works in admin panel

**Step 4: Commit**

```
git add frontend/src/App.tsx
git commit -m "feat(admin): wire admin routes in App.tsx"
```

---

### Task 8: Build Verification and Cleanup

**Files:**
- None new, verification only

**Step 1: Run full build**

Run: `cd frontend && npm run build`
Expected: Build succeeds with no errors

**Step 2: Run lint**

Run: `cd frontend && npx eslint .`
Expected: No errors

**Step 3: Final commit if any cleanup needed**

Only commit if lint/build required fixes.
