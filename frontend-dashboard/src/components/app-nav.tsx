import { Link } from "@tanstack/react-router";
import { useServerFn } from "@tanstack/react-start";
import { Waypoints } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { handleLogout } from "@/server/auth.functions";

interface AppNavProps {
  user?: {
    email?: string;
    name?: string;
    username?: string;
    subject?: string;
  };
}

export function AppNav({ user }: AppNavProps) {
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const logout = useServerFn(handleLogout);

  const onLogout = async () => {
    setIsLoggingOut(true);
    try {
      await logout();
    } catch (error) {
      setIsLoggingOut(false);
      console.error("Logout failed:", error);
    }
  };

  const displayName = user?.username || user?.email || "User";

  return (
    <nav className="sticky top-0 z-20 flex items-center justify-between border-b border-slate-200/80 bg-white/95 px-4 py-3 backdrop-blur sm:px-6 lg:px-8">
      <div className="mx-auto flex w-full max-w-[1480px] items-center justify-between">
        <Link
          to="/dashboard"
          className="flex items-center gap-2.5 rounded-md text-slate-900 focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-indigo-500"
        >
          <span className="flex size-8 items-center justify-center rounded-lg bg-indigo-600 text-white shadow-sm">
            <Waypoints aria-hidden="true" className="size-4" />
          </span>
          <span className="font-semibold tracking-tight">Trapigo</span>
          <span className="hidden rounded-md bg-slate-100 px-2 py-1 text-[10px] font-semibold tracking-wide text-slate-500 uppercase sm:inline">
            Gateway
          </span>
        </Link>

        <div className="flex items-center gap-3 sm:gap-4">
          {displayName && (
            <span className="max-w-36 truncate text-xs text-slate-600 sm:max-w-56 sm:text-sm">
              {displayName}
            </span>
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
      </div>
    </nav>
  );
}
