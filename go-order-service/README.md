# go-order-service

A small Go CRUD service for orders and order items.

## Architecture

This service follows a practical VSA + DDD-inspired structure:

- feature-first layout under `internal/features/orders`
- `OrderItem` remains inside the Orders feature as part of the `Order` aggregate
- repository pattern for persistence
- CQRS-style separation between commands and queries within the same feature slice
- endpoints call command/query handlers directly
- PostgreSQL via `database/sql` and `github.com/jackc/pgx/v5/stdlib`
- money stored in integer cents using `int64`
- IDs stored as `BIGINT` in Postgres and `int64` in Go

The key design preference here is to keep the feature focused without over-fragmenting it:

- command logic lives in dedicated command objects or use-case files
- query logic lives in dedicated query objects or use-case files
- the transport HTTP handler stays close to the feature and invokes handlers directly
- the application remains easy to navigate and test without turning into a large service god object

## Stack

- Go
- PostgreSQL
- `database/sql`
- `github.com/jackc/pgx/v5/stdlib`

## Project layout

```text
go-order-service/
├── cmd/
│   └── app/
│       └── main.go
├── db/
│   └── migrations/
│       ├── README.md
│       └── initial/
│           ├── README.md
│           └── 001_initial_schema.sql
├── internal/
│   ├── features/
│   │   └── orders/
│   │       ├── command/
│   │       │   ├── create_order.go
│   │       │   ├── update_order_status.go
│   │       │   └── delete_order.go
│   │       ├── domain/
│   │       │   ├── order.go
│   │       │   ├── order_item.go
│   │       │   └── errors.go
│   │       ├── query/
│   │       │   ├── get_order_by_id.go
│   │       │   └── list_orders.go
│   │       ├── repository/
│   │       │   └── repository.go
│   │       └── transporthttp/
│   │           └── order_handler.go
│   └── platform/
│       ├── config/
│       │   └── config.go
│       └── database/
│           └── database.go
├── go.mod
├── README.md
├── GO_ORDER_SERVICE_PLAN.md
└── docker-compose.yml
```

## CQRS-oriented intent

This project is intentionally small, so it does not go full-bore CQRS with separate microservices or a formal event bus.

Instead, we take a pragmatic approach:

- `command/` contains write-side use cases
- `query/` contains read-side use cases
- `transporthttp/` exposes the feature through HTTP and invokes the handlers directly

This keeps the structure readable while respecting the VSA style and the DDD idea that the order aggregate owns its items.

## Database expectations

Postgres tables are designed around `BIGINT` IDs and money stored in cents:

```sql
CREATE TABLE IF NOT EXISTS order (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()  
);

CREATE TABLE IF NOT EXISTS order_item (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES order(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL,
    subtotal_cents BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Run

```bash
go run ./cmd/app
```

## Test

```bash
go test ./...
```
