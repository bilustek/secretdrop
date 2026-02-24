import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 3000,
    proxy: {
      "/api": "http://localhost:8080",
      "/auth/google": "http://localhost:8080",
      "/auth/github": "http://localhost:8080",
      "/auth/apple": "http://localhost:8080",
      "/auth/token": "http://localhost:8080",
      "/auth/refresh": "http://localhost:8080",
      "/auth/logout": "http://localhost:8080",
      "/billing/checkout": "http://localhost:8080",
      "/billing/portal": "http://localhost:8080",
      "/billing/webhook": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
    },
  },
})
