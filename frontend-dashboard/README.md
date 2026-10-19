# TanStack Start + shadcn/ui

This is a template for a new TanStack Start project with React, TypeScript, and shadcn/ui.

## React Compiler

React Compiler is enabled in `vite.config.ts` through
`@vitejs/plugin-react`'s `reactCompilerPreset` and `@rolldown/plugin-babel`.
It uses the Babel compiler with the default React 19 target; no separate
compiler runtime package is needed.

The compiler automatically memoizes eligible components and calculations.
Components use plain calculations rather than manual `useMemo` or `useCallback`
wrappers. Effects, their dependency arrays, state, and refs are still needed
where appropriate.

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
AUTH_PUBLIC_ORIGIN=http://localhost:8081
AUTH_GATEWAY_ORIGIN=http://localhost:8081
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

Route guards protect navigation, not private server operations. There is no
`requireAuth` middleware in this project. New server functions must define and
enforce their own server-side authorization; never treat a client-supplied
subject or trace ID as proof of ownership. The existing gateway uses an
in-memory OAuth state store; multi-instance deployments need a shared,
expiring, single-use state store.

## Dashboard request-flow contract

`src/lib/request-flow.ts` defines bounded request, policy, and trace evidence
types. Span timings include their measurement boundary; total client-observed
response time through the BFF result is reported separately and excludes trace
lookup. Selected replicas and successful delivery are separate evidence.
Missing evidence is rendered as unknown rather than success. The dashboard
starts `not-connected`; fixtures in `src/lib/request-flow.test.ts` are test-only
and do not simulate traffic.

## Phase 5 request and trace integration

`src/lib/showcase-request.ts` contains the user-triggered flow for the
Send Request control. It never runs during page load. It invokes a
TanStack Start GET server function, which makes exactly one fixed
`GET /api/v1/go-orders` request to `TRAPIGO_GATEWAY_ORIGIN`. The server consumes and discards a bounded response body and returns the real
HTTP status, trace ID, and retry information. The browser measures total
response time through this result; trace polling is excluded. It never returns
orders data.

The dashboard also has a **Send request - direct to Trapigo** button. It
fetches `GET /api/v1/go-orders` from the browser, sends the browser's auth
cookies and a W3C `traceparent`, and measures through response-body consumption.
Trace registration and lookup happen afterward and are excluded from that
measurement. `VITE_TRAPIGO_GATEWAY_ORIGIN` selects the browser-reachable gateway
origin; it defaults to `http://localhost:8081` in development and the dashboard
origin in production. Cross-origin requests require credentialed CORS support
at the gateway, including unauthenticated OPTIONS preflight for the
`traceparent` header and exposure of `Retry-After`. An origin allowlist alone
does not enable browser CORS. A same-origin reverse-proxy route avoids this
requirement. Browser-blocked responses are shown as connection failures, not
as successful HTTP responses.

The BFF creates a trace ID, remembers the latest demo request, and forwards
only the two gateway auth cookies plus
server-generated W3C `traceparent`.
This header provides correlation, not a recorded BFF span. Its remote parent
ID is intentionally not exported; the gateway is the root of the displayed
trace scope. The gateway still authenticates the actual orders request.
Sending a request does not call `/web/auth/me` first. A
separate trace lookup revalidates login through Trapigo, checks the active trace ID,
then queries Jaeger's versioned v3 single-trace endpoint. Lookup is bounded.
For this single-tab demo, only the latest request is retained in one in-memory
record; another request replaces it. There is no cookie fingerprinting or
per-session trace isolation: any authenticated user who knows the active ID can
read that trace. This is for a single-user local demo, not a shared/public
dashboard. Add per-session authorization before supporting multiple users.

Only Trapigo gateway and orders-service telemetry is visualized. Neither the
browser nor the BFF records or exports spans. The existing Go telemetry pipeline
continues sending spans to the Collector and Jaeger. The BFF only reads Jaeger
and normalizes known gateway/orders spans and policy attributes into the UI
contract. No browser telemetry ingress, Node OTel preload, or dashboard access
to the Collector is needed. Gateway request duration is not browser round-trip
time.

Server functions use TanStack Start's normal setup, without a custom `src/start.ts`
or global request middleware. Gateway authentication, rate limiting, CSRF/origin
policies, routing, and replica selection remain Trapigo's responsibilities.
Jaeger lookup requires a gateway-verified login and is restricted to the latest
demo trace, without maintaining a separate BFF session registry.

For a host-run dashboard with the repository's Compose stack, configure:

```dotenv
TRAPIGO_GATEWAY_ORIGIN=http://localhost:8081
JAEGER_QUERY_ORIGIN=http://localhost:16686
```

Keep the Collector unavailable to browsers and public networks. No new port
mapping or Collector recreation is needed for the dashboard. If the dashboard
runs in a container, use fixed private service addresses instead of `localhost`
and restrict Jaeger access to the application network. Production
gateway and Jaeger origins must use HTTPS. The single active trace record is
process-local; multiple tabs/sessions or BFF replicas need a different storage
and authorization design.

The normalizer accepts the Go HTTP instrumentation's route-based server span
names (`GET /` at the gateway and `GET /api/v1/go-orders` at orders), as well as
the explicit operation names used by older instrumentation. Policy spans use
the gateway's actual `trapigo.policy.outcome` values.

## Phase 6 dashboard controls

- Send Request issues one orders GET, displays its real status, then reads the
  matching trace. Lookup waits 1.5 seconds between attempts, stops on complete
  evidence or a lookup error, and makes at most ten trace-query calls.
- Start Traffic repeats that same serialized cycle with at least five seconds
  after each cycle. No business request overlaps an unfinished trace lookup:
  the BFF only retains the latest trace. Stop prevents the next orders request;
  the current cycle can finish. Hidden tabs pause future sends and lookups.
- HTTP 429 honors bounded `Retry-After` before the next send. Gateway
  authentication rejection, expired login, connection/body failure, or lookup
  error stops traffic. An orders-service 401 is not called session expiry when
  gateway authentication evidence shows it passed. An unattributed 401 pauses
  traffic with an explicit message.
- Trace reads independently verify login through `/web/auth/me` before Jaeger;
  these are real gateway calls and can consume the shared IP rate-limit bucket.
  There is still no session preflight before the showcased orders request.
- Highlights replay observed gateway evidence with a finite animation and honor
  reduced motion. Selection alone never means successful service delivery.
  Unmeasured policies, logging, durations, and health stay explicitly unknown.
- Recent history retains five results locally; counts increment once per
  request with an observed selection for the three configured public replica
  labels. Neither history nor counts query old BFF records or represent global
  metrics. Both reset when this page unmounts or reloads.
- The span inspector shows gateway/policy/proxy/service durations and parent IDs.
  No browser/BFF spans, fingerprint registry, telemetry ingress, or custom
  TanStack Start entry point are needed.

## Validation (Node 24+)

```bash
pnpm exec node --test src/server/auth-gateway.test.ts src/lib/request-flow.test.ts
pnpm run typecheck
pnpm exec biome check .
pnpm run build
```

Production builds use Nitro's default Node.js preset and can be started with
`pnpm run start`. Development uses the normal Vite command; neither startup
command preloads telemetry. Set `NITRO_PRESET` to a supported Nitro preset to target a different deployment
platform.

## Adding components

To add components to your app, run the following command:

```bash
npx shadcn@latest add button
```

This will place the ui components in the `components` directory.

## Using components

To use the components in your app, import them as follows:

```tsx
import { Button } from "@/components/ui/button"
```
