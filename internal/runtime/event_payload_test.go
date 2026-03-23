package runtime

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestBuildEventPayloadCapturesAgentAndToolCalls(t *testing.T) {
	msg := schema.AssistantMessage("", []schema.ToolCall{
		{
			ID: "call-1",
			Function: schema.FunctionCall{
				Name:      "search_docs",
				Arguments: `{"query":"billing"}`,
			},
		},
	})

	event := adk.EventFromMessage(msg, nil, schema.Assistant, "")
	event.AgentName = "sage-doc"
	event.RunPath = mustRunSteps(t, "sage", "sage-doc")

	payload := buildEventPayload(event, event.Output.MessageOutput, msg)
	if payload == nil {
		t.Fatal("buildEventPayload() = nil, want payload")
	}
	if payload.AgentName != "sage-doc" {
		t.Fatalf("payload.AgentName = %q, want %q", payload.AgentName, "sage-doc")
	}
	if len(payload.RunPath) != 2 || payload.RunPath[0] != "sage" || payload.RunPath[1] != "sage-doc" {
		t.Fatalf("payload.RunPath = %#v, want [sage sage-doc]", payload.RunPath)
	}
	if payload.Role != string(schema.Assistant) {
		t.Fatalf("payload.Role = %q, want %q", payload.Role, string(schema.Assistant))
	}
	if len(payload.ToolCalls) != 1 {
		t.Fatalf("len(payload.ToolCalls) = %d, want %d", len(payload.ToolCalls), 1)
	}
	if payload.ToolCalls[0].Name != "search_docs" {
		t.Fatalf("payload.ToolCalls[0].Name = %q, want %q", payload.ToolCalls[0].Name, "search_docs")
	}
	if payload.ToolCalls[0].Arguments != `{"query":"billing"}` {
		t.Fatalf("payload.ToolCalls[0].Arguments = %q, want %q", payload.ToolCalls[0].Arguments, `{"query":"billing"}`)
	}
}

func TestBuildEventPayloadCapturesToolResult(t *testing.T) {
	msg := &schema.Message{
		Role:       schema.Tool,
		Content:    "FAIL TestUserAuth expected 200 got 401",
		ToolCallID: "call-9",
		ToolName:   "go_test",
	}

	event := adk.EventFromMessage(msg, nil, schema.Tool, "go_test")
	event.AgentName = "sage"
	event.RunPath = mustRunSteps(t, "planner", "sage")

	payload := buildEventPayload(event, event.Output.MessageOutput, msg)
	if payload == nil {
		t.Fatal("buildEventPayload() = nil, want payload")
	}
	if payload.ToolResult == nil {
		t.Fatal("payload.ToolResult = nil, want structured tool result")
	}
	if payload.ToolResult.ToolName != "go_test" {
		t.Fatalf("payload.ToolResult.ToolName = %q, want %q", payload.ToolResult.ToolName, "go_test")
	}
	if payload.ToolResult.ToolCallID != "call-9" {
		t.Fatalf("payload.ToolResult.ToolCallID = %q, want %q", payload.ToolResult.ToolCallID, "call-9")
	}
	if payload.ToolResult.Content != "FAIL TestUserAuth expected 200 got 401" {
		t.Fatalf("payload.ToolResult.Content = %q, want tool output content", payload.ToolResult.Content)
	}
}

func mustRunSteps(t *testing.T, names ...string) []adk.RunStep {
	t.Helper()

	steps := make([]adk.RunStep, 0, len(names))
	for _, name := range names {
		var step adk.RunStep
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(struct {
			AgentName string
		}{AgentName: name}); err != nil {
			t.Fatalf("encode run step %q: %v", name, err)
		}
		if err := step.GobDecode(buf.Bytes()); err != nil {
			t.Fatalf("decode run step %q: %v", name, err)
		}
		steps = append(steps, step)
	}

	return steps
}
