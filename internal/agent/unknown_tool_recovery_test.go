package agent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func TestUnknownToolRecoveryAgentRecoversStreamingUnknownTool(t *testing.T) {
	ctx := context.Background()
	inner := &streamingUnknownToolAgent{}

	agent := wrapUnknownToolRecoveryAgent(inner)
	if err := drainAgentEvents(agent.Run(ctx, &adk.AgentInput{EnableStreaming: true})); err != nil {
		t.Fatalf("agent.Run() error = %v", err)
	}

	if inner.runCalls != 2 {
		t.Fatalf("run calls = %d, want %d", inner.runCalls, 2)
	}
	if len(inner.secondRunInput) != 2 {
		t.Fatalf("second run messages len = %d, want %d", len(inner.secondRunInput), 2)
	}
	if got := inner.secondRunInput[0].ToolCalls[0].Function.Name; got != "run_shell_command" {
		t.Fatalf("assistant recovery tool name = %q, want %q", got, "run_shell_command")
	}
	if got := inner.secondRunInput[1].Role; got != schema.Tool {
		t.Fatalf("tool recovery role = %q, want %q", got, schema.Tool)
	}
}

func TestUnknownToolRecoveryAgentRecoversUnknownToolOnResume(t *testing.T) {
	ctx := context.Background()
	inner := &resumeUnknownToolAgent{}

	agent := wrapUnknownToolRecoveryAgent(inner)
	resumable, ok := agent.(adk.ResumableAgent)
	if !ok {
		t.Fatal("wrapped agent does not implement adk.ResumableAgent")
	}

	info := &adk.ResumeInfo{
		InterruptState: []byte("checkpoint"),
		ResumeData: &adk.ChatModelAgentResumeData{
			HistoryModifier: func(_ context.Context, history []adk.Message) []adk.Message {
				return append(cloneMessages(history), schema.SystemMessage("base resume history"))
			},
		},
	}
	if err := drainAgentEvents(resumable.Resume(ctx, info)); err != nil {
		t.Fatalf("agent.Resume() error = %v", err)
	}

	if inner.resumeCalls != 2 {
		t.Fatalf("resume calls = %d, want %d", inner.resumeCalls, 2)
	}
	if len(inner.secondResumeHistory) != 3 {
		t.Fatalf("second resume history len = %d, want %d", len(inner.secondResumeHistory), 3)
	}
	if got := inner.secondResumeHistory[0].Role; got != schema.System {
		t.Fatalf("resume history[0] role = %q, want %q", got, schema.System)
	}
	if got := inner.secondResumeHistory[1].ToolCalls[0].Function.Name; got != "run_shell_command" {
		t.Fatalf("resume recovery tool name = %q, want %q", got, "run_shell_command")
	}
	if got := inner.secondResumeHistory[2].Role; got != schema.Tool {
		t.Fatalf("resume recovery role = %q, want %q", got, schema.Tool)
	}
}

type streamingUnknownToolAgent struct {
	runCalls       int
	secondRunInput []adk.Message
}

func (a *streamingUnknownToolAgent) Name(context.Context) string        { return "streaming-unknown-tool" }
func (a *streamingUnknownToolAgent) Description(context.Context) string { return "test agent" }

func (a *streamingUnknownToolAgent) Run(_ context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	a.runCalls++
	if a.runCalls == 2 {
		a.secondRunInput = cloneMessages(input.Messages)
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(adk.EventFromMessage(schema.AssistantMessage("recovered", nil), nil, schema.Assistant, ""))
		}()
		return iter
	}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		gen.Send(adk.EventFromMessage(nil, schema.StreamReaderFromArray([]*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID:   "stream-unknown-1",
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "run_shell_command",
					Arguments: `{"command":"dir"}`,
				},
			}}),
		}), schema.Assistant, ""))
		gen.Send(&adk.AgentEvent{Err: errUnknownTool("run_shell_command")})
	}()
	return iter
}

type resumeUnknownToolAgent struct {
	resumeCalls         int
	secondResumeHistory []adk.Message
}

func (a *resumeUnknownToolAgent) Name(context.Context) string        { return "resume-unknown-tool" }
func (a *resumeUnknownToolAgent) Description(context.Context) string { return "test agent" }

func (a *resumeUnknownToolAgent) Run(_ context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		gen.Send(&adk.AgentEvent{Err: context.Canceled})
	}()
	return iter
}

func (a *resumeUnknownToolAgent) Resume(ctx context.Context, info *adk.ResumeInfo, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	a.resumeCalls++
	if a.resumeCalls == 2 {
		resumeData, ok := info.ResumeData.(*adk.ChatModelAgentResumeData)
		if !ok {
			panic("resume data type mismatch")
		}
		a.secondResumeHistory = resumeData.HistoryModifier(ctx, nil)
		iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
		go func() {
			defer gen.Close()
			gen.Send(adk.EventFromMessage(schema.AssistantMessage("recovered", nil), nil, schema.Assistant, ""))
		}()
		return iter
	}

	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		gen.Send(adk.EventFromMessage(schema.AssistantMessage("", []schema.ToolCall{{
			ID:   "resume-unknown-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "run_shell_command",
				Arguments: `{"command":"dir"}`,
			},
		}}), nil, schema.Assistant, ""))
		gen.Send(&adk.AgentEvent{Err: errUnknownTool("run_shell_command")})
	}()
	return iter
}

func errUnknownTool(name string) error {
	return &unknownToolError{name: name}
}

type unknownToolError struct {
	name string
}

func (e *unknownToolError) Error() string {
	return "tool " + e.name + " not found in toolsNode indexes"
}
