# SecretDrop Frontend Design

## Stack

| Technology | Version | Purpose |
|-----------|---------|---------|
| React | 19 | UI framework |
| TypeScript | 5.x | Type safety |
| Tailwind CSS | 4.2 | Styling |
| Vite | latest | Build tool |
| React Router | 7 | Client-side routing |
| Lucide React | latest | Icons |

## Visual Style

- Minimal/clean aesthetic, typography-focused
- Dark and light mode with Tailwind `dark:` class strategy
- Font: Inter (sans-serif)
- Subtle gray borders, clean spacing

## Routes

| Path | Page | Auth Required |
|------|------|---------------|
| `/` | Landing (marketing + pricing + OAuth buttons) | No |
| `/dashboard` | Create secret form + usage stats + billing | Yes |
| `/s/:token#key` | Secret reveal | No |
| `/auth/callback` | OAuth return handler | No |

## Pages

### Landing (`/`)

Marketing page for unauthenticated users. Sections:

1. **Hero** — tagline ("Share secrets that disappear after one read"), subtitle about encryption, CTA button
2. **Features** — three cards: Encrypt (AES-256-GCM), Share (via email), Burn (after read)
3. **Pricing** — Free (1 secret lifetime) vs Pro ($2.99/mo, 100 secrets)
4. **Footer** — minimal links

Logged-in users visiting `/` are redirected to `/dashboard`.

### Dashboard (`/dashboard`)

Single page with:

1. **Create Secret form** — textarea (max 4KB) + recipient email input (up to 5) + submit
2. **Success state** — shows "encrypted and ready" + list of recipients + expiry time
3. **Usage stats** — tier badge, secrets used/limit, upgrade CTA for free tier
4. **User menu** (avatar dropdown) — profile info, billing portal link, sign out

### Reveal (`/s/:token#key`)

Public page, no auth required:

1. **Pre-reveal** — email input + "Reveal Secret" button + warning about one-time view
2. **Post-reveal** — secret text display + "Copy to Clipboard" button + deletion confirmation
3. **Error states** — expired (410), not found (404), decrypt failed (403)

### Auth Callback (`/auth/callback`)

Silent handler page:

1. Extract `access_token` and `refresh_token` from URL query params
2. Store in `localStorage`
3. Redirect to `/dashboard`

## Auth Flow

```
Landing [Get Started]
  -> GET /auth/google or /auth/github (backend redirect)
  -> Provider consent screen
  -> GET /auth/{provider}/callback (backend)
  -> Backend redirects to: FRONTEND_BASE_URL/auth/callback?access_token=xxx&refresh_token=yyy
  -> AuthCallback.tsx stores tokens in localStorage
  -> Navigate to /dashboard
```

## State Management

React Context + custom hooks, no external libraries.

### AuthContext

```typescript
interface AuthState {
  user: User | null
  accessToken: string | null
  refreshToken: string | null
  isAuthenticated: boolean
  isLoading: boolean
}

// Actions: login(tokens), logout(), refreshUser()
```

### ThemeContext

```typescript
interface ThemeState {
  theme: 'light' | 'dark'
  toggle: () => void
}
// Persisted in localStorage, respects system preference as default
```

## Component Structure

```
src/
├── main.tsx
├── App.tsx
├── index.css
├── api/
│   └── client.ts               # Fetch wrapper with JWT + auto-refresh
├── context/
│   ├── AuthContext.tsx
│   └── ThemeContext.tsx
├── components/
│   ├── Layout.tsx               # Nav + footer wrapper
│   ├── ProtectedRoute.tsx       # Auth guard -> redirect to /
│   ├── ThemeToggle.tsx
│   └── ui/                      # Button, Input, Card, Textarea, Badge
├── pages/
│   ├── Landing.tsx
│   ├── Dashboard.tsx
│   ├── Reveal.tsx
│   └── AuthCallback.tsx
└── hooks/
    ├── useAuth.ts
    └── useApi.ts
```

## API Client

Native `fetch` with a thin wrapper:

- Reads `access_token` from `localStorage`
- Sets `Authorization: Bearer <token>` header
- Sets `Content-Type: application/json`
- Parses error responses into typed `AppError`
- Handles 401 by clearing tokens and redirecting to `/`

## Token Storage

- `localStorage` with keys: `access_token`, `refresh_token`
- Cleared on logout or 401 response
- Read on app startup to restore auth state (call `GET /api/v1/me` to validate)

## Error Handling

| HTTP Status | Error Type | UI Response |
|-------------|-----------|-------------|
| 401 | Unauthorized | Clear tokens, redirect to `/` |
| 403 | `limit_reached` | Show upgrade CTA |
| 403 | `decrypt_failed` | "Unable to decrypt this secret" |
| 404 | `not_found` | "Secret not found" |
| 410 | `expired` / `already_viewed` | "This secret has expired or already been viewed" |
| 422 | Validation | Show field-level errors |
| Network | Connection error | Toast notification |

## Dark/Light Mode

- Tailwind `dark:` variant with class strategy
- Toggle in nav bar
- Default follows `prefers-color-scheme`
- Preference persisted in `localStorage`
