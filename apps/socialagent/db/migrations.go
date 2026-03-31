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

		`CREATE TABLE IF NOT EXISTS user_profiles (
			user_id        UUID        PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			summary        TEXT        NOT NULL DEFAULT '',
			interests      TEXT[]      NOT NULL DEFAULT '{}',
			looking_for    TEXT        NOT NULL DEFAULT '',
			last_summary_at TIMESTAMPTZ,
			created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS activity_log (
			id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			display_name  TEXT        NOT NULL DEFAULT '',
			activity_type TEXT        NOT NULL DEFAULT 'message',
			content       TEXT        NOT NULL DEFAULT '',
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_activity_log_created_at ON activity_log(created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS user_insights (
			id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			insight    TEXT        NOT NULL,
			source     TEXT        NOT NULL DEFAULT 'agent',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_insights_user_id ON user_insights(user_id)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
