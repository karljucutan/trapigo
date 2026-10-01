# API Gateway Browser BFF Authentication - Implementation Plan

This document provides a step-by-step implementation guide for the authentication plan described in `auth_plan.md`.

---

## Architecture Overview

The `auth` feature is organized into **shared** and **client-specific** layers:

```
internal/features/auth/
│
├── SHARED LAYER (used by all authentication clients)
│   ├── domain/               # Domain models (errors, claims, oauth_state)
│   ├── infrastructure/       # Keycloak client, JWT validator, cookies, state store
│   ├── middleware/           # Auth detection & validation (Bearer vs Cookie)
│   └── gateway/              # Request forwarding with auth headers
│
└── CLIENT-SPECIFIC LAYER
    └── web/                  # Browser/BFF implementation
        ├── application/      # Login/callback/logout/me commands & queries
        └── transporthttp/    # HTTP handlers for /web/auth/* endpoints
```

**Key principle**: JWT validation, OAuth state management, and auth detection are **shared** across all authentication mechanisms. The `web/` subdirectory contains only browser-specific endpoints (`/web/auth/login`, `/web/auth/callback`, etc.).

Both browser and native clients use:
- The same `jwt_validator` (Step 1)
- The same `authentication` middleware (Step 4)
- The same `request_gateway` (Step 5)

The difference is:
- **Browser**: Sends cookies, uses `/web/auth/*` endpoints to initiate login
- **Native**: Sends Bearer tokens, authenticates directly with Keycloak (no gateway involvement)

---

## Directory Structure

Create the following structure under `internal/features/auth/`:

```
internal/features/auth/
├── domain/                          # Shared domain models (used by all auth clients)
│   ├── oauth_state.go               # OAuth state/nonce/PKCE state management
│   ├── claims.go                    # JWT claims domain model
│   └── errors.go                    # Auth-specific errors
├── infrastructure/                  # Shared infrastructure (used by all auth clients)
│   ├── keycloak.go                  # Keycloak client & API calls
│   ├── jwt_validator.go             # JWT validation & key caching (SHARED)
│   ├── cookie_manager.go            # Cookie creation/deletion
│   └── oauth_state_store.go         # OAuth state persistence
├── middleware/                      # Shared middleware (applies to all /api/* routes)
│   ├── authentication.go            # Auth detection (Bearer vs Cookie) & validation
│   └── authentication_test.go       # Tests for auth detection and validation
├── gateway/                         # Shared gateway logic
│   ├── request_gateway.go           # Authenticated request forwarding to downstream API
│   └── request_gateway_test.go
├── web/                             # Browser/BFF specific implementation
│   ├── application/
│   │   ├── command/
│   │   │   ├── login.go             # Initiate login flow
│   │   │   ├── callback.go          # Handle callback & exchange code
│   │   │   └── logout.go            # Clear auth cookies
│   │   └── query/
│   │       └── get_current_user.go  # /web/auth/me endpoint
│   └── transporthttp/
│       ├── auth_handler.go          # HTTP handlers for /web/auth/* endpoints
│       └── auth_handler_test.go
└── native/                          # Native app specific (optional, for future)
    └── ... (TBD)
```

**Note**: The `auth` feature is now the primary feature. The `web/` subdirectory contains browser/BFF-specific logic, while `infrastructure/`, `middleware/`, and `gateway/` contain shared logic used by all authentication clients (browser, native, etc.).

---

## Required Dependencies

Before implementing, add the following Go modules to `go.mod`:

```bash
go get golang.org/x/oauth2
go get github.com/golang-jwt/jwt/v5
```

**OAuth2 Library (Official Go)**:
- **Package**: `golang.org/x/oauth2`
- **Purpose**: Standard OAuth2 client flow support
- **Usage**: Build authorization URL, exchange authorization code, refresh access tokens
- **Why**: Official Go package, reduces manual OAuth2 protocol code and request-shaping errors

**JWT Library**:
- **Package**: `github.com/golang-jwt/jwt/v5`
- **Purpose**: Parse, validate, and verify JWT signatures
- **Usage**: JWT signature verification with JWKS keys, standard claim validation
- **Why**: Production-grade, actively maintained, used by major Go projects
- **Alternative**: Could implement from scratch, but not recommended (crypto complexity, security risk)

---

## Implementation Steps

### Step 1: Keycloak Integration & JWT Validation

**Goal**: Establish secure communication with Keycloak and validate JWT tokens locally.

**Files to create**:
- `internal/features/auth/infrastructure/keycloak.go`
- `internal/features/auth/infrastructure/jwt_validator.go`
- `internal/features/auth/domain/errors.go`

**Tasks**:

1. **Create Keycloak client** (`keycloak.go`)
   - [x] Implement OIDC discovery endpoint client
   - [x] Configure `oauth2.Config` using discovered endpoints and gateway config:
     - `ClientID`, `ClientSecret`, `RedirectURL`
     - endpoint auth URL and token URL from discovery
   - [x] Build authorization URL using `oauth2.Config.AuthCodeURL(...)` with:
     - `client_id`, `response_type=code`, `scope=openid`
     - `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`
   - [x] Implement PKCE flow:
     - Generate `code_verifier` (43-128 chars, unreserved characters)
     - Generate `code_challenge = BASE64URL(SHA256(code_verifier))`
   - [x] Implement token exchange using `oauth2.Config.Exchange(...)`:
     - Exchange authorization code with PKCE verifier
     - Return access token, refresh token, token type, expiry fields
   - [x] Implement token refresh using `oauth2.TokenSource(...)`:
     - Refresh from `refresh_token`
     - Return refreshed token fields
   - [x] Error handling for Keycloak failures

2. **Create JWT validator** (`jwt_validator.go`) — **SHARED component**
   - [x] **Dependency**: Add `github.com/golang-jwt/jwt/v5` to `go.mod`
     - Provides: JWT parsing, signature verification, standard claim validation
     - Library handles: RSA/ECDSA verification, format validation, base64URL decoding
     - We implement: JWKS caching, claim validation logic, error mapping
   - [x] Fetch and cache Keycloak JWKS from `/.well-known/jwks.json`
   - [x] Implement JWKS key rotation/refresh logic
   - [x] Validate JWT signature using cached keys via `jwt.ParseWithClaims()`
   - [x] Validate JWT claims:
     - `iss` (issuer matches Keycloak realm)
     - `aud` (audience matches client_id or expected value)
     - `exp` (expiration time not passed)
     - `nbf` (not-before time if present)
   - [x] Extract user information from JWT claims (sub, preferred_username, email)
   - [x] Return validation result with extracted claims
   - **Note**: This validator is reused by both browser (web) and native authentication flows in the middleware

3. **Create domain models & errors** (`domain/`)
   - [x] `domain/errors.go`:
     - `KeycloakConnectionError`
     - `InvalidJWTError`
     - `ExpiredTokenError`
     - `InvalidStateError`
     - `InvalidCallbackError`
   - [x] `domain/claims.go` — JWT claims domain model (shared across auth flows)

**Testing**:
- [x] Unit tests for PKCE generation
- [x] Mock Keycloak responses for token exchange
- [x] Mock JWKS and test JWT validation with various claim combinations

**Validation**:
- [x] Can connect to Keycloak OIDC discovery endpoint
- [x] Can parse JWKS and validate JWTs offline
- [x] PKCE flow generates valid code_challenge

---

### Step 2: Cookie Management & OAuth State Storage

**Goal**: Securely manage authentication tokens in cookies and maintain temporary OAuth state.

**Files to create**:
- `internal/features/auth/infrastructure/cookie_manager.go`
- `internal/features/auth/infrastructure/oauth_state_store.go`
- `internal/features/auth/domain/oauth_state.go`

**Tasks**:

1. **Create OAuth state domain model** (`domain/oauth_state.go`)
  - [x] `OAuthState` struct containing:
     - `state` (CSRF protection)
     - `nonce` (ID token validation)
     - `code_verifier` (PKCE)
     - `created_at` (expiration tracking)
  - [x] Validation methods (not expired, valid format)

2. **Create OAuth state store** (`oauth_state_store.go`)
  - [x] Interface: `OAuthStateStore`
     - `SaveState(ctx context.Context, state *OAuthState) error`
     - `GetState(ctx context.Context, stateValue string) (*OAuthState, error)`
     - `DeleteState(ctx context.Context, stateValue string) error`
  - [x] In-memory implementation with automatic cleanup of expired states
  - [x] Note: Consider Redis implementation for multi-instance deployments later

3. **Create cookie manager** (`cookie_manager.go`)
  - [x] Constants for cookie names:
     - `web_access_token`
     - `web_refresh_token`
  - [x] Function to create access token cookie:
     - HttpOnly, Secure (production), SameSite=Lax, Path=/
     - Configurable max-age based on token expiration
  - [x] Function to create refresh token cookie:
     - Same attributes as access token
     - Longer max-age
  - [x] Function to create "clear" cookies (max-age=0, same attributes)
  - [x] Helper to extract token from cookie by name

**Testing**:
- [x] Unit tests for cookie creation with correct attributes
- [x] Unit tests for cookie clearing
- [x] Unit tests for OAuth state expiration

**Validation**:
- [x] Cookies have correct HttpOnly, Secure, SameSite attributes
- [x] Cookies are properly cleared (max-age=0)
- [x] OAuth states expire after configured duration

---

### Step 3: Browser Authentication Endpoints

**Goal**: Implement the four `/web/auth/*` endpoints.

**Files to create** (all under `internal/features/auth/web/`):
- `application/command/login.go`
- `application/command/callback.go`
- `application/command/logout.go`
- `application/query/get_current_user.go`
- `transporthttp/auth_handler.go`
- `transporthttp/auth_handler_test.go`

**Tasks**:

1. **Create login command** (`application/command/login.go`)
  - [x] Generate `state`, `nonce`, `code_verifier`
  - [x] Calculate `code_challenge`
  - [x] Save OAuth state to store with expiration
  - [x] Return Keycloak authorization URL

2. **Create callback command** (`application/command/callback.go`)
  - [x] Extract `code` and `state` from query parameters
  - [x] Retrieve OAuth state from store
  - [x] Validate `state` matches stored value
  - [x] Exchange code with Keycloak using `code_verifier`
  - [x] Receive access and refresh tokens
  - [x] Validate tokens (signature, issuer, audience)
  - [x] Delete OAuth state from store (one-time use)
  - [x] Return tokens for cookie setting

3. **Create logout command** (`application/command/logout.go`)
  - [x] Optionally revoke refresh token with Keycloak
  - [x] Return cookie clear instructions

4. **Create get current user query** (`application/query/get_current_user.go`)
  - [x] Accept validated JWT or extract from context
  - [x] Extract user info from JWT claims
  - [x] Return user info without exposing tokens

5. **Create HTTP handlers** (`transporthttp/auth_handler.go`)
  - [x] `HandleLogin(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/login`
     - Call login command
     - Redirect to Keycloak authorization URL
  - [x] `HandleCallback(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/callback`
     - Call callback command
     - Set cookies in response
     - Redirect to frontend application
  - [x] `HandleLogout(w http.ResponseWriter, r *http.Request)`
     - POST `/web/auth/logout`
     - Call logout command
     - Clear cookies
     - Redirect or return 200
  - [x] `HandleMe(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/me`
     - Extract auth from context (set by middleware)
     - Call get current user query
     - Return JSON with user info or 401

**Testing**:
- [x] Test login endpoint redirects correctly
- [x] Test callback with valid authorization code
- [x] Test callback with invalid state (reject)
- [x] Test callback with expired OAuth state (reject)
- [x] Test logout clears cookies
- [x] Test `/web/auth/me` returns authenticated user
- [x] Test `/web/auth/me` returns 401 when unauthenticated

**Validation**:
- [x] `/web/auth/login` redirects to Keycloak
- [x] `/web/auth/callback?code=...&state=...` exchanges code and sets cookies
- [x] `/web/auth/logout` clears cookies
- [x] `/web/auth/me` returns user info when authenticated

---

### Step 4: Authentication Middleware

**Goal**: Detect and validate authentication, with automatic token refresh for browser clients.

**Files to create** (both under `internal/features/auth/middleware/`):
- `authentication.go`
- `authentication_test.go`

**Tasks**:

1. **Create authentication middleware** (`middleware/authentication.go`) — **SHARED component**
  - [x] Implement precedence for auth detection:
     1. `Authorization: Bearer <JWT>` (native/direct-token)
     2. `web_access_token` cookie (browser/BFF)
     3. No credentials → 401
   - **Note**: This middleware applies to ALL `/api/*` routes and intelligently routes to either Bearer or Cookie flow
  - [x] For Bearer token:
     - Extract token from header
     - Validate JWT using jwt_validator
     - If expired → 401 (do not refresh)
     - If valid → extract claims and add to request context
  - [x] For browser cookie:
     - Extract `web_access_token` cookie
     - Validate JWT using jwt_validator
     - If valid → extract claims and add to request context
     - If expired → attempt refresh:
       - Check if `web_refresh_token` cookie exists
       - Call Keycloak refresh endpoint
       - If refresh fails → clear cookies and return 401
       - If refresh succeeds:
         - Set `web_access_token` cookie with new token
         - Set `web_refresh_token` cookie if new token returned
         - Extract claims from new token and add to context
  - [x] Add `http.Handler` wrapper for easy middleware integration
  - [x] Context keys for storing authenticated user/claims

**Testing**:
- [x] Test Bearer token validation (valid/expired/invalid)
- [x] Test cookie extraction and validation
- [x] Test token refresh on expired access token
- [x] Test 401 for missing credentials
- [x] Test precedence when both Bearer and cookie present (Bearer wins)

**Validation**:
- [x] Middleware correctly detects auth mechanism
- [x] Expired bearer tokens return 401
- [x] Expired browser access tokens are refreshed transparently
- [x] Valid tokens allow request to proceed

---

### Step 5: Downstream Request Gateway

**Goal**: Forward authenticated requests to downstream APIs with normalized authorization.

**Files to create** (both under `internal/features/auth/gateway/`):
- `request_gateway.go`
- `request_gateway_test.go`

**Tasks**:

1. **Create request gateway** (`gateway/request_gateway.go`)
  - [x] Accept authenticated request with user context
  - [x] Extract access token from context
  - [x] Add `Authorization: Bearer <access_token>` to outbound request
  - [x] Proxy request to downstream API (orders service, etc.)
  - [x] Handle downstream errors:
     - 401 → may indicate token validation issue at downstream, return 401
     - 403 → authorization/permission issue, return 403
     - Other errors → proxy error response
  - [x] Return response to client unchanged

**Testing**:
- [x] Test request forwarding with valid token
- [x] Test authorization header added to upstream request
- [x] Test downstream 401 is returned to client
- [x] Test downstream 500 is returned to client

**Validation**:
- [x] Authenticated requests are forwarded with Bearer token
- [x] Downstream API responses are passed through correctly

---

### Step 6: Wiring & Integration

**Goal**: Connect all components and integrate into the API Gateway.

**Tasks**:

1. **Create main router/bootstrap** (`internal/app/gateway/bootstrap/app.go` or similar)
  - [x] Initialize shared Keycloak client (from `auth/infrastructure/keycloak.go`)
  - [x] Initialize shared JWT validator with Keycloak JWKS cache (from `auth/infrastructure/jwt_validator.go`)
  - [x] Initialize shared cookie manager (from `auth/infrastructure/cookie_manager.go`)
  - [x] Initialize shared OAuth state store (from `auth/infrastructure/oauth_state_store.go`)
  - [x] Initialize shared authentication middleware (from `auth/middleware/authentication.go`)
  - [x] Register `/web/auth/*` routes with web-specific auth handlers (from `auth/web/transporthttp/`)
  - [x] Wrap `/api/*` routes with shared authentication middleware
  - [x] Set up downstream request gateway for `/api/*` (from `auth/gateway/`)
   - **Note**: All `/api/*` routes use the same shared authentication middleware, which internally handles both Bearer and Cookie flows

2. **Configuration** (`internal/platform/config/config.go`)
  - [x] `keycloak.issuer_url`
  - [x] `keycloak.client_id`
  - [x] `keycloak.client_secret`
  - [x] `keycloak.redirect_uri` (e.g., `https://example.com/web/auth/callback`)
  - [x] `auth.cookie_secure` (true in production)
  - [x] `auth.cookie_same_site` (Lax or Strict)
  - [x] `auth.state_expiration` (e.g., 10 minutes)
  - [x] `auth.token_cache_ttl` (JWKS cache)

3. **Environment setup**
  - [x] Create `.env.example` with auth configuration
  - [x] Document required Keycloak client setup

**Testing**:
- [x] Integration test: Login flow end-to-end
- [x] Integration test: Browser makes `/api/*` request after login
- [x] Integration test: Token refresh on expired access token
- [x] Integration test: Logout clears cookies

---

### Step 7: Security Hardening & Testing

**Goal**: Verify all security requirements are met.

**Tasks**:

1. **Security checklist** (from auth_plan.md section 24)
  - [x] Authorization Code Flow is used
  - [x] PKCE S256 is used
  - [x] `state` is generated and validated
  - [x] `nonce` is generated (for ID token validation if using ID token)
  - [x] Redirect URI is exact and configured in Keycloak
  - [x] Client secret is never exposed to browser/native app
  - [x] Access token cookie is HttpOnly
  - [x] Refresh token cookie is HttpOnly
  - [x] Cookies use Secure in production
  - [x] SameSite policy is intentional (Lax default)
  - [x] JWT signature is validated
  - [x] JWT issuer is validated
  - [x] JWT audience is validated
  - [x] JWT expiration is validated
  - [x] Tokens are never logged
  - [x] Tokens are never returned in API response bodies

2. **CSRF Protection** (section 17)
  - [x] Verify SameSite cookie behavior
  - [x] Consider CSRF token for state-changing requests if needed
  - [x] Test Origin header validation for sensitive endpoints

3. **Error handling**
  - [x] Keycloak errors don't leak sensitive info
  - [x] Expired tokens result in 401, not 500
  - [x] Invalid state in callback is rejected (not exchanged)

4. **Integration testing**
  - [x] Test complete browser login/logout cycle
  - [x] Test native app direct token flow (no browser auth)
  - [x] Test token refresh scenarios
  - [x] Test concurrent requests with token refresh
  - [x] Test invalid tokens are rejected
  - [x] Test downstream API receives correct Authorization header

---

## Configuration Example

Add to `trapigo-gateway.yaml`:

```yaml
keycloak:
  issuer_url: "https://keycloak.example.com/realms/master"
  client_id: "trapigo-gateway"
  client_secret: "${KEYCLOAK_CLIENT_SECRET}"  # from env or vault
  redirect_uri: "https://api.example.com/web/auth/callback"

auth:
  cookie_secure: true
  cookie_same_site: "Lax"
  state_expiration_seconds: 600  # 10 minutes
  jwt_cache_ttl_seconds: 3600    # 1 hour

```

---

## Environment Variables

Create a `.env.example`:

```bash
KEYCLOAK_CLIENT_SECRET=your-client-secret-here
KEYCLOAK_ISSUER_URL=https://keycloak.example.com/realms/master
KEYCLOAK_CLIENT_ID=trapigo-gateway
KEYCLOAK_REDIRECT_URI=https://api.example.com/web/auth/callback
ORDERS_SERVICE_URL=http://go-order-service:8080
```

---

## Key Milestones

| Step | Milestone | Completed |
|------|-----------|-----------|
| 1 | Keycloak integration working | [x] |
| 2 | Cookie & OAuth state management | [x] |
| 3 | `/web/auth/*` endpoints functional | [x] |
| 4 | Authentication middleware routing requests | [x] |
| 5 | Downstream API requests forwarded with Bearer token | [x] |
| 6 | End-to-end browser login flow working | [x] |
| 7 | All security requirements verified | [x] |

---

## Testing Strategy

- **Unit tests**: Individual functions (PKCE, JWT validation, cookie creation)
- **Integration tests**: Component interaction (login → callback → cookie)
- **End-to-end tests**: Full user flows (browser login → API call → logout)
- **Security tests**: Invalid state, expired tokens, missing credentials

---

## Future: Native App Support

The current structure is designed to support native apps in the future without refactoring. To add native app support:

```
internal/features/auth/
├── ... (shared infrastructure unchanged)
└── native/                          # New: Native app specific (optional)
    ├── application/command/
    │   └── validate_token.go        # Validate Bearer tokens
    └── transporthttp/
        └── native_handler.go        # Specific endpoints if needed
```

Native apps would:
- Use the **same** `jwt_validator` from `auth/infrastructure/`
- Use the **same** `authentication` middleware from `auth/middleware/`
- Use the **same** `request_gateway` from `auth/gateway/`
- Only require Bearer token validation (no cookie management, no refresh from gateway)

This is why the shared layer is organized the way it is — to support multiple authentication clients using the same core validation logic.

---

