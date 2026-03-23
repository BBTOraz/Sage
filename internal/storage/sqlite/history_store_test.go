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

	for _, want := range []string{"checkpoints", "sessions", "runs", "history_events", "chat_turns"} {
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

func TestHistoryStoreLatestSessionReturnsNewestRow(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-1",
		MetadataJSON: `{"mode":"guard"}`,
	}); err != nil {
		t.Fatalf("UpsertSession(session-1) error = %v", err)
	}
	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-2",
		MetadataJSON: `{"mode":"auto"}`,
	}); err != nil {
		t.Fatalf("UpsertSession(session-2) error = %v", err)
	}

	session, err := store.LatestSession(context.Background())
	if err != nil {
		t.Fatalf("LatestSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("LatestSession() = nil, want latest session")
	}
	if session.ID != "session-2" {
		t.Fatalf("LatestSession().ID = %q, want %q", session.ID, "session-2")
	}
}

func TestHistoryStoreGetSessionReturnsRequestedRow(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-2",
		MetadataJSON: `{"title":"Incident review"}`,
	}); err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}

	session, err := store.GetSession(context.Background(), "session-2")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("GetSession() = nil, want session")
	}
	if session.ID != "session-2" {
		t.Fatalf("GetSession().ID = %q, want %q", session.ID, "session-2")
	}
	if session.MetadataJSON != `{"title":"Incident review"}` {
		t.Fatalf("GetSession().MetadataJSON = %q, want title metadata", session.MetadataJSON)
	}
}

func TestHistoryStoreListSessionsReturnsRecentPreviews(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-1",
		MetadataJSON: `{"title":"Planner migration"}`,
	}); err != nil {
		t.Fatalf("UpsertSession(session-1) error = %v", err)
	}
	if err := store.UpsertRun(context.Background(), Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Status:       "completed",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertRun(run-1) error = %v", err)
	}
	if err := store.AppendEvents(context.Background(), []HistoryEvent{
		{
			SessionID:   "session-1",
			RunID:       "run-1",
			Sequence:    1,
			Kind:        "assistant_chunk",
			Role:        "assistant",
			Content:     "Need to adapt executor context",
			PayloadJSON: `{}`,
		},
	}); err != nil {
		t.Fatalf("AppendEvents(session-1) error = %v", err)
	}

	if err := store.UpsertSession(context.Background(), Session{
		ID:           "session-2",
		MetadataJSON: `{"title":"Incident review"}`,
	}); err != nil {
		t.Fatalf("UpsertSession(session-2) error = %v", err)
	}
	if err := store.UpsertRun(context.Background(), Run{
		ID:           "run-2",
		SessionID:    "session-2",
		Status:       "completed",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertRun(run-2) error = %v", err)
	}
	if err := store.AppendEvents(context.Background(), []HistoryEvent{
		{
			SessionID:   "session-2",
			RunID:       "run-2",
			Sequence:    1,
			Kind:        "assistant_chunk",
			Role:        "assistant",
			Content:     "Investigating prod incident",
			PayloadJSON: `{}`,
		},
	}); err != nil {
		t.Fatalf("AppendEvents(session-2) error = %v", err)
	}

	summaries, err := store.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("ListSessions() len = %d, want %d", len(summaries), 2)
	}
	if summaries[0].ID != "session-2" {
		t.Fatalf("ListSessions()[0].ID = %q, want %q", summaries[0].ID, "session-2")
	}
	if summaries[0].Preview != "Investigating prod incident" {
		t.Fatalf("ListSessions()[0].Preview = %q, want %q", summaries[0].Preview, "Investigating prod incident")
	}
	if summaries[1].Preview != "Need to adapt executor context" {
		t.Fatalf("ListSessions()[1].Preview = %q, want %q", summaries[1].Preview, "Need to adapt executor context")
	}
}

func TestHistoryStoreCreateCompleteAndListChatTurns(t *testing.T) {
	db := openTestDB(t)
	store := NewHistoryStore(db)
	ctx := context.Background()

	if err := store.UpsertSession(ctx, Session{
		ID:           "session-1",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertSession() error = %v", err)
	}
	if err := store.UpsertRun(ctx, Run{
		ID:           "run-1",
		SessionID:    "session-1",
		Status:       "completed",
		MetadataJSON: `{}`,
	}); err != nil {
		t.Fatalf("UpsertRun() error = %v", err)
	}

	if err := store.CreateChatTurn(ctx, ChatTurn{
		SessionID: "session-1",
		RunID:     "run-1",
		UserInput: "first question",
	}); err != nil {
		t.Fatalf("CreateChatTurn() error = %v", err)
	}

	if err := store.CompleteChatTurn(ctx, "run-1", "first answer"); err != nil {
		t.Fatalf("CompleteChatTurn() error = %v", err)
	}

	turns, err := store.ListCompletedChatTurns(ctx, "session-1")
	if err != nil {
		t.Fatalf("ListCompletedChatTurns() error = %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("ListCompletedChatTurns() len = %d, want %d", len(turns), 1)
	}
	if turns[0].UserInput != "first question" {
		t.Fatalf("turns[0].UserInput = %q, want %q", turns[0].UserInput, "first question")
	}
	if turns[0].AssistantOutput != "first answer" {
		t.Fatalf("turns[0].AssistantOutput = %q, want %q", turns[0].AssistantOutput, "first answer")
	}
}
