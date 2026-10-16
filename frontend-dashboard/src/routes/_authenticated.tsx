import {
  createFileRoute,
  Outlet,
  redirect,
  useRouter,
} from "@tanstack/react-router";
import { AppNav } from "@/components/app-nav";
import { Button } from "@/components/ui/button";
import { getRouteSession } from "@/server/auth.functions";

export const Route = createFileRoute("/_authenticated")({
  beforeLoad: async ({ location }) => {
    const { user, loginHref, logoutAction } = await getRouteSession({
      data: location.href,
    });
    if (!user) {
      throw redirect({
        href: loginHref,
        headers: { "Cache-Control": "private, no-store", Vary: "Cookie" },
      });
    }
    return { user, logoutAction };
  },
  component: AuthLayout,
  errorComponent: AuthError,
});

function AuthLayout() {
  const { user, logoutAction } = Route.useRouteContext();

  return (
    <div className="flex min-h-svh flex-col">
      <AppNav user={user} logoutAction={logoutAction} />
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  );
}

function AuthError({ reset }: { reset: () => void }) {
  const router = useRouter();

  return (
    <main className="flex min-h-svh flex-col items-center justify-center gap-4 p-6">
      <p role="alert">Unable to verify your session. Please try again.</p>
      <Button onClick={() => void router.invalidate().then(reset)}>
        Try again
      </Button>
    </main>
  );
}
