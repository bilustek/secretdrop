import { use, useState, useRef, useEffect, type FormEvent } from "react"
import { Plus, X, Loader2, Copy, Check, Rocket } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { api, AppError, type CreateSecretResponse } from "../api/client"
import { useRecentEmails } from "../hooks/useRecentEmails"

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
  const { addEmails, suggest } = useRecentEmails()
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const suggestionsRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const suggestions = showSuggestions ? suggest(emailInput, emails) : []

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        suggestionsRef.current &&
        !suggestionsRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setShowSuggestions(false)
      }
    }
    if (showSuggestions) document.addEventListener("mousedown", handleClickOutside)
    return () => document.removeEventListener("mousedown", handleClickOutside)
  }, [showSuggestions])

  if (!auth || !auth.user) return null

  const { user, refreshUser } = auth
  const maxRecipients = user.tier === "pro" ? 5 : 1

  const addEmail = () => {
    const trimmed = emailInput.trim()
    if (trimmed && !emails.includes(trimmed) && emails.length < maxRecipients) {
      setEmails([...emails, trimmed])
      setEmailInput("")
      setShowSuggestions(false)
      setSelectedIndex(-1)
    }
  }

  const selectSuggestion = (email: string) => {
    if (!emails.includes(email) && emails.length < maxRecipients) {
      setEmails([...emails, email])
      setEmailInput("")
      setShowSuggestions(false)
      setSelectedIndex(-1)
      inputRef.current?.focus()
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
      addEmails(emails)
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
              className="mt-6 inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium hover:opacity-90 transition-opacity"
            >
              <Rocket size={16} />
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
            className="inline-flex items-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
          >
            <Rocket size={18} />
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
            rows={10}
            required
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-3 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white resize-none"
          />
          <p className="text-xs text-gray-400 mt-1 text-right">{text.length}/4096</p>

          <div className="mt-4">
            <label className="text-sm font-medium">Recipients</label>
            <div className="flex gap-2 mt-1">
              <div className="relative flex-1">
                <input
                  ref={inputRef}
                  type="email"
                  value={emailInput}
                  onChange={(e) => {
                    setEmailInput(e.target.value)
                    setShowSuggestions(true)
                    setSelectedIndex(-1)
                  }}
                  onFocus={() => setShowSuggestions(true)}
                  onKeyDown={(e) => {
                    if (e.key === "ArrowDown") {
                      e.preventDefault()
                      setSelectedIndex((i) => Math.min(i + 1, suggestions.length - 1))
                    } else if (e.key === "ArrowUp") {
                      e.preventDefault()
                      setSelectedIndex((i) => Math.max(i - 1, -1))
                    } else if (e.key === "Enter") {
                      e.preventDefault()
                      if (selectedIndex >= 0 && suggestions[selectedIndex]) {
                        selectSuggestion(suggestions[selectedIndex])
                      } else {
                        addEmail()
                      }
                    } else if (e.key === "Escape") {
                      setShowSuggestions(false)
                    }
                  }}
                  placeholder="email@example.com"
                  className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-2 text-sm focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
                  autoComplete="off"
                />
                {suggestions.length > 0 && (
                  <div
                    ref={suggestionsRef}
                    className="absolute z-50 left-0 right-0 mt-1 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-lg py-1 max-h-40 overflow-y-auto"
                  >
                    {suggestions.map((email, i) => (
                      <button
                        key={email}
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => selectSuggestion(email)}
                        className={`w-full text-left px-3 py-2 text-sm transition-colors ${
                          i === selectedIndex
                            ? "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white"
                            : "text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-900"
                        }`}
                      >
                        {email}
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <button
                type="button"
                onClick={addEmail}
                disabled={emails.length >= maxRecipients}
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
            <p className="text-xs text-gray-400 mt-1">{emails.length}/{maxRecipients} recipients</p>
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
          <span className={`inline-block px-2 py-0.5 rounded text-xs font-medium mr-2 uppercase ${user.tier === "pro" ? "bg-gray-900 text-white dark:bg-white dark:text-gray-900" : "bg-gray-100 dark:bg-gray-800"}`}>
            {user.tier}
          </span>
          {user.secrets_used} / {user.secrets_limit} secrets used
        </p>
        {user.tier === "free" && (
          <button
            type="button"
            onClick={handleUpgrade}
            className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-sm font-medium hover:opacity-90 transition-opacity"
          >
            <Rocket size={14} />
            Upgrade to Pro
          </button>
        )}
      </div>
    </div>
  )
}
