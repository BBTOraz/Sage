package sqlite

import "context"

type Session struct {
	ID           string
	MetadataJSON string
}

type Run struct {
	ID           string
	SessionID    string
	Status       string
	MetadataJSON string
}

type HistoryEvent struct {
	ID          int64
	SessionID   string
	RunID       string
	Sequence    int
	Kind        string
	Role        string
	Content     string
	PayloadJSON string
	CreatedAt   string
}

type HistoryStore struct {
	db DB
}

func NewHistoryStore(db DB) *HistoryStore {
	return &HistoryStore{db: db}
}

func (s *HistoryStore) UpsertSession(ctx context.Context, session Session) error {
	const query = `
		INSERT INTO sessions (session_id, metadata_json, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(session_id) DO UPDATE SET
			metadata_json = excluded.metadata_json,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, session.ID, session.MetadataJSON)
	return err
}

func (s *HistoryStore) UpsertRun(ctx context.Context, run Run) error {
	const query = `
		INSERT INTO runs (run_id, session_id, status, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET
			session_id = excluded.session_id,
			status = excluded.status,
			metadata_json = excluded.metadata_json,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, run.ID, run.SessionID, run.Status, run.MetadataJSON)
	return err
}

func (s *HistoryStore) LastRunSequence(ctx context.Context, runID string) (int, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(sequence), 0)
		FROM history_events
		WHERE run_id = ?
	`, runID)

	var sequence int
	if err := row.Scan(&sequence); err != nil {
		return 0, err
	}

	return sequence, nil
}

func (s *HistoryStore) AppendEvents(ctx context.Context, events []HistoryEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO history_events (session_id, run_id, sequence, kind, role, content, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, event := range events {
		if _, err := stmt.ExecContext(ctx,
			event.SessionID,
			event.RunID,
			event.Sequence,
			event.Kind,
			event.Role,
			event.Content,
			event.PayloadJSON,
		); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	return tx.Commit()
}

func (s *HistoryStore) ListEvents(ctx context.Context, sessionID string) ([]HistoryEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, run_id, sequence, kind, role, content, payload_json, created_at
		FROM history_events
		WHERE session_id = ?
		ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []HistoryEvent
	for rows.Next() {
		var event HistoryEvent
		if err := rows.Scan(
			&event.ID,
			&event.SessionID,
			&event.RunID,
			&event.Sequence,
			&event.Kind,
			&event.Role,
			&event.Content,
			&event.PayloadJSON,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, rows.Err()
}
