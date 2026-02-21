import { useState, useEffect, useCallback } from "react"
import { ArrowUp, ArrowDown, Copy, Check } from "lucide-react"
import { adminApi, type AdminSubscription, type AdminSubscriptionsResponse } from "../../api/admin"
import { ConfirmModal } from "../../components/ConfirmModal"

const PER_PAGE = 20

type SortOrder = "asc" | "desc"

export default function AdminSubscriptions() {
  const [data, setData] = useState<AdminSubscriptionsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [search, setSearch] = useState("")
  const [debouncedSearch, setDebouncedSearch] = useState("")
  const [statusFilter, setStatusFilter] = useState("")
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc")
  const [page, setPage] = useState(1)
  const [confirmSub, setConfirmSub] = useState<AdminSubscription | null>(null)
  const [actionError, setActionError] = useState("")
  const [copiedId, setCopiedId] = useState<string | null>(null)

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(search), 300)
    return () => clearTimeout(timer)
  }, [search])

  const fetchSubscriptions = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const result = await adminApi.fetchSubscriptions({
        q: debouncedSearch || undefined,
        status: statusFilter || undefined,
        sort: "created_at",
        order: sortOrder,
        page,
        per_page: PER_PAGE,
      })
      setData(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load subscriptions")
    } finally {
      setLoading(false)
    }
  }, [debouncedSearch, statusFilter, sortOrder, page])

  useEffect(() => {
    fetchSubscriptions()
  }, [fetchSubscriptions])

  const handleSort = () => {
    setSortOrder(sortOrder === "asc" ? "desc" : "asc")
    setPage(1)
  }

  const handleCancel = async (sub: AdminSubscription) => {
    setConfirmSub(null)
    setActionError("")
    try {
      await adminApi.cancelSubscription(sub.user_id)
      await fetchSubscriptions()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to cancel subscription")
    }
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / PER_PAGE)) : 1

  const SortIndicator = () => {
    return sortOrder === "asc" ? (
      <ArrowUp size={14} className="inline ml-1" />
    ) : (
      <ArrowDown size={14} className="inline ml-1" />
    )
  }

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Subscriptions</h1>
          {data && (
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              {data.total} {data.total === 1 ? "subscription" : "subscriptions"}
            </p>
          )}
        </div>
        <div className="flex flex-col sm:flex-row gap-3">
          <input
            type="text"
            placeholder="Search by email or name..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setPage(1)
            }}
            className="rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white sm:w-64"
          />
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value)
              setPage(1)
            }}
            className="rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-3 py-2 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          >
            <option value="">All</option>
            <option value="active">Active</option>
            <option value="canceled">Canceled</option>
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
      ) : data && data.subscriptions.length > 0 ? (
        <>
          <div className="overflow-x-auto border border-gray-200 dark:border-gray-800 rounded-lg">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    User Email
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    User Name
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Stripe Sub ID
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Status
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Period Start
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Period End
                  </th>
                  <th
                    className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400 cursor-pointer select-none hover:text-gray-900 dark:hover:text-white transition-colors"
                    onClick={handleSort}
                  >
                    Created At
                    <SortIndicator />
                  </th>
                  <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {data.subscriptions.map((sub) => (
                  <tr
                    key={sub.id}
                    className="border-b border-gray-200 dark:border-gray-800 last:border-b-0 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors"
                  >
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{sub.user_email}</td>
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{sub.user_name}</td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(sub.stripe_subscription_id)
                          setCopiedId(sub.stripe_subscription_id)
                          setTimeout(() => setCopiedId(null), 1500)
                        }}
                        className="group flex items-center gap-1.5 font-mono text-gray-900 dark:text-gray-100 truncate max-w-[160px] hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                        title={sub.stripe_subscription_id}
                      >
                        <span className="truncate">{sub.stripe_subscription_id}</span>
                        {copiedId === sub.stripe_subscription_id ? (
                          <Check size={14} className="shrink-0 text-green-500" />
                        ) : (
                          <Copy size={14} className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
                        )}
                      </button>
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block px-2 py-0.5 rounded text-xs font-medium capitalize ${
                          sub.status === "active"
                            ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
                            : "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                        }`}
                      >
                        {sub.status}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                      {new Date(sub.current_period_start).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                      {new Date(sub.current_period_end).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3 text-gray-500 dark:text-gray-400">
                      {new Date(sub.created_at).toLocaleDateString()}
                    </td>
                    <td className="px-4 py-3">
                      {sub.status === "active" && (
                        <button
                          type="button"
                          onClick={() => setConfirmSub(sub)}
                          className="px-3 py-1.5 rounded-lg text-xs font-medium border border-red-300 dark:border-red-700 text-red-700 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
                        >
                          Cancel
                        </button>
                      )}
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
        <div className="text-center py-16 text-gray-500 dark:text-gray-400">No subscriptions found.</div>
      )}

      {confirmSub && (
        <ConfirmModal
          title="Cancel Subscription"
          message="This will cancel the subscription and downgrade the user to the free tier. This action cannot be undone."
          confirmLabel="Cancel Subscription"
          confirmVariant="danger"
          onConfirm={() => handleCancel(confirmSub)}
          onCancel={() => setConfirmSub(null)}
        />
      )}
    </div>
  )
}
