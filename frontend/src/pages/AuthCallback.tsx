import { useEffect } from "react"
import { useNavigate } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"
import { api } from "../api/client"

export default function AuthCallback() {
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    if (!auth) {
      navigate("/", { replace: true })
      return
    }

    auth.refreshUser().then((user) => {
      if (user) {
        const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone
        if (browserTz && browserTz !== user.timezone) {
          api.updateTimezone(browserTz).catch(() => {
            // Timezone sync is best-effort — don't block login
          })
        }
      }

      navigate("/dashboard", { replace: true })
    })
  }, [auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Signing in...</p>
    </div>
  )
}
