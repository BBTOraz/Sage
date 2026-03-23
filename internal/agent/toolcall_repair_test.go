package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestRepairToolArgumentsJSONExtractsJSONFromCodeFence(t *testing.T) {
	repaired, ok := repairToolArgumentsJSON("```json\n{\"path\":\"internal/app.go\"}\n```")
	if !ok {
		t.Fatal("expected repairToolArgumentsJSON() to repair fenced JSON")
	}
	if repaired != `{"path":"internal/app.go"}` {
		t.Fatalf("repaired = %q, want strict JSON object", repaired)
	}
}

func TestRepairingToolCallingModelRepairsToolArguments(t *testing.T) {
	base := &fakeToolCallingModel{
		generateOutputs: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{
				{
					ID: "call-1",
					Function: schema.FunctionCall{
						Name:      "plan",
						Arguments: "```json\n{\"steps\":[\"one\"]}\n```",
					},
				},
			}),
		},
	}

	wrapped := wrapRepairingToolCallingModel(base)
	msg, err := wrapped.Generate(context.Background(), []*schema.Message{schema.UserMessage("plan it")})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(msg.ToolCalls) != 1 {
		t.Fatalf("tool calls len = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Arguments != `{"steps":["one"]}` {
		t.Fatalf("arguments = %q, want repaired strict JSON", msg.ToolCalls[0].Function.Arguments)
	}
}

func TestRepairingToolCallingModelRetriesWithStrictJSONHint(t *testing.T) {
	base := &fakeToolCallingModel{
		generateOutputs: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{
				{
					ID: "call-1",
					Function: schema.FunctionCall{
						Name:      "plan",
						Arguments: "<tool_call><function=plan><parameters></parameters>",
					},
				},
			}),
			schema.AssistantMessage("", []schema.ToolCall{
				{
					ID: "call-1",
					Function: schema.FunctionCall{
						Name:      "plan",
						Arguments: `{"steps":["strict output"]}`,
					},
				},
			}),
		},
	}

	wrapped := wrapRepairingToolCallingModel(base)
	msg, err := wrapped.Generate(context.Background(), []*schema.Message{schema.UserMessage("plan it")})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if base.generateCalls != 2 {
		t.Fatalf("generate calls = %d, want %d", base.generateCalls, 2)
	}
	if len(base.lastGenerateInput) == 0 {
		t.Fatal("expected retry input to be captured")
	}
	lastInput := base.lastGenerateInput[len(base.lastGenerateInput)-1]
	if lastInput.Role != schema.System || !strings.Contains(strings.ToLower(lastInput.Content), "strict json") {
		t.Fatalf("retry hint message = %#v, want strict json system hint", lastInput)
	}
	if msg.ToolCalls[0].Function.Arguments != `{"steps":["strict output"]}` {
		t.Fatalf("arguments = %q, want retried strict JSON", msg.ToolCalls[0].Function.Arguments)
	}
}

type fakeToolCallingModel struct {
	generateOutputs   []*schema.Message
	generateCalls     int
	lastGenerateInput []*schema.Message
}

func (f *fakeToolCallingModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	f.generateCalls++
	f.lastGenerateInput = cloneMessages(input)
	if len(f.generateOutputs) == 0 {
		return schema.AssistantMessage("", nil), nil
	}
	msg := f.generateOutputs[0]
	f.generateOutputs = f.generateOutputs[1:]
	return cloneMessage(msg), nil
}

func (f *fakeToolCallingModel) Stream(ctx context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := f.Generate(ctx, input)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (f *fakeToolCallingModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return f, nil
}
