import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"
import { api, type MeResponse } from "../api/client"

interface AuthContextValue {
  user: MeResponse | null
  isAuthenticated: boolean
  isLoading: boolean
  logout: () => void
  refreshUser: () => Promise<MeResponse | null>
}

export const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<MeResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  const fetchUser = useCallback(async (): Promise<MeResponse | null> => {
    try {
      const me = await api.me()
      setUser(me)

      return me
    } catch {
      setUser(null)

      return null
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchUser()
  }, [fetchUser])

  const logout = useCallback(async () => {
    try {
      await api.logout()
    } catch {
      // If logout fails server-side, clear client state anyway —
      // user explicitly requested sign-out.
    }

    setUser(null)
  }, [])

  return (
    <AuthContext value={{ user, isAuthenticated: !!user, isLoading, logout, refreshUser: fetchUser }}>
      {children}
    </AuthContext>
  )
}
