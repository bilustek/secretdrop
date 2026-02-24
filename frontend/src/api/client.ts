import { API_URL } from "./config"

const API_BASE = `${API_URL}/api/v1`

interface ApiError {
  error: {
    type: string
    message: string
  }
}

export class AppError extends Error {
  type: string
  status: number

  constructor(type: string, message: string, status: number) {
    super(message)
    this.name = "AppError"
    this.type = type
    this.status = status
  }
}

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/)
  return match ? decodeURIComponent(match[1]) : ""
}

let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  try {
    const res = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({}),
    })

    return res.ok
  } catch {
    return false
  }
}

async function tryRefresh(): Promise<boolean> {
  if (refreshPromise) return refreshPromise

  refreshPromise = refreshTokens().finally(() => {
    refreshPromise = null
  })

  return refreshPromise
}

function forceLogout(): never {
  window.location.href = "/"
  throw new AppError("unauthorized", "Session expired", 401)
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await authenticatedFetch(`${API_BASE}${path}`, options)

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

async function authenticatedFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  const method = (options.method ?? "GET").toUpperCase()
  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    const csrf = getCSRFToken()
    if (csrf) {
      headers["X-CSRF-Token"] = csrf
    }
  }

  const res = await fetch(url, { ...options, headers, credentials: "include" })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      // After refresh, CSRF token cookie may have changed
      const newMethod = (options.method ?? "GET").toUpperCase()
      if (newMethod !== "GET" && newMethod !== "HEAD" && newMethod !== "OPTIONS") {
        const newCsrf = getCSRFToken()
        if (newCsrf) {
          headers["X-CSRF-Token"] = newCsrf
        }
      }

      return fetch(url, { ...options, headers, credentials: "include" })
    }

    forceLogout()
  }

  return res
}

export interface MeResponse {
  email: string
  name: string
  avatar_url: string
  tier: string
  secrets_used: number
  secrets_limit: number
  max_text_length: number
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

async function softAuthFetch<T>(path: string): Promise<T> {
  const url = `${API_BASE}${path}`
  const opts: RequestInit = {
    headers: { "Content-Type": "application/json" },
    credentials: "include",
  }

  let res = await fetch(url, opts)

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      res = await fetch(url, opts)
    }
  }

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

export const api = {
  me: () => softAuthFetch<MeResponse>("/me"),

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
    authenticatedFetch(`${API_URL}/billing/checkout`, {
      method: "POST",
    }).then((r) => r.json() as Promise<CheckoutResponse>),

  portal: () =>
    authenticatedFetch(`${API_URL}/billing/portal`, {
      method: "POST",
    }).then((r) => r.json() as Promise<{ url: string }>),

  deleteAccount: () =>
    authenticatedFetch(`${API_BASE}/me`, {
      method: "DELETE",
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to delete account")
    }),

  logout: () =>
    authenticatedFetch(`${API_URL}/auth/logout`, {
      method: "POST",
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to logout")
    }),
}
