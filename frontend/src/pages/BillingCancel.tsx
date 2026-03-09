import { useEffect } from "react"
import { useNavigate } from "react-router"

export default function BillingCancel() {
  const navigate = useNavigate()

  useEffect(() => {
    navigate("/dashboard", { replace: true })
  }, [navigate])

  return (
    <div className="min-h-screen flex items-center justify-center">
      <p className="text-gray-500">Returning to dashboard...</p>
    </div>
  )
}
