# Goose Migrations

This directory holds PostgreSQL migration files for the order service using Goose.

## Commands

From the project root:

```bash
# install goose if needed
# go install github.com/pressly/goose/v3/cmd/goose@latest

# create a new migration
# goose -dir db/migrations create add_new_table sql

# run all pending migrations
# goose -dir db/migrations postgres "postgres://postgres:postgres@localhost:5432/trapigo?sslmode=disable" up

# roll back the last migration
# goose -dir db/migrations postgres "postgres://postgres:postgres@localhost:5432/trapigo?sslmode=disable" down

# check migration status
# goose -dir db/migrations postgres "postgres://postgres:postgres@localhost:5432/trapigo?sslmode=disable" status
```

## Migration naming

Use timestamped filenames such as:

```text
20260824_120000_add_orders_table.sql
```

Store each migration in a dedicated SQL file under this folder.
