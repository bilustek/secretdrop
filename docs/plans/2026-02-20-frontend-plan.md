# SecretDrop Frontend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a React SPA frontend for SecretDrop with OAuth login, secret creation, secret reveal, and Stripe billing integration.

**Architecture:** Vite-bundled React 19 SPA with React Router v7 for client-side routing, Tailwind CSS v4.2 for styling, and React Context for auth/theme state. Native fetch for API calls. Dark/light mode support.

**Tech Stack:** React 19, TypeScript, Vite, Tailwind CSS 4.2, React Router 7.13, Lucide React

**Design doc:** `docs/plans/2026-02-20-frontend-design.md`

---

## Task 0: Backend — OAuth Callback Redirect

The OAuth callbacks currently return JSON (`writeJSON`). For the web frontend, the browser navigates to the provider and returns to the callback URL — it needs a redirect to the frontend, not a JSON response.

**Files:**
- Modify: `backend/internal/auth/google.go:73-145` (HandleGoogleCallback)
- Modify: `backend/internal/auth/github.go:79-172` (HandleGithubCallback)
- Modify: `backend/internal/auth/auth.go` (add frontendBaseURL to Service)
- Modify: `backend/cmd/secretdrop/main.go` (pass frontendBaseURL to auth service)
- Test: `backend/internal/auth/auth_test.go`

**Step 1: Add frontendURL option to auth Service**

In `backend/internal/auth/auth.go`, add a `frontendBaseURL` field to `Service` and a `WithFrontendBaseURL` option:

```go
type Service struct {
	secret           []byte
	accessExpiry     time.Duration
	refreshExpiry    time.Duration
	googleClientID   string
	frontendBaseURL  string
}

func WithFrontendBaseURL(url string) Option {
	return func(s *Service) error {
		s.frontendBaseURL = url
		return nil
	}
}
```

**Step 2: Add redirectWithTokens helper**

In `backend/internal/auth/google.go` (alongside the existing `writeJSON` helper), add:

```go
func (s *Service) redirectWithTokens(w http.ResponseWriter, r *http.Request, pair *TokenPair) {
	u, _ := url.Parse(s.frontendBaseURL)
	u.Path = "/auth/callback"
	q := u.Query()
	q.Set("access_token", pair.AccessToken)
	q.Set("refresh_token", pair.RefreshToken)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusTemporaryRedirect)
}
```

Add `"net/url"` to imports.

**Step 3: Update HandleGoogleCallback to redirect**

Replace the last line of `HandleGoogleCallback` (line 144):
```go
// Before:
writeJSON(w, http.StatusOK, pair)

// After:
s.redirectWithTokens(w, r, pair)
```

**Step 4: Update HandleGithubCallback to redirect**

Replace the last line of `HandleGithubCallback` (line 170):
```go
// Before:
writeJSON(w, http.StatusOK, pair)

// After:
s.redirectWithTokens(w, r, pair)
```

**Step 5: Pass frontendBaseURL in main.go**

In `backend/cmd/secretdrop/main.go`, when creating the auth service, add the option:

```go
authSvc, err := auth.New(
	cfg.JWTSecret(),
	auth.WithGoogleClientID(cfg.GoogleClientID()),
	auth.WithFrontendBaseURL(cfg.FrontendBaseURL()),
)
```

**Step 6: Run backend tests**

Run: `cd backend && go test -race ./...`
Expected: All tests pass.

**Step 7: Run linter**

Run: `cd backend && golangci-lint run ./...`
Expected: No issues.

**Step 8: Commit**

```bash
git add backend/internal/auth/ backend/cmd/secretdrop/main.go
git commit -m "redirect OAuth callbacks to frontend with tokens in query params"
```

---

## Task 1: Project Scaffolding

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/tsconfig.json`
- Create: `frontend/tsconfig.app.json`
- Create: `frontend/tsconfig.node.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/index.html`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/index.css`
- Create: `frontend/src/vite-env.d.ts`

**Step 1: Scaffold Vite project**

```bash
cd frontend
npm create vite@latest . -- --template react-ts
```

If the directory has `.gitkeep`, remove it first: `rm .gitkeep`

**Step 2: Install dependencies**

```bash
cd frontend
npm install react-router@7.13
npm install tailwindcss @tailwindcss/vite lucide-react
```

**Step 3: Configure Vite with Tailwind plugin**

Replace `frontend/vite.config.ts`:

```typescript
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      "/api": "http://localhost:8080",
      "/auth": "http://localhost:8080",
      "/billing": "http://localhost:8080",
    },
  },
})
```

**Step 4: Set up Tailwind CSS v4 with dark mode and theme**

Replace `frontend/src/index.css`:

```css
@import "tailwindcss";

@custom-variant dark (&:where(.dark, .dark *));

@theme {
  --font-sans: "Inter", system-ui, -apple-system, sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, monospace;
}
```

**Step 5: Create minimal App.tsx**

```tsx
export default function App() {
  return (
    <div className="min-h-screen bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
      <h1 className="text-2xl font-bold p-8">SecretDrop</h1>
    </div>
  )
}
```

**Step 6: Create main.tsx**

```tsx
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import App from "./App"
import "./index.css"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

**Step 7: Add Inter font to index.html**

In `frontend/index.html` `<head>`, add:

```html
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet" />
```

**Step 8: Verify dev server**

```bash
cd frontend && npm run dev
```

Expected: Dev server starts on http://localhost:3000, shows "SecretDrop" heading. Tailwind classes working.

**Step 9: Commit**

```bash
git add frontend/
git commit -m "scaffold frontend with Vite, React 19, Tailwind v4, React Router v7"
```

---

## Task 2: Theme System (Dark/Light Mode)

**Files:**
- Create: `frontend/src/context/ThemeContext.tsx`
- Create: `frontend/src/components/ThemeToggle.tsx`
- Modify: `frontend/index.html` (add inline dark mode script)

**Step 1: Add dark mode init script to index.html**

In `frontend/index.html`, add this inline script inside `<head>` (prevents flash of wrong theme):

```html
<script>
  document.documentElement.classList.toggle(
    "dark",
    localStorage.theme === "dark" ||
      (!("theme" in localStorage) &&
        window.matchMedia("(prefers-color-scheme: dark)").matches)
  );
</script>
```

**Step 2: Create ThemeContext**

`frontend/src/context/ThemeContext.tsx`:

```tsx
import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"

type Theme = "light" | "dark"

interface ThemeContextValue {
  theme: Theme
  toggle: () => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

function getInitialTheme(): Theme {
  if (typeof window === "undefined") return "light"
  if (localStorage.theme === "dark") return "dark"
  if (!("theme" in localStorage) && window.matchMedia("(prefers-color-scheme: dark)").matches) return "dark"
  return "light"
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark")
    localStorage.theme = theme
  }, [theme])

  const toggle = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"))
  }, [])

  return (
    <ThemeContext value={{ theme, toggle }}>
      {children}
    </ThemeContext>
  )
}
```

**Step 3: Create ThemeToggle component**

`frontend/src/components/ThemeToggle.tsx`:

```tsx
import { use } from "react"
import { Moon, Sun } from "lucide-react"
import { ThemeContext } from "../context/ThemeContext"

export function ThemeToggle() {
  const ctx = use(ThemeContext)
  if (!ctx) throw new Error("ThemeToggle must be used within ThemeProvider")

  return (
    <button
      type="button"
      onClick={ctx.toggle}
      className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
      aria-label={ctx.theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
    >
      {ctx.theme === "dark" ? <Sun size={20} /> : <Moon size={20} />}
    </button>
  )
}
```

**Step 4: Wire ThemeProvider into main.tsx**

```tsx
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { ThemeProvider } from "./context/ThemeContext"
import App from "./App"
import "./index.css"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <App />
    </ThemeProvider>
  </StrictMode>,
)
```

**Step 5: Verify toggle works**

Run dev server, toggle dark/light mode, verify classes toggle on `<html>`, verify persistence after page refresh.

**Step 6: Commit**

```bash
git add frontend/src/context/ThemeContext.tsx frontend/src/components/ThemeToggle.tsx frontend/src/main.tsx frontend/index.html
git commit -m "add dark/light theme system with localStorage persistence"
```

---

## Task 3: API Client

**Files:**
- Create: `frontend/src/api/client.ts`

**Step 1: Create the API client**

`frontend/src/api/client.ts`:

```typescript
const API_BASE = "/api/v1"

interface ApiError {
  error: {
    type: string
    message: string
  }
}

export class AppError extends Error {
  constructor(
    public type: string,
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = "AppError"
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem("access_token")

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  })

  if (res.status === 401) {
    localStorage.removeItem("access_token")
    localStorage.removeItem("refresh_token")
    window.location.href = "/"
    throw new AppError("unauthorized", "Session expired", 401)
  }

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

// --- API Types ---

export interface MeResponse {
  email: string
  name: string
  avatar_url: string
  tier: string
  secrets_used: number
  secrets_limit: number
}

export interface CreateSecretRequest {
  text: string
  to: string[]
}

export interface RecipientLink {
  email: string
  link: string
}

export interface CreateSecretResponse {
  id: string
  expires_at: string
  recipients: RecipientLink[]
}

export interface RevealRequest {
  email: string
  key: string
}

export interface RevealResponse {
  text: string
}

export interface CheckoutResponse {
  url: string
}

// --- API Functions ---

export const api = {
  me: () => request<MeResponse>("/me"),

  createSecret: (data: CreateSecretRequest) =>
    request<CreateSecretResponse>("/secrets", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  revealSecret: (token: string, data: RevealRequest) =>
    request<RevealResponse>(`/secrets/${token}/reveal`, {
      method: "POST",
      body: JSON.stringify(data),
    }),

  checkout: () =>
    fetch("/billing/checkout", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${localStorage.getItem("access_token")}`,
        "Content-Type": "application/json",
      },
    }).then((r) => r.json() as Promise<CheckoutResponse>),

  portal: () =>
    fetch("/billing/portal", {
      method: "POST",
      headers: {
        Authorization: `Bearer ${localStorage.getItem("access_token")}`,
        "Content-Type": "application/json",
      },
    }).then((r) => r.json() as Promise<{ url: string }>),
}
```

Note: `checkout` and `portal` use `/billing/` prefix (not `/api/v1/`), so they call `fetch` directly.

**Step 2: Verify it compiles**

```bash
cd frontend && npx tsc --noEmit
```

Expected: No type errors.

**Step 3: Commit**

```bash
git add frontend/src/api/client.ts
git commit -m "add typed API client with JWT auth and error handling"
```

---

## Task 4: Auth Context

**Files:**
- Create: `frontend/src/context/AuthContext.tsx`
- Modify: `frontend/src/main.tsx` (wrap with AuthProvider)

**Step 1: Create AuthContext**

`frontend/src/context/AuthContext.tsx`:

```tsx
import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"
import { api, type MeResponse } from "../api/client"

interface AuthContextValue {
  user: MeResponse | null
  isAuthenticated: boolean
  isLoading: boolean
  login: (accessToken: string, refreshToken: string) => void
  logout: () => void
  refreshUser: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const fetchUser = useCallback(async () => {
    const token = localStorage.getItem("access_token")
    if (!token) {
      setIsLoading(false)
      return
    }
    try {
      const me = await api.me()
      setUser(me)
    } catch {
      localStorage.removeItem("access_token")
      localStorage.removeItem("refresh_token")
      setUser(null)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  const login = useCallback((accessToken: string, refreshToken: string) => {
    localStorage.setItem("access_token", accessToken)
    localStorage.setItem("refresh_token", refreshToken)
  }, [])

  const logout = useCallback(() => {
    localStorage.removeItem("access_token")
    localStorage.removeItem("refresh_token")
    setUser(null)
  }, [])

  return (
    <AuthContext value={{ user, isAuthenticated: !!user, isLoading, login, logout, refreshUser: fetchUser }}>
      {children}
    </AuthContext>
  )
}
```

**Step 2: Wire AuthProvider into main.tsx**

```tsx
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { ThemeProvider } from "./context/ThemeContext"
import { AuthProvider } from "./context/AuthContext"
import App from "./App"
import "./index.css"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider>
      <AuthProvider>
        <App />
      </AuthProvider>
    </ThemeProvider>
  </StrictMode>,
)
```

**Step 3: Verify it compiles**

```bash
cd frontend && npx tsc --noEmit
```

**Step 4: Commit**

```bash
git add frontend/src/context/AuthContext.tsx frontend/src/main.tsx
git commit -m "add auth context with token management and user fetch"
```

---

## Task 5: Layout, Router, and Protected Routes

**Files:**
- Create: `frontend/src/components/Layout.tsx`
- Create: `frontend/src/components/ProtectedRoute.tsx`
- Create: `frontend/src/pages/Landing.tsx` (placeholder)
- Create: `frontend/src/pages/Dashboard.tsx` (placeholder)
- Create: `frontend/src/pages/Reveal.tsx` (placeholder)
- Create: `frontend/src/pages/AuthCallback.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/main.tsx`

**Step 1: Create AuthCallback page**

`frontend/src/pages/AuthCallback.tsx`:

```tsx
import { useEffect } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"

export default function AuthCallback() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    const accessToken = params.get("access_token")
    const refreshToken = params.get("refresh_token")

    if (accessToken && refreshToken && auth) {
      auth.login(accessToken, refreshToken)
      auth.refreshUser().then(() => navigate("/dashboard", { replace: true }))
    } else {
      navigate("/", { replace: true })
    }
  }, [params, auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Signing in...</p>
    </div>
  )
}
```

**Step 2: Create ProtectedRoute**

`frontend/src/components/ProtectedRoute.tsx`:

```tsx
import { use } from "react"
import { Navigate, Outlet } from "react-router"
import { AuthContext } from "../context/AuthContext"

export function ProtectedRoute() {
  const auth = use(AuthContext)
  if (!auth) throw new Error("ProtectedRoute must be within AuthProvider")

  if (auth.isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <p className="text-gray-500">Loading...</p>
      </div>
    )
  }

  if (!auth.isAuthenticated) {
    return <Navigate to="/" replace />
  }

  return <Outlet />
}
```

**Step 3: Create Layout**

`frontend/src/components/Layout.tsx`:

```tsx
import { use } from "react"
import { Link, Outlet } from "react-router"
import { Lock } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { ThemeToggle } from "./ThemeToggle"

export function Layout() {
  const auth = use(AuthContext)
  if (!auth) throw new Error("Layout must be within AuthProvider")

  return (
    <div className="min-h-screen bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
      <header className="border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-5xl mx-auto px-4 h-16 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 font-semibold text-lg">
            <Lock size={20} />
            SecretDrop
          </Link>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            {auth.isAuthenticated && auth.user && (
              <div className="flex items-center gap-3">
                <Link to="/dashboard" className="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100">
                  Dashboard
                </Link>
                <img
                  src={auth.user.avatar_url}
                  alt={auth.user.name}
                  className="w-8 h-8 rounded-full"
                />
              </div>
            )}
          </div>
        </div>
      </header>

      <main>
        <Outlet />
      </main>

      <footer className="border-t border-gray-200 dark:border-gray-800 py-8 mt-auto">
        <div className="max-w-5xl mx-auto px-4 text-center text-sm text-gray-500">
          SecretDrop — Encrypted one-time secret sharing
        </div>
      </footer>
    </div>
  )
}
```

**Step 4: Create placeholder pages**

`frontend/src/pages/Landing.tsx`:

```tsx
export default function Landing() {
  return <div className="p-8">Landing — TODO</div>
}
```

`frontend/src/pages/Dashboard.tsx`:

```tsx
export default function Dashboard() {
  return <div className="p-8">Dashboard — TODO</div>
}
```

`frontend/src/pages/Reveal.tsx`:

```tsx
export default function Reveal() {
  return <div className="p-8">Reveal — TODO</div>
}
```

**Step 5: Wire up router in App.tsx**

```tsx
import { Routes, Route } from "react-router"
import { Layout } from "./components/Layout"
import { ProtectedRoute } from "./components/ProtectedRoute"
import Landing from "./pages/Landing"
import Dashboard from "./pages/Dashboard"
import Reveal from "./pages/Reveal"
import AuthCallback from "./pages/AuthCallback"

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Landing />} />
        <Route path="s/:token" element={<Reveal />} />
        <Route element={<ProtectedRoute />}>
          <Route path="dashboard" element={<Dashboard />} />
        </Route>
      </Route>
      <Route path="auth/callback" element={<AuthCallback />} />
    </Routes>
  )
}
```

**Step 6: Wrap with BrowserRouter in main.tsx**

```tsx
import { StrictMode } from "react"
import { createRoot } from "react-dom/client"
import { BrowserRouter } from "react-router"
import { ThemeProvider } from "./context/ThemeContext"
import { AuthProvider } from "./context/AuthContext"
import App from "./App"
import "./index.css"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <ThemeProvider>
        <AuthProvider>
          <App />
        </AuthProvider>
      </ThemeProvider>
    </BrowserRouter>
  </StrictMode>,
)
```

**Step 7: Verify navigation works**

Run dev server. Check:
- `/` shows Landing placeholder
- `/dashboard` redirects to `/` (not logged in)
- `/s/test` shows Reveal placeholder
- Dark/light toggle works in nav

**Step 8: Commit**

```bash
git add frontend/src/
git commit -m "add router, layout, protected routes, and auth callback"
```

---

## Task 6: Landing Page

**Files:**
- Modify: `frontend/src/pages/Landing.tsx`

**Reference:** Design doc sections: Hero, Features (3 cards), Pricing (Free/Pro).

**Step 1: Implement Landing page**

Replace `frontend/src/pages/Landing.tsx`:

```tsx
import { use } from "react"
import { Navigate } from "react-router"
import { Lock, Mail, Flame, Shield } from "lucide-react"
import { AuthContext } from "../context/AuthContext"

const API_BASE = import.meta.env.DEV ? "" : ""

export default function Landing() {
  const auth = use(AuthContext)

  if (auth?.isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  return (
    <div>
      {/* Hero */}
      <section className="max-w-3xl mx-auto px-4 py-24 text-center">
        <h1 className="text-4xl sm:text-5xl font-bold tracking-tight">
          Share secrets that disappear
          <br />
          after one read.
        </h1>
        <p className="mt-4 text-lg text-gray-600 dark:text-gray-400">
          End-to-end encrypted. Zero-knowledge. One-time links.
        </p>
        <div className="mt-8 flex flex-col sm:flex-row gap-3 justify-center">
          <a
            href={`${API_BASE}/auth/google`}
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
          >
            Sign in with Google
          </a>
          <a
            href={`${API_BASE}/auth/github`}
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            Sign in with GitHub
          </a>
        </div>
      </section>

      {/* Features */}
      <section className="max-w-5xl mx-auto px-4 py-16">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Shield size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">AES-256-GCM Encryption</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Your secrets are encrypted before storage. The key never touches our servers.
            </p>
          </div>
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Mail size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">Share via Email</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Recipients get a one-time link. Only the intended recipient can decrypt.
            </p>
          </div>
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Flame size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">Burn After Reading</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Secrets are permanently deleted after viewing or when they expire.
            </p>
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section className="max-w-3xl mx-auto px-4 py-16">
        <h2 className="text-2xl font-bold text-center mb-8">Simple Pricing</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
          <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
            <h3 className="font-semibold text-lg">Free</h3>
            <p className="text-3xl font-bold mt-2">$0</p>
            <p className="text-sm text-gray-500 mt-1">forever</p>
            <ul className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <li>1 secret (lifetime)</li>
              <li>Up to 5 recipients</li>
              <li>AES-256-GCM encryption</li>
            </ul>
            <a
              href={`${API_BASE}/auth/google`}
              className="mt-6 block text-center px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              Get Started
            </a>
          </div>
          <div className="border-2 border-gray-900 dark:border-white rounded-xl p-6 relative">
            <span className="absolute -top-3 left-4 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-xs font-medium px-2 py-1 rounded">
              Popular
            </span>
            <h3 className="font-semibold text-lg">Pro</h3>
            <p className="text-3xl font-bold mt-2">$2.99</p>
            <p className="text-sm text-gray-500 mt-1">per month</p>
            <ul className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <li>100 secrets per month</li>
              <li>Up to 5 recipients</li>
              <li>AES-256-GCM encryption</li>
            </ul>
            <a
              href={`${API_BASE}/auth/google`}
              className="mt-6 block text-center px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
            >
              Start Free, Upgrade Later
            </a>
          </div>
        </div>
      </section>
    </div>
  )
}
```

**Step 2: Verify visually**

Run dev server. Check landing page renders correctly in both light and dark mode. Verify OAuth links point to `/auth/google` and `/auth/github`.

**Step 3: Commit**

```bash
git add frontend/src/pages/Landing.tsx
git commit -m "implement landing page with hero, features, and pricing sections"
```

---

## Task 7: Dashboard Page

**Files:**
- Modify: `frontend/src/pages/Dashboard.tsx`

**Step 1: Implement Dashboard with create form and usage stats**

Replace `frontend/src/pages/Dashboard.tsx`:

```tsx
import { use, useState, type FormEvent } from "react"
import { Plus, X, Loader2, Copy, Check } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { api, AppError, type CreateSecretResponse } from "../api/client"

export default function Dashboard() {
  const auth = use(AuthContext)
  if (!auth || !auth.user) return null

  const { user, refreshUser } = auth
  const [text, setText] = useState("")
  const [emails, setEmails] = useState<string[]>([])
  const [emailInput, setEmailInput] = useState("")
  const [result, setResult] = useState<CreateSecretResponse | null>(null)
  const [error, setError] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  const addEmail = () => {
    const trimmed = emailInput.trim()
    if (trimmed && !emails.includes(trimmed) && emails.length < 5) {
      setEmails([...emails, trimmed])
      setEmailInput("")
    }
  }

  const removeEmail = (email: string) => {
    setEmails(emails.filter((e) => e !== email))
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setIsSubmitting(true)

    try {
      const response = await api.createSecret({ text, to: emails })
      setResult(response)
      setText("")
      setEmails([])
      refreshUser()
    } catch (err) {
      if (err instanceof AppError) {
        if (err.type === "limit_reached") {
          setError("You've reached your secret limit. Upgrade to Pro for more.")
        } else {
          setError(err.message)
        }
      } else {
        setError("Something went wrong. Please try again.")
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleUpgrade = async () => {
    try {
      const { url } = await api.checkout()
      window.location.href = url
    } catch {
      setError("Failed to start checkout. Please try again.")
    }
  }

  const handleManageBilling = async () => {
    try {
      const { url } = await api.portal()
      window.location.href = url
    } catch {
      setError("Failed to open billing portal. Please try again.")
    }
  }

  if (result) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-16">
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-8">
          <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-4">
            <Check size={20} />
            <h2 className="font-semibold">Secret created and encrypted</h2>
          </div>
          <p className="text-sm text-gray-500 mb-6">
            Expires at {new Date(result.expires_at).toLocaleString()}
          </p>
          <div className="space-y-3">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Links sent to:</p>
            {result.recipients.map((r) => (
              <div key={r.email} className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-900">
                <span className="text-sm">{r.email}</span>
                <CopyButton text={r.link} />
              </div>
            ))}
          </div>
          <button
            type="button"
            onClick={() => setResult(null)}
            className="mt-6 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            Create Another
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-2xl mx-auto px-4 py-16">
      {/* Create Form */}
      <form onSubmit={handleSubmit}>
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
          <h2 className="font-semibold text-lg mb-4">Create a Secret</h2>

          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Enter your secret message..."
            maxLength={4096}
            rows={5}
            required
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-3 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white resize-none"
          />
          <p className="text-xs text-gray-400 mt-1 text-right">{text.length}/4096</p>

          <div className="mt-4">
            <label className="text-sm font-medium">Recipients</label>
            <div className="flex gap-2 mt-1">
              <input
                type="email"
                value={emailInput}
                onChange={(e) => setEmailInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addEmail() } }}
                placeholder="email@example.com"
                className="flex-1 rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
              />
              <button
                type="button"
                onClick={addEmail}
                disabled={emails.length >= 5}
                className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors disabled:opacity-50"
              >
                <Plus size={16} />
              </button>
            </div>
            {emails.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {emails.map((email) => (
                  <span key={email} className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-gray-100 dark:bg-gray-800 text-sm">
                    {email}
                    <button type="button" onClick={() => removeEmail(email)} className="hover:text-red-500">
                      <X size={14} />
                    </button>
                  </span>
                ))}
              </div>
            )}
            <p className="text-xs text-gray-400 mt-1">{emails.length}/5 recipients</p>
          </div>

          {error && (
            <p className="mt-4 text-sm text-red-600 dark:text-red-400">{error}</p>
          )}

          <button
            type="submit"
            disabled={isSubmitting || !text.trim() || emails.length === 0}
            className="mt-4 w-full px-4 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {isSubmitting && <Loader2 size={16} className="animate-spin" />}
            Create Secret
          </button>
        </div>
      </form>

      {/* Usage & Billing */}
      <div className="mt-8 border border-gray-200 dark:border-gray-800 rounded-xl p-6">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-gray-500">
              <span className="inline-block px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-800 mr-2 uppercase">
                {user.tier}
              </span>
              {user.secrets_used} / {user.secrets_limit} secrets used
            </p>
          </div>
          {user.tier === "free" ? (
            <button
              type="button"
              onClick={handleUpgrade}
              className="text-sm font-medium text-gray-900 dark:text-white hover:underline"
            >
              Upgrade to Pro →
            </button>
          ) : (
            <button
              type="button"
              onClick={handleManageBilling}
              className="text-sm text-gray-500 hover:underline"
            >
              Manage Billing
            </button>
          )}
        </div>
      </div>

      {/* Sign Out */}
      <div className="mt-4 text-center">
        <button
          type="button"
          onClick={auth.logout}
          className="text-sm text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
        >
          Sign Out
        </button>
      </div>
    </div>
  )
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button type="button" onClick={handleCopy} className="p-1 hover:text-gray-600 dark:hover:text-gray-300">
      {copied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
    </button>
  )
}
```

**Step 2: Verify visually**

Run dev server (logged-in state needed for full testing — mock or skip for now). Verify form layout, email chips, character counter.

**Step 3: Commit**

```bash
git add frontend/src/pages/Dashboard.tsx
git commit -m "implement dashboard with secret creation form, usage stats, and billing"
```

---

## Task 8: Reveal Page

**Files:**
- Modify: `frontend/src/pages/Reveal.tsx`

**Step 1: Implement Reveal page**

Replace `frontend/src/pages/Reveal.tsx`:

```tsx
import { useState, type FormEvent } from "react"
import { useParams, useLocation } from "react-router"
import { Lock, Copy, Check, Loader2, AlertTriangle } from "lucide-react"
import { api, AppError } from "../api/client"

export default function Reveal() {
  const { token } = useParams<{ token: string }>()
  const location = useLocation()
  const key = location.hash.slice(1) // remove leading #

  const [email, setEmail] = useState("")
  const [secret, setSecret] = useState<string | null>(null)
  const [error, setError] = useState<{ type: string; message: string } | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!token || !key) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24 text-center">
        <AlertTriangle size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Invalid Link</h2>
        <p className="text-gray-500">This secret link appears to be broken or incomplete.</p>
      </div>
    )
  }

  const handleReveal = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)

    try {
      const response = await api.revealSecret(token, { email, key })
      setSecret(response.text)
    } catch (err) {
      if (err instanceof AppError) {
        setError({ type: err.type, message: friendlyError(err.type) })
      } else {
        setError({ type: "unknown", message: "Something went wrong. Please try again." })
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!secret) return
    await navigator.clipboard.writeText(secret)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  // Show revealed secret
  if (secret !== null) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24">
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
          <h2 className="font-semibold mb-4">Secret</h2>
          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 p-4">
            <pre className="whitespace-pre-wrap text-sm break-words font-mono">{secret}</pre>
          </div>
          <div className="mt-4 flex items-center justify-between">
            <button
              type="button"
              onClick={handleCopy}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              {copied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
              {copied ? "Copied" : "Copy to Clipboard"}
            </button>
          </div>
          <p className="mt-4 text-xs text-gray-400">
            This secret has been permanently deleted from our servers.
          </p>
        </div>
      </div>
    )
  }

  // Show error
  if (error) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24 text-center">
        <AlertTriangle size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">
          {error.type === "expired" || error.type === "already_viewed"
            ? "Secret Unavailable"
            : "Unable to Reveal"}
        </h2>
        <p className="text-gray-500">{error.message}</p>
      </div>
    )
  }

  // Show reveal form
  return (
    <div className="max-w-lg mx-auto px-4 py-24">
      <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-8 text-center">
        <Lock size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Someone sent you a secret</h2>
        <p className="text-sm text-gray-500 mb-6">
          Enter your email to reveal it. This secret will be permanently deleted after viewing.
        </p>

        <form onSubmit={handleReveal}>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="Your email address"
            required
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-3 text-sm text-center focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
          <button
            type="submit"
            disabled={isLoading || !email.trim()}
            className="mt-4 w-full px-4 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {isLoading && <Loader2 size={16} className="animate-spin" />}
            Reveal Secret
          </button>
        </form>
      </div>
    </div>
  )
}

function friendlyError(type: string): string {
  switch (type) {
    case "expired":
      return "This secret has expired and is no longer available."
    case "already_viewed":
      return "This secret has already been viewed and was permanently deleted."
    case "not_found":
      return "This secret was not found. It may have expired or been viewed already."
    case "decrypt_failed":
      return "Unable to decrypt this secret. The link may be corrupted."
    default:
      return "Something went wrong. Please try again."
  }
}
```

**Step 2: Verify visually**

Run dev server. Navigate to `/s/test#somekey`. Verify form renders, error states look right.

**Step 3: Commit**

```bash
git add frontend/src/pages/Reveal.tsx
git commit -m "implement reveal page with email verification, copy, and error states"
```

---

## Task 9: Final Polish and Integration Test

**Files:**
- Modify: `frontend/index.html` (title, favicon, meta)
- Verify: All routes work end-to-end

**Step 1: Update index.html metadata**

In `frontend/index.html`, update `<title>` and add meta tags:

```html
<title>SecretDrop — Encrypted One-Time Secret Sharing</title>
<meta name="description" content="Share secrets that disappear after one read. End-to-end encrypted, zero-knowledge." />
```

**Step 2: Build check**

```bash
cd frontend && npm run build
```

Expected: Clean build, no errors.

**Step 3: TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: No type errors.

**Step 4: Commit**

```bash
git add frontend/
git commit -m "finalize frontend with metadata and build verification"
```

---

## Summary

| Task | Description | Backend | Frontend |
|------|-------------|---------|----------|
| 0 | OAuth callback redirect | Yes | — |
| 1 | Project scaffolding | — | Yes |
| 2 | Dark/light theme system | — | Yes |
| 3 | API client with JWT | — | Yes |
| 4 | Auth context | — | Yes |
| 5 | Router, layout, protected routes | — | Yes |
| 6 | Landing page | — | Yes |
| 7 | Dashboard page | — | Yes |
| 8 | Reveal page | — | Yes |
| 9 | Final polish + build check | — | Yes |

Total: 10 tasks, ~9 commits.
