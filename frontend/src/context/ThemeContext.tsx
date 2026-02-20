import { createContext, useCallback, useEffect, useState, type ReactNode } from "react"

type Theme = "light" | "dark"

interface ThemeContextValue {
  theme: Theme
  toggle: () => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

function getInitialTheme(): Theme {
  if (typeof window === "undefined") return "light"
  if (localStorage.theme === "dark") return "dark"
  if (!("theme" in localStorage) && window.matchMedia("(prefers-color-scheme: dark)").matches) return "dark"
  return "light"
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(getInitialTheme)

  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark")
    localStorage.theme = theme
  }, [theme])

  const toggle = useCallback(() => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"))
  }, [])

  return (
    <ThemeContext value={{ theme, toggle }}>
      {children}
    </ThemeContext>
  )
}
