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
go get github.com/golang-jwt/jwt/v5
```

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
   - [ ] Implement OIDC discovery endpoint client
   - [ ] Build authorization URL with:
     - `client_id`, `response_type=code`, `scope=openid`
     - `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`
   - [ ] Implement PKCE flow:
     - Generate `code_verifier` (43-128 chars, unreserved characters)
     - Generate `code_challenge = BASE64URL(SHA256(code_verifier))`
   - [ ] Implement token exchange:
     - POST to Keycloak `/token` endpoint with authorization code
     - Send `grant_type=authorization_code`, `client_id`, `client_secret`, `code`, `redirect_uri`, `code_verifier`
   - [ ] Implement token refresh:
     - POST with `grant_type=refresh_token`, `refresh_token`
   - [ ] Error handling for Keycloak failures

2. **Create JWT validator** (`jwt_validator.go`) — **SHARED component**
   - [ ] **Dependency**: Add `github.com/golang-jwt/jwt/v5` to `go.mod`
     - Provides: JWT parsing, signature verification, standard claim validation
     - Library handles: RSA/ECDSA verification, format validation, base64URL decoding
     - We implement: JWKS caching, claim validation logic, error mapping
   - [ ] Fetch and cache Keycloak JWKS from `/.well-known/jwks.json`
   - [ ] Implement JWKS key rotation/refresh logic
   - [ ] Validate JWT signature using cached keys via `jwt.ParseWithClaims()`
   - [ ] Validate JWT claims:
     - `iss` (issuer matches Keycloak realm)
     - `aud` (audience matches client_id or expected value)
     - `exp` (expiration time not passed)
     - `nbf` (not-before time if present)
   - [ ] Extract user information from JWT claims (sub, preferred_username, email)
   - [ ] Return validation result with extracted claims
   - **Note**: This validator is reused by both browser (web) and native authentication flows in the middleware

3. **Create domain models & errors** (`domain/`)
   - [ ] `domain/errors.go`:
     - `KeycloakConnectionError`
     - `InvalidJWTError`
     - `ExpiredTokenError`
     - `InvalidStateError`
     - `InvalidCallbackError`
   - [ ] `domain/claims.go` — JWT claims domain model (shared across auth flows)

**Testing**:
- [ ] Unit tests for PKCE generation
- [ ] Mock Keycloak responses for token exchange
- [ ] Mock JWKS and test JWT validation with various claim combinations

**Validation**:
- [ ] Can connect to Keycloak OIDC discovery endpoint
- [ ] Can parse JWKS and validate JWTs offline
- [ ] PKCE flow generates valid code_challenge

---

### Step 2: Cookie Management & OAuth State Storage

**Goal**: Securely manage authentication tokens in cookies and maintain temporary OAuth state.

**Files to create**:
- `internal/features/auth/infrastructure/cookie_manager.go`
- `internal/features/auth/infrastructure/oauth_state_store.go`
- `internal/features/auth/domain/oauth_state.go`

**Tasks**:

1. **Create OAuth state domain model** (`domain/oauth_state.go`)
   - [ ] `OAuthState` struct containing:
     - `state` (CSRF protection)
     - `nonce` (ID token validation)
     - `code_verifier` (PKCE)
     - `created_at` (expiration tracking)
   - [ ] Validation methods (not expired, valid format)

2. **Create OAuth state store** (`oauth_state_store.go`)
   - [ ] Interface: `OAuthStateStore`
     - `SaveState(ctx context.Context, state *OAuthState) error`
     - `GetState(ctx context.Context, stateValue string) (*OAuthState, error)`
     - `DeleteState(ctx context.Context, stateValue string) error`
   - [ ] In-memory implementation with automatic cleanup of expired states
   - [ ] Note: Consider Redis implementation for multi-instance deployments later

3. **Create cookie manager** (`cookie_manager.go`)
   - [ ] Constants for cookie names:
     - `web_access_token`
     - `web_refresh_token`
   - [ ] Function to create access token cookie:
     - HttpOnly, Secure (production), SameSite=Lax, Path=/
     - Configurable max-age based on token expiration
   - [ ] Function to create refresh token cookie:
     - Same attributes as access token
     - Longer max-age
   - [ ] Function to create "clear" cookies (max-age=0, same attributes)
   - [ ] Helper to extract token from cookie by name

**Testing**:
- [ ] Unit tests for cookie creation with correct attributes
- [ ] Unit tests for cookie clearing
- [ ] Unit tests for OAuth state expiration

**Validation**:
- [ ] Cookies have correct HttpOnly, Secure, SameSite attributes
- [ ] Cookies are properly cleared (max-age=0)
- [ ] OAuth states expire after configured duration

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
   - [ ] Generate `state`, `nonce`, `code_verifier`
   - [ ] Calculate `code_challenge`
   - [ ] Save OAuth state to store with expiration
   - [ ] Return Keycloak authorization URL

2. **Create callback command** (`application/command/callback.go`)
   - [ ] Extract `code` and `state` from query parameters
   - [ ] Retrieve OAuth state from store
   - [ ] Validate `state` matches stored value
   - [ ] Exchange code with Keycloak using `code_verifier`
   - [ ] Receive access and refresh tokens
   - [ ] Validate tokens (signature, issuer, audience)
   - [ ] Delete OAuth state from store (one-time use)
   - [ ] Return tokens for cookie setting

3. **Create logout command** (`application/command/logout.go`)
   - [ ] Optionally revoke refresh token with Keycloak
   - [ ] Return cookie clear instructions

4. **Create get current user query** (`application/query/get_current_user.go`)
   - [ ] Accept validated JWT or extract from context
   - [ ] Extract user info from JWT claims
   - [ ] Return user info without exposing tokens

5. **Create HTTP handlers** (`transporthttp/auth_handler.go`)
   - [ ] `HandleLogin(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/login`
     - Call login command
     - Redirect to Keycloak authorization URL
   - [ ] `HandleCallback(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/callback`
     - Call callback command
     - Set cookies in response
     - Redirect to frontend application
   - [ ] `HandleLogout(w http.ResponseWriter, r *http.Request)`
     - POST `/web/auth/logout`
     - Call logout command
     - Clear cookies
     - Redirect or return 200
   - [ ] `HandleMe(w http.ResponseWriter, r *http.Request)`
     - GET `/web/auth/me`
     - Extract auth from context (set by middleware)
     - Call get current user query
     - Return JSON with user info or 401

**Testing**:
- [ ] Test login endpoint redirects correctly
- [ ] Test callback with valid authorization code
- [ ] Test callback with invalid state (reject)
- [ ] Test callback with expired OAuth state (reject)
- [ ] Test logout clears cookies
- [ ] Test `/web/auth/me` returns authenticated user
- [ ] Test `/web/auth/me` returns 401 when unauthenticated

**Validation**:
- [ ] `/web/auth/login` redirects to Keycloak
- [ ] `/web/auth/callback?code=...&state=...` exchanges code and sets cookies
- [ ] `/web/auth/logout` clears cookies
- [ ] `/web/auth/me` returns user info when authenticated

---

### Step 4: Authentication Middleware

**Goal**: Detect and validate authentication, with automatic token refresh for browser clients.

**Files to create** (both under `internal/features/auth/middleware/`):
- `authentication.go`
- `authentication_test.go`

**Tasks**:

1. **Create authentication middleware** (`middleware/authentication.go`) — **SHARED component**
   - [ ] Implement precedence for auth detection:
     1. `Authorization: Bearer <JWT>` (native/direct-token)
     2. `web_access_token` cookie (browser/BFF)
     3. No credentials → 401
   - **Note**: This middleware applies to ALL `/api/*` routes and intelligently routes to either Bearer or Cookie flow
   - [ ] For Bearer token:
     - Extract token from header
     - Validate JWT using jwt_validator
     - If expired → 401 (do not refresh)
     - If valid → extract claims and add to request context
   - [ ] For browser cookie:
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
   - [ ] Add `http.Handler` wrapper for easy middleware integration
   - [ ] Context keys for storing authenticated user/claims

**Testing**:
- [ ] Test Bearer token validation (valid/expired/invalid)
- [ ] Test cookie extraction and validation
- [ ] Test token refresh on expired access token
- [ ] Test 401 for missing credentials
- [ ] Test precedence when both Bearer and cookie present (Bearer wins)

**Validation**:
- [ ] Middleware correctly detects auth mechanism
- [ ] Expired bearer tokens return 401
- [ ] Expired browser access tokens are refreshed transparently
- [ ] Valid tokens allow request to proceed

---

### Step 5: Downstream Request Gateway

**Goal**: Forward authenticated requests to downstream APIs with normalized authorization.

**Files to create** (both under `internal/features/auth/gateway/`):
- `request_gateway.go`
- `request_gateway_test.go`

**Tasks**:

1. **Create request gateway** (`gateway/request_gateway.go`)
   - [ ] Accept authenticated request with user context
   - [ ] Extract access token from context
   - [ ] Add `Authorization: Bearer <access_token>` to outbound request
   - [ ] Proxy request to downstream API (orders service, etc.)
   - [ ] Handle downstream errors:
     - 401 → may indicate token validation issue at downstream, return 401
     - 403 → authorization/permission issue, return 403
     - Other errors → proxy error response
   - [ ] Return response to client unchanged

**Testing**:
- [ ] Test request forwarding with valid token
- [ ] Test authorization header added to upstream request
- [ ] Test downstream 401 is returned to client
- [ ] Test downstream 500 is returned to client

**Validation**:
- [ ] Authenticated requests are forwarded with Bearer token
- [ ] Downstream API responses are passed through correctly

---

### Step 6: Wiring & Integration

**Goal**: Connect all components and integrate into the API Gateway.

**Tasks**:

1. **Create main router/bootstrap** (`internal/app/gateway/bootstrap/app.go` or similar)
   - [ ] Initialize shared Keycloak client (from `auth/infrastructure/keycloak.go`)
   - [ ] Initialize shared JWT validator with Keycloak JWKS cache (from `auth/infrastructure/jwt_validator.go`)
   - [ ] Initialize shared cookie manager (from `auth/infrastructure/cookie_manager.go`)
   - [ ] Initialize shared OAuth state store (from `auth/infrastructure/oauth_state_store.go`)
   - [ ] Initialize shared authentication middleware (from `auth/middleware/authentication.go`)
   - [ ] Register `/web/auth/*` routes with web-specific auth handlers (from `auth/web/transporthttp/`)
   - [ ] Wrap `/api/*` routes with shared authentication middleware
   - [ ] Set up downstream request gateway for `/api/*` (from `auth/gateway/`)
   - **Note**: All `/api/*` routes use the same shared authentication middleware, which internally handles both Bearer and Cookie flows

2. **Configuration** (`internal/platform/config/config.go`)
   - [ ] `keycloak.issuer_url`
   - [ ] `keycloak.client_id`
   - [ ] `keycloak.client_secret`
   - [ ] `keycloak.redirect_uri` (e.g., `https://example.com/web/auth/callback`)
   - [ ] `auth.cookie_secure` (true in production)
   - [ ] `auth.cookie_same_site` (Lax or Strict)
   - [ ] `auth.state_expiration` (e.g., 10 minutes)
   - [ ] `auth.token_cache_ttl` (JWKS cache)
   - [ ] `downstream_api.base_url` (e.g., `http://go-order-service:8080`)

3. **Environment setup**
   - [ ] Create `.env.example` with auth configuration
   - [ ] Document required Keycloak client setup

**Testing**:
- [ ] Integration test: Login flow end-to-end
- [ ] Integration test: Browser makes `/api/*` request after login
- [ ] Integration test: Token refresh on expired access token
- [ ] Integration test: Logout clears cookies

---

### Step 7: Security Hardening & Testing

**Goal**: Verify all security requirements are met.

**Tasks**:

1. **Security checklist** (from auth_plan.md section 24)
   - [ ] Authorization Code Flow is used
   - [ ] PKCE S256 is used
   - [ ] `state` is generated and validated
   - [ ] `nonce` is generated (for ID token validation if using ID token)
   - [ ] Redirect URI is exact and configured in Keycloak
   - [ ] Client secret is never exposed to browser/native app
   - [ ] Access token cookie is HttpOnly
   - [ ] Refresh token cookie is HttpOnly
   - [ ] Cookies use Secure in production
   - [ ] SameSite policy is intentional (Lax default)
   - [ ] JWT signature is validated
   - [ ] JWT issuer is validated
   - [ ] JWT audience is validated
   - [ ] JWT expiration is validated
   - [ ] Tokens are never logged
   - [ ] Tokens are never returned in API response bodies

2. **CSRF Protection** (section 17)
   - [ ] Verify SameSite cookie behavior
   - [ ] Consider CSRF token for state-changing requests if needed
   - [ ] Test Origin header validation for sensitive endpoints

3. **Error handling**
   - [ ] Keycloak errors don't leak sensitive info
   - [ ] Expired tokens result in 401, not 500
   - [ ] Invalid state in callback is rejected (not exchanged)

4. **Integration testing**
   - [ ] Test complete browser login/logout cycle
   - [ ] Test native app direct token flow (no browser auth)
   - [ ] Test token refresh scenarios
   - [ ] Test concurrent requests with token refresh
   - [ ] Test invalid tokens are rejected
   - [ ] Test downstream API receives correct Authorization header

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

downstream_api:
  orders_service: "http://go-order-service:8080"
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
| 1 | Keycloak integration working | [ ] |
| 2 | Cookie & OAuth state management | [ ] |
| 3 | `/web/auth/*` endpoints functional | [ ] |
| 4 | Authentication middleware routing requests | [ ] |
| 5 | Downstream API requests forwarded with Bearer token | [ ] |
| 6 | End-to-end browser login flow working | [ ] |
| 7 | All security requirements verified | [ ] |

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

