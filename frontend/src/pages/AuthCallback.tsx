import { useEffect } from "react"
import { useNavigate, useSearchParams } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"

export default function AuthCallback() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    const accessToken = params.get("access_token")
    const refreshToken = params.get("refresh_token")

    if (accessToken && refreshToken && auth) {
      auth.login(accessToken, refreshToken)
      auth.refreshUser().then(() => navigate("/dashboard", { replace: true }))
    } else {
      navigate("/", { replace: true })
    }
  }, [params, auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Signing in...</p>
    </div>
  )
}
