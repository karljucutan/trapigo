# TanStack Start + shadcn/ui

This is a template for a new TanStack Start project with React, TypeScript, and shadcn/ui.

## Authentication

The landing page is public. `/dashboard` is a child of the `_authenticated`
pathless layout. Its `beforeLoad` calls a POST server function to validate the
incoming HttpOnly cookies through the gateway's `/web/auth/me` endpoint.
This works for both SSR and client-side navigation; hydration does not require
a second React effect or browser-side token storage.

An unauthenticated response redirects the browser to
`http://localhost/web/auth/login?return_to=http%3A%2F%2Flocalhost%3A3000%2Fdashboard`.
The Go gateway validates the frontend origin and retains this URL in OAuth state
until the callback. Gateway errors are shown as errors, not treated as logout.
Refreshed/cleared auth cookies are forwarded to the browser, and session
responses are private and non-cacheable. The landing link disables preloading
so hovering does not trigger authentication or a login redirect.

Configure these **server-side** environment variables (development defaults
are shown):

```dotenv
APP_ORIGIN=http://localhost:3000
AUTH_PUBLIC_ORIGIN=http://localhost
AUTH_GATEWAY_ORIGIN=http://localhost
```

`AUTH_GATEWAY_ORIGIN` is the server-to-server gateway address; it defaults to
`AUTH_PUBLIC_ORIGIN`. Production requires explicit `APP_ORIGIN` and
`AUTH_PUBLIC_ORIGIN` and HTTPS origins. Keep the gateway's
`FRONTEND_REDIRECT_URL` on the same frontend origin as `APP_ORIGIN`.

The frontend and gateway must receive the same auth cookies. Localhost cookies
are shared across ports. In production, use the same hostname via a reverse
proxy (cookie path `/`) and enable secure cookies; distinct hostnames require a
deliberate shared-session/cookie design. Do not cache protected HTML/RPC
responses at a CDN.

Route guards protect navigation, not private server operations. Apply the
exported `requireAuth` middleware from `src/lib/auth.functions.ts` to every
protected Start server function:

```tsx
import { createServerFn } from "@tanstack/react-start"
import { requireAuth } from "@/lib/auth.functions"

export const getPrivateData = createServerFn({ method: "POST" })
  .middleware([requireAuth])
  .handler(({ context }) => {
    // Authorize access to the requested resource using context.user.subject.
    return { subject: context.user.subject }
  })
```

The dashboard currently contains only the generated placeholder, not private
data. A mounted UI is not continuous session validation: protected requests
must revalidate, and login/logout should invalidate the router. The existing
gateway uses an in-memory OAuth state store; multi-instance deployments need a
shared, expiring, single-use state store.

Validation (Node 24+):

```bash
node --test src/lib/auth-gateway.test.ts
npm run generate-routes
npm run typecheck
npm run build
```

## Adding components

To add components to your app, run the following command:

```bash
npx shadcn@latest add button
```

This will place the ui components in the `components` directory.

## Using components

To use the components in your app, import them as follows:

```tsx
import { Button } from "@/components/ui/button";
```
