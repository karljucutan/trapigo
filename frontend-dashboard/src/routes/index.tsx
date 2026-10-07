import { createFileRoute, Link } from "@tanstack/react-router"
import { Button } from "@/components/ui/button"

export const Route = createFileRoute("/")({ component: App })

function App() {
  return (
    <main className="flex min-h-svh items-center justify-center p-6">
      <Button
        render={<Link to="/dashboard" preload={false} />}
        nativeButton={false}
      >
        View Dashboard
      </Button>
    </main>
  )
}
