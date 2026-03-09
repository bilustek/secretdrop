import { use, useCallback, useEffect, useState } from "react"
import { Navigate } from "react-router"
import { Shield, Mail, Flame, KeyRound, Hash, Eye, Database } from "lucide-react"
import { AuthContext } from "../context/AuthContext"
import { API_URL } from "../api/config"
import { api, type Plan } from "../api/client"

const showGoogle = import.meta.env.VITE_ENABLE_GOOGLE_SIGNIN !== "false"
const showApple = import.meta.env.VITE_ENABLE_APPLE_SIGNIN !== "false"

interface Headline {
  text: string
  highlight: string
}

const HEADLINES: Headline[] = [
  { text: "Share secrets that disappear after one read.", highlight: "secrets" },
  { text: "Stop pasting API keys in Slack.", highlight: "API keys" },
  { text: "One-time links. Zero-knowledge encryption.", highlight: "One-time" },
  { text: "Share .env variables without the risk.", highlight: ".env" },
  { text: "Encrypted. Delivered. Destroyed.", highlight: "Destroyed" },
]

const TYPE_SPEED = 50
const DELETE_SPEED = 30
const PAUSE_AFTER_TYPE = 2000
const PAUSE_AFTER_DELETE = 400

function useTypewriter(headlines: Headline[]) {
  const [index, setIndex] = useState(0)
  const [charCount, setCharCount] = useState(0)
  const [isDeleting, setIsDeleting] = useState(false)

  const current = headlines[index]

  const tick = useCallback(() => {
    if (!isDeleting) {
      setCharCount((c) => c + 1)
      if (charCount + 1 === current.text.length) {
        setTimeout(() => setIsDeleting(true), PAUSE_AFTER_TYPE)
        return
      }
    } else {
      setCharCount((c) => c - 1)
      if (charCount - 1 === 0) {
        setIsDeleting(false)
        setIndex((i) => (i + 1) % headlines.length)
        return
      }
    }
  }, [headlines, current, charCount, isDeleting])

  useEffect(() => {
    let delay = isDeleting ? DELETE_SPEED : TYPE_SPEED
    if (!isDeleting && charCount === current.text.length) delay = PAUSE_AFTER_TYPE
    if (isDeleting && charCount === 0) delay = PAUSE_AFTER_DELETE

    const timer = setTimeout(tick, delay)
    return () => clearTimeout(timer)
  }, [tick, isDeleting, charCount, current])

  const typed = current.text.slice(0, charCount)
  const hlStart = current.text.indexOf(current.highlight)
  const hlEnd = hlStart + current.highlight.length

  if (hlStart === -1 || charCount <= hlStart) {
    return { before: typed, highlighted: "", after: "" }
  }

  return {
    before: typed.slice(0, hlStart),
    highlighted: typed.slice(hlStart, Math.min(charCount, hlEnd)),
    after: charCount > hlEnd ? typed.slice(hlEnd) : "",
  }
}

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18A10.96 10.96 0 0 0 1 12c0 1.77.42 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
    </svg>
  )
}

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
    </svg>
  )
}

function AppleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M17.05 20.28c-.98.95-2.05.88-3.08.4-1.09-.5-2.08-.48-3.24 0-1.44.62-2.2.44-3.06-.4C2.79 15.25 3.51 7.59 9.05 7.31c1.35.07 2.29.74 3.08.8 1.18-.24 2.31-.93 3.57-.84 1.51.12 2.65.72 3.4 1.8-3.12 1.87-2.38 5.98.48 7.13-.57 1.5-1.31 2.99-2.54 4.09zM12.03 7.25c-.15-2.23 1.66-4.07 3.74-4.25.29 2.58-2.34 4.5-3.74 4.25z" />
    </svg>
  )
}

function formatPrice(plan: Plan): { amount: string; period: string } {
  if (plan.price_cents === 0) {
    return { amount: "Free", period: "forever" }
  }
  const dollars = (plan.price_cents / 100).toFixed(2)
  return { amount: `$${dollars}`, period: "per month" }
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${bytes / (1024 * 1024)}MB`
  return `${bytes / 1024}KB`
}

export default function Landing() {
  const auth = use(AuthContext)
  const { before, highlighted, after } = useTypewriter(HEADLINES)

  const [plans, setPlans] = useState<Plan[]>([])
  const [signInOpen, setSignInOpen] = useState(false)

  useEffect(() => {
    api.plans().then(setPlans).catch(() => {})
  }, [])

  useEffect(() => {
    if (!signInOpen) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setSignInOpen(false)
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [signInOpen])

  if (auth?.isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  return (
    <div>
      {/* Hero */}
      <section className="max-w-3xl mx-auto px-4 py-24 text-center">
        <h1 className="text-4xl sm:text-5xl font-bold tracking-tight h-[2.8em] sm:h-[1.4em] flex items-center justify-center">
          <span>
            {before}
            {highlighted && (
              <span className="text-indigo-500 dark:text-indigo-400">{highlighted}</span>
            )}
            {after}
            <span className="inline-block w-[3px] h-[1em] bg-gray-900 dark:bg-white ml-0.5 align-middle animate-blink" />
          </span>
        </h1>
        <div className="mt-8 flex flex-col sm:flex-row gap-3 justify-center">
          {showGoogle && (
            <a
              href={`${API_URL}/auth/google`}
              className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
            >
              <GoogleIcon className="w-5 h-5" />
              Sign in with Google
            </a>
          )}
          <a
            href={`${API_URL}/auth/github`}
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            <GitHubIcon className="w-5 h-5" />
            Sign in with GitHub
          </a>
          {showApple && (
            <a
              href={`${API_URL}/auth/apple`}
              className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-black text-white font-medium hover:opacity-90 transition-opacity dark:bg-white dark:text-black"
            >
              <AppleIcon className="w-5 h-5" />
              Sign in with Apple
            </a>
          )}
        </div>
      </section>

      {/* Open Source */}
      <section className="max-w-4xl mx-auto px-4 pb-8">
        <div className="rounded-2xl border border-gray-200 dark:border-gray-800 bg-gray-50/50 dark:bg-gray-900/50 p-8 sm:p-10">
          <p className="text-center text-xs font-semibold text-indigo-500 dark:text-indigo-400 uppercase tracking-widest mb-2">
            Open Source
          </p>
          <h2 className="text-2xl sm:text-3xl font-bold text-center mb-2">
            Trust issues? Good. Read the source.
          </h2>
          <p className="text-center text-gray-500 dark:text-gray-400 mb-8 max-w-xl mx-auto">
            The entire project is public under the MIT license. No hidden endpoints, no secret telemetry, no &ldquo;just trust us&rdquo; moments.
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <a
              href="https://github.com/bilustek/secretdrop"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-start gap-4 rounded-xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 p-5 hover:border-indigo-300 dark:hover:border-indigo-700 hover:shadow-md transition-all group"
            >
              <div className="flex-shrink-0 w-12 h-12 rounded-xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center group-hover:bg-indigo-100 dark:group-hover:bg-indigo-900/50 transition-colors">
                <GitHubIcon className="w-6 h-6 text-gray-700 dark:text-gray-300" />
              </div>
              <div className="min-w-0">
                <p className="font-semibold text-gray-900 dark:text-white group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
                  bilustek/secretdrop
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 leading-relaxed">
                  Full application &mdash; Go API, React frontend, OAuth, Stripe billing &amp; everything in between.
                </p>
                <span className="inline-flex items-center gap-1 mt-2.5 text-xs font-medium text-indigo-500 dark:text-indigo-400 opacity-0 group-hover:opacity-100 transition-opacity">
                  View on GitHub
                  <svg className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
                  </svg>
                </span>
              </div>
            </a>
            <a
              href="https://github.com/bilustek/secretdropvault"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-start gap-4 rounded-xl border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 p-5 hover:border-indigo-300 dark:hover:border-indigo-700 hover:shadow-md transition-all group"
            >
              <div className="flex-shrink-0 w-12 h-12 rounded-xl bg-gray-100 dark:bg-gray-800 flex items-center justify-center group-hover:bg-indigo-100 dark:group-hover:bg-indigo-900/50 transition-colors">
                <Shield size={24} className="text-gray-700 dark:text-gray-300" />
              </div>
              <div className="min-w-0">
                <p className="font-semibold text-gray-900 dark:text-white group-hover:text-indigo-600 dark:group-hover:text-indigo-400 transition-colors">
                  bilustek/secretdropvault
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1 leading-relaxed">
                  Standalone encryption engine &mdash; AES-256-GCM with HKDF-SHA256 key derivation.
                </p>
                <span className="inline-flex items-center gap-1 mt-2.5 text-xs font-medium text-indigo-500 dark:text-indigo-400 opacity-0 group-hover:opacity-100 transition-opacity">
                  View on GitHub
                  <svg className="w-3.5 h-3.5 transition-transform group-hover:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
                  </svg>
                </span>
              </div>
            </a>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="max-w-5xl mx-auto px-4 py-16">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Shield size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">AES-256-GCM Encryption</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Your secrets are encrypted before storage. The key never touches our servers.
            </p>
          </div>
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Mail size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">Share via Email</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Recipients get a one-time link and must verify their email before the secret is revealed. Only the intended recipient can decrypt.
            </p>
          </div>
          <div className="text-center p-6">
            <div className="inline-flex items-center justify-center w-12 h-12 rounded-lg bg-gray-100 dark:bg-gray-800 mb-4">
              <Flame size={24} className="text-gray-700 dark:text-gray-300" />
            </div>
            <h3 className="font-semibold mb-2">Burn After Reading</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Secrets are permanently deleted after viewing or when they expire.
            </p>
          </div>
        </div>
      </section>

      {/* How It Works — Trust Section */}
      <section className="border-y border-gray-200 dark:border-gray-800">
        <div className="max-w-2xl mx-auto px-4 py-20">
          <h2 className="text-2xl font-bold text-center mb-3">
            Why you can trust SecretDrop
          </h2>
          <p className="text-center text-gray-500 dark:text-gray-400 mb-12 max-w-2xl mx-auto">
            We designed SecretDrop so that <span className="text-gray-900 dark:text-white font-medium">even we cannot read your secrets</span>. Here's how.
          </p>

          <div className="space-y-10">
            <div className="flex gap-5">
              <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-indigo-50 dark:bg-indigo-950 flex items-center justify-center">
                <KeyRound size={20} className="text-indigo-500 dark:text-indigo-400" />
              </div>
              <div>
                <h3 className="font-semibold mb-1">The encryption key never reaches our servers</h3>
                <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  When you create a secret, a random AES-256-GCM key is generated <span className="font-medium text-gray-900 dark:text-white">in your browser</span>.
                  The key is placed in the URL fragment (<code className="text-xs bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded">#</code> part) — browsers and proxies
                  never send this to the server. We only store the encrypted ciphertext.
                </p>
              </div>
            </div>

            <div className="flex gap-5">
              <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-indigo-50 dark:bg-indigo-950 flex items-center justify-center">
                <Hash size={20} className="text-indigo-500 dark:text-indigo-400" />
              </div>
              <div>
                <h3 className="font-semibold mb-1">Recipient-bound decryption via HKDF</h3>
                <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  Each key is derived using HKDF-SHA256 bound to the recipient's email. Even if someone intercepts the link,
                  they cannot decrypt it without the exact email address. We store only a SHA-256 hash of the email — the raw
                  address is never persisted.
                </p>
              </div>
            </div>

            <div className="flex gap-5">
              <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-indigo-50 dark:bg-indigo-950 flex items-center justify-center">
                <Eye size={20} className="text-indigo-500 dark:text-indigo-400" />
              </div>
              <div>
                <h3 className="font-semibold mb-1">One read, then gone forever</h3>
                <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  Once a recipient opens the link, the encrypted record is <span className="font-medium text-gray-900 dark:text-white">permanently deleted</span> from
                  the database. A second request returns 404. Unread secrets are automatically purged after expiry.
                </p>
              </div>
            </div>

            <div className="flex gap-5">
              <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-indigo-50 dark:bg-indigo-950 flex items-center justify-center">
                <Database size={20} className="text-indigo-500 dark:text-indigo-400" />
              </div>
              <div>
                <h3 className="font-semibold mb-1">Database breach? Still safe.</h3>
                <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  Even with full database access, an attacker would find only AES-256-GCM ciphertext and hashed emails.
                  Without the key (which only exists in the one-time link), the data is computationally impossible to decrypt.
                </p>
              </div>
            </div>
          </div>

        </div>
      </section>

      {/* Pricing */}
      <section className="max-w-5xl mx-auto px-4 py-16">
        <h2 className="text-2xl font-bold text-center mb-8">Simple Pricing</h2>
        <div className={`grid grid-cols-1 gap-6 ${plans.length === 2 ? "sm:grid-cols-2 max-w-3xl mx-auto" : "sm:grid-cols-2 lg:grid-cols-3"}`}>
          {plans.map((plan) => {
            const { amount, period } = formatPrice(plan)
            const isPopular = plan.tier === "pro"
            return (
              <div
                key={plan.tier}
                className={`rounded-xl p-6 relative ${isPopular ? "border-2 border-gray-900 dark:border-white" : "border border-gray-200 dark:border-gray-800"}`}
              >
                {isPopular && (
                  <span className="absolute -top-3 left-4 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-xs font-medium px-2 py-1 rounded">
                    Popular
                  </span>
                )}
                <h3 className="font-semibold text-lg capitalize">{plan.tier}</h3>
                <p className="text-3xl font-bold mt-2">{amount}</p>
                <p className="text-sm text-gray-500 mt-1">{period}</p>
                <ul className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
                  <li>{plan.secrets_limit} secrets {plan.price_cents === 0 ? "(lifetime)" : "per month"}</li>
                  <li>Up to {plan.recipients_limit} recipient{plan.recipients_limit > 1 ? "s" : ""}</li>
                  <li>Up to {formatBytes(plan.max_text_length)} per secret</li>
                  <li>AES-256-GCM encryption</li>
                </ul>
                <button
                  type="button"
                  onClick={() => {
                    if (plan.price_cents > 0) {
                      localStorage.setItem("pending_checkout_tier", plan.tier)
                    }
                    setSignInOpen(true)
                  }}
                  className={`mt-6 block w-full text-center px-4 py-2 rounded-lg font-medium transition-all ${
                    plan.price_cents === 0
                      ? "border border-gray-300 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-900"
                      : "bg-gray-900 dark:bg-white text-white dark:text-gray-900 hover:opacity-90"
                  }`}
                >
                  {plan.price_cents === 0 ? "Get Started" : "Start Free, Upgrade Later"}
                </button>
              </div>
            )
          })}
        </div>
      </section>

      {signInOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
          onClick={() => setSignInOpen(false)}
          role="presentation"
        >
          <div
            className="max-w-sm w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
            onClick={(e) => e.stopPropagation()}
            role="presentation"
          >
            <h2 className="text-lg font-semibold text-center">Sign in to continue</h2>
            <div className="mt-6 flex flex-col gap-3">
              <a
                href={`${API_URL}/auth/google`}
                className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
              >
                <GoogleIcon className="w-5 h-5" />
                Sign in with Google
              </a>
              <a
                href={`${API_URL}/auth/github`}
                className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
              >
                <GitHubIcon className="w-5 h-5" />
                Sign in with GitHub
              </a>
              {showApple && (
                <a
                  href={`${API_URL}/auth/apple`}
                  className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-black text-white font-medium hover:opacity-90 transition-opacity dark:bg-white dark:text-black"
                >
                  <AppleIcon className="w-5 h-5" />
                  Sign in with Apple
                </a>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
