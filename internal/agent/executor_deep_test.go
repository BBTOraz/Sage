package agent

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"bilge-lib/internal/approval"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestExecutorDeepFilesystemRegistersFileToolsWithoutExecute(t *testing.T) {
	model := &capturingToolCallingModel{}

	agent, err := NewExecutorDeep(context.Background(), ExecutorDeepConfig{
		Model:        model,
		ApprovalMode: approval.Guard,
		Capabilities: ExecutorDeepCapabilities{
			Filesystem: ExecutorDeepFilesystemConfig{
				Enabled:       true,
				WorkspaceRoot: t.TempDir(),
				EnableExecute: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutorDeep() error = %v", err)
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	if err := drainAgentEvents(runner.Query(context.Background(), "respond directly")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	toolNames := model.ToolNames()
	for _, want := range []string{"ls", "read_file", "write_file", "edit_file", "glob", "grep"} {
		if !slices.Contains(toolNames, want) {
			t.Fatalf("registered tools = %v, want %q", toolNames, want)
		}
	}
	if slices.Contains(toolNames, "execute") {
		t.Fatalf("registered tools = %v, execute must not be registered", toolNames)
	}
}

func TestExecutorDeepFilesystemCanBeDisabled(t *testing.T) {
	model := &capturingToolCallingModel{}

	agent, err := NewExecutorDeep(context.Background(), ExecutorDeepConfig{
		Model:        model,
		ApprovalMode: approval.Guard,
		Capabilities: ExecutorDeepCapabilities{
			Filesystem: ExecutorDeepFilesystemConfig{
				Enabled: false,
			},
		},
	})
	if err != nil {
		t.Fatalf("NewExecutorDeep() error = %v", err)
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	if err := drainAgentEvents(runner.Query(context.Background(), "respond directly")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	toolNames := model.ToolNames()
	for _, forbidden := range []string{"ls", "read_file", "write_file", "edit_file", "glob", "grep", "execute"} {
		if slices.Contains(toolNames, forbidden) {
			t.Fatalf("registered tools = %v, forbidden tool %q should be absent", toolNames, forbidden)
		}
	}
}

func TestExecutorDeepUsesIterationCap200(t *testing.T) {
	ctx := context.Background()
	model := &loopingTaskToolCallingModel{
		subagentName:         "test-subagent",
		toolCallsBeforeFinal: 21,
	}
	agent, err := NewExecutorDeep(ctx, ExecutorDeepConfig{
		Model:        model,
		ApprovalMode: approval.Guard,
		SubAgents:    []adk.Agent{&testSubAgent{name: model.subagentName}},
	})
	if err != nil {
		t.Fatalf("NewExecutorDeep() error = %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	if err := drainAgentEvents(runner.Query(ctx, "long task")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	if model.toolCallCount != 21 {
		t.Fatalf("tool call count = %d, want %d", model.toolCallCount, 21)
	}
}

type capturingToolCallingModel struct {
	toolNames []string
}

func (m *capturingToolCallingModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.captureTools(opts...)
	return schema.AssistantMessage("ok", nil), nil
}

func (m *capturingToolCallingModel) Stream(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.captureTools(opts...)
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("ok", nil)}), nil
}

func (m *capturingToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.toolNames = collectToolNames(tools)
	return m, nil
}

func (m *capturingToolCallingModel) ToolNames() []string {
	out := make([]string, len(m.toolNames))
	copy(out, m.toolNames)
	return out
}

func (m *capturingToolCallingModel) captureTools(opts ...model.Option) {
	options := model.GetCommonOptions(nil, opts...)
	m.toolNames = collectToolNames(options.Tools)
}

func collectToolNames(tools []*schema.ToolInfo) []string {
	names := make([]string, 0, len(tools))
	for _, toolInfo := range tools {
		names = append(names, toolInfo.Name)
	}
	return names
}

type loopingTaskToolCallingModel struct {
	toolNames            []string
	subagentName         string
	toolCallCount        int
	toolCallsBeforeFinal int
}

func (m *loopingTaskToolCallingModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.captureTools(opts...)
	if m.toolCallCount < m.toolCallsBeforeFinal {
		m.toolCallCount++
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:   fmt.Sprintf("task-%d", m.toolCallCount),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "task",
				Arguments: fmt.Sprintf(`{"subagent_type":"%s","description":"work item %d"}`, m.subagentName, m.toolCallCount),
			},
		}}), nil
	}
	return schema.AssistantMessage("done", nil), nil
}

func (m *loopingTaskToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *loopingTaskToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.toolNames = collectToolNames(tools)
	return m, nil
}

func (m *loopingTaskToolCallingModel) captureTools(opts ...model.Option) {
	options := model.GetCommonOptions(nil, opts...)
	m.toolNames = collectToolNames(options.Tools)
}

type testSubAgent struct {
	name string
}

func (a *testSubAgent) Name(context.Context) string        { return a.name }
func (a *testSubAgent) Description(context.Context) string { return "test subagent" }

func (a *testSubAgent) Run(_ context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		gen.Send(adk.EventFromMessage(schema.AssistantMessage("subagent ok", nil), nil, schema.Assistant, ""))
	}()
	return iter
}

func drainAgentEvents(iter *adk.AsyncIterator[*adk.AgentEvent]) error {
	for {
		event, ok := iter.Next()
		if !ok {
			return nil
		}
		if event.Err != nil {
			return event.Err
		}
	}
}
