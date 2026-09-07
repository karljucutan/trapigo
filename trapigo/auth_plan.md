# API Gateway `/web` Browser BFF Authentication Plan

## 1. Goal

Extend the existing simple Go API Gateway to support two client authentication models:

### Browser clients

- Use the `/web/auth/*` endpoints.
- The Go API Gateway acts as the Browser BFF.
- Use OpenID Connect Authorization Code Flow with PKCE against Keycloak.
- Store the access token and refresh token in separate `HttpOnly` cookies.
- Browser JavaScript never reads, stores, or refreshes the tokens.
- The gateway transparently refreshes the access token when an API request contains an expired access token.
- The gateway sends the updated token to the browser through `Set-Cookie`.

### Native applications

- Authenticate directly with Keycloak using Authorization Code + PKCE.
- Manage their own access and refresh tokens.
- Call the API Gateway using `Authorization: Bearer <access_token>`.
- The API Gateway validates the JWT and forwards the request to the downstream API.
- The API Gateway does not refresh the native application's tokens.

---

# 2. Overall Architecture

```text
                             ┌──────────────┐
                             │   Keycloak   │
                             └──────▲───────┘
                                    │
                         OIDC/token │ operations
                                    │
                 ┌──────────────────┴──────────────────┐
                 │                                     │
            Browser Client                       Native Application
                 │                                     │
                 │ /web/auth/*                         │
                 ▼                                     │
        ┌───────────────────────┐                      │
        │   Go API Gateway/BFF  │◄─────────────────────┘
        └───────────┬───────────┘   Authorization:
                    │               Bearer <JWT>
                    │
                    │ Authorization: Bearer <JWT>
                    ▼
             ┌───────────────┐
             │ Downstream API│
             └───────────────┘
```

The important distinction is:

```text
Browser:
    Browser → Go Gateway/BFF → Keycloak
             Cookie          token operations

Native:
    Native App → Keycloak
    Native App → Go Gateway → API
                 Bearer JWT
```

---

# 3. Route Structure

Add a browser-specific authentication area under `/web`:

```text
/web/auth/login
/web/auth/callback
/web/auth/logout
/web/auth/me
```

There is intentionally no required:

```text
/web/auth/refresh
```

The browser BFF performs refresh transparently while processing `/api/*` requests.

The existing API routes remain under:

```text
/api/*
```

---

# 4. Keycloak Client Configuration

Create/configure a Keycloak OIDC client specifically for the browser BFF.

Recommended characteristics:

```text
Client type:
    Confidential

Standard Flow:
    Enabled

Implicit Flow:
    Disabled

PKCE:
    Use S256
```

The Go gateway securely stores the client secret when using a confidential client.

Configure an exact redirect URI such as:

```text
https://example.com/web/auth/callback
```

Do not use broad redirect URI patterns unless there is a specific requirement.

The browser application and BFF should normally use HTTPS in production.

---

# 5. `/web/auth/login`

## Endpoint

```http
GET /web/auth/login
```

## Purpose

Start the OIDC Authorization Code + PKCE login flow.

## Gateway responsibilities

Generate:

```text
state
nonce
code_verifier
code_challenge
```

Where:

```text
code_challenge = BASE64URL(SHA256(code_verifier))
```

The gateway must retain the temporary values required to validate the callback.

Then redirect the browser to Keycloak's authorization endpoint.

## Authorization request

Conceptually:

```http
GET https://keycloak.example.com/realms/{realm}/protocol/openid-connect/auth
    ?client_id={client_id}
    &response_type=code
    &scope=openid
    &redirect_uri={redirect_uri}
    &state={state}
    &nonce={nonce}
    &code_challenge={code_challenge}
    &code_challenge_method=S256
```

The browser follows the redirect to Keycloak.

---

# 6. Keycloak Authentication

Keycloak handles the user authentication process.

After successful authentication, Keycloak redirects the browser back to the configured callback URI:

```http
GET /web/auth/callback?code=...&state=...
```

The browser receives the authorization code, not the final access/refresh tokens.

---

# 7. `/web/auth/callback`

## Endpoint

```http
GET /web/auth/callback
```

## Gateway responsibilities

1. Read `code`.
2. Read `state`.
3. Validate `state` against the value generated during `/login`.
4. Retrieve the associated `code_verifier`.
5. Exchange the authorization code with Keycloak.
6. Validate the resulting OIDC response as required by the implementation.
7. Set the access and refresh token cookies.
8. Redirect the browser to the frontend application.

## Token exchange

The gateway calls Keycloak server-to-server:

```http
POST /realms/{realm}/protocol/openid-connect/token
Content-Type: application/x-www-form-urlencoded
```

Example form values:

```text
grant_type=authorization_code
client_id={client_id}
client_secret={client_secret}
code={code}
redirect_uri={redirect_uri}
code_verifier={code_verifier}
```

Keycloak returns token information including an access token and refresh token.

The gateway must not return those tokens directly to the browser as JSON.

---

# 8. Token Cookies

Store the tokens in two separate cookies.

Recommended initial design:

```http
Set-Cookie: web_access_token=<access_token>; HttpOnly; Secure; SameSite=Lax; Path=/
Set-Cookie: web_refresh_token=<refresh_token>; HttpOnly; Secure; SameSite=Lax; Path=/
```

Recommended cookie properties:

```text
HttpOnly:
    Yes

Secure:
    Yes in production

SameSite:
    Lax by default; adjust only when the deployment architecture requires it

Domain:
    Prefer host-only cookies unless cross-subdomain sharing is intentionally required

Path:
    / for the transparent-refresh design described in this plan
```

The refresh token cookie should be available when `/api/*` requests are processed because the gateway may need to refresh the token during an API request.

Do not use `Path=/web/auth` for the refresh token if the gateway is expected to refresh it while handling `/api/*`.

Cookie names:

```text
web_access_token
web_refresh_token
```

The browser frontend does not access these cookies through JavaScript.

---

# 9. Browser API Request

After successful login, the browser calls the normal API endpoints.

Example:

```http
GET /api/orders
```

The browser automatically sends the cookies:

```http
Cookie: web_access_token=...
Cookie: web_refresh_token=...
```

The frontend does not need to manually add:

```http
Authorization: Bearer ...
```

for browser BFF requests.

---

# 10. `/api/*` Authentication Mechanism Detection

For `/api/*`, the gateway determines the authentication mechanism from the request.

## Browser/BFF authentication

If the request contains the browser access-token cookie:

```http
Cookie: web_access_token=...
```

then the gateway processes it using the browser/BFF authentication flow.

The flow is:

```text
Extract web_access_token
        ↓
Validate JWT
        ↓
Valid?
   ┌────┴────┐
  Yes        No / Expired
   │             │
   │          Use web_refresh_token
   │             │
   │          Refresh with Keycloak
   │             │
   │          Receive new tokens
   │             │
   │          Set-Cookie with new token(s)
   │             │
   └──────┬──────┘
          ↓
Forward request to downstream API
using:
Authorization: Bearer <valid access token>
```

## Native/direct-token authentication

If the request contains:

```http
Authorization: Bearer <access_token>
```

then the gateway treats it as a native/direct-token request.

The flow is:

```text
Extract Bearer token
        ↓
Validate JWT
        ↓
Valid?
   ┌────┴────┐
  Yes        No
   │          │
   │        401 Unauthorized
   │
   ▼
Forward request to downstream API
```

The native application's refresh token is never handled by the gateway.

The native application is responsible for refreshing its access token directly with Keycloak.

---

# 11. Authentication Precedence

Define one deterministic rule when a request contains both authentication mechanisms.

Recommended precedence:

```text
1. Authorization: Bearer <JWT>
2. web_access_token cookie
3. No credentials → 401 Unauthorized
```

Example ambiguous request:

```http
Authorization: Bearer abc...
Cookie: web_access_token=xyz...
```

The gateway should use the bearer token according to the defined precedence rule.

Do not silently switch between authentication mechanisms.

Optionally, a future implementation may reject requests containing both credentials to make accidental credential mixing more visible.

---

# 12. JWT Validation

The gateway should validate JWT access tokens locally rather than making a Keycloak introspection request for every API call.

Validate at minimum:

```text
Signature
Issuer (iss)
Audience (aud)
Expiration (exp)
Not-before (nbf), when applicable
Required scopes/roles
```

Use Keycloak's published signing keys/JWKS.

The gateway should cache the signing keys and refresh them according to the JWKS/key-rotation behavior rather than downloading them for every request.

Conceptually:

```text
Keycloak
    │
    │ JWKS / signing keys
    ▼
Go API Gateway
    │
    ├── Validate signature
    ├── Validate issuer
    ├── Validate audience
    ├── Validate expiration
    └── Validate permissions
```

---

# 13. Transparent Browser Token Refresh

There is no need for the browser to call `/web/auth/refresh` in this design.

When the gateway receives:

```http
GET /api/orders
Cookie: web_access_token=<expired>
Cookie: web_refresh_token=<refresh>
```

the gateway should:

1. Detect that the access token is expired.
2. Send the refresh token to Keycloak's token endpoint.
3. Receive the new access token.
4. Receive a refresh token as well when Keycloak returns one.
5. Set the new access-token cookie.
6. Replace the refresh-token cookie whenever Keycloak returns a new refresh token.
7. Retry/continue the original downstream API request using the new access token.
8. Return the downstream API response to the browser.

Conceptually:

```text
Browser
  │
  │ /api/orders
  │ Cookie: access=A
  │         refresh=R
  ▼
Gateway/BFF
  │
  │ A expired
  │
  │ POST Keycloak /token
  │ refresh_token=R
  ▼
Keycloak
  │
  ├── access_token=B
  └── refresh_token=R2 (when rotation/configuration returns one)
  │
  ▼
Gateway/BFF
  │
  ├── Set-Cookie: web_access_token=B
  ├── Set-Cookie: web_refresh_token=R2 (when returned)
  │
  └── Authorization: Bearer B
              ↓
        Downstream API
```

Important:

The refresh token cookie is **not necessarily unchanged** after a refresh.

Depending on Keycloak configuration:

```text
Rotation disabled:
    old refresh token may remain valid

Rotation enabled:
    Keycloak can issue a new refresh token
    and invalidate the previous refresh token
```

Therefore the gateway implementation should always inspect the token response and overwrite the refresh-token cookie whenever a new refresh token is returned.

---

# 14. Native Application Authentication

Native applications should bypass `/web/auth/*`.

Their flow is:

```text
Native App
    │
    │ Authorization Code + PKCE
    ▼
Keycloak
    │
    ├── access_token
    └── refresh_token
    │
    ▼
Native App securely stores tokens
    │
    │ Authorization: Bearer <access_token>
    ▼
Go API Gateway
    │
    │ Validate JWT
    ▼
Downstream API
```

The native application should not use a client secret embedded in the application.

The native client manages its own access-token refresh with Keycloak.

---

# 15. `/web/auth/me`

## Endpoint

```http
GET /web/auth/me
```

Purpose:

Allow the frontend to determine whether the current browser authentication is valid and retrieve basic authenticated-user information without exposing tokens.

Example response:

```json
{
  "authenticated": true,
  "user": {
    "id": "...",
    "username": "...",
    "email": "..."
  }
}
```

The gateway can normally derive the required information from validated JWT claims instead of calling Keycloak UserInfo for every request.

For an unauthenticated browser:

```http
401 Unauthorized
```

or a suitable application-specific unauthenticated response.

---

# 16. `/web/auth/logout`

## Endpoint

```http
POST /web/auth/logout
```

or, if the application's navigation requires it:

```http
GET /web/auth/logout
```

Prefer `POST` for an application state-changing logout operation.

Gateway responsibilities:

1. Clear `web_access_token`.
2. Clear `web_refresh_token`.
3. Optionally revoke the refresh token through Keycloak.
4. Optionally perform Keycloak OIDC logout when full SSO logout is required.
5. Redirect the browser to the application login/home page when appropriate.

Cookie deletion must use compatible cookie attributes, especially the same cookie name/path/domain combination.

---

# 17. CSRF Protection

Because browser authentication uses cookies, the API automatically receives the authentication credentials whenever the browser sends a request.

Therefore the BFF must consider CSRF for state-changing endpoints.

At minimum:

- Use an appropriate `SameSite` cookie policy.
- Validate the `Origin` header for sensitive state-changing requests where practical.
- Consider an explicit CSRF token mechanism for state-changing browser operations when required by the application's deployment model.

`HttpOnly` protects the token from JavaScript cookie access, but `HttpOnly` does not by itself prevent CSRF.

---

# 18. Downstream API Authentication

The downstream API should not need to know whether the original client was a browser or native app.

The gateway normalizes both authentication models into:

```http
Authorization: Bearer <access_token>
```

Therefore:

```text
Browser
   │
   │ Cookie
   ▼
Gateway
   │
   │ Authorization: Bearer JWT
   ▼
API
```

and:

```text
Native App
   │
   │ Authorization: Bearer JWT
   ▼
Gateway
   │
   │ Authorization: Bearer JWT
   ▼
API
```

The downstream APIs can therefore use the same JWT validation rules.

---

# 19. Recommended Go Gateway Components

Keep the implementation separated by responsibility.

Suggested structure:

```text
/internal
    /auth
        oidc.go
        keycloak.go
        jwt.go
        cookies.go
        oauth_state.go

    /web
        auth_handler.go

    /middleware
        authentication.go

    /proxy
        gateway.go
```

Possible responsibilities:

```text
OIDC client
    Build Keycloak authorization URL
    Exchange authorization code
    Refresh tokens

JWT validator
    Validate access-token JWTs
    Cache Keycloak signing keys

Cookie manager
    Create access-token cookie
    Create refresh-token cookie
    Clear cookies

Web auth handler
    /web/auth/login
    /web/auth/callback
    /web/auth/logout
    /web/auth/me

Authentication middleware
    Detect Bearer vs cookie authentication
    Validate credentials
    Perform browser refresh when necessary
    Populate request authentication context

Gateway/proxy
    Route authenticated request to downstream services
```

Avoid putting all authentication behavior directly inside the generic reverse-proxy code.

---

# 20. Suggested Request Processing Pipeline

For an incoming `/api/*` request:

```text
Request
  ↓
Route matching
  ↓
Authentication middleware
  ↓
Does Authorization header exist?
  │
  ├── Yes → Validate Bearer JWT
  │          │
  │          ├── Valid → Continue
  │          └── Invalid/expired → 401
  │
  └── No → Does web_access_token cookie exist?
             │
             ├── No → 401
             │
             └── Yes → Validate JWT
                        │
                        ├── Valid → Continue
                        │
                        └── Expired → Refresh with Keycloak
                                      │
                                      ├── Refresh failed → 401
                                      │
                                      └── Refresh succeeded
                                             │
                                             ├── Set new access cookie
                                             ├── Set new refresh cookie when returned
                                             └── Continue
  ↓
Add Authorization: Bearer <valid access token>
  ↓
Forward to downstream API
  ↓
Return response
```

---

# 21. Cookie Naming and Scope

Use browser-specific names so these cookies are clearly separated from any future cookie-based authentication mechanisms.

Recommended:

```text
web_access_token
web_refresh_token
```

Do not expose the tokens through a response JSON object such as:

```json
{
  "accessToken": "...",
  "refreshToken": "..."
}
```

The browser should only receive them through `Set-Cookie`.

---

# 22. Error Handling

## Login failure

```http
GET /web/auth/callback?error=...
```

Return/redirect to an appropriate authentication error page.

## Invalid callback state

Reject the request.

Do not exchange the authorization code when `state` validation fails.

## Expired browser access token + invalid refresh token

Clear the browser authentication cookies and return:

```http
401 Unauthorized
```

The frontend can then redirect the user to:

```text
/web/auth/login
```

## Invalid native bearer token

Return:

```http
401 Unauthorized
```

Do not attempt browser-cookie refresh.

---

# 23. Important Statelessness Definition

The gateway is stateless with respect to the browser's authenticated session because it does not need a server-side session record for every request.

The browser carries the authentication material in the HttpOnly cookies:

```text
web_access_token
web_refresh_token
```

However, this does not mean the overall authentication system is completely stateless.

Keycloak can still maintain state related to refresh tokens, revocation, rotation, sessions, and other identity-management behavior.

Therefore the accurate description is:

```text
Stateless BFF authentication at the API Gateway
using HttpOnly token cookies.
```

---

# 24. Security Requirements

Before considering the implementation complete, verify all of the following:

```text
[ ] Authorization Code Flow is used
[ ] PKCE S256 is used
[ ] state is generated and validated
[ ] nonce is generated and validated when using the ID token
[ ] Redirect URI is exact and configured in Keycloak
[ ] Client secret is never exposed to the browser/native app
[ ] Access token cookie is HttpOnly
[ ] Refresh token cookie is HttpOnly
[ ] Cookies use Secure in production
[ ] SameSite policy is intentional
[ ] JWT signature is validated
[ ] JWT issuer is validated
[ ] JWT audience is validated
[ ] JWT expiration is validated
[ ] Required roles/scopes are validated
[ ] Keycloak signing keys are cached
[ ] Native Bearer requests do not use browser refresh logic
[ ] Browser cookie requests do use transparent refresh logic
[ ] New refresh token replaces the old cookie when Keycloak returns one
[ ] Browser state-changing requests have CSRF protection
[ ] Tokens are never logged
[ ] Tokens are never returned in API response bodies
```

---

# 25. Implementation Order

Implement in this order:

## Step 1 — Keycloak integration

Implement:

```text
OIDC discovery
Authorization URL generation
PKCE
Authorization-code exchange
Token refresh
JWT/JWKS validation
```

## Step 2 — Browser authentication endpoints

Implement:

```text
GET /web/auth/login
GET /web/auth/callback
POST /web/auth/logout
GET /web/auth/me
```

Add cookie handling.

## Step 3 — API authentication middleware

Implement the `/api/*` authentication decision:

```text
Authorization: Bearer ...
        ↓
Native/direct-token authentication
```

Otherwise:

```text
web_access_token cookie
        ↓
Browser/BFF authentication
```

For expired browser access tokens:

```text
web_refresh_token
        ↓
Keycloak refresh
        ↓
Set-Cookie new access token
        ↓
Replace refresh-token cookie when Keycloak returns a new one
```

## Step 4 — Downstream request forwarding

Normalize authenticated requests to:

```http
Authorization: Bearer <valid_access_token>
```

before forwarding to downstream APIs.

## Step 5 — Security hardening

Verify:

```text
CSRF protection
Cookie configuration
JWT validation
Key rotation behavior
Logout behavior
Error handling
Token logging prevention
```

---

# 26. Final Target Behavior

The finished gateway should behave like this.

### Browser

```text
Browser
   │
   │ GET /web/auth/login
   ▼
Gateway
   │
   │ redirect
   ▼
Keycloak
   │
   │ callback with code
   ▼
Gateway
   │
   │ token exchange
   ▼
Keycloak
   │
   │ access + refresh tokens
   ▼
Gateway
   │
   │ Set-Cookie
   ├── web_access_token
   └── web_refresh_token
   │
   ▼
Browser
```

Then:

```text
Browser
   │
   │ /api/orders
   │ Cookies
   ▼
Gateway
   │
   ├── Access token valid
   │       ↓
   │   Forward request
   │
   └── Access token expired
           ↓
       Refresh with Keycloak
           ↓
       Set new cookie(s)
           ↓
       Forward request
   │
   ▼
Downstream API
```

### Native app

```text
Native App
   │
   │ Authorization Code + PKCE
   ▼
Keycloak
   │
   │ access + refresh tokens
   ▼
Native App
   │
   │ Authorization: Bearer <access_token>
   ▼
Go API Gateway
   │
   │ Validate JWT
   ▼
Downstream API
```

The native app refreshes its own token directly with Keycloak.

The browser BFF refreshes its own cookie-based tokens through Keycloak.

---

# 27. Key Design Decision

The central design decision is:

```text
                    /api/*
                       │
          ┌────────────┴────────────┐
          │                         │
 Authorization: Bearer          web_access_token cookie
          │                         │
          ▼                         ▼
 Native/direct token           Browser/BFF token
 authentication               authentication
          │                         │
          │                         ├── Refresh when expired
          │                         └── Update HttpOnly cookies
          │
          └────────────┬────────────┘
                       │
                       ▼
              Valid access token
                       │
                       ▼
               Downstream APIs
```

This allows the same Go API Gateway to support both browser and native clients without requiring native applications to use the `/web` BFF authentication flow.
