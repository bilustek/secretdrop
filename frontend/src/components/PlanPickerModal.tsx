import { useEffect, useState } from "react"
import { api, type Plan } from "../api/client"

interface Props {
  onClose: () => void
}

function formatPrice(plan: Plan): { amount: string; period: string } {
  if (plan.price_cents === 0) return { amount: "Free", period: "forever" }
  const dollars = (plan.price_cents / 100).toFixed(2)
  return { amount: `$${dollars}`, period: "per month" }
}

function formatBytes(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${bytes / (1024 * 1024)}MB`
  return `${bytes / 1024}KB`
}

export function PlanPickerModal({ onClose }: Props) {
  const [plans, setPlans] = useState<Plan[]>([])
  const [loading, setLoading] = useState<string | null>(null)

  useEffect(() => {
    api.plans().then((p) => setPlans(p.filter((pl) => pl.price_cents > 0))).catch(() => {})
  }, [])

  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose()
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [onClose])

  const handleSelect = async (tier: string) => {
    setLoading(tier)
    try {
      const { url } = await api.checkout(tier)
      window.location.href = url
    } catch {
      setLoading(null)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm"
      onClick={onClose}
      role="presentation"
    >
      <div
        className="max-w-2xl w-full mx-4 rounded-lg border border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-950 shadow-xl p-6"
        onClick={(e) => e.stopPropagation()}
        role="presentation"
      >
        <h2 className="text-lg font-semibold text-center mb-6">Choose a Plan</h2>
        {plans.length === 0 ? (
          <p className="text-center text-gray-500 py-8">Loading plans...</p>
        ) : (
          <div className={`grid gap-4 ${plans.length === 1 ? "grid-cols-1 max-w-xs mx-auto" : "grid-cols-1 sm:grid-cols-2"}`}>
            {plans.map((plan) => {
              const { amount, period } = formatPrice(plan)
              const isPopular = plan.tier === "pro"
              return (
                <div
                  key={plan.tier}
                  className={`rounded-xl p-5 relative ${isPopular ? "border-2 border-gray-900 dark:border-white" : "border border-gray-200 dark:border-gray-800"}`}
                >
                  {isPopular && (
                    <span className="absolute -top-3 left-4 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-xs font-medium px-2 py-1 rounded">
                      Popular
                    </span>
                  )}
                  <h3 className="font-semibold text-lg capitalize">{plan.tier}</h3>
                  <p className="text-2xl font-bold mt-1">{amount}</p>
                  <p className="text-sm text-gray-500">{period}</p>
                  <ul className="mt-3 space-y-1.5 text-sm text-gray-600 dark:text-gray-400">
                    <li>{plan.secrets_limit} secrets per month</li>
                    <li>Up to {plan.recipients_limit} recipient{plan.recipients_limit > 1 ? "s" : ""}</li>
                    <li>Up to {formatBytes(plan.max_text_length)} per secret</li>
                  </ul>
                  <button
                    type="button"
                    disabled={loading !== null}
                    onClick={() => handleSelect(plan.tier)}
                    className="mt-4 block w-full text-center px-4 py-2 rounded-lg bg-gray-900 dark:bg-white text-white dark:text-gray-900 font-medium hover:opacity-90 transition-opacity disabled:opacity-50"
                  >
                    {loading === plan.tier ? "Redirecting..." : `Choose ${plan.tier.charAt(0).toUpperCase() + plan.tier.slice(1)}`}
                  </button>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
