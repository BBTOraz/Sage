package sqlite

import (
	"context"
	"database/sql"
	"errors"
)

type CheckpointStore struct {
	db DB
}

func NewCheckpointStore(db DB) *CheckpointStore {
	return &CheckpointStore{db: db}
}

func (s *CheckpointStore) Get(ctx context.Context, checkpointID string) ([]byte, bool, error) {
	const query = `SELECT payload FROM checkpoints WHERE checkpoint_id = ?`

	var payload []byte
	err := s.db.QueryRowContext(ctx, query, checkpointID).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return payload, true, nil
}

func (s *CheckpointStore) Set(ctx context.Context, checkpointID string, payload []byte) error {
	const query = `
		INSERT INTO checkpoints (checkpoint_id, payload, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(checkpoint_id) DO UPDATE SET
			payload = excluded.payload,
			updated_at = CURRENT_TIMESTAMP
	`

	_, err := s.db.ExecContext(ctx, query, checkpointID, payload)
	return err
}
