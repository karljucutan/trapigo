# TrapiGo

Trapiko: Built in Go. Inspired by Traefik.

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