package tui

import (
	"bilge-lib/internal/runtime"
	"testing"
)

func TestTranscriptTreeCreatesAgentBranchFromPayload(t *testing.T) {
	tree := newTranscriptTree()

	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-1"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage-doc",
			RunPath:   []string{"planner", "sage", "sage-doc"},
			Role:      "assistant",
		},
	})

	if len(tree.roots) != 1 {
		t.Fatalf("len(tree.roots) = %d, want %d", len(tree.roots), 1)
	}

	planner := tree.nodes[tree.roots[0]]
	if planner == nil || planner.Kind != transcriptNodeAgent {
		t.Fatalf("planner node = %#v, want top-level agent node", planner)
	}
	if planner.AgentName != "planner" {
		t.Fatalf("planner.AgentName = %q, want %q", planner.AgentName, "planner")
	}

	sage := tree.nodes[planner.Children[0]]
	if sage == nil || sage.AgentName != "sage" {
		t.Fatalf("sage node = %#v, want agent sage", sage)
	}

	doc := tree.nodes[sage.Children[0]]
	if doc == nil || doc.AgentName != "sage-doc" {
		t.Fatalf("doc node = %#v, want agent sage-doc", doc)
	}
	if doc.Status != string(runtime.RunStatusRunning) {
		t.Fatalf("doc.Status = %q, want %q", doc.Status, runtime.RunStatusRunning)
	}
}

func TestTranscriptTreeAttachesToolCallAndUpdatesResult(t *testing.T) {
	tree := newTranscriptTree()

	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-7"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage-doc",
			RunPath:   []string{"planner", "sage-doc"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{
				{
					ID:        "call-1",
					Name:      "search_docs",
					Arguments: `{"query":"billing"}`,
				},
			},
		},
	})

	tool := tree.nodes["tool:run-7:call-1"]
	if tool == nil {
		t.Fatal("tool node = nil, want tool child node")
	}
	if tool.Kind != transcriptNodeTool {
		t.Fatalf("tool.Kind = %q, want %q", tool.Kind, transcriptNodeTool)
	}
	if tool.Title != "search_docs" {
		t.Fatalf("tool.Title = %q, want %q", tool.Title, "search_docs")
	}
	if tool.Summary != `{"query":"billing"}` {
		t.Fatalf("tool.Summary = %q, want tool args", tool.Summary)
	}

	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-7"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage-doc",
			RunPath:   []string{"planner", "sage-doc"},
			Role:      "tool",
			ToolResult: &runtime.ToolResultPayload{
				ToolCallID: "call-1",
				ToolName:   "search_docs",
				Content:    "2 hits",
			},
		},
	})

	tool = tree.nodes["tool:run-7:call-1"]
	if tool.Status != string(runtime.RunStatusCompleted) {
		t.Fatalf("tool.Status = %q, want %q", tool.Status, runtime.RunStatusCompleted)
	}
	if tool.Result != "2 hits" {
		t.Fatalf("tool.Result = %q, want %q", tool.Result, "2 hits")
	}
}

func TestTranscriptTreeBindsPendingApprovalToExistingToolNode(t *testing.T) {
	tree := newTranscriptTree()

	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-9"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"planner", "sage"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{
				{
					ID:        "call-7",
					Name:      "write_file",
					Arguments: `{"path":"auth/handler_test.go"}`,
				},
			},
		},
	})

	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-9"),
		Status: runtime.RunStatusInterrupted,
		Type:   runtime.EventRunInterrupted,
		Approval: &runtime.PendingApproval{
			RunID:     runtime.RunID("run-9"),
			ToolName:  "write_file",
			Arguments: `{"path":"auth/handler_test.go"}`,
		},
	})

	tool := tree.nodes["tool:run-9:call-7"]
	if tool == nil {
		t.Fatal("tool node = nil, want existing tool node")
	}
	if tool.Status != string(runtime.RunStatusInterrupted) {
		t.Fatalf("tool.Status = %q, want %q", tool.Status, runtime.RunStatusInterrupted)
	}
	if tool.Approval == nil {
		t.Fatal("tool.Approval = nil, want pending approval attached")
	}
}
