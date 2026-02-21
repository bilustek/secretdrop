import { use, useCallback, useEffect, useState } from "react"
import { Navigate } from "react-router"
import { Shield, Mail, Flame } from "lucide-react"
import { AuthContext } from "../context/AuthContext"

const showGoogle = import.meta.env.VITE_ENABLE_GOOGLE_SIGNIN !== "false"

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

export default function Landing() {
  const auth = use(AuthContext)
  const { before, highlighted, after } = useTypewriter(HEADLINES)

  const [signInOpen, setSignInOpen] = useState(false)

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
              href="/auth/google"
              className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
            >
              <GoogleIcon className="w-5 h-5" />
              Sign in with Google
            </a>
          )}
          <a
            href="/auth/github"
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
            <GitHubIcon className="w-5 h-5" />
            Sign in with GitHub
          </a>
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
              Recipients get a one-time link. Only the intended recipient can decrypt.
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

      {/* Pricing */}
      <section className="max-w-3xl mx-auto px-4 py-16">
        <h2 className="text-2xl font-bold text-center mb-8">Simple Pricing</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-6">
          <div className="border border-gray-200 dark:border-gray-800 rounded-xl p-6">
            <h3 className="font-semibold text-lg">Free</h3>
            <p className="text-3xl font-bold mt-2">$0</p>
            <p className="text-sm text-gray-500 mt-1">forever</p>
            <ul className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <li>5 secrets (lifetime)</li>
              <li>1 recipient per secret</li>
              <li>AES-256-GCM encryption</li>
            </ul>
            <button
              type="button"
              onClick={() => setSignInOpen(true)}
              className="mt-6 block w-full text-center px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              Get Started
            </button>
          </div>
          <div className="border-2 border-gray-900 dark:border-white rounded-xl p-6 relative">
            <span className="absolute -top-3 left-4 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-xs font-medium px-2 py-1 rounded">
              Popular
            </span>
            <h3 className="font-semibold text-lg">Pro</h3>
            <p className="text-3xl font-bold mt-2">$2.99</p>
            <p className="text-sm text-gray-500 mt-1">per month</p>
            <ul className="mt-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
              <li>100 secrets per month</li>
              <li>Up to 5 recipients</li>
              <li>AES-256-GCM encryption</li>
            </ul>
            <button
              type="button"
              onClick={() => setSignInOpen(true)}
              className="mt-6 block w-full text-center px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
            >
              Start Free, Upgrade Later
            </button>
          </div>
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
                href="/auth/google"
                className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
              >
                <GoogleIcon className="w-5 h-5" />
                Sign in with Google
              </a>
              <a
                href="/auth/github"
                className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
              >
                <GitHubIcon className="w-5 h-5" />
                Sign in with GitHub
              </a>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
