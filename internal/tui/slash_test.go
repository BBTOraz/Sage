package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestResolveSlashCommand(t *testing.T) {
	cmd, arg, ok := resolveSlashCommand("/ingest C:\\Docs")
	if !ok {
		t.Fatal("resolveSlashCommand() ok = false, want true")
	}
	if cmd.Name != "ingest" {
		t.Fatalf("command = %q, want ingest", cmd.Name)
	}
	if arg != "C:\\Docs" {
		t.Fatalf("arg = %q, want %q", arg, "C:\\Docs")
	}
}

func TestAutocompleteSlashCommand(t *testing.T) {
	got, ok := autocompleteSlashCommand("/in")
	if !ok {
		t.Fatal("autocompleteSlashCommand() ok = false, want true")
	}
	if got != "/ingest " {
		t.Fatalf("autocompleteSlashCommand() = %q, want %q", got, "/ingest ")
	}
}

func TestAutocompleteSessionSlashCommand(t *testing.T) {
	got, ok := autocompleteSlashCommand("/se")
	if !ok {
		t.Fatal("autocompleteSlashCommand() ok = false, want true")
	}
	if got != "/session " {
		t.Fatalf("autocompleteSlashCommand() = %q, want %q", got, "/session ")
	}
}

func TestSlashOverlayVisibleAfterTypingSlash(t *testing.T) {
	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())

	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30}); cmd != nil {
		_ = cmd
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model, ok := updated.(*model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}
	if !model.slash.Active {
		t.Fatal("slash.Active = false, want true after typing '/'")
	}

	view := model.View().Content
	if !strings.Contains(view, "Commands") {
		t.Fatalf("view does not contain command overlay:\n%s", view)
	}
	if !strings.Contains(view, "enter send · / commands") {
		t.Fatalf("view does not contain help footer while slash overlay is active:\n%s", view)
	}
}

func TestSessionOverlayVisibleAfterTypingSessionCommand(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:    runtime.SessionID("session-1"),
		Mode:  "",
		State: runtime.SessionStateIdle,
		Title: "Planner migration",
	}
	store := &replayHistoryStoreStub{
		loadedSession: &session,
		sessionSummaries: []runtime.SessionSummary{
			{
				ID:        runtime.SessionID("session-1"),
				Title:     "Planner migration",
				Preview:   "Need to adapt executor context",
				State:     runtime.SessionStateIdle,
				UpdatedAt: mustTestTime(t, "2026-03-22T10:30:00Z"),
			},
		},
	}

	manager := runtime.NewManager("", nil, nil, store)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := expectModel(t, updated)

	model.area.SetValue("/session ")
	model.syncSessionSuggestFromArea()
	model.refreshViewport()

	view := stripANSITest(model.View().Content)
	if !strings.Contains(view, "Sessions") {
		t.Fatalf("view does not contain session overlay:\n%s", view)
	}
	if !strings.Contains(view, "New chat") {
		t.Fatalf("view does not contain new chat action:\n%s", view)
	}
	if !strings.Contains(view, "Planner migration") {
		t.Fatalf("view does not contain stored session summary:\n%s", view)
	}
}

func TestSessionSelectionLoadsRequestedTranscript(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:          runtime.SessionID("session-1"),
		Mode:        "",
		State:       runtime.SessionStateIdle,
		ActiveRunID: runtime.RunID("run-1"),
		Title:       "Planner migration",
	}
	store := &replayHistoryStoreStub{
		loadedSession: &session,
		sessionSummaries: []runtime.SessionSummary{
			{
				ID:        runtime.SessionID("session-1"),
				Title:     "Planner migration",
				Preview:   "Need to adapt executor context",
				State:     runtime.SessionStateIdle,
				UpdatedAt: mustTestTime(t, "2026-03-22T10:30:00Z"),
			},
		},
		events: []runtime.HistoryEventRecord{
			mustHistoryRecord(t, session.ID, runtime.Event{
				RunID:  runtime.RunID("run-1"),
				Status: runtime.RunStatusRunning,
				Type:   runtime.EventAssistantChunk,
				Text:   "Need to adapt executor context",
			}, 1),
		},
	}

	manager := runtime.NewManager("", nil, nil, store)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := expectModel(t, updated)

	model.area.SetValue("/session ")
	model.syncSessionSuggestFromArea()
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = expectModel(t, updated)

	if model.sessionID != session.ID {
		t.Fatalf("sessionID = %q, want %q", model.sessionID, session.ID)
	}
	if len(model.messages) != 1 || model.messages[0].RunID != runtime.RunID("run-1") {
		t.Fatalf("messages = %#v, want replayed requested session", model.messages)
	}
}

func TestHandleSendSessionCommandOpensSessionPicker(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:    runtime.SessionID("session-1"),
		Mode:  "",
		State: runtime.SessionStateIdle,
		Title: "Planner migration",
	}
	store := &replayHistoryStoreStub{
		loadedSession: &session,
		sessionSummaries: []runtime.SessionSummary{
			{
				ID:        runtime.SessionID("session-1"),
				Title:     "Planner migration",
				Preview:   "Need to adapt executor context",
				State:     runtime.SessionStateIdle,
				UpdatedAt: mustTestTime(t, "2026-03-22T10:30:00Z"),
			},
		},
	}

	manager := runtime.NewManager("", nil, nil, store)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := expectModel(t, updated)

	model.area.SetValue("/session")
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = expectModel(t, updated)

	if !model.sessionSuggest.active {
		t.Fatal("sessionSuggest.active = false, want true after submitting /session")
	}
	if got := model.area.Value(); got != "/session " {
		t.Fatalf("textarea value = %q, want %q", got, "/session ")
	}

	view := stripANSITest(model.View().Content)
	if !strings.Contains(view, "Sessions") {
		t.Fatalf("view does not contain session overlay after /session submit:\n%s", view)
	}
	if strings.Contains(view, "unknown slash command") {
		t.Fatalf("view still shows unknown slash command after /session submit:\n%s", view)
	}
}

func TestEnterOnSlashOverlaySelectsHighlightedCommand(t *testing.T) {
	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := expectModel(t, updated)

	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model = expectModel(t, updated)
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = expectModel(t, updated)

	if got := model.area.Value(); got != "/ingest " {
		t.Fatalf("textarea value = %q, want first slash command to be inserted on enter", got)
	}
	if !model.dirSuggest.active {
		t.Fatal("dirSuggest.active = false, want /ingest selection flow after enter")
	}

	view := stripANSITest(model.View().Content)
	if strings.Contains(view, "unknown slash command") {
		t.Fatalf("view still shows unknown slash command after slash enter selection:\n%s", view)
	}
}

func TestEnterOnSlashOverlayWithSessionQueryOpensSessionPicker(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:    runtime.SessionID("session-1"),
		Mode:  "",
		State: runtime.SessionStateIdle,
		Title: "Planner migration",
	}
	store := &replayHistoryStoreStub{
		loadedSession: &session,
		sessionSummaries: []runtime.SessionSummary{
			{
				ID:        runtime.SessionID("session-1"),
				Title:     "Planner migration",
				Preview:   "Need to adapt executor context",
				State:     runtime.SessionStateIdle,
				UpdatedAt: mustTestTime(t, "2026-03-22T10:30:00Z"),
			},
		},
	}

	manager := runtime.NewManager("", nil, nil, store)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model := expectModel(t, updated)

	model.area.SetValue("/s")
	model.updateSlashState()
	updated, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = expectModel(t, updated)

	if got := model.area.Value(); got != "/session " {
		t.Fatalf("textarea value = %q, want session slash command selected on enter", got)
	}
	if !model.sessionSuggest.active {
		t.Fatal("sessionSuggest.active = false, want session picker after enter")
	}
}

func mustTestTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return parsed
}
