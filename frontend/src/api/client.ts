const API_BASE = "/api/v1"

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

  deleteAccount: () =>
    fetch(`${API_BASE}/me`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${localStorage.getItem("access_token")}`,
      },
    }).then((r) => {
      if (!r.ok) throw new Error("Failed to delete account")
    }),
}
