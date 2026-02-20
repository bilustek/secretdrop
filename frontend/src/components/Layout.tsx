import { use, useEffect, useRef, useState } from "react"
import { Link, Outlet, useNavigate } from "react-router"
import { Lock, CreditCard, LogOut } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { ThemeToggle } from "./ThemeToggle"
import { api } from "../api/client"

export function Layout() {
  const auth = use(AuthContext)
  if (!auth) throw new Error("Layout must be within AuthProvider")

  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    if (menuOpen) document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [menuOpen])

  const handleManageBilling = async () => {
    setMenuOpen(false)
    try {
      const { url } = await api.portal()
      window.location.href = url
    } catch {
      navigate("/dashboard")
    }
  }

  const handleLogout = () => {
    setMenuOpen(false)
    auth.logout()
  }

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
                <div className="relative" ref={menuRef}>
                  <button
                    type="button"
                    onClick={() => setMenuOpen(!menuOpen)}
                    className="rounded-full focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                  >
                    <img
                      src={auth.user.avatar_url}
                      alt=""
                      referrerPolicy="no-referrer"
                      className="w-8 h-8 rounded-full"
                    />
                  </button>
                  {menuOpen && (
                    <div className="absolute right-0 mt-2 w-48 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-lg py-1 z-50">
                      <p className="px-4 py-2 text-sm font-medium truncate border-b border-gray-200 dark:border-gray-800">
                        {auth.user.name}
                      </p>
                      {auth.user.tier === "pro" && (
                        <button
                          type="button"
                          onClick={handleManageBilling}
                          className="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900"
                        >
                          <CreditCard size={16} />
                          Manage Billing
                        </button>
                      )}
                      <button
                        type="button"
                        onClick={handleLogout}
                        className="w-full flex items-center gap-2 px-4 py-2 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900"
                      >
                        <LogOut size={16} />
                        Sign Out
                      </button>
                    </div>
                  )}
                </div>
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
