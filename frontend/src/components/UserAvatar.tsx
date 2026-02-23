import { useEffect, useState } from "react"
import { User } from "lucide-react"

interface UserAvatarProps {
  avatarUrl: string
  name: string
  email: string
  size?: number
}

async function sha256Hex(input: string): Promise<string> {
  const bytes = new TextEncoder().encode(input.trim().toLowerCase())
  const hash = await crypto.subtle.digest("SHA-256", bytes)
  return Array.from(new Uint8Array(hash), (b) => b.toString(16).padStart(2, "0")).join("")
}

function getInitials(name: string, email: string): string {
  if (name) {
    const parts = name.trim().split(/\s+/)
    if (parts.length >= 2) {
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return parts[0][0].toUpperCase()
  }
  if (email) {
    return email[0].toUpperCase()
  }
  return ""
}

export function UserAvatar({ avatarUrl, name, email, size = 32 }: UserAvatarProps) {
  const [imgFailed, setImgFailed] = useState(false)
  const [gravatarUrl, setGravatarUrl] = useState<string | null>(null)
  const [gravatarFailed, setGravatarFailed] = useState(false)

  useEffect(() => {
    if (avatarUrl || !email) return
    let cancelled = false
    sha256Hex(email).then((hash) => {
      if (!cancelled) {
        setGravatarUrl(`https://gravatar.com/avatar/${hash}?d=404&s=${size * 2}`)
      }
    })
    return () => { cancelled = true }
  }, [avatarUrl, email, size])

  // 1. Provider avatar (Google, GitHub)
  if (avatarUrl && !imgFailed) {
    return (
      <img
        src={avatarUrl}
        alt=""
        referrerPolicy="no-referrer"
        onError={() => setImgFailed(true)}
        className="rounded-full"
        style={{ width: size, height: size }}
      />
    )
  }

  // 2. Gravatar (SHA-256 hash)
  if (gravatarUrl && !gravatarFailed) {
    return (
      <img
        src={gravatarUrl}
        alt=""
        referrerPolicy="no-referrer"
        onError={() => setGravatarFailed(true)}
        className="rounded-full"
        style={{ width: size, height: size }}
      />
    )
  }

  // 3. Initials from name or email
  const initials = getInitials(name, email)
  if (initials) {
    return (
      <div
        className="rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center font-medium text-gray-600 dark:text-gray-300"
        style={{ width: size, height: size, fontSize: size * 0.4 }}
      >
        {initials}
      </div>
    )
  }

  // 4. Generic user icon
  return (
    <div
      className="rounded-full bg-gray-200 dark:bg-gray-700 flex items-center justify-center text-gray-500 dark:text-gray-400"
      style={{ width: size, height: size }}
    >
      <User size={size * 0.55} />
    </div>
  )
}
