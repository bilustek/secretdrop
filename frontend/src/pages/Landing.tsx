import { use } from "react"
import { Navigate } from "react-router"
import { Shield, Mail, Flame } from "lucide-react"
import { AuthContext } from "../context/AuthContext"

export default function Landing() {
  const auth = use(AuthContext)

  if (auth?.isAuthenticated) {
    return <Navigate to="/dashboard" replace />
  }

  return (
    <div>
      {/* Hero */}
      <section className="max-w-3xl mx-auto px-4 py-24 text-center">
        <h1 className="text-4xl sm:text-5xl font-bold tracking-tight">
          Share secrets that disappear
          <br />
          after one read.
        </h1>
        <p className="mt-4 text-lg text-gray-600 dark:text-gray-400">
          End-to-end encrypted. Zero-knowledge. One-time links.
        </p>
        <div className="mt-8 flex flex-col sm:flex-row gap-3 justify-center">
          <a
            href="/auth/google"
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
          >
            Sign in with Google
          </a>
          <a
            href="/auth/github"
            className="inline-flex items-center justify-center gap-2 px-6 py-3 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
          >
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
              <li>1 secret (lifetime)</li>
              <li>Up to 5 recipients</li>
              <li>AES-256-GCM encryption</li>
            </ul>
            <a
              href="/auth/google"
              className="mt-6 block text-center px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 font-medium hover:bg-gray-50 dark:hover:bg-gray-900 transition-colors"
            >
              Get Started
            </a>
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
            <a
              href="/auth/google"
              className="mt-6 block text-center px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity"
            >
              Start Free, Upgrade Later
            </a>
          </div>
        </div>
      </section>
    </div>
  )
}
