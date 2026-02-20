import { useEffect } from "react"
import { useNavigate } from "react-router"
import { use } from "react"
import { AuthContext } from "../context/AuthContext"

export default function BillingSuccess() {
  const navigate = useNavigate()
  const auth = use(AuthContext)

  useEffect(() => {
    if (auth) {
      auth.refreshUser().then(() => navigate("/dashboard", { replace: true }))
    } else {
      navigate("/dashboard", { replace: true })
    }
  }, [auth, navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Updating your subscription...</p>
    </div>
  )
}
