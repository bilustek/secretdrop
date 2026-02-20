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
