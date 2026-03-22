package app

import (
	"bilge-lib/core"
	"bilge-lib/internal/agent"
	"bilge-lib/internal/runtime"
	sqlitestore "bilge-lib/internal/storage/sqlite"
	"bilge-lib/internal/tui"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

func Run(ctx context.Context, cfg *Config) error {
	env, err := LoadEnvConfig()
	if err != nil {
		return err
	}

	db, err := openPersistence(ctx, env)
	if err != nil {
		return err
	}
	defer db.Close()

	checkPointStore := sqlitestore.NewCheckpointStore(db)
	historyStore := &sqliteRuntimeHistoryStore{store: sqlitestore.NewHistoryStore(db)}

	application, err := agent.NewApplication(ctx, agent.ApplicationConfig{
		Env:             env,
		ApprovalMode:    cfg.Mode,
		CheckPointStore: checkPointStore,
	})
	if err != nil {
		return err
	}
	runner, err := application.Runner(ctx)
	if err != nil {
		return err
	}

	manager := runtime.NewManager(cfg.Mode, runner, application, historyStore)
	return tui.Run(ctx, manager)
}

func openPersistence(ctx context.Context, env core.EnvConfig) (sqlitestore.DB, error) {
	dbPath, err := resolveStateDBPath(env.WorkspaceRoot)
	if err != nil {
		return nil, err
	}

	return sqlitestore.Open(ctx, sqlitestore.Config{
		Path: dbPath,
	})
}

func resolveStateDBPath(workspaceRoot string) (string, error) {
	root := workspaceRoot
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = cwd
	}

	return filepath.Join(root, ".sage", "state.db"), nil
}

type sqliteRuntimeHistoryStore struct {
	store *sqlitestore.HistoryStore
}

func (s *sqliteRuntimeHistoryStore) SaveSession(ctx context.Context, snapshot runtime.SessionSnapshot) error {
	metadata, err := json.Marshal(map[string]any{
		"mode":          snapshot.Mode,
		"state":         snapshot.State,
		"active_run_id": snapshot.ActiveRunID,
	})
	if err != nil {
		return err
	}

	return s.store.UpsertSession(ctx, sqlitestore.Session{
		ID:           string(snapshot.ID),
		MetadataJSON: string(metadata),
	})
}

func (s *sqliteRuntimeHistoryStore) SaveRun(ctx context.Context, snapshot runtime.RunSnapshot) error {
	metadata, err := json.Marshal(map[string]any{
		"started_at": snapshot.StartedAt,
		"updated_at": snapshot.UpdatedAt,
	})
	if err != nil {
		return err
	}

	return s.store.UpsertRun(ctx, sqlitestore.Run{
		ID:           string(snapshot.ID),
		SessionID:    string(snapshot.SessionID),
		Status:       string(snapshot.Status),
		MetadataJSON: string(metadata),
	})
}

func (s *sqliteRuntimeHistoryStore) AppendEvent(ctx context.Context, event runtime.HistoryEventRecord) error {
	return s.store.AppendEvents(ctx, []sqlitestore.HistoryEvent{
		{
			SessionID:   string(event.SessionID),
			RunID:       string(event.RunID),
			Sequence:    event.Sequence,
			Kind:        string(event.Type),
			Role:        eventRole(event.Type),
			Content:     event.Text,
			PayloadJSON: event.PayloadJSON,
		},
	})
}

func eventRole(eventType runtime.EventType) string {
	switch eventType {
	case runtime.EventAssistantChunk, runtime.EventAssistantDone:
		return "assistant"
	default:
		return "system"
	}
}
