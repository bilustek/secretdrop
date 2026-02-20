import { Routes, Route } from "react-router"
import { Layout } from "./components/Layout"
import { ProtectedRoute } from "./components/ProtectedRoute"
import Landing from "./pages/Landing"
import Dashboard from "./pages/Dashboard"
import Reveal from "./pages/Reveal"
import AuthCallback from "./pages/AuthCallback"
import BillingSuccess from "./pages/BillingSuccess"

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Landing />} />
        <Route path="s/:token" element={<Reveal />} />
        <Route element={<ProtectedRoute />}>
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="billing/success" element={<BillingSuccess />} />
        </Route>
      </Route>
      <Route path="auth/callback" element={<AuthCallback />} />
    </Routes>
  )
}
