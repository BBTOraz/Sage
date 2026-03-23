package tui

import (
	"bilge-lib/internal/approval"
	"bilge-lib/internal/runtime"
	"context"
	"encoding/json"
	"testing"
)

func TestNewModelStartsFreshWithoutReplayingLatestSession(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:          runtime.SessionID("session-replay"),
		Mode:        approval.Guard,
		State:       runtime.SessionStateIdle,
		ActiveRunID: runtime.RunID("run-11"),
	}

	store := &replayHistoryStoreStub{
		latestSession: &session,
		events: []runtime.HistoryEventRecord{
			mustHistoryRecord(t, session.ID, runtime.Event{
				RunID:  runtime.RunID("run-11"),
				Status: runtime.RunStatusRunning,
				Type:   runtime.EventAssistantChunk,
				Text:   "Нахожу проблему в auth handler.",
				Payload: &runtime.EventPayload{
					AgentName: "sage-doc",
					RunPath:   []string{"planner", "sage-doc"},
					Role:      "assistant",
					ToolCalls: []runtime.ToolCallPayload{
						{
							ID:        "call-7",
							Name:      "read_file",
							Arguments: `{"path":"auth/handler_test.go"}`,
						},
					},
				},
			}, 1),
			mustHistoryRecord(t, session.ID, runtime.Event{
				RunID:  runtime.RunID("run-11"),
				Status: runtime.RunStatusRunning,
				Type:   runtime.EventAssistantChunk,
				Payload: &runtime.EventPayload{
					AgentName: "sage-doc",
					RunPath:   []string{"planner", "sage-doc"},
					Role:      "tool",
					ToolResult: &runtime.ToolResultPayload{
						ToolCallID: "call-7",
						ToolName:   "read_file",
						Content:    "package auth",
					},
				},
			}, 2),
			mustHistoryRecord(t, session.ID, runtime.Event{
				RunID:  runtime.RunID("run-11"),
				Status: runtime.RunStatusCompleted,
				Type:   runtime.EventRunCompleted,
			}, 3),
		},
	}

	manager := runtime.NewManager(approval.Guard, nil, nil, store)
	freshSession := manager.GetSession()
	ui := newModel(manager, context.Background())

	if ui.sessionID != freshSession.ID {
		t.Fatalf("sessionID = %q, want fresh session %q", ui.sessionID, freshSession.ID)
	}
	if len(ui.messages) != 0 {
		t.Fatalf("messages = %#v, want empty fresh chat", ui.messages)
	}
	if ui.pendingApproval != nil {
		t.Fatalf("pendingApproval = %#v, want nil on fresh start", ui.pendingApproval)
	}
}

func TestModelLoadSessionReplaysStructuredTranscript(t *testing.T) {
	session := runtime.SessionSnapshot{
		ID:          runtime.SessionID("session-replay"),
		Mode:        approval.Guard,
		State:       runtime.SessionStateIdle,
		ActiveRunID: runtime.RunID("run-11"),
	}

	events := []runtime.Event{
		{
			RunID:  runtime.RunID("run-11"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "Нахожу проблему в auth handler.",
			Payload: &runtime.EventPayload{
				AgentName: "sage-doc",
				RunPath:   []string{"planner", "sage-doc"},
				Role:      "assistant",
				ToolCalls: []runtime.ToolCallPayload{
					{
						ID:        "call-7",
						Name:      "read_file",
						Arguments: `{"path":"auth/handler_test.go"}`,
					},
				},
			},
		},
		{
			RunID:  runtime.RunID("run-11"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Payload: &runtime.EventPayload{
				AgentName: "sage-doc",
				RunPath:   []string{"planner", "sage-doc"},
				Role:      "tool",
				ToolResult: &runtime.ToolResultPayload{
					ToolCallID: "call-7",
					ToolName:   "read_file",
					Content:    "package auth",
				},
			},
		},
		{
			RunID:  runtime.RunID("run-11"),
			Status: runtime.RunStatusCompleted,
			Type:   runtime.EventRunCompleted,
		},
	}

	manager := runtime.NewManager(approval.Guard, nil, nil)
	ui := newModel(manager, context.Background())
	ui.loadSession(session, events)

	if len(ui.messages) != 1 || ui.messages[0].Kind != MessageRun || ui.messages[0].RunID != runtime.RunID("run-11") {
		t.Fatalf("messages = %#v, want one replayed run anchor", ui.messages)
	}

	tool := ui.transcript.nodes["tool:run-11:call-7"]
	if tool == nil {
		t.Fatal("tool node = nil, want replayed tool node")
	}
	if tool.Result != "package auth" {
		t.Fatalf("tool.Result = %q, want %q", tool.Result, "package auth")
	}

	agent := ui.transcript.nodeForID(ui.transcript.lastAgentByRun[runtime.RunID("run-11")])
	if agent == nil || agent.Status != string(runtime.RunStatusCompleted) {
		t.Fatalf("agent node = %#v, want completed replayed agent", agent)
	}
}

type replayHistoryStoreStub struct {
	latestSession    *runtime.SessionSnapshot
	loadedSession    *runtime.SessionSnapshot
	sessionSummaries []runtime.SessionSummary
	events           []runtime.HistoryEventRecord
}

func (s *replayHistoryStoreStub) SaveSession(context.Context, runtime.SessionSnapshot) error {
	return nil
}
func (s *replayHistoryStoreStub) SaveRun(context.Context, runtime.RunSnapshot) error { return nil }
func (s *replayHistoryStoreStub) LastEventSequence(context.Context, runtime.RunID) (int, error) {
	return 0, nil
}
func (s *replayHistoryStoreStub) AppendEvent(context.Context, runtime.HistoryEventRecord) error {
	return nil
}
func (s *replayHistoryStoreStub) CreateTranscriptTurn(context.Context, runtime.SessionTranscriptTurn) error {
	return nil
}
func (s *replayHistoryStoreStub) CompleteTranscriptTurn(context.Context, runtime.RunID, string) error {
	return nil
}
func (s *replayHistoryStoreStub) LatestSession(context.Context) (runtime.SessionSnapshot, bool, error) {
	if s.latestSession == nil {
		return runtime.SessionSnapshot{}, false, nil
	}
	return *s.latestSession, true, nil
}
func (s *replayHistoryStoreStub) LoadSession(_ context.Context, sessionID runtime.SessionID) (runtime.SessionSnapshot, bool, error) {
	if s.loadedSession == nil || s.loadedSession.ID != sessionID {
		return runtime.SessionSnapshot{}, false, nil
	}
	return *s.loadedSession, true, nil
}
func (s *replayHistoryStoreStub) ListSessions(context.Context) ([]runtime.SessionSummary, error) {
	return s.sessionSummaries, nil
}
func (s *replayHistoryStoreStub) ListEvents(context.Context, runtime.SessionID) ([]runtime.HistoryEventRecord, error) {
	return s.events, nil
}
func (s *replayHistoryStoreStub) ListTranscriptTurns(context.Context, runtime.SessionID) ([]runtime.SessionTranscriptTurn, error) {
	return nil, nil
}

func mustHistoryRecord(t *testing.T, sessionID runtime.SessionID, event runtime.Event, sequence int) runtime.HistoryEventRecord {
	t.Helper()

	payload, err := json.Marshal(struct {
		Status   runtime.RunStatus        `json:"status"`
		Text     string                   `json:"text,omitempty"`
		Error    string                   `json:"error,omitempty"`
		Approval *runtime.PendingApproval `json:"approval,omitempty"`
		Payload  *runtime.EventPayload    `json:"payload,omitempty"`
	}{
		Status:   event.Status,
		Text:     event.Text,
		Approval: event.Approval,
		Payload:  event.Payload,
	})
	if err != nil {
		t.Fatalf("json.Marshal(history payload) error = %v", err)
	}

	return runtime.HistoryEventRecord{
		SessionID:   sessionID,
		RunID:       event.RunID,
		Sequence:    sequence,
		Type:        event.Type,
		Status:      event.Status,
		Text:        event.Text,
		PayloadJSON: string(payload),
	}
}
