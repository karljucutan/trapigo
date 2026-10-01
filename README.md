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

Required environment variables are listed in [trapigo/.env.example](trapigo/.env.example).