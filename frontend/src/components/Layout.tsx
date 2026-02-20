import { use } from "react"
import { Link, Outlet } from "react-router"
import { Lock } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { ThemeToggle } from "./ThemeToggle"

export function Layout() {
  const auth = use(AuthContext)
  if (!auth) throw new Error("Layout must be within AuthProvider")

  return (
    <div className="min-h-screen flex flex-col bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
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

      <main className="flex-1">
        <Outlet />
      </main>

      <footer className="border-t border-gray-200 dark:border-gray-800 py-8">
        <div className="max-w-5xl mx-auto px-4 text-center text-sm text-gray-500">
          SecretDrop — Encrypted one-time secret sharing
        </div>
      </footer>
    </div>
  )
}
