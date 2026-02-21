import { useState, useEffect, useCallback } from "react"
import { ArrowUp, ArrowDown, Pencil, X } from "lucide-react"
import { adminApi, type AdminUser, type AdminUsersResponse, type TierLimits } from "../../api/admin"
import { ConfirmModal } from "../../components/ConfirmModal"

const PER_PAGE = 20

type SortField = "email" | "created_at"
type SortOrder = "asc" | "desc"

export default function AdminUsers() {
  const [data, setData] = useState<AdminUsersResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [search, setSearch] = useState("")
  const [tierFilter, setTierFilter] = useState("")
  const [sortField, setSortField] = useState<SortField>("created_at")
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc")
  const [page, setPage] = useState(1)
  const [confirmUser, setConfirmUser] = useState<AdminUser | null>(null)
  const [actionError, setActionError] = useState("")
  const [tiers, setTiers] = useState<TierLimits[]>([])
  const [editLimitUserId, setEditLimitUserId] = useState<number | null>(null)
  const [editLimitValue, setEditLimitValue] = useState("")

  const fetchUsers = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const result = await adminApi.fetchUsers({
        q: search || undefined,
        tier: tierFilter || undefined,
        sort: sortField,
        order: sortOrder,
        page,
        per_page: PER_PAGE,
      })
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load users")
    } finally {
      setLoading(false)
    }
  }, [search, tierFilter, sortField, sortOrder, page])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  useEffect(() => {
    adminApi.fetchLimits().then(setTiers).catch(() => {})
  }, [])

  // Debounced search
  const [searchInput, setSearchInput] = useState("")

  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput)
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder(sortOrder === "asc" ? "desc" : "asc")
    } else {
      setSortField(field)
      setSortOrder("asc")
    }
    setPage(1)
  }

  const handleTierChange = async (user: AdminUser) => {
    setConfirmUser(null)
    setActionError("")
    try {
      const newTier = user.tier === "free" ? "pro" : "free"
      await adminApi.updateUser(user.id, { tier: newTier })
      await fetchUsers()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to update tier")
    }
  }

  const handleLimitSave = async (userId: number) => {
    setActionError("")
    try {
      const val = editLimitValue.trim()
      if (val === "") {
        await adminApi.updateUser(userId, { clear_secrets_limit: true })
      } else {
        const num = parseInt(val, 10)
        if (isNaN(num) || num <= 0) {
          setActionError("Limit must be a positive number or empty to clear")
          return
        }
        await adminApi.updateUser(userId, { secrets_limit_override: num })
      }
      setEditLimitUserId(null)
      await fetchUsers()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to update limit")
    }
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PER_PAGE)) : 1

  const SortIndicator = ({ field }: { field: SortField }) => {
    if (sortField !== field) return null
    return sortOrder === "asc" ? (
      <ArrowUp size={14} className="inline ml-1" />
    ) : (
      <ArrowDown size={14} className="inline ml-1" />
    )
  }

  const tierOptions = tiers.map((t) => t.tier)

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Users</h1>
          {data && (
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {data.total} {data.total === 1 ? "user" : "users"}
            </p>
          )}
        </div>
        <div className="flex flex-col sm:flex-row gap-3">
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder="Search by email or name..."
            className="rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
          <select
            value={tierFilter}
            onChange={(e) => {
              setTierFilter(e.target.value)
              setPage(1)
            }}
            className="rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          >
            <option value="">All Tiers</option>
            {tierOptions.map((t) => (
              <option key={t} value={t}>
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </option>
            ))}
          </select>
        </div>
      </div>

      {error && (
        <div className="rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-400 mb-4">
          {error}
        </div>
      )}

      {actionError && (
        <div className="rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 px-4 py-3 text-sm text-red-700 dark:text-red-400 mb-4">
          {actionError}
        </div>
      )}

      {loading ? (
        <div className="text-center py-16 text-gray-500 dark:text-gray-400">Loading...</div>
      ) : data && data.users.length > 0 ? (
        <>
          <div className="overflow-x-auto border border-gray-200 dark:border-gray-800 rounded-lg">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
                  <th
                    className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400 cursor-pointer select-none hover:text-gray-900 dark:hover:text-white transition-colors"
                    onClick={() => handleSort("email")}
                  >
                    Email
                    <SortIndicator field="email" />
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Name
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Provider
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Tier
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Usage
                  </th>
                  <th
                    className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400 cursor-pointer select-none hover:text-gray-900 dark:hover:text-white transition-colors"
                    onClick={() => handleSort("created_at")}
                  >
                    Created At
                    <SortIndicator field="created_at" />
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {data.users.map((user) => (
                  <tr
                    key={user.id}
                    className="border-b border-gray-200 dark:border-gray-800 last:border-b-0 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors"
                  >
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{user.email}</td>
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{user.name}</td>
                    <td className="px-4 py-3">
                      <span className="inline-block px-2 py-0.5 rounded text-xs font-medium capitalize bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300">
                        {user.provider}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block px-2 py-0.5 rounded text-xs font-medium uppercase ${
                          user.tier === "pro"
                            ? "bg-gray-900 text-white dark:bg-white dark:text-gray-900"
                            : "bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300"
                        }`}
                      >
                        {user.tier}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">
                      {editLimitUserId === user.id ? (
                        <span className="flex items-center gap-1">
                          <input
                            type="number"
                            min="1"
                            placeholder="tier default"
                            value={editLimitValue}
                            onChange={(e) => setEditLimitValue(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") handleLimitSave(user.id)
                              if (e.key === "Escape") setEditLimitUserId(null)
                            }}
                            className="w-20 rounded border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-1.5 py-0.5 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-1 focus:ring-gray-900 dark:focus:ring-white"
                            autoFocus
                          />
                          <button
                            type="button"
                            onClick={() => setEditLimitUserId(null)}
                            className="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
                          >
                            <X size={14} />
                          </button>
                        </span>
                      ) : (
                        <span className="flex items-center gap-1.5">
                          <span>
                            {user.secrets_used}/{user.secrets_limit}
                          </span>
                          {user.secrets_limit_override !== null && (
                            <span className="px-1 py-0.5 rounded text-[10px] font-medium bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400">
                              custom
                            </span>
                          )}
                          <button
                            type="button"
                            onClick={() => {
                              setEditLimitUserId(user.id)
                              setEditLimitValue(
                                user.secrets_limit_override !== null
                                  ? String(user.secrets_limit_override)
                                  : "",
                              )
                              setActionError("")
                            }}
                            className="p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity"
                            title="Edit limit override"
                          >
                            <Pencil size={12} />
                          </button>
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                      {new Date(user.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        onClick={() => setConfirmUser(user)}
                        className="px-3 py-1.5 rounded-lg text-xs font-medium border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
                      >
                        {user.tier === "free" ? "Upgrade to Pro" : "Downgrade to Free"}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between mt-4">
            <button
              type="button"
              onClick={() => setPage(page - 1)}
              disabled={page <= 1}
              className="px-4 py-2 rounded-lg text-sm font-medium border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Prev
            </button>
            <span className="text-sm text-gray-500 dark:text-gray-400">
              Page {page} of {totalPages}
            </span>
            <button
              type="button"
              onClick={() => setPage(page + 1)}
              disabled={page >= totalPages}
              className="px-4 py-2 rounded-lg text-sm font-medium border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Next
            </button>
          </div>
        </>
      ) : (
        <div className="text-center py-16 text-gray-500 dark:text-gray-400">No users found.</div>
      )}

      {confirmUser && (
        <ConfirmModal
          title="Change User Tier"
          message={`Are you sure you want to ${confirmUser.tier === "free" ? "upgrade" : "downgrade"} ${confirmUser.email} to ${confirmUser.tier === "free" ? "Pro" : "Free"}?`}
          confirmLabel={confirmUser.tier === "free" ? "Upgrade to Pro" : "Downgrade to Free"}
          onConfirm={() => handleTierChange(confirmUser)}
          onCancel={() => setConfirmUser(null)}
        />
      )}
    </div>
  )
}
