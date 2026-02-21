import { Routes, Route, Navigate } from "react-router"
import { Layout } from "./components/Layout"
import { ProtectedRoute } from "./components/ProtectedRoute"
import { AdminLayout } from "./components/AdminLayout"
import Landing from "./pages/Landing"
import Dashboard from "./pages/Dashboard"
import Reveal from "./pages/Reveal"
import AuthCallback from "./pages/AuthCallback"
import BillingSuccess from "./pages/BillingSuccess"
import Terms from "./pages/Terms"
import Privacy from "./pages/Privacy"
import Contact from "./pages/Contact"
import AdminLogin from "./pages/admin/Login"
import AdminUsers from "./pages/admin/Users"
import AdminSubscriptions from "./pages/admin/Subscriptions"

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<Landing />} />
        <Route path="s/:token" element={<Reveal />} />
        <Route path="terms" element={<Terms />} />
        <Route path="privacy" element={<Privacy />} />
        <Route path="contact" element={<Contact />} />
        <Route element={<ProtectedRoute />}>
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="billing/success" element={<BillingSuccess />} />
        </Route>
      </Route>
      <Route path="auth/callback" element={<AuthCallback />} />
      <Route path="admin/login" element={<AdminLogin />} />
      <Route path="admin" element={<AdminLayout />}>
        <Route index element={<Navigate to="/admin/users" replace />} />
        <Route path="users" element={<AdminUsers />} />
        <Route path="subscriptions" element={<AdminSubscriptions />} />
      </Route>
    </Routes>
  )
}
