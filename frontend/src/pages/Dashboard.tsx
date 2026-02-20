import { use, useState, type FormEvent } from "react"
import { Plus, X, Loader2, Copy, Check } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { api, AppError, type CreateSecretResponse } from "../api/client"

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <button type="button" onClick={handleCopy} className="p-1 hover:text-gray-600 dark:hover:text-gray-300">
      {copied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
    </button>
  )
}

export default function Dashboard() {
  const auth = use(AuthContext)
  const [text, setText] = useState("")
  const [emails, setEmails] = useState<string[]>([])
  const [emailInput, setEmailInput] = useState("")
  const [result, setResult] = useState<CreateSecretResponse | null>(null)
  const [error, setError] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)

  if (!auth || !auth.user) return null

  const { user, refreshUser } = auth

  const addEmail = () => {
    const trimmed = emailInput.trim()
    if (trimmed && !emails.includes(trimmed) && emails.length < 5) {
      setEmails([...emails, trimmed])
      setEmailInput("")
    }
  }

  const removeEmail = (email: string) => {
    setEmails(emails.filter((e) => e !== email))
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError("")
    setIsSubmitting(true)

    try {
      const response = await api.createSecret({ text, to: emails })
      setResult(response)
      setText("")
      setEmails([])
      refreshUser()
    } catch (err) {
      if (err instanceof AppError) {
        if (err.type === "limit_reached") {
          setError("You've reached your secret limit. Upgrade to Pro for more.")
        } else {
          setError(err.message)
        }
      } else {
        setError("Something went wrong. Please try again.")
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleUpgrade = async () => {
    try {
      const { url } = await api.checkout()
      window.location.href = url
    } catch {
      setError("Failed to start checkout. Please try again.")
    }
  }


  if (result) {
    return (
      <div className="max-w-5xl mx-auto px-4 py-16">
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-8">
          <div className="flex items-center gap-2 text-green-600 dark:text-green-400 mb-4">
            <Check size={20} />
            <h2 className="font-semibold">Secret created and encrypted</h2>
          </div>
          <p className="text-sm text-gray-500 mb-6">
            Expires at {new Date(result.expires_at).toLocaleString()}
          </p>
          <div className="space-y-3">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Links sent to:</p>
            {result.recipients.map((r) => (
              <div key={r.email} className="flex items-center justify-between p-3 rounded-lg bg-gray-50 dark:bg-gray-900">
                <span className="text-sm">{r.email}</span>
                <CopyButton text={r.link} />
              </div>
            ))}
          </div>
          {user.secrets_used < user.secrets_limit ? (
            <button
              type="button"
              onClick={() => setResult(null)}
              className="mt-6 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              Create Another
            </button>
          ) : user.tier === "free" ? (
            <button
              type="button"
              onClick={handleUpgrade}
              className="mt-6 px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium hover:opacity-90 transition-opacity"
            >
              Upgrade to Pro for More Secrets
            </button>
          ) : (
            <button
              type="button"
              onClick={() => setResult(null)}
              className="mt-6 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              Create Another
            </button>
          )}
        </div>
      </div>
    )
  }

  const limitReached = user.secrets_used >= user.secrets_limit

  if (limitReached && user.tier === "free") {
    return (
      <div className="max-w-5xl mx-auto px-4 py-16">
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-8 text-center">
          <h2 className="font-semibold text-lg mb-2">You've used your free secret</h2>
          <p className="text-sm text-gray-500 mb-6">
            Upgrade to Pro for up to {100} secrets per month.
          </p>
          <button
            type="button"
            onClick={handleUpgrade}
            className="px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
          >
            Upgrade to Pro
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-5xl mx-auto px-4 py-16">
      <form onSubmit={handleSubmit}>
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
          <h2 className="font-semibold text-lg mb-4">Create a Secret</h2>

          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            placeholder="Enter your secret message..."
            maxLength={4096}
            rows={5}
            required
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-3 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white resize-none"
          />
          <p className="text-xs text-gray-400 mt-1 text-right">{text.length}/4096</p>

          <div className="mt-4">
            <label className="text-sm font-medium">Recipients</label>
            <div className="flex gap-2 mt-1">
              <input
                type="email"
                value={emailInput}
                onChange={(e) => setEmailInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addEmail() } }}
                placeholder="email@example.com"
                className="flex-1 rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
              />
              <button
                type="button"
                onClick={addEmail}
                disabled={emails.length >= 5}
                className="px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors disabled:opacity-50"
              >
                <Plus size={16} />
              </button>
            </div>
            {emails.length > 0 && (
              <div className="flex flex-wrap gap-2 mt-2">
                {emails.map((email) => (
                  <span key={email} className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-gray-100 dark:bg-gray-800 text-sm">
                    {email}
                    <button type="button" onClick={() => removeEmail(email)} className="hover:text-red-500">
                      <X size={14} />
                    </button>
                  </span>
                ))}
              </div>
            )}
            <p className="text-xs text-gray-400 mt-1">{emails.length}/5 recipients</p>
          </div>

          {error && (
            <p className="mt-4 text-sm text-red-600 dark:text-red-400">{error}</p>
          )}

          <button
            type="submit"
            disabled={isSubmitting || !text.trim() || emails.length === 0}
            className="mt-4 w-full px-4 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {isSubmitting && <Loader2 size={16} className="animate-spin" />}
            Create Secret
          </button>
        </div>
      </form>

      <div className="mt-8 flex items-center justify-between text-sm text-gray-500">
        <p>
          <span className="inline-block px-2 py-0.5 rounded text-xs font-medium bg-gray-100 dark:bg-gray-800 mr-2 uppercase">
            {user.tier}
          </span>
          {user.secrets_used} / {user.secrets_limit} secrets used
        </p>
        {user.tier === "free" && (
          <button
            type="button"
            onClick={handleUpgrade}
            className="px-4 py-1.5 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium hover:opacity-90 transition-opacity"
          >
            Upgrade to Pro
          </button>
        )}
      </div>
    </div>
  )
}
