import { Link } from "@tanstack/react-router";
import { Waypoints } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";

interface AppNavProps {
  logoutAction: string;
  user?: {
    email?: string;
    name?: string;
    username?: string;
    subject?: string;
  };
}

export function AppNav({ user, logoutAction }: AppNavProps) {
  const [isLoggingOut, setIsLoggingOut] = useState(false);

  const displayName = user?.username || user?.email || "User";

  return (
    <nav className="sticky top-0 z-20 flex items-center justify-between border-b border-border/80 bg-background/95 px-4 py-3 backdrop-blur sm:px-6 lg:px-8">
      <div className="mx-auto flex w-full max-w-[1480px] items-center justify-between">
        <Link
          to="/dashboard"
          className="flex items-center gap-2.5 rounded-md text-foreground focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary"
        >
          <span className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
            <Waypoints aria-hidden="true" className="size-4" />
          </span>
          <span className="font-semibold tracking-tight">Trapigo</span>
          <span className="hidden rounded-md bg-muted px-2 py-1 text-[10px] font-semibold tracking-wide text-muted-foreground uppercase sm:inline">
            Gateway
          </span>
        </Link>

        <div className="flex items-center gap-3 sm:gap-4">
          {displayName && (
            <span className="max-w-36 truncate text-xs text-muted-foreground sm:max-w-56 sm:text-sm">
              {displayName}
            </span>
          )}
          <form
            action={logoutAction}
            method="post"
            onSubmit={() => setIsLoggingOut(true)}
          >
            <Button
              type="submit"
              disabled={isLoggingOut}
              variant="outline"
              size="sm"
            >
              {isLoggingOut ? "Logging out..." : "Logout"}
            </Button>
          </form>
        </div>
      </div>
    </nav>
  );
}
