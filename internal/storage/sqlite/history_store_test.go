package sqlite

import (
	"context"
	"slices"
	"testing"
)

func TestOpenInitializesSchemaOnEmptyDB(t *testing.T) {
	db := openTestDB(t)

	tables, err := listTables(context.Background(), db)
	if err != nil {
		t.Fatalf("listTables() error = %v", err)
	}

	for _, want := range []string{"checkpoints", "sessions", "runs", "history_events"} {
		if !slices.Contains(tables, want) {
			t.Fatalf("tables = %v, want %q", tables, want)
		}
	}
}

func TestHistoryStoreAppendAndListSessionEvents(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-1",
		MetadataJSON: `{"topic":"agent-migration"}`,
	}); err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}

	if err := store.UpsertRun(context.Background(), Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Status:       "running",
		MetadataJSON: `{"mode":"chat"}`,
	}); err != nil {
		t.Fatalf("UpsertRun() error = %v", err)
	}

	err := store.AppendEvents(context.Background(), []HistoryEvent{
		{
			SessionID:   "session-1",
			RunID:       "run-1",
			Sequence:    1,
			Kind:        "message",
			Role:        "user",
			Content:     "plan the migration",
			PayloadJSON: `{"source":"user"}`,
		},
		{
			SessionID:   "session-1",
			RunID:       "run-1",
			Sequence:    2,
			Kind:        "message",
			Role:        "assistant",
			Content:     "working on it",
			PayloadJSON: `{"source":"assistant"}`,
		},
	})
	if err != nil {
		t.Fatalf("AppendEvents() error = %v", err)
	}

	got, err := store.ListEvents(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListEvents() len = %d, want %d", len(got), 2)
	}
	if got[0].Sequence != 1 || got[0].Content != "plan the migration" {
		t.Fatalf("ListEvents()[0] = %+v, want sequence 1 user content", got[0])
	}
	if got[1].Sequence != 2 || got[1].Content != "working on it" {
		t.Fatalf("ListEvents()[1] = %+v, want sequence 2 assistant content", got[1])
	}
}

func TestHistoryStoreLastRunSequenceReturnsStoredMax(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-1",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}

	if err := store.UpsertRun(context.Background(), Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Status:       "running",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertRun() error = %v", err)
	}

	if err := store.AppendEvents(context.Background(), []HistoryEvent{
		{SessionID: "session-1", RunID: "run-1", Sequence: 1, Kind: "event", PayloadJSON: `{}`},
		{SessionID: "session-1", RunID: "run-1", Sequence: 4, Kind: "event", PayloadJSON: `{}`},
		{SessionID: "session-1", RunID: "run-1", Sequence: 3, Kind: "event", PayloadJSON: `{}`},
	}); err != nil {
		t.Fatalf("AppendEvents() error = %v", err)
	}

	got, err := store.LastRunSequence(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("LastRunSequence() error = %v", err)
	}
	if got != 4 {
		t.Fatalf("LastRunSequence() = %d, want %d", got, 4)
	}
}
