package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS checkpoints (
		checkpoint_id TEXT PRIMARY KEY,
		payload BLOB NOT NULL,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS runs (
		run_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		status TEXT NOT NULL,
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_runs_session_id ON runs(session_id)`,
	`CREATE TABLE IF NOT EXISTS history_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		sequence INTEGER NOT NULL,
		kind TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		payload_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(session_id) REFERENCES sessions(session_id) ON DELETE CASCADE,
		FOREIGN KEY(run_id) REFERENCES runs(run_id) ON DELETE CASCADE,
		UNIQUE(run_id, sequence)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_history_events_session_id ON history_events(session_id, id)`,
	`CREATE INDEX IF NOT EXISTS idx_history_events_run_sequence ON history_events(run_id, sequence)`,
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migrations: %w", err)
	}

	for _, statement := range schemaStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply sqlite migration: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migrations: %w", err)
	}
	return nil
}
