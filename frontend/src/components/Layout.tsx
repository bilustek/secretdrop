import { use, useEffect, useRef, useState } from "react"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { Lock, CreditCard, LogOut, Trash2, User } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { ThemeToggle } from "./ThemeToggle"
import { api } from "../api/client"

export function Layout() {
  const auth = use(AuthContext)
  if (!auth) throw new Error("Layout must be within AuthProvider")

  const [menuOpen, setMenuOpen] = useState(false)
  const [deleteModalOpen, setDeleteModalOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()
  const { pathname } = useLocation()

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [pathname])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setMenuOpen(false)
      }
    }
    if (menuOpen) document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [menuOpen])

  useEffect(() => {
    if (!deleteModalOpen) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDeleteModalOpen(false)
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [deleteModalOpen])

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
                      <div className="px-4 py-2 border-b border-gray-200 dark:border-gray-800">
                        <p className="flex items-center justify-center gap-2 text-xs text-gray-500 py-2">
                          <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium uppercase ${auth.user.tier === "pro" ? "bg-gray-900 text-white dark:bg-white dark:text-gray-900" : "bg-gray-100 dark:bg-gray-800"}`}>
                            {auth.user.tier}
                          </span>
                          <span>{auth.user.secrets_used} / {auth.user.secrets_limit} used</span>
                        </p>
                        <p className="flex items-center gap-2 text-sm font-medium truncate mt-1.5">
                          <User size={16} className="shrink-0 text-gray-500" />
                          {auth.user.name}
                        </p>
                      </div>
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
                      <button
                        type="button"
                        onClick={() => {
                          setMenuOpen(false)
                          setDeleteModalOpen(true)
                        }}
                        className="w-full flex items-center gap-2 px-4 py-2 text-sm text-red-600 dark:text-red-400 hover:bg-gray-50 dark:hover:bg-gray-900"
                      >
                        <Trash2 size={16} />
                        Delete Account
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

      <footer className="border-t border-gray-200 dark:border-gray-800 py-6">
        <div className="max-w-5xl mx-auto px-4 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 text-sm text-gray-500 dark:text-gray-400">
          <p>&copy; 2026 Bilustek, LLC. +1 (320) 317-7912</p>
          <nav className="flex gap-6">
            <Link to="/terms" className="hover:text-gray-900 dark:hover:text-white transition-colors">
              Terms &amp; Conditions
            </Link>
            <Link to="/privacy" className="hover:text-gray-900 dark:hover:text-white transition-colors">
              Privacy Policy
            </Link>
            <Link to="/contact" className="hover:text-gray-900 dark:hover:text-white transition-colors">
              Contact
            </Link>
          </nav>
        </div>
      </footer>

      {deleteModalOpen && auth.user && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
          onClick={() => setDeleteModalOpen(false)}
          role="presentation"
        >
          <div
            className="max-w-md w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
            onClick={(e) => e.stopPropagation()}
            role="presentation"
          >
            <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">
              Delete your account?
            </h2>
            <div className="mt-4 space-y-3 text-sm text-gray-600 dark:text-gray-400">
              <p>
                You have {auth.user.secrets_limit - auth.user.secrets_used} of{" "}
                {auth.user.secrets_limit} secrets remaining.
              </p>
              {auth.user.tier === "pro" && (
                <p>Your Pro subscription will be cancelled.</p>
              )}
              <p className="text-red-600 dark:text-red-400">
                This action is permanent and cannot be undone.
              </p>
              <p>Thank you for using SecretDrop.</p>
            </div>
            <div className="mt-6 flex gap-3 justify-end">
              <button
                type="button"
                onClick={() => setDeleteModalOpen(false)}
                className="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
              >
                Keep My Account
              </button>
              <button
                type="button"
                onClick={async () => {
                  try {
                    await api.deleteAccount()
                    auth.logout()
                  } catch (err) {
                    console.error("delete account failed:", err)
                    setDeleteModalOpen(false)
                  }
                }}
                className="px-4 py-2 text-sm font-medium rounded-lg bg-red-600 text-white hover:bg-red-700 transition-colors"
              >
                Delete My Account
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
