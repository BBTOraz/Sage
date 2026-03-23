package agent

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const strictJSONToolCallHint = "Your previous tool call arguments were invalid. Return a strict JSON tool call with arguments matching the tool schema exactly. Do not use markdown fences, XML tags, or extra commentary."

func repairToolArgumentsJSON(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed, true
	}

	candidates := []string{trimmed}
	if stripped, ok := stripCodeFence(trimmed); ok {
		candidates = append(candidates, stripped)
	}
	if unquoted, err := strconv.Unquote(trimmed); err == nil {
		candidates = append(candidates, strings.TrimSpace(unquoted))
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if json.Valid([]byte(candidate)) {
			return candidate, true
		}
		if extracted, ok := extractFirstJSONObjectOrArray(candidate); ok {
			return extracted, true
		}
	}

	return "", false
}

func stripCodeFence(raw string) (string, bool) {
	if !strings.HasPrefix(raw, "```") {
		return "", false
	}

	body := strings.TrimPrefix(raw, "```")
	if idx := strings.Index(body, "\n"); idx >= 0 {
		body = body[idx+1:]
	}
	if idx := strings.LastIndex(body, "```"); idx >= 0 {
		body = body[:idx]
	}
	return strings.TrimSpace(body), true
}

func extractFirstJSONObjectOrArray(raw string) (string, bool) {
	for start := 0; start < len(raw); start++ {
		open := raw[start]
		if open != '{' && open != '[' {
			continue
		}

		closeChar := byte('}')
		if open == '[' {
			closeChar = ']'
		}

		depth := 0
		inString := false
		escaped := false
		for end := start; end < len(raw); end++ {
			ch := raw[end]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inString = false
				}
				continue
			}

			switch ch {
			case '"':
				inString = true
			case open:
				depth++
			case closeChar:
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(raw[start : end+1])
					if json.Valid([]byte(candidate)) {
						return candidate, true
					}
				}
			}
		}
	}

	return "", false
}

func repairToolCallMessage(msg *schema.Message) (*schema.Message, bool) {
	if msg == nil || len(msg.ToolCalls) == 0 {
		return msg, false
	}

	cloned := cloneMessage(msg)
	unrepaired := false
	for idx, toolCall := range cloned.ToolCalls {
		if json.Valid([]byte(strings.TrimSpace(toolCall.Function.Arguments))) {
			continue
		}
		repaired, ok := repairToolArgumentsJSON(toolCall.Function.Arguments)
		if !ok {
			unrepaired = true
			continue
		}
		cloned.ToolCalls[idx].Function.Arguments = repaired
	}

	return cloned, unrepaired
}

type repairingToolCallingModel struct {
	base model.ToolCallingChatModel
}

func wrapRepairingToolCallingModel(base model.ToolCallingChatModel) model.ToolCallingChatModel {
	return &repairingToolCallingModel{base: base}
}

func (m *repairingToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	msg, err := m.base.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}

	repaired, needsRetry := repairToolCallMessage(msg)
	if !needsRetry {
		return repaired, nil
	}

	retryInput := append(cloneMessages(input), schema.SystemMessage(strictJSONToolCallHint))
	retryMsg, err := m.base.Generate(ctx, retryInput, opts...)
	if err != nil {
		return repaired, err
	}

	repairedRetry, _ := repairToolCallMessage(retryMsg)
	return repairedRetry, nil
}

func (m *repairingToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *repairingToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.base.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return wrapRepairingToolCallingModel(bound), nil
}

func toolArgumentsRepairHandler() func(ctx context.Context, name, arguments string) (string, error) {
	return func(ctx context.Context, name, arguments string) (string, error) {
		repaired, ok := repairToolArgumentsJSON(arguments)
		if ok {
			return repaired, nil
		}
		return arguments, nil
	}
}
