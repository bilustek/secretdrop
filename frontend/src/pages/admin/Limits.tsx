import { useState, useEffect, useCallback } from "react"
import { Plus, Trash2, Save, X } from "lucide-react"
import { adminApi, type TierLimits } from "../../api/admin"
import { ConfirmModal } from "../../components/ConfirmModal"

interface EditRow {
  tier: string
  secrets_limit: string
  recipients_limit: string
  stripe_price_id: string
  price_cents: string
  currency: string
  isNew: boolean
}

export default function AdminLimits() {
  const [limits, setLimits] = useState<TierLimits[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [actionError, setActionError] = useState("")
  const [editRow, setEditRow] = useState<EditRow | null>(null)
  const [deleteTier, setDeleteTier] = useState<string | null>(null)

  const fetchLimits = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const result = await adminApi.fetchLimits()
      setLimits(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load limits")
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchLimits()
  }, [fetchLimits])

  const handleSave = async () => {
    if (!editRow) return
    setActionError("")

    const secretsLimit = parseInt(editRow.secrets_limit, 10)
    const recipientsLimit = parseInt(editRow.recipients_limit, 10)

    if (!editRow.tier.trim()) {
      setActionError("Tier name is required")
      return
    }

    if (isNaN(secretsLimit) || secretsLimit <= 0) {
      setActionError("Secrets limit must be a positive number")
      return
    }

    if (isNaN(recipientsLimit) || recipientsLimit <= 0) {
      setActionError("Recipients limit must be a positive number")
      return
    }

    try {
      await adminApi.upsertLimits(editRow.tier.trim().toLowerCase(), {
        secrets_limit: secretsLimit,
        recipients_limit: recipientsLimit,
        stripe_price_id: editRow.stripe_price_id ?? "",
        price_cents: parseInt(editRow.price_cents, 10) || 0,
        currency: editRow.currency ?? "usd",
      })
      setEditRow(null)
      await fetchLimits()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to save limits")
    }
  }

  const handleDelete = async (tier: string) => {
    setDeleteTier(null)
    setActionError("")
    try {
      await adminApi.deleteLimits(tier)
      await fetchLimits()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to delete tier")
    }
  }

  const startEdit = (tl: TierLimits) => {
    setEditRow({
      tier: tl.tier,
      secrets_limit: String(tl.secrets_limit),
      recipients_limit: String(tl.recipients_limit),
      stripe_price_id: tl.stripe_price_id,
      price_cents: String(tl.price_cents),
      currency: tl.currency,
      isNew: false,
    })
    setActionError("")
  }

  const startAdd = () => {
    setEditRow({
      tier: "",
      secrets_limit: "100",
      recipients_limit: "5",
      stripe_price_id: "",
      price_cents: "0",
      currency: "usd",
      isNew: true,
    })
    setActionError("")
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">Limits</h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Configure secrets and recipients limits per tier
          </p>
        </div>
        <button
          type="button"
          onClick={startAdd}
          disabled={editRow !== null}
          className="flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm font-medium bg-gray-900 dark:bg-white text-white dark:text-gray-900 hover:bg-gray-800 dark:hover:bg-gray-200 transition-colors disabled:opacity-50"
        >
          <Plus size={16} />
          Add Tier
        </button>
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
      ) : (
        <div className="overflow-x-auto border border-gray-200 dark:border-gray-800 rounded-lg">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-900">
                <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">Tier</th>
                <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">Secrets Limit</th>
                <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">Recipients Limit</th>
                <th className="text-left px-4 py-3 font-medium text-gray-500 dark:text-gray-400">Actions</th>
              </tr>
            </thead>
            <tbody>
              {limits.map((tl) =>
                editRow && !editRow.isNew && editRow.tier === tl.tier ? (
                  <tr key={tl.tier} className="border-b border-gray-200 dark:border-gray-800 bg-blue-50/50 dark:bg-blue-900/10">
                    <td className="px-4 py-3 font-medium text-gray-900 dark:text-gray-100 capitalize">{tl.tier}</td>
                    <td className="px-4 py-3">
                      <input
                        type="number"
                        min="1"
                        value={editRow.secrets_limit}
                        onChange={(e) => setEditRow({ ...editRow, secrets_limit: e.target.value })}
                        className="w-24 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-2 py-1 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                      />
                    </td>
                    <td className="px-4 py-3">
                      <input
                        type="number"
                        min="1"
                        value={editRow.recipients_limit}
                        onChange={(e) => setEditRow({ ...editRow, recipients_limit: e.target.value })}
                        className="w-24 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-2 py-1 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                      />
                    </td>
                    <td className="px-4 py-3 flex gap-2">
                      <button type="button" onClick={handleSave} className="p-1.5 rounded-lg text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors" title="Save">
                        <Save size={16} />
                      </button>
                      <button type="button" onClick={() => setEditRow(null)} className="p-1.5 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors" title="Cancel">
                        <X size={16} />
                      </button>
                    </td>
                  </tr>
                ) : (
                  <tr key={tl.tier} className="border-b border-gray-200 dark:border-gray-800 last:border-b-0 hover:bg-gray-50 dark:hover:bg-gray-900/50 transition-colors">
                    <td className="px-4 py-3 font-medium text-gray-900 dark:text-gray-100 capitalize">{tl.tier}</td>
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{tl.secrets_limit}</td>
                    <td className="px-4 py-3 text-gray-900 dark:text-gray-100">{tl.recipients_limit}</td>
                    <td className="px-4 py-3 flex gap-2">
                      <button
                        type="button"
                        onClick={() => startEdit(tl)}
                        disabled={editRow !== null}
                        className="px-3 py-1.5 rounded-lg text-xs font-medium border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors disabled:opacity-50"
                      >
                        Edit
                      </button>
                      {tl.tier !== "free" && (
                        <button
                          type="button"
                          onClick={() => setDeleteTier(tl.tier)}
                          disabled={editRow !== null}
                          className="p-1.5 rounded-lg text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors disabled:opacity-50"
                          title="Delete tier"
                        >
                          <Trash2 size={16} />
                        </button>
                      )}
                    </td>
                  </tr>
                ),
              )}
              {editRow?.isNew && (
                <tr className="border-b border-gray-200 dark:border-gray-800 bg-blue-50/50 dark:bg-blue-900/10">
                  <td className="px-4 py-3">
                    <input
                      type="text"
                      placeholder="e.g. vip"
                      value={editRow.tier}
                      onChange={(e) => setEditRow({ ...editRow, tier: e.target.value })}
                      className="w-32 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-2 py-1 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                      autoFocus
                    />
                  </td>
                  <td className="px-4 py-3">
                    <input
                      type="number"
                      min="1"
                      value={editRow.secrets_limit}
                      onChange={(e) => setEditRow({ ...editRow, secrets_limit: e.target.value })}
                      className="w-24 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-2 py-1 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <input
                      type="number"
                      min="1"
                      value={editRow.recipients_limit}
                      onChange={(e) => setEditRow({ ...editRow, recipients_limit: e.target.value })}
                      className="w-24 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-900 px-2 py-1 text-sm text-gray-900 dark:text-white focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                    />
                  </td>
                  <td className="px-4 py-3 flex gap-2">
                    <button type="button" onClick={handleSave} className="p-1.5 rounded-lg text-green-600 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors" title="Save">
                      <Save size={16} />
                    </button>
                    <button type="button" onClick={() => setEditRow(null)} className="p-1.5 rounded-lg text-gray-500 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors" title="Cancel">
                      <X size={16} />
                    </button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {deleteTier && (
        <ConfirmModal
          title="Delete Tier"
          message={`This will delete the "${deleteTier}" tier and its limits. Users on this tier will fall back to hardcoded defaults. This action cannot be undone.`}
          confirmLabel="Delete Tier"
          confirmVariant="danger"
          onConfirm={() => handleDelete(deleteTier)}
          onCancel={() => setDeleteTier(null)}
        />
      )}
    </div>
  )
}
