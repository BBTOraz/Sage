package runtime

import (
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

type EventPayload struct {
	AgentName  string              `json:"agent_name,omitempty"`
	RunPath    []string            `json:"run_path,omitempty"`
	Role       string              `json:"role,omitempty"`
	ToolCalls  []ToolCallPayload   `json:"tool_calls,omitempty"`
	ToolResult *ToolResultPayload  `json:"tool_result,omitempty"`
}

type ToolCallPayload struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolResultPayload struct {
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Content    string `json:"content,omitempty"`
}

func buildEventPayload(event *adk.AgentEvent, variant *adk.MessageVariant, msg *schema.Message) *EventPayload {
	payload := &EventPayload{}

	if event != nil {
		payload.AgentName = event.AgentName
		payload.RunPath = normalizeRunPath(event.RunPath)
	}

	if variant != nil {
		payload.Role = string(variant.Role)
	}

	if msg != nil {
		if len(msg.ToolCalls) > 0 {
			payload.ToolCalls = make([]ToolCallPayload, 0, len(msg.ToolCalls))
			for _, call := range msg.ToolCalls {
				payload.ToolCalls = append(payload.ToolCalls, ToolCallPayload{
					ID:        call.ID,
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				})
			}
		}

		if msg.ToolCallID != "" || msg.ToolName != "" || (variant != nil && variant.Role == schema.Tool) {
			toolName := msg.ToolName
			if toolName == "" && variant != nil {
				toolName = variant.ToolName
			}
			payload.ToolResult = &ToolResultPayload{
				ToolCallID: msg.ToolCallID,
				ToolName:   toolName,
				Content:    msg.Content,
			}
		}
	}

	if payload.AgentName == "" &&
		len(payload.RunPath) == 0 &&
		payload.Role == "" &&
		len(payload.ToolCalls) == 0 &&
		payload.ToolResult == nil {
		return nil
	}

	return payload
}

func normalizeRunPath(steps []adk.RunStep) []string {
	if len(steps) == 0 {
		return nil
	}

	path := make([]string, 0, len(steps))
	for _, step := range steps {
		name := step.String()
		if name == "" {
			continue
		}
		path = append(path, name)
	}
	if len(path) == 0 {
		return nil
	}
	return path
}
