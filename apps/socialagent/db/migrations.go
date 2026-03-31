package db

import "database/sql"

// migrate runs all DDL statements needed to bring the database schema
// up to date.  It is safe to call repeatedly — all statements use
// CREATE … IF NOT EXISTS / CREATE INDEX … IF NOT EXISTS.
func migrate(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			telegram_id     BIGINT      UNIQUE NOT NULL,
			conversation_id UUID        UNIQUE NOT NULL DEFAULT gen_random_uuid(),
			display_name    TEXT        NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
