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
    adminRequest<AdminUsersResponse>(`/users${buildQuery(params as Record<string, string | number | undefined>)}`),

  updateTier: (id: number, tier: string) =>
    adminRequest<{ status: string }>(`/users/${id}`, {
      method: "PATCH",
      body: JSON.stringify({ tier }),
    }),

  fetchSubscriptions: (params: SubscriptionListParams = {}) =>
    adminRequest<AdminSubscriptionsResponse>(`/subscriptions${buildQuery(params as Record<string, string | number | undefined>)}`),

  cancelSubscription: (id: number) =>
    adminRequest<void>(`/subscriptions/${id}`, { method: "DELETE" }),
}
