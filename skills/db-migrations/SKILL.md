---
name: db-migrations
description: "Create, run, and rollback database migrations using goose or golang-migrate. Trigger: when running database migrations, creating migration files, rolling back migrations, using goose, using golang-migrate"
version: 1
argument-hint: "[up|down|status|create <name>]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# Database Migrations

You are now operating in database migration mode.

## Goose (Recommended for Go Projects)

### Installation

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Create Migrations

```bash
# Create a SQL migration
goose -dir migrations create add_users_table sql

# Create a Go migration (for complex data transforms)
goose -dir migrations create backfill_user_roles go
```

This creates timestamped files like `migrations/20260310120000_add_users_table.sql`.

### SQL Migration File Format

```sql
-- +goose Up
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_email ON users(email);

-- +goose Down
DROP TABLE IF EXISTS users;
```

### Running Migrations

```bash
# Apply all pending migrations
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" up

# Apply one migration
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" up-by-one

# Rollback one migration
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" down

# Rollback to a specific version
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" down-to 20260310000000

# Rollback and re-apply last migration
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" redo

# Show migration status
goose -dir migrations postgres "postgresql://postgres:dev@localhost:5432/myapp" status
```

### SQLite with Goose

```bash
# SQLite driver
goose -dir migrations sqlite3 "./app.db" up
goose -dir migrations sqlite3 "./app.db" status
```

### Goose in Go Code (Embedded)

```go
import (
    "embed"
    "database/sql"
    "github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func runMigrations(db *sql.DB) error {
    goose.SetBaseFS(migrations)
    if err := goose.SetDialect("postgres"); err != nil {
        return err
    }
    return goose.Up(db, "migrations")
}
```

## golang-migrate

### Installation

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

### Create Migrations

golang-migrate uses paired files: one for up, one for down.

```bash
migrate create -ext sql -dir migrations -seq add_users_table
# Creates:
#   migrations/000001_add_users_table.up.sql
#   migrations/000001_add_users_table.down.sql
```

### Running Migrations

```bash
# Apply all migrations
migrate -path migrations -database "postgresql://postgres:dev@localhost:5432/myapp?sslmode=disable" up

# Rollback N steps
migrate -path migrations -database "..." down 1

# Go to a specific version
migrate -path migrations -database "..." goto 3

# Show current version
migrate -path migrations -database "..." version
```

## Simple SQL Migration Pattern (No Tool)

For small projects without a migration tool:

```bash
# Create migration directory
mkdir -p migrations

# Number migrations sequentially
# migrations/001_create_users.sql
# migrations/002_add_sessions.sql

# Apply with a simple script
for f in migrations/*.sql; do
  psql "$DATABASE_URL" -f "$f"
  echo "Applied: $f"
done
```

## Best Practices

- Always write a `Down` migration that perfectly reverses the `Up`.
- Test the rollback locally before merging.
- Never edit a migration that has already been applied to production.
- Use transactions in SQL migrations when the database supports them.
- Keep migrations small and focused — one logical change per migration.
- Run `status` before `up` in CI to confirm the expected state.

## Migration Status Check

```bash
# Goose status shows applied/pending
goose -dir migrations postgres "$DATABASE_URL" status

# Example output:
#    Applied At                  Migration
#    =======================================
#    2026-03-01 10:00:00 +0000   001_create_users.sql
#    Pending                     002_add_sessions.sql
```
