# TrapiGo

Trapiko: Built in Go. Inspired by Traefik. Simple API Gateway.

TrapiGo is a small API gateway project with a web dashboard, a sample orders
service, Keycloak-based identity, and an OpenTelemetry observability stack.

## Components

| Component | Technology | Role |
| --- | --- | --- |
| Frontend dashboard | [TanStack Start](frontend-dashboard/) | Web UI for the TrapiGo dashboard and authenticated user experience. |
| API gateway | TrapiGo (Go) | Routes incoming requests to configured services, reverse-proxies traffic, and applies gateway middleware. |
| Orders service | [go-order-service](go-order-service/) (Go) | Example CRUD API for orders and order items, backed by PostgreSQL. |
| Identity and access management | Keycloak | Handles login and logout through OpenID Connect and provides tokens the gateway validates. |
| Telemetry collection | OpenTelemetry Collector | Receives OTLP telemetry from the gateway and services, exports traces to Jaeger, and exposes metrics for Prometheus. |
| Distributed tracing | Jaeger | Stores and visualizes distributed traces. |
| Metrics | Prometheus | Scrapes and stores metrics exposed by the OpenTelemetry Collector. |
| Metrics dashboards | Grafana | Visualizes Prometheus metrics; provisioned to connect to the local observability stack. |

## API gateway

The gateway routes requests by configured path prefix and forwards them to
their upstream services. The example configuration exposes:

- `/api/v1/go-orders` → `go-order-service`
- `/auth` → Keycloak

For services with multiple configured backends, TrapiGo selects a backend
using round-robin load balancing and proxies the request and response. The
sample Compose stack runs three `go-order-service` replicas.

Gateway middleware and supporting features include:

- **Rate limiting:** in-memory token-bucket limits keyed by client IP; rejected
  requests receive HTTP `429` and a `Retry-After` header.
- **Authentication and authorization:** Keycloak OIDC login, callback, and
  logout flows; JWT validation, required-scope checks, and token refresh.
- **Origin and CSRF protection:** validates configured browser origins and
  protects requests against cross-origin request forgery.
- **Request logging:** records request method, path, status, and duration.
- **OpenTelemetry:** instruments gateway and upstream requests for distributed
  tracing and exports telemetry through the Collector.

The rate limiter is local to a gateway process; it is not a distributed limit
shared across replicas.

## Local stack

The root [Compose file](compose.yml) defines the gateway, orders service and
its PostgreSQL database, Keycloak and its database, and the observability
services. Run the TanStack Start dashboard separately; its development server
defaults to `http://localhost:3000`. The gateway is published at
`http://localhost:8081`. Jaeger, Prometheus, and Grafana are available at
`http://localhost:16686`, `http://localhost:9090`, and
`http://localhost:3001`, respectively.

## Keycloak Client Setup

Configure a confidential Keycloak client for the gateway BFF flow.

1. Create a client in your realm.
2. Set `Client Protocol` to `openid-connect`.
3. Set `Access Type` to `confidential` and enable service account credentials if needed.
4. Set `Valid Redirect URIs` to the exact callback URL used by the gateway, for example `http://localhost/web/auth/callback`.
5. Set `Web Origins` to your frontend origin (for example `http://localhost:3000`) or a strict allowlist.
6. Copy the generated client secret into `KEYCLOAK_CLIENT_SECRET`.
7. Set `Valid Post Logout Redirect URIs` to the exact `auth.frontend_redirect_url` configured in the gateway, for example `http://localhost:3000`. Keycloak requires an exact match, including any trailing slash.

Browser logout clears Trapigo's access, refresh, and HttpOnly ID-token cookies, then redirects the browser directly to Keycloak's RP-Initiated Logout endpoint. The validated ID token is sent as `id_token_hint`, allowing Keycloak to identify the session and normally skip the confirmation prompt. Older sessions without the ID-token cookie fall back to Keycloak's confirmation flow. Cookie deletion alone does not end Keycloak SSO. Require HTTPS and `cookie_secure: true` in production.

Required environment variables are listed in [trapigo/.env.example](trapigo/.env.example).

## Browser requests to Trapigo

The existing origin-validation middleware uses `allowed_origins` for
credentialed CORS on its protected routes. An allowed frontend can preflight a
GET with the `traceparent` header without authentication; the actual GET still
requires authentication. Responses expose `Retry-After` to browser JavaScript.
Other preflight methods and non-safelisted request headers are not enabled.
Logging, rate limiting, and CSRF middleware retain their current order.
Responses rejected before origin validation (including rate-limit rejections)
do not receive CORS headers and may appear as browser network errors.