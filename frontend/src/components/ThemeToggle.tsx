import { use } from "react"
import { Moon, Sun } from "lucide-react"
import { ThemeContext } from "../context/ThemeContext"

export function ThemeToggle() {
  const ctx = use(ThemeContext)
  if (!ctx) throw new Error("ThemeToggle must be used within ThemeProvider")

  return (
    <button
      type="button"
      onClick={ctx.toggle}
      className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
      aria-label={ctx.theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
    >
      {ctx.theme === "dark" ? <Sun size={20} /> : <Moon size={20} />}
    </button>
  )
}
