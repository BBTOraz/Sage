package sqlite

import (
	"context"
	"database/sql"
)

type Session struct {
	ID           string
	MetadataJSON string
}

type SessionSummary struct {
	ID           string
	MetadataJSON string
	UpdatedAt    string
	Preview      string
}

type Run struct {
	ID           string
	SessionID    string
	Status       string
	MetadataJSON string
}

type ChatTurn struct {
	ID              int64
	SessionID       string
	RunID           string
	UserInput       string
	AssistantOutput string
	Completed       bool
	CreatedAt       string
	UpdatedAt       string
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

func (s *HistoryStore) LatestSession(ctx context.Context) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT session_id, metadata_json
		FROM sessions
		ORDER BY updated_at DESC, created_at DESC, rowid DESC
		LIMIT 1
	`)

	var session Session
	if err := row.Scan(&session.ID, &session.MetadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
}

func (s *HistoryStore) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT session_id, metadata_json
		FROM sessions
		WHERE session_id = ?
	`, sessionID)

	var session Session
	if err := row.Scan(&session.ID, &session.MetadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &session, nil
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

func (s *HistoryStore) CreateChatTurn(ctx context.Context, turn ChatTurn) error {
	const query = `
		INSERT INTO chat_turns (session_id, run_id, user_input, assistant_output, completed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(run_id) DO UPDATE SET
			user_input = excluded.user_input,
			updated_at = CURRENT_TIMESTAMP
	`

	completed := 0
	if turn.Completed {
		completed = 1
	}

	_, err := s.db.ExecContext(ctx, query, turn.SessionID, turn.RunID, turn.UserInput, turn.AssistantOutput, completed)
	return err
}

func (s *HistoryStore) CompleteChatTurn(ctx context.Context, runID string, assistantOutput string) error {
	const query = `
		UPDATE chat_turns
		SET assistant_output = ?, completed = 1, updated_at = CURRENT_TIMESTAMP
		WHERE run_id = ?
	`

	_, err := s.db.ExecContext(ctx, query, assistantOutput, runID)
	return err
}

func (s *HistoryStore) ListCompletedChatTurns(ctx context.Context, sessionID string) ([]ChatTurn, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, run_id, user_input, assistant_output, completed, created_at, updated_at
		FROM chat_turns
		WHERE session_id = ? AND completed = 1
		ORDER BY id ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var turns []ChatTurn
	for rows.Next() {
		var turn ChatTurn
		var completed int
		if err := rows.Scan(
			&turn.ID,
			&turn.SessionID,
			&turn.RunID,
			&turn.UserInput,
			&turn.AssistantOutput,
			&completed,
			&turn.CreatedAt,
			&turn.UpdatedAt,
		); err != nil {
			return nil, err
		}
		turn.Completed = completed != 0
		turns = append(turns, turn)
	}

	return turns, rows.Err()
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

func (s *HistoryStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			s.session_id,
			s.metadata_json,
			s.updated_at,
			COALESCE((
				SELECT he.content
				FROM history_events he
				WHERE he.session_id = s.session_id
					AND he.content <> ''
				ORDER BY he.id DESC
				LIMIT 1
			), '')
		FROM sessions s
		ORDER BY s.updated_at DESC, s.created_at DESC, s.rowid DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []SessionSummary
	for rows.Next() {
		var summary SessionSummary
		if err := rows.Scan(
			&summary.ID,
			&summary.MetadataJSON,
			&summary.UpdatedAt,
			&summary.Preview,
		); err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, rows.Err()
}
