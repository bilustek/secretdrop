import { useState, useCallback } from "react"

const STORAGE_KEY = "secretdrop:recent-emails"
const MAX_ENTRIES = 20

function load(): string[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((e): e is string => typeof e === "string")
  } catch {
    return []
  }
}

function save(emails: string[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(emails.slice(0, MAX_ENTRIES)))
}

export function useRecentEmails() {
  const [recentEmails, setRecentEmails] = useState(load)

  const addEmails = useCallback((newEmails: string[]) => {
    setRecentEmails((prev) => {
      const lower = new Set(prev.map((e) => e.toLowerCase()))
      const merged = [...prev]
      for (const email of newEmails) {
        if (!lower.has(email.toLowerCase())) {
          merged.unshift(email)
          lower.add(email.toLowerCase())
        }
      }
      const trimmed = merged.slice(0, MAX_ENTRIES)
      save(trimmed)
      return trimmed
    })
  }, [])

  const suggest = useCallback(
    (input: string, exclude: string[]): string[] => {
      if (!input) return []
      const q = input.toLowerCase()
      const excluded = new Set(exclude.map((e) => e.toLowerCase()))
      return recentEmails.filter(
        (e) => e.toLowerCase().includes(q) && !excluded.has(e.toLowerCase()),
      )
    },
    [recentEmails],
  )

  return { recentEmails, addEmails, suggest }
}
