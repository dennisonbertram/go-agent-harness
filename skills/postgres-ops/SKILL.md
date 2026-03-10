---
name: postgres-ops
description: "Set up and manage local PostgreSQL via Docker and psql for development and testing. Trigger: when setting up postgres, connecting to PostgreSQL, creating databases, running psql, using Docker postgres"
version: 1
argument-hint: "[database-url or action]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# PostgreSQL Operations

You are now operating in PostgreSQL management mode.

## Start Local PostgreSQL with Docker

```bash
# Start a local Postgres container (development defaults)
docker run -d \
  --name postgres \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=dev \
  postgres:16

# Verify it started
docker ps --filter name=postgres
```

## Connect with psql

```bash
# Connect using URL
psql postgresql://postgres:dev@localhost:5432/postgres

# Connect with individual flags
psql -h localhost -p 5432 -U postgres -d postgres

# One-shot command (non-interactive)
psql postgresql://postgres:dev@localhost:5432/postgres -c "SELECT version();"
```

## Database Management

```bash
# Create a database
docker exec postgres psql -U postgres -c "CREATE DATABASE myapp;"

# List databases
psql postgresql://postgres:dev@localhost:5432/postgres -c "\l"

# Drop a database (use with caution)
psql postgresql://postgres:dev@localhost:5432/postgres -c "DROP DATABASE IF EXISTS myapp;"

# Create a user
psql postgresql://postgres:dev@localhost:5432/postgres -c \
  "CREATE USER appuser WITH PASSWORD 'secret';"

# Grant privileges
psql postgresql://postgres:dev@localhost:5432/postgres -c \
  "GRANT ALL PRIVILEGES ON DATABASE myapp TO appuser;"
```

## Schema Inspection

```bash
# List tables in current database
psql $DATABASE_URL -c "\dt"

# Describe a table
psql $DATABASE_URL -c "\d+ users"

# List all schemas
psql $DATABASE_URL -c "\dn"

# Show table row counts
psql $DATABASE_URL -c "SELECT schemaname, tablename, n_live_tup FROM pg_stat_user_tables ORDER BY n_live_tup DESC;"
```

## Query Analysis

```bash
# Explain query plan
psql $DATABASE_URL -c "EXPLAIN ANALYZE SELECT * FROM users WHERE email = 'test@example.com';"

# Check slow queries (requires pg_stat_statements)
psql $DATABASE_URL -c "SELECT query, calls, total_time, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Check active connections
psql $DATABASE_URL -c "SELECT pid, usename, application_name, state, query FROM pg_stat_activity WHERE state != 'idle';"

# Check index usage
psql $DATABASE_URL -c "SELECT indexname, idx_scan, idx_tup_read FROM pg_stat_user_indexes ORDER BY idx_scan ASC;"
```

## Backup and Restore

```bash
# Custom format backup (recommended for pg_restore)
pg_dump -Fc $DATABASE_URL > backup.dump

# Plain SQL backup
pg_dump $DATABASE_URL > backup.sql

# Schema only
pg_dump --schema-only $DATABASE_URL > schema.sql

# Restore from custom format
pg_restore -d $DATABASE_URL backup.dump

# Restore from SQL
psql $DATABASE_URL < backup.sql
```

## Cleanup

```bash
# Stop and remove the container
docker stop postgres && docker rm postgres

# Remove with volumes (deletes all data)
docker stop postgres && docker rm -v postgres
```

## Connection String Format

```
postgresql://[user]:[password]@[host]:[port]/[database]?sslmode=disable

# Examples:
# Local dev (no SSL):
postgresql://postgres:dev@localhost:5432/myapp?sslmode=disable

# Production (with SSL):
postgresql://appuser:secret@db.example.com:5432/myapp?sslmode=require
```

## Safety Rules

- Never run `DROP DATABASE` or `DROP TABLE` in production without a backup.
- Use transactions for multi-statement schema changes: `BEGIN; ...; COMMIT;`
- Always test migrations on a copy of production data first.
- Store connection strings in environment variables, never in source code.
