import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"
import { api, type MeResponse } from "../api/client"

interface AuthContextValue {
  user: MeResponse | null
  isAuthenticated: boolean
  isLoading: boolean
  logout: () => void
  refreshUser: () => Promise<void>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const fetchUser = useCallback(async () => {
    try {
      const me = await api.me()
      setUser(me)
    } catch {
      setUser(null)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  const logout = useCallback(async () => {
    await api.logout()
    setUser(null)
  }, [])

  return (
    <AuthContext value={{ user, isAuthenticated: !!user, isLoading, logout, refreshUser: fetchUser }}>
      {children}
    </AuthContext>
  )
}
