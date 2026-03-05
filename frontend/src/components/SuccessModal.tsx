import { useEffect, useState } from "react"
import { Mail, X, Copy, Check } from "lucide-react"

interface Recipient {
  email: string
  link: string
}

interface SuccessModalProps {
  recipients: Recipient[]
  onClose: () => void
}

function CopyLinkButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="p-1.5 rounded-md hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 transition-all duration-150 hover:scale-110"
    >
      {copied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
    </button>
  )
}

export function SuccessModal({ recipients, onClose }: SuccessModalProps) {
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="max-w-md w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
        onClick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2 text-green-600 dark:text-green-400">
            <Mail size={20} />
            <h2 className="text-lg font-semibold">Emails sent to your recipients!</h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1 rounded-md hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        <p className="text-sm text-gray-600 dark:text-gray-400 mb-5">
          If the email doesn&apos;t arrive, ask them to check their spam folder.
          You can also copy the links below and share them directly.
        </p>

        <div className="space-y-2">
          {recipients.map((r) => (
            <div
              key={r.email}
              className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-900"
            >
              <span className="text-sm truncate mr-2">{r.email}</span>
              <CopyLinkButton text={r.link} />
            </div>
          ))}
        </div>

        <div className="mt-6 flex justify-end">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium rounded-lg bg-gray-900 text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200 transition-colors"
          >
            Got it
          </button>
        </div>
      </div>
    </div>
  )
}
