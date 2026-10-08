import { useState } from "react"
import { useServerFn } from "@tanstack/react-start"
import { Button } from "@/components/ui/button"
import { handleLogout } from "@/server/auth.functions"

interface AppNavProps {
  user?: {
    email?: string
    name?: string
    username?: string
    subject?: string
  }
}

export function AppNav({ user }: AppNavProps) {
  const [isLoggingOut, setIsLoggingOut] = useState(false)
  const logout = useServerFn(handleLogout)

  const onLogout = async () => {
    setIsLoggingOut(true)
    try {
      await logout()
    } catch (error) {
      setIsLoggingOut(false)
      console.error("Logout failed:", error)
    }
  }

  const displayName = user?.username || user?.email || "User"

  return (
    <nav className="flex items-center justify-between border-b border-gray-200 bg-white px-6 py-4 shadow-sm">
      <div className="flex items-center gap-8">
        <h1 className="font-semibold text-lg">Trapigo</h1>
      </div>

      <div className="flex items-center gap-4">
        {displayName && (
          <span className="text-sm text-gray-600">{displayName}</span>
        )}
        <Button
          onClick={onLogout}
          disabled={isLoggingOut}
          variant="outline"
          size="sm"
        >
          {isLoggingOut ? "Logging out..." : "Logout"}
        </Button>
      </div>
    </nav>
  )
}
