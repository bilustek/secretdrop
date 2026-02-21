import { Link, Navigate, Outlet, useLocation } from "react-router"
import { LogOut, Shield, Users, CreditCard } from "lucide-react"
import { ThemeToggle } from "./ThemeToggle"
import { hasCredentials, clearCredentials } from "../api/admin"

export function AdminLayout() {
  const { pathname } = useLocation()

  if (!hasCredentials()) {
    return <Navigate to="/admin/login" replace />
  }

  const handleLogout = () => {
    clearCredentials()
    window.location.href = "/admin/login"
  }

  const navLinks = [
    { to: "/admin/users", label: "Users", icon: Users },
    { to: "/admin/subscriptions", label: "Subscriptions", icon: CreditCard },
  ]

  return (
    <div className="min-h-screen flex flex-col bg-white dark:bg-gray-950 text-gray-900 dark:text-gray-100">
      <header className="border-b border-gray-200 dark:border-gray-800">
        <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <Link to="/admin/users" className="flex items-center gap-2 font-semibold text-lg">
              <Shield size={20} />
              SecretDrop Admin
            </Link>
            <nav className="hidden sm:flex items-center gap-1">
              {navLinks.map(({ to, label, icon: Icon }) => (
                <Link
                  key={to}
                  to={to}
                  className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                    pathname === to
                      ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white"
                      : "text-gray-500 hover:text-gray-900 dark:hover:text-white"
                  }`}
                >
                  <Icon size={16} />
                  {label}
                </Link>
              ))}
            </nav>
          </div>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <button
              type="button"
              onClick={handleLogout}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm font-medium text-gray-500 hover:text-gray-900 dark:hover:text-white transition-colors"
            >
              <LogOut size={16} />
              <span className="hidden sm:inline">Logout</span>
            </button>
          </div>
        </div>
      </header>

      {/* Mobile nav */}
      <div className="sm:hidden border-b border-gray-200 dark:border-gray-800">
        <div className="flex">
          {navLinks.map(({ to, label, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
                pathname === to
                  ? "border-b-2 border-gray-900 dark:border-white text-gray-900 dark:text-white"
                  : "text-gray-500 hover:text-gray-900 dark:hover:text-white"
              }`}
            >
              <Icon size={16} />
              {label}
            </Link>
          ))}
        </div>
      </div>

      <main className="flex-1">
        <div className="max-w-6xl mx-auto px-4 py-6">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
