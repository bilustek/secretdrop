import { useState, type FormEvent } from "react"
import { useParams, useLocation } from "react-router"
import { Lock, Copy, Check, Loader2, AlertTriangle } from "lucide-react"
import { api, AppError } from "../api/client"

function friendlyError(type: string): string {
  switch (type) {
    case "expired":
      return "This secret has expired and is no longer available."
    case "already_viewed":
      return "This secret has already been viewed and was permanently deleted."
    case "not_found":
      return "This secret was not found. It may have expired or been viewed already."
    case "decrypt_failed":
      return "Unable to decrypt this secret. The link may be corrupted."
    default:
      return "Something went wrong. Please try again."
  }
}

export default function Reveal() {
  const { token } = useParams<{ token: string }>()
  const location = useLocation()
  const key = location.hash.slice(1)

  const [email, setEmail] = useState("")
  const [secret, setSecret] = useState<string | null>(null)
  const [error, setError] = useState<{ type: string; message: string } | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!token || !key) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24 text-center">
        <AlertTriangle size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Invalid Link</h2>
        <p className="text-gray-500">This secret link appears to be broken or incomplete.</p>
      </div>
    )
  }

  const handleReveal = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setIsLoading(true)

    try {
      const response = await api.revealSecret(token, { email, key })
      setSecret(response.text)
    } catch (err) {
      if (err instanceof AppError) {
        setError({ type: err.type, message: friendlyError(err.type) })
      } else {
        setError({ type: "unknown", message: "Something went wrong. Please try again." })
      }
    } finally {
      setIsLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!secret) return
    await navigator.clipboard.writeText(secret)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (secret !== null) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24">
        <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
          <h2 className="font-semibold mb-4">Secret</h2>
          <div className="rounded-lg bg-gray-50 dark:bg-gray-900 p-4">
            <pre className="whitespace-pre-wrap text-sm break-words font-mono">{secret}</pre>
          </div>
          <div className="mt-4 flex items-center justify-between">
            <button
              type="button"
              onClick={handleCopy}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 text-sm font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              {copied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
              {copied ? "Copied" : "Copy to Clipboard"}
            </button>
          </div>
          <p className="mt-4 text-xs text-gray-400">
            This secret has been permanently deleted from our servers.
          </p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="max-w-lg mx-auto px-4 py-24 text-center">
        <AlertTriangle size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">
          {error.type === "expired" || error.type === "already_viewed"
            ? "Secret Unavailable"
            : "Unable to Reveal"}
        </h2>
        <p className="text-gray-500">{error.message}</p>
      </div>
    )
  }

  return (
    <div className="max-w-lg mx-auto px-4 py-24">
      <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-8 text-center">
        <Lock size={32} className="mx-auto text-gray-400 mb-4" />
        <h2 className="text-xl font-semibold mb-2">Someone sent you a secret</h2>
        <p className="text-sm text-gray-500 mb-6">
          Enter your email to reveal it. This secret will be permanently deleted after viewing.
        </p>

        <form onSubmit={handleReveal}>
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="Your email address"
            required
            className="w-full rounded-lg border border-gray-200 dark:border-gray-700 bg-transparent p-3 text-sm text-center focus:outline-none focus:ring-2 focus:ring-gray-900 dark:focus:ring-white"
          />
          <button
            type="submit"
            disabled={isLoading || !email.trim()}
            className="mt-4 w-full px-4 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity disabled:opacity-50 flex items-center justify-center gap-2"
          >
            {isLoading && <Loader2 size={16} className="animate-spin" />}
            Reveal Secret
          </button>
        </form>
      </div>
    </div>
  )
}
