package agent

import (
	"context"
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
