package tui

import (
	"bilge-lib/internal/runtime"
	"regexp"
	"strings"
	"testing"
)

func TestRenderTranscriptTreeRendersNestedToolBlockBelowAgent(t *testing.T) {
	tree := newTranscriptTree()
	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-1"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Text:   "Запускаю тесты и собираю падения.",
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{
				{
					ID:        "call-1",
					Name:      "bash",
					Arguments: `{"cmd":"go test ./..."}`,
				},
			},
		},
	})

	output := stripANSITest(renderTranscript(
		[]Message{
			{Kind: MessageUser, Text: "почини падения тестов"},
			{Kind: MessageRun, RunID: runtime.RunID("run-1")},
		},
		tree,
		"",
		nil,
		approvalToggle{},
		"",
		100,
	))

	if !strings.Contains(output, "A Sage") {
		t.Fatalf("output does not contain agent marker row:\n%s", output)
	}
	if !strings.Contains(output, "Запускаю тесты и собираю падения.") {
		t.Fatalf("output does not contain agent body:\n%s", output)
	}
	if !strings.Contains(output, "bash") {
		t.Fatalf("output does not contain tool name:\n%s", output)
	}
	if !strings.Contains(output, "└─") && !strings.Contains(output, "├─") {
		t.Fatalf("output does not contain tree connector:\n%s", output)
	}

	agentIndex := strings.Index(output, "A Sage")
	toolIndex := strings.Index(output, "bash")
	if agentIndex == -1 || toolIndex == -1 || toolIndex <= agentIndex {
		t.Fatalf("tool block is not rendered after the owning agent:\n%s", output)
	}
}

func TestRenderTranscriptTreeKeepsToolCollapsedByDefaultAndExpandable(t *testing.T) {
	tree := newTranscriptTree()
	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-2"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Text:   "Ищу причины падения.",
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"planner", "sage"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{
				{
					ID:        "call-2",
					Name:      "search_docs",
					Arguments: `{"query":"auth handler"}`,
				},
			},
		},
	})
	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-2"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"planner", "sage"},
			Role:      "tool",
			ToolResult: &runtime.ToolResultPayload{
				ToolCallID: "call-2",
				ToolName:   "search_docs",
				Content:    "2 hits",
			},
		},
	})

	collapsed := stripANSITest(renderTranscript(
		[]Message{{Kind: MessageRun, RunID: runtime.RunID("run-2")}},
		tree,
		"",
		nil,
		approvalToggle{},
		"",
		100,
	))

	if !strings.Contains(collapsed, "press enter to expand") {
		t.Fatalf("collapsed output missing expand hint:\n%s", collapsed)
	}
	if strings.Contains(collapsed, "2 hits") {
		t.Fatalf("collapsed output unexpectedly contains tool result:\n%s", collapsed)
	}

	tree.nodes["tool:run-2:call-2"].Expanded = true

	expanded := stripANSITest(renderTranscript(
		[]Message{{Kind: MessageRun, RunID: runtime.RunID("run-2")}},
		tree,
		"",
		nil,
		approvalToggle{},
		"",
		100,
	))

	if !strings.Contains(expanded, "\"query\": \"auth handler\"") {
		t.Fatalf("expanded output missing formatted tool args:\n%s", expanded)
	}
	if !strings.Contains(expanded, "2 hits") {
		t.Fatalf("expanded output missing tool result:\n%s", expanded)
	}
}

func TestRenderTranscriptTreeHidesInternalAgentsAndKeepsVisibleSage(t *testing.T) {
	tree := newTranscriptTree()
	tree.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-3"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Text:   "Готовлю исправление.",
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "assistant",
		},
	})

	output := stripANSITest(renderTranscript(
		[]Message{{Kind: MessageRun, RunID: runtime.RunID("run-3")}},
		tree,
		"",
		nil,
		approvalToggle{},
		"",
		100,
	))

	if !strings.Contains(output, "A Sage") {
		t.Fatalf("output missing visible sage row:\n%s", output)
	}
	for _, hidden := range []string{"plan_execute_replan", "execute_replan", "planner", "sage-doc"} {
		if strings.Contains(output, hidden) {
			t.Fatalf("output still renders hidden internal agent %q:\n%s", hidden, output)
		}
	}
}

func TestRenderTranscriptShowsPendingApprovalWithoutAgentRoots(t *testing.T) {
	output := stripANSITest(renderTranscript(
		[]Message{{Kind: MessageRun, RunID: runtime.RunID("run-pending")}},
		newTranscriptTree(),
		"",
		&runtime.PendingApproval{
			RunID:     runtime.RunID("run-pending"),
			ToolName:  "write_file",
			Arguments: `{"path":"auth/handler_test.go"}`,
		},
		approvalToggle{},
		"",
		100,
	))

	if !strings.Contains(output, "write_file") {
		t.Fatalf("output missing pending tool name:\n%s", output)
	}
	if !strings.Contains(output, "Allow this tool call?") {
		t.Fatalf("output missing approval prompt:\n%s", output)
	}
}

func TestRenderTranscriptHidesInternalPlanExecuteAgentsAndPlannerJSON(t *testing.T) {
	tree := newTranscriptTree()
	runID := runtime.RunID("run-dialogue")

	tree.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "planner",
			RunPath:   []string{"plan_execute_replan", "planner"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "plan-1",
				Name:      "plan",
				Arguments: `{"steps":["Поздороваться с пользователем"]}`,
			}},
		},
	})
	tree.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Text:   "Привет! Чем могу помочь?",
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "assistant",
		},
	})
	tree.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "replanner",
			RunPath:   []string{"plan_execute_replan", "replanner"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "respond-1",
				Name:      "respond",
				Arguments: `{"response":"Привет! Чем могу помочь?"}`,
			}},
		},
	})

	output := stripANSITest(renderTranscript(
		[]Message{{Kind: MessageRun, RunID: runID}},
		tree,
		"",
		nil,
		approvalToggle{},
		"",
		100,
	))

	for _, hidden := range []string{
		"plan_execute_replan",
		"planner",
		"execute_replan",
		"replanner",
		`"steps"`,
		`"response"`,
	} {
		if strings.Contains(output, hidden) {
			t.Fatalf("output still leaks internal planning artifact %q:\n%s", hidden, output)
		}
	}
	if !strings.Contains(output, "A Sage") {
		t.Fatalf("output missing visible user-facing sage turn:\n%s", output)
	}
	if !strings.Contains(output, "Привет! Чем могу помочь?") {
		t.Fatalf("output missing sage answer:\n%s", output)
	}
}

func stripANSITest(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
