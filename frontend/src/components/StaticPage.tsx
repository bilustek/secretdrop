import type { ReactNode } from "react"

interface StaticPageProps {
  title: string
  children: ReactNode
}

export function StaticPage({ title, children }: StaticPageProps) {
  return (
    <div className="max-w-3xl mx-auto px-4 py-16">
      <h1 className="text-3xl font-bold mb-8">{title}</h1>
      <div className="prose dark:prose-invert max-w-none">{children}</div>
    </div>
  )
}
