# go-order-service

A small Go service for managing orders and order items using a modern Go stack:

- standard library database access via `database/sql`
- PostgreSQL driver via `github.com/jackc/pgx/v5/stdlib`
- repository-based persistence layer
- vertical slice architecture (VSA)
- Go community-style project layout

This service is intended to be a clean example of a practical CRUD API built without an ORM, while still following idiomatic and maintainable Go patterns.

## Goals

- Manage `orders` and `order_items`
- Keep business logic separated from storage details
- Use the Repository pattern for data access
- Prefer explicit, testable code over framework-heavy abstractions
- Align the project structure with Trapigo-style VSA and the common Go project layout

## Tech Stack

- Go 1.22+
- PostgreSQL
- `database/sql`
- `github.com/jackc/pgx/v5/stdlib`
- standard Go packages only for core logic and HTTP server wiring

## Architectural approach

### Pragmatic layered VSA

The project uses a feature-first Vertical Slice layout, but keeps a light layering inside each feature slice so boundaries stay explicit without becoming over-engineered.

This means the `orders` slice can be organized as:

- `domain/` for aggregate rules and entities
- `application/` for use cases, commands, queries, and repository ports
- `infrastructure/` for Postgres, Redis, and external adapters
- `transporthttp/` for HTTP request/response handling

This gives us the best of both approaches:

- feature-oriented organization from VSA
- dependency direction and isolation from a layered design
- flexibility to add adapters (database, cache, message broker) without coupling core logic to external systems

### Repository Pattern

The project is organized around the Repository pattern so the application layer does not depend directly on SQL details.

Responsibilities:

- domain models define the business entities
- repository interfaces define the persistence contract
- repositories encapsulate persistence logic
- services orchestrate business rules
- handlers/transport layer handle HTTP concerns

This makes it easier to:

- switch persistence implementations later
- test business logic without DB wiring
- keep SQL knowledge isolated in repository code
- add Redis or other caching layers behind the same interface

### VSA + DDD modeling decision

For this service, we are using VSA as the package organization model, but we also keep a DDD-style aggregate mindset around the `Order` feature.

Important decision:

- `OrderItem` stays inside the `orders` feature
- `Order` is treated as the aggregate root
- `OrderItem` belongs to the `Order` aggregate and is not treated as a separate top-level feature

This gives us the best of both approaches:

- feature-first code organization from VSA
- aggregate ownership from DDD tactical modeling

In practice, this means the `orders` slice owns:

- `domain/order.go`
- `domain/order_item.go`
- repository contracts and implementations
- use-case/service logic
- HTTP transport for order APIs

We only create a separate feature if it becomes an independent business capability later.

## Recommended project layout

```text
go-order-service/
├── cmd/
│   └── app/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── bootstrap/
│   │       └── app.go
│   ├── features/
│   │   └── orders/
│   │       ├── domain/
│   │       │   ├── order.go
│   │       │   ├── order_item.go
│   │       │   ├── errors.go
│   │       │   └── order_repository.go
│   │       ├── application/
│   │       │   ├── command/
│   │       │   │   ├── create_order.go
│   │       │   │   ├── update_order_status.go
│   │       │   │   └── delete_order.go
│   │       │   └── query/
│   │       │       ├── get_order_by_id.go
│   │       │       └── list_orders.go
│   │       ├── infrastructure/
│   │       │   ├── repository/
│   │       │   │   ├── postgres_order_repository.go
│   │       │   │   └── cached_order_repository.go
│   │       │   └── cache/
│   │       │       └── redis_client.go
│   │       └── transporthttp/
│   │           ├── handler.go
│   │           └── routes.go
│   └── platform/
│       ├── config/
│       │   └── config.go
│       ├── database/
│       │   └── connection.go
│       ├── server/
│       │   └── server.go
│       └── observability/
│           └── logging.go
├── pkg/
│   └── httpx/
│       └── response.go
├── configs/
│   └── app.yaml
├── test/
│   └── integration/
│       └── orders_test.go
├── go.mod
├── .env.example
├── Makefile
├── README.md
└── docker-compose.yml
```

This layout keeps the feature-first VSA organization while making the boundary explicit:

- `domain/` owns business rules and the aggregate
- `application/` owns use cases and repository interfaces/ports
- `infrastructure/` owns DB and cache adapters
- `transporthttp/` owns the HTTP adapter

This is the recommended structure for a pragmatic DDD + VSA service without introducing heavy architectural ceremony.

## Domain model

### Order

An order normally contains:

- `id` (BIGINT identity key)
- `customer_id` (BIGINT)
- `status`
- `total_amount_cents` (stored as integer minor units)
- `created_at`
- `updated_at`

### OrderItem

An order item normally contains:

- `id` (BIGINT identity key)
- `order_id` (BIGINT foreign key)
- `product_id` (BIGINT)
- `quantity`
- `unit_price_cents` (stored as integer minor units)
- `subtotal_cents` (stored as integer minor units)
- `created_at`
- `updated_at`

### ID strategy

Use `BIGINT` as the default identity type for this service.

Why:

- it is the most common simple choice in Go + Postgres apps
- it keeps the schema easier to reason about than UUIDs for a small CRUD service
- it works well with `database/sql` and `pgx` without extra helper libraries
- it is easy to paginate, filter, and join on

If the service later becomes multi-tenant, distributed, or externally exposed at scale, UUIDs may be reconsidered. For the initial CRUD version, `BIGINT` is the better default.

### Money storage decision

Use integer minor units instead of database decimal types for currency values in this project.

Why:

- Go does not have a native money type like C# `decimal`
- `float64` is unsafe for money precision
- Postgres `BIGINT` is easy to reason about and commonly used for financial values
- This keeps currency storage consistent, testable, and portable across languages

Rule:

- store all money values in cents
- use `BIGINT` in Postgres
- use `int64` in Go
- convert to/from display strings only at the API boundary

Example:

- $12.34 -> 1234
- $0.99 -> 99
- $1,000.00 -> 100000

## CRUD operations

### Orders

- Create order
- Get order by ID
- List orders
- Update order
- Delete order

### Order items

- Create order item
- Get order item by ID
- List order items for an order
- Update order item
- Delete order item

## Database access pattern

The service should use a `database/sql` connection with the PostgreSQL `pgx` stdlib driver:

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)

func OpenDB(dsn string) (*sql.DB, error) {
    return sql.Open("pgx", dsn)
}
```

This keeps the implementation idiomatic and standard-library friendly while using PostgreSQL support from pgx.

## Suggested repository shape

```go
package repository

type OrderRepository interface {
    Create(ctx context.Context, order Order) (Order, error)
    GetByID(ctx context.Context, id int64) (Order, error)
    List(ctx context.Context) ([]Order, error)
    Update(ctx context.Context, order Order) (Order, error)
    Delete(ctx context.Context, id int64) error
}
```

The repository layer handles:

- SQL query construction
- scanning rows into structs
- transaction boundaries when needed
- error mapping and domain-level validation coordination

## Recommended service flow

1. HTTP handler receives request
2. handler validates input DTOs
3. service layer applies business rules
4. repository performs persistence
5. result is returned as a domain model or API response

This keeps the app readable and maintainable as it grows.

## Example SQL structure

```sql
CREATE TABLE orders (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    total_amount_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE order_items (
    id BIGINT GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL,
    subtotal_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Go model mapping should use integer cents, not float values:

```go
type Order struct {
    ID               int64
    CustomerID       int64
    Status           string
    TotalAmountCents int64
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type OrderItem struct {
    ID              int64
    OrderID         int64
    ProductID       int64
    Quantity        int
    UnitPriceCents  int64
    SubtotalCents   int64
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

## Local setup

### Prerequisites

- Go installed
- PostgreSQL running locally or via Docker

### Example environment

```env
DB_DSN=postgres://postgres:postgres@localhost:5432/orders_db
PORT=8080
LOG_LEVEL=debug
```

### Run the service

```bash
go mod tidy
go run ./cmd/app
```

### With Docker

```bash
docker compose up --build
```

## Testing

Use Go tests for:

- repository behavior
- service validation rules
- HTTP handler responses
- end-to-end CRUD flow

Example:

```bash
go test ./...
```

## Notes

This README reflects the intended architecture for the service in a modern Go style. The project should stay lightweight, explicit, and production-friendly without introducing a heavy framework or DSL-based ORM.

The goal is to keep the codebase easy to understand, test, and evolve while following the same general conventions used by the Trapigo gateway and the common Go community project layout patterns.
