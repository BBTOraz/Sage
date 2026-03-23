package app

import (
	"bilge-lib/core"
	"bilge-lib/internal/agent"
	"bilge-lib/internal/approval"
	"bilge-lib/internal/runtime"
	sqlitestore "bilge-lib/internal/storage/sqlite"
	"bilge-lib/internal/tui"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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
		"title":         snapshot.Title,
		"archived":      snapshot.Archived,
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

func (s *sqliteRuntimeHistoryStore) LastEventSequence(ctx context.Context, runID runtime.RunID) (int, error) {
	return s.store.LastRunSequence(ctx, string(runID))
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

func (s *sqliteRuntimeHistoryStore) CreateTranscriptTurn(ctx context.Context, turn runtime.SessionTranscriptTurn) error {
	return s.store.CreateChatTurn(ctx, sqlitestore.ChatTurn{
		SessionID: string(turn.SessionID),
		RunID:     string(turn.RunID),
		UserInput: turn.UserInput,
	})
}

func (s *sqliteRuntimeHistoryStore) CompleteTranscriptTurn(ctx context.Context, runID runtime.RunID, assistantOutput string) error {
	return s.store.CompleteChatTurn(ctx, string(runID), assistantOutput)
}

func (s *sqliteRuntimeHistoryStore) LatestSession(ctx context.Context) (runtime.SessionSnapshot, bool, error) {
	session, err := s.store.LatestSession(ctx)
	if err != nil || session == nil {
		return runtime.SessionSnapshot{}, false, err
	}

	return decodeSessionSnapshot(session)
}

func (s *sqliteRuntimeHistoryStore) LoadSession(ctx context.Context, sessionID runtime.SessionID) (runtime.SessionSnapshot, bool, error) {
	session, err := s.store.GetSession(ctx, string(sessionID))
	if err != nil || session == nil {
		return runtime.SessionSnapshot{}, false, err
	}

	return decodeSessionSnapshot(session)
}

func (s *sqliteRuntimeHistoryStore) ListSessions(ctx context.Context) ([]runtime.SessionSummary, error) {
	summaries, err := s.store.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]runtime.SessionSummary, 0, len(summaries))
	for _, summary := range summaries {
		metadata, err := decodeSessionMetadata(summary.MetadataJSON)
		if err != nil {
			return nil, err
		}

		out = append(out, runtime.SessionSummary{
			ID:          runtime.SessionID(summary.ID),
			Mode:        metadata.Mode,
			State:       metadata.State,
			ActiveRunID: metadata.ActiveRunID,
			Title:       metadata.Title,
			Preview:     summary.Preview,
			Archived:    metadata.Archived,
			UpdatedAt:   parseSQLiteTimestamp(summary.UpdatedAt),
		})
	}
	return out, nil
}

func decodeSessionSnapshot(session *sqlitestore.Session) (runtime.SessionSnapshot, bool, error) {
	metadata, err := decodeSessionMetadata(session.MetadataJSON)
	if err != nil {
		return runtime.SessionSnapshot{}, false, err
	}

	return runtime.SessionSnapshot{
		ID:          runtime.SessionID(session.ID),
		Mode:        metadata.Mode,
		State:       metadata.State,
		ActiveRunID: metadata.ActiveRunID,
		Title:       metadata.Title,
		Archived:    metadata.Archived,
	}, true, nil
}

func (s *sqliteRuntimeHistoryStore) ListEvents(ctx context.Context, sessionID runtime.SessionID) ([]runtime.HistoryEventRecord, error) {
	events, err := s.store.ListEvents(ctx, string(sessionID))
	if err != nil {
		return nil, err
	}

	records := make([]runtime.HistoryEventRecord, 0, len(events))
	for _, event := range events {
		records = append(records, runtime.HistoryEventRecord{
			SessionID:   runtime.SessionID(event.SessionID),
			RunID:       runtime.RunID(event.RunID),
			Sequence:    event.Sequence,
			Type:        runtime.EventType(event.Kind),
			Status:      decodeStatus(event.PayloadJSON),
			Text:        event.Content,
			PayloadJSON: event.PayloadJSON,
		})
	}
	return records, nil
}

func (s *sqliteRuntimeHistoryStore) ListTranscriptTurns(ctx context.Context, sessionID runtime.SessionID) ([]runtime.SessionTranscriptTurn, error) {
	turns, err := s.store.ListCompletedChatTurns(ctx, string(sessionID))
	if err != nil {
		return nil, err
	}

	out := make([]runtime.SessionTranscriptTurn, 0, len(turns))
	for _, turn := range turns {
		out = append(out, runtime.SessionTranscriptTurn{
			SessionID:       runtime.SessionID(turn.SessionID),
			RunID:           runtime.RunID(turn.RunID),
			UserInput:       turn.UserInput,
			AssistantOutput: turn.AssistantOutput,
		})
	}
	return out, nil
}

func eventRole(eventType runtime.EventType) string {
	switch eventType {
	case runtime.EventAssistantChunk, runtime.EventAssistantDone:
		return "assistant"
	default:
		return "system"
	}
}

func decodeStatus(payloadJSON string) runtime.RunStatus {
	if payloadJSON == "" {
		return ""
	}

	var payload struct {
		Status runtime.RunStatus `json:"status"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return ""
	}
	return payload.Status
}

type sessionMetadata struct {
	Mode        approval.Mode        `json:"mode"`
	State       runtime.SessionState `json:"state"`
	ActiveRunID runtime.RunID        `json:"active_run_id"`
	Title       string               `json:"title"`
	Archived    bool                 `json:"archived"`
}

func decodeSessionMetadata(raw string) (sessionMetadata, error) {
	var metadata sessionMetadata
	if raw == "" {
		return metadata, nil
	}
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		return sessionMetadata{}, err
	}
	return metadata, nil
}

func parseSQLiteTimestamp(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}

	parsed, err := time.ParseInLocation(time.DateTime, raw, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
