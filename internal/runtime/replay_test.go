package runtime

import (
	"bilge-lib/internal/approval"
	"context"
	"testing"
)

func TestRestoreLatestSessionReplaysStructuredHistory(t *testing.T) {
	history := &stubHistoryStore{}
	manager := NewManager(approval.Guard, nil, nil, history)

	latest := SessionSnapshot{
		ID:          SessionID("session-prev"),
		Mode:        approval.Guard,
		State:       SessionStateIdle,
		ActiveRunID: RunID("run-7"),
	}
	history.latestSession = &latest

	chunkRecord, err := newHistoryEventRecord(latest.ID, 1, Event{
		RunID:  RunID("run-7"),
		Status: RunStatusRunning,
		Type:   EventAssistantChunk,
		Text:   "Ищу падение тестов.",
		Payload: &EventPayload{
			AgentName: "sage-doc",
			RunPath:   []string{"planner", "sage-doc"},
			Role:      "assistant",
			ToolCalls: []ToolCallPayload{
				{
					ID:        "call-1",
					Name:      "read_file",
					Arguments: `{"path":"auth/handler_test.go"}`,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("newHistoryEventRecord(chunk) error = %v", err)
	}

	interruptRecord, err := newHistoryEventRecord(latest.ID, 2, Event{
		RunID:  RunID("run-7"),
		Status: RunStatusInterrupted,
		Type:   EventRunInterrupted,
		Approval: &PendingApproval{
			RunID:     RunID("run-7"),
			ToolName:  "write_file",
			Arguments: `{"path":"auth/handler_test.go"}`,
		},
	})
	if err != nil {
		t.Fatalf("newHistoryEventRecord(interrupt) error = %v", err)
	}

	history.events = []HistoryEventRecord{chunkRecord, interruptRecord}

	session, events, err := manager.RestoreLatestSession(context.Background())
	if err != nil {
		t.Fatalf("RestoreLatestSession() error = %v", err)
	}
	if session.ID != latest.ID {
		t.Fatalf("restored session id = %q, want %q", session.ID, latest.ID)
	}
	if manager.GetSession().ID != latest.ID {
		t.Fatalf("manager session id = %q, want %q", manager.GetSession().ID, latest.ID)
	}
	if len(events) != 2 {
		t.Fatalf("replayed events len = %d, want %d", len(events), 2)
	}
	if events[0].Payload == nil || events[0].Payload.AgentName != "sage-doc" {
		t.Fatalf("replayed chunk payload = %#v, want agent sage-doc", events[0].Payload)
	}
	if events[0].Payload == nil || len(events[0].Payload.ToolCalls) != 1 || events[0].Payload.ToolCalls[0].Name != "read_file" {
		t.Fatalf("replayed tool calls = %#v, want read_file", events[0].Payload)
	}
	if events[1].Approval == nil || events[1].Approval.ToolName != "write_file" {
		t.Fatalf("replayed approval = %#v, want write_file approval", events[1].Approval)
	}
}
