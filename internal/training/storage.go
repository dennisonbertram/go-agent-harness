package training

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS traces (
	run_id TEXT PRIMARY KEY,
	task_id TEXT,
	outcome TEXT,
	steps INTEGER,
	cost_usd REAL,
	efficiency_score REAL,
	first_try_rate REAL,
	anti_pattern_count INTEGER,
	created_at TEXT,
	bundle_json TEXT
);

CREATE TABLE IF NOT EXISTS findings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT,
	batch_id TEXT,
	type TEXT,
	priority TEXT,
	target TEXT,
	issue TEXT,
	proposed TEXT,
	rationale TEXT,
	confidence TEXT,
	evidence_count INTEGER,
	status TEXT DEFAULT 'pending',
	created_at TEXT
);

CREATE TABLE IF NOT EXISTS applied_changes (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	git_commit TEXT,
	finding_id INTEGER,
	description TEXT,
	reverted INTEGER DEFAULT 0,
	created_at TEXT
);

CREATE TABLE IF NOT EXISTS patterns (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	failure_mode TEXT UNIQUE,
	frequency INTEGER,
	last_seen TEXT,
	description TEXT
);
`

// Store provides SQLite-backed persistence for training data.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store with the SQLite database at dbPath.
func NewStore(dbPath string) (*Store, error) {
	dsn := dbPath + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open training db: %w", err)
	}
	// Single writer connection prevents SQLITE_BUSY under concurrent writes.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &Store{db: db}, nil
}

// SaveTrace persists a trace bundle and its score to the database.
func (s *Store) SaveTrace(bundle TraceBundle, score ScoreResult) error {
	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("marshal bundle: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO traces (run_id, task_id, outcome, steps, cost_usd, efficiency_score, first_try_rate, anti_pattern_count, created_at, bundle_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		bundle.RunID,
		bundle.TaskID,
		bundle.Outcome,
		bundle.Steps,
		bundle.CostUSD,
		score.Efficiency,
		score.FirstTryRate,
		score.AntiPatternCount,
		time.Now().UTC().Format(time.RFC3339),
		string(bundleJSON),
	)
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}
	return nil
}

// GetTrace retrieves a trace bundle by run ID.
func (s *Store) GetTrace(runID string) (*TraceBundle, error) {
	var bundleJSON string
	err := s.db.QueryRow("SELECT bundle_json FROM traces WHERE run_id = ?", runID).Scan(&bundleJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("trace not found: %s", runID)
		}
		return nil, fmt.Errorf("query trace: %w", err)
	}

	var bundle TraceBundle
	if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
		return nil, fmt.Errorf("unmarshal bundle: %w", err)
	}
	return &bundle, nil
}

// SaveFindings persists a set of findings for a run.
func (s *Store) SaveFindings(runID string, findings []Finding) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO findings (run_id, type, priority, target, issue, proposed, rationale, confidence, evidence_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range findings {
		_, err := stmt.Exec(runID, f.Type, f.Priority, f.Target, f.Issue, f.Proposed, f.Rationale, string(f.Confidence), f.EvidenceCount, now)
		if err != nil {
			return fmt.Errorf("insert finding: %w", err)
		}
	}

	return tx.Commit()
}

// GetPendingFindings returns all findings with status 'pending'.
func (s *Store) GetPendingFindings() ([]Finding, error) {
	rows, err := s.db.Query(`
		SELECT type, priority, target, issue, proposed, rationale, confidence, evidence_count
		FROM findings WHERE status = 'pending' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query pending findings: %w", err)
	}
	defer rows.Close()

	var findings []Finding
	for rows.Next() {
		var f Finding
		var conf string
		if err := rows.Scan(&f.Type, &f.Priority, &f.Target, &f.Issue, &f.Proposed, &f.Rationale, &conf, &f.EvidenceCount); err != nil {
			return nil, fmt.Errorf("scan finding: %w", err)
		}
		f.Confidence = Confidence(conf)
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// SaveAppliedChange records an applied change linked to a finding.
func (s *Store) SaveAppliedChange(commit string, findingID int64, desc string) error {
	_, err := s.db.Exec(`
		INSERT INTO applied_changes (git_commit, finding_id, description, created_at)
		VALUES (?, ?, ?, ?)`,
		commit, findingID, desc, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("insert applied change: %w", err)
	}
	return nil
}

// AppliedChange represents a row from the applied_changes table.
type AppliedChange struct {
	ID          int64  `json:"id"`
	GitCommit   string `json:"git_commit"`
	FindingID   int64  `json:"finding_id"`
	Description string `json:"description"`
	Reverted    bool   `json:"reverted"`
	CreatedAt   string `json:"created_at"`
}

// CountTraces returns the number of rows in the traces table.
func (s *Store) CountTraces() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM traces").Scan(&n); err != nil {
		return 0, fmt.Errorf("count traces: %w", err)
	}
	return n, nil
}

// CountFindings returns the number of rows in the findings table.
func (s *Store) CountFindings() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM findings").Scan(&n); err != nil {
		return 0, fmt.Errorf("count findings: %w", err)
	}
	return n, nil
}

// CountAppliedChanges returns the number of rows in the applied_changes table.
func (s *Store) CountAppliedChanges() (int, error) {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM applied_changes").Scan(&n); err != nil {
		return 0, fmt.Errorf("count applied changes: %w", err)
	}
	return n, nil
}

// QueryHistory returns applied changes created at or after the given time.
func (s *Store) QueryHistory(since time.Time) ([]AppliedChange, error) {
	rows, err := s.db.Query(`
		SELECT id, git_commit, finding_id, description, reverted, created_at
		FROM applied_changes
		WHERE created_at >= ?
		ORDER BY created_at`, since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var changes []AppliedChange
	for rows.Next() {
		var c AppliedChange
		var reverted int
		if err := rows.Scan(&c.ID, &c.GitCommit, &c.FindingID, &c.Description, &reverted, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan applied change: %w", err)
		}
		c.Reverted = reverted != 0
		changes = append(changes, c)
	}
	return changes, rows.Err()
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
