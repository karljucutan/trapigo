# Trapigo Guide: Go Standard Project Layout + VSA

This guide combines:

- Common Go project layout patterns from the community reference
- Practical Go module organization from official Go docs
- Vertical Slice Architecture (VSA) for feature-first development

It is tailored for Trapigo as an API gateway.

## Guideline Context

The Go project layout described here is a practical structure for Trapigo. It is not a rigid standard, but it provides a clear convention for organizing code as the project grows.

This structure should be applied consistently so that entrypoints, business behavior, and shared runtime concerns remain easy to locate.

## Core Idea

Use this combination:

- cmd for executable entrypoints
- internal as the privacy boundary for app code
- features inside internal for vertical slices
- platform inside internal for commodity runtime capabilities
- pkg only for truly public reusable libraries

In short: features belong under internal.

## Recommended Trapigo Structure

```text
trapigo/
├── cmd/
│   └── gateway/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── gateway/
│   │       └── bootstrap/
│   │           └── app.go
│   ├── features/
│   │   ├── routing/
│   │   │   ├── domain/
│   │   │   │   ├── route.go
│   │   │   │   └── matcher.go
│   │   │   ├── service/
│   │   │   │   └── service.go
│   │   │   └── transporthttp/
│   │   │       └── routes.go
│   │   ├── proxy/
│   │   │   ├── domain/
│   │   │   │   ├── upstream.go
│   │   │   │   └── policy.go
│   │   │   ├── service/
│   │   │   │   └── service.go
│   │   │   └── transporthttp/
│   │   │       └── handler.go
│   │   ├── middleware/
│   │   │   ├── service/
│   │   │   │   └── chain.go
│   │   │   └── transporthttp/
│   │   │       └── handlers.go
│   │   ├── discovery/
│   │   │   ├── domain/
│   │   │   │   └── target.go
│   │   │   └── service/
│   │   │       └── resolver.go
│   │   ├── trafficpolicy/
│   │   │   ├── domain/
│   │   │   │   └── policy.go
│   │   │   └── service/
│   │   │       └── evaluator.go
│   │   ├── admin/
│   │   │   └── transporthttp/
│   │   │       ├── handler.go
│   │   │       └── routes.go
│   │   ├── users/
│   │   │   ├── domain/
│   │   │   │   ├── model.go
│   │   │   │   └── errors.go
│   │   │   ├── service/
│   │   │   │   └── service.go
│   │   │   └── transporthttp/
│   │   │       ├── handler.go
│   │   │       └── routes.go
│   │   ├── orders/
│   │   │   ├── domain/
│   │   │   │   ├── model.go
│   │   │   │   └── errors.go
│   │   │   ├── service/
│   │   │   │   └── service.go
│   │   │   └── transporthttp/
│   │   │       ├── handler.go
│   │   │       └── routes.go
│   │   └── health/
│   │       └── transporthttp/
│   │           ├── handler.go
│   │           └── routes.go
│   └── platform/
│       ├── config/
│       │   └── env.go
│       ├── runtime/
│       │   ├── signal.go
│       │   └── shutdown.go
│       ├── server/
│       │   ├── public.go
│       │   └── admin.go
│       └── observability/
│           ├── logging.go
│           ├── metrics.go
│           └── tracing.go
├── api/
│   └── openapi.yaml
├── configs/
│   └── .env.example
├── deployments/
│   ├── compose.yml
│   └── compose.override.yml
├── test/
│   └── integration/
│       └── gateway_test.go
└── README.md
```

Quick visual grouping:

- Flow of startup: cmd -> internal/app -> internal/features + internal/platform
- Product behavior: routing, proxying, middleware, discovery, and policies live in internal/features
- Shared technical runtime: internal/platform contains config, server lifecycle, and observability

## Directory Responsibilities

### cmd

Small main packages only.

main.go should only:

- load config
- build dependencies
- wire routes
- start and stop servers gracefully

No heavy business logic here.

### internal

Private code boundary enforced by the Go compiler.

Anything inside internal is not importable by external modules. This is ideal for service logic and app internals.

### internal/features

Vertical slices by feature or bounded context.

Each feature owns:

- domain rules and models
- service or use-case logic
- transport adapters such as HTTP handlers

For Trapigo specifically, gateway-defining capabilities are features, including:

- routing and matcher behavior
- reverse proxy behavior and upstream selection
- middleware pipeline and policy enforcement
- discovery and traffic policy decisions

Keep dependencies mostly inward:

transport -> service -> domain

### internal/platform

Shared technical runtime building blocks used across features.

Examples:

- config loading
- HTTP server startup and graceful shutdown
- logging and tracing
- metrics and process lifecycle helpers

No feature business decisions here.

### pkg

Use only if you intentionally expose stable reusable APIs for outside consumers.

If code is not intended for reuse outside this module, keep it in internal.

## VSA Rules of Thumb for Go

1. Organize by feature first, not by technical layer globally.
2. Keep each feature as independent as possible.
3. Allow shared infra in internal/platform, but avoid creating a giant dumping ground.
4. Do not create cross-feature imports unless necessary.
5. Use interfaces at boundaries where testability or replacement matters.

## Package Dependency Direction

Use this direction to avoid cycles:

- cmd/gateway imports internal/app/gateway/bootstrap
- bootstrap imports internal/platform and internal/features
- feature transport imports same feature service
- feature service imports same feature domain
- internal/platform imports only generic dependencies, never feature domain logic
- feature-to-feature imports should be minimized and explicit through service interfaces

## Application Wiring Flow

1. main.go initializes the application entrypoint.
2. the bootstrap layer loads config and creates shared platform components.
3. feature routes are registered into the gateway mux.
4. the public gateway and admin servers start.
5. graceful shutdown closes both servers.

## Naming Conventions

- Package names: short, lowercase, no underscores.
- Avoid generic names like utils or common.
- Prefer explicit names like transporthttp, reverseproxy, bootstrap.

## When To Add More Top-Level Directories

Add only when needed:

- api when you publish OpenAPI or protobuf contracts
- deployments when infra config grows
- docs when architecture decisions and runbooks grow
- scripts for repeatable dev/CI tasks

## Testing Conventions

Use tests in the most natural place:

- Unit tests usually live next to the code they test, using Go’s `_test.go` files in the same package directory.
- Broader external or integration tests belong under a separate directory such as test/ or test/integration/.

For Trapigo, a good split is:

- package-local tests for routing logic, proxy behavior, and small service units
- integration tests under test/integration/ for end-to-end gateway behavior

## FAQ

### Is features the same as internal

No.

- internal is the visibility boundary
- features is an organization style

Use internal/features together for VSA in Go services.

### Should everything go to pkg

No.

Default to internal unless you explicitly want external reuse.

### Is this too heavy for a small codebase

Start small. Keep only cmd, internal, and maybe test.
Grow directories only when a real need appears.

