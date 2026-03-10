---
name: sqlite-ops
description: "Initialize and manage SQLite databases for development, testing, and embedded use cases. Trigger: when working with SQLite, sqlite3 CLI, embedded database, Go modernc sqlite, WAL mode"
version: 1
argument-hint: "[database-file]"
allowed-tools:
  - bash
  - read
  - write
  - grep
  - glob
---
# SQLite Operations

You are now operating in SQLite database management mode.

## Basic sqlite3 CLI Usage

```bash
# Open or create a database
sqlite3 app.db

# Run a single command non-interactively
sqlite3 app.db "SELECT * FROM users LIMIT 5;"

# Run a SQL file
sqlite3 app.db < schema.sql
```

## Schema Inspection

```bash
# List all tables
sqlite3 app.db ".tables"

# Show schema for all tables
sqlite3 app.db ".schema"

# Show schema for a specific table
sqlite3 app.db ".schema users"

# Show full CREATE statements including indexes
sqlite3 app.db ".fullschema"

# Show table info (columns, types, constraints)
sqlite3 app.db "PRAGMA table_info(users);"

# Show indexes on a table
sqlite3 app.db "PRAGMA index_list(users);"
```

## Common Queries

```bash
# Count rows
sqlite3 app.db "SELECT count(*) FROM users;"

# Query with formatting
sqlite3 -column -header app.db "SELECT id, name, email FROM users LIMIT 10;"

# JSON output
sqlite3 -json app.db "SELECT * FROM users LIMIT 5;"

# CSV output
sqlite3 -csv -header app.db "SELECT * FROM users;" > export.csv
```

## Backup and Restore

```bash
# Dump entire database to SQL
sqlite3 app.db ".dump" > backup.sql

# Dump a single table
sqlite3 app.db ".dump users" > users.sql

# Restore from dump
sqlite3 new.db < backup.sql

# Online backup (safe while DB is open)
sqlite3 app.db ".backup backup.db"

# Copy using the backup API
sqlite3 app.db "VACUUM INTO 'backup.db';"
```

## WAL Mode (Write-Ahead Logging)

WAL mode improves concurrent read performance and is recommended for multi-reader applications.

```bash
# Enable WAL mode
sqlite3 app.db "PRAGMA journal_mode=WAL;"

# Check current journal mode
sqlite3 app.db "PRAGMA journal_mode;"

# WAL checkpoint (flush WAL to main database)
sqlite3 app.db "PRAGMA wal_checkpoint(FULL);"
```

## Performance Tuning

```bash
# Check and set cache size (negative = kilobytes)
sqlite3 app.db "PRAGMA cache_size = -64000;"  # 64MB

# Enable memory-mapped I/O
sqlite3 app.db "PRAGMA mmap_size = 268435456;"  # 256MB

# Synchronous mode (NORMAL = good balance)
sqlite3 app.db "PRAGMA synchronous = NORMAL;"

# Foreign key enforcement (off by default)
sqlite3 app.db "PRAGMA foreign_keys = ON;"

# Analyze query plan
sqlite3 app.db "EXPLAIN QUERY PLAN SELECT * FROM users WHERE email = 'test@example.com';"
```

## Go modernc SQLite Usage

The `modernc.org/sqlite` driver is a pure-Go SQLite driver (no CGo required).

```go
import (
    "database/sql"
    _ "modernc.org/sqlite"
)

// Open with WAL mode and foreign keys
db, err := sql.Open("sqlite", "app.db?_journal_mode=WAL&_foreign_keys=on")
if err != nil {
    return fmt.Errorf("open db: %w", err)
}
defer db.Close()

// In-memory database for tests
testDB, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
```

## Integrity Checks

```bash
# Full integrity check
sqlite3 app.db "PRAGMA integrity_check;"

# Quick check (faster, catches most issues)
sqlite3 app.db "PRAGMA quick_check;"

# Check for foreign key violations
sqlite3 app.db "PRAGMA foreign_key_check;"
```

## Safety Rules

- Always use parameterized queries in Go — never fmt.Sprintf SQL.
- Enable WAL mode for applications with concurrent readers.
- Take regular backups using `.backup` or `VACUUM INTO`.
- Enable `PRAGMA foreign_keys = ON` explicitly — it is off by default.
- Use `:memory:` databases in tests to avoid test pollution.
