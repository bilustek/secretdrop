import { useEffect } from "react"

interface ConfirmModalProps {
  title: string
  message: string
  confirmLabel?: string
  confirmVariant?: "danger" | "default"
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmModal({
  title,
  message,
  confirmLabel = "Confirm",
  confirmVariant = "default",
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onCancel])

  const confirmClass =
    confirmVariant === "danger"
      ? "bg-red-600 text-white hover:bg-red-700"
      : "bg-gray-900 text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onCancel}
      role="presentation"
    >
      <div
        className="max-w-md w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
        onClick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">{title}</h2>
        <p className="mt-3 text-sm text-gray-600 dark:text-gray-400">{message}</p>
        <div className="mt-6 flex gap-3 justify-end">
          <button
            type="button"
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium rounded-lg border border-gray-300 dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            Cancel
          </button>
          <button type="button" onClick={onConfirm} className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${confirmClass}`}>
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
