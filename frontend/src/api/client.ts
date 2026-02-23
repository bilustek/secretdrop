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

// Mutex to prevent concurrent refresh attempts.
let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  const refreshToken = localStorage.getItem("refresh_token")
  if (!refreshToken) return false

  try {
    const res = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })

    if (!res.ok) return false

    const pair = (await res.json()) as { access_token: string; refresh_token: string }
    localStorage.setItem("access_token", pair.access_token)
    localStorage.setItem("refresh_token", pair.refresh_token)

    return true
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
  localStorage.removeItem("access_token")
  localStorage.removeItem("refresh_token")
  window.location.href = "/"
  throw new AppError("unauthorized", "Session expired", 401)
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
    const refreshed = await tryRefresh()
    if (refreshed) {
      const newToken = localStorage.getItem("access_token")
      headers["Authorization"] = `Bearer ${newToken}`

      const retry = await fetch(`${API_BASE}${path}`, { ...options, headers })
      if (!retry.ok) {
        if (retry.status === 401) forceLogout()

        const body: ApiError = await retry.json()
        throw new AppError(body.error.type, body.error.message, retry.status)
      }

      return retry.json() as Promise<T>
    }

    forceLogout()
  }

  if (!res.ok) {
    const body: ApiError = await res.json()
    throw new AppError(body.error.type, body.error.message, res.status)
  }

  return res.json() as Promise<T>
}

async function authenticatedFetch(url: string, options: RequestInit = {}): Promise<Response> {
  const token = localStorage.getItem("access_token")

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((options.headers as Record<string, string>) ?? {}),
  }

  if (token) {
    headers["Authorization"] = `Bearer ${token}`
  }

  const res = await fetch(url, { ...options, headers })

  if (res.status === 401) {
    const refreshed = await tryRefresh()
    if (refreshed) {
      const newToken = localStorage.getItem("access_token")
      headers["Authorization"] = `Bearer ${newToken}`

      return fetch(url, { ...options, headers })
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
}
