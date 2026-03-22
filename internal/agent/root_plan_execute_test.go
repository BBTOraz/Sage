package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestApplicationRootAgentBuildsPlanExecuteWorkflow(t *testing.T) {
	model := &stubToolCallingModel{}

	app := &Application{
		config: ApplicationConfig{},
		model:  model,
	}

	root, err := app.RootAgent(context.Background())
	if err != nil {
		t.Fatalf("RootAgent() error = %v", err)
	}

	if got := root.Name(context.Background()); got != "plan_execute_replan" {
		t.Fatalf("RootAgent().Name() = %q, want %q", got, "plan_execute_replan")
	}

	if model.withToolsCalls != 2 {
		t.Fatalf("tool-calling model WithTools() calls = %d, want %d", model.withToolsCalls, 2)
	}
}

func TestApplicationRootAgentRejectsModelWithoutToolCalling(t *testing.T) {
	app := &Application{
		config: ApplicationConfig{},
		model:  &stubBaseModel{},
	}

	_, err := app.RootAgent(context.Background())
	if err == nil {
		t.Fatal("RootAgent() error = nil, want tool-calling compatibility error")
	}

	if !strings.Contains(err.Error(), "tool-calling") {
		t.Fatalf("RootAgent() error = %q, want tool-calling compatibility hint", err)
	}
}

func TestPlanExecuteRootFlow(t *testing.T) {
	model := newScriptedPlanExecuteModel([]*schema.Message{
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:   "plan-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "plan",
				Arguments: `{"steps":["inspect architecture"]}`,
			},
		}}),
		schema.AssistantMessage("executor result", nil),
		schema.AssistantMessage("", []schema.ToolCall{{
			ID:   "respond-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "respond",
				Arguments: `{"response":"final answer"}`,
			},
		}}),
	})

	app := &Application{
		config: ApplicationConfig{},
		model:  model,
	}

	runner, err := app.Runner(context.Background())
	if err != nil {
		t.Fatalf("Runner() error = %v", err)
	}

	if err := drainAgentEvents(runner.Query(context.Background(), "start")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	if len(model.state.callKinds) != 3 {
		t.Fatalf("model call count = %d, want %d", len(model.state.callKinds), 3)
	}

	want := []string{"planner", "executor", "replanner"}
	for idx, kind := range want {
		if model.state.callKinds[idx] != kind {
			t.Fatalf("model call kinds = %v, want %v", model.state.callKinds, want)
		}
	}
}

func TestPlanExecuteRootUsesIterationCap200(t *testing.T) {
	ctx := context.Background()
	model := &replanningToolCallingModel{
		state: &replanningToolCallingState{
			respondAfterReplans: 11,
		},
	}
	root, err := NewRootPlanExecute(ctx, model, &loopingExecutorAgent{})
	if err != nil {
		t.Fatalf("NewRootPlanExecute() error = %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: root})
	if err := drainAgentEvents(runner.Query(ctx, "long running task")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	if model.state.replannerCalls != 11 {
		t.Fatalf("replanner calls = %d, want %d", model.state.replannerCalls, 11)
	}
}

type stubBaseModel struct{}

func (m *stubBaseModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("ok", nil), nil
}

func (m *stubBaseModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("ok", nil)}), nil
}

type stubToolCallingModel struct {
	withToolsCalls int
}

func (m *stubToolCallingModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("ok", nil), nil
}

func (m *stubToolCallingModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("ok", nil)}), nil
}

func (m *stubToolCallingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	m.withToolsCalls++
	return m, nil
}

type scriptedPlanExecuteModel struct {
	state          *scriptedPlanExecuteState
	boundToolNames []string
}

type scriptedPlanExecuteState struct {
	responses  []*schema.Message
	callKinds  []string
	callCursor int
}

func newScriptedPlanExecuteModel(responses []*schema.Message) *scriptedPlanExecuteModel {
	return &scriptedPlanExecuteModel{
		state: &scriptedPlanExecuteState{
			responses: responses,
		},
	}
}

func (m *scriptedPlanExecuteModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.state.callKinds = append(m.state.callKinds, classifyPlanExecuteCall(m.boundToolNames, opts...))
	if m.state.callCursor >= len(m.state.responses) {
		return schema.AssistantMessage("unexpected extra call", nil), nil
	}
	response := m.state.responses[m.state.callCursor]
	m.state.callCursor++
	return response, nil
}

func (m *scriptedPlanExecuteModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *scriptedPlanExecuteModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &scriptedPlanExecuteModel{
		state:          m.state,
		boundToolNames: collectToolInfoNames(tools),
	}, nil
}

func classifyPlanExecuteCall(boundToolNames []string, opts ...model.Option) string {
	toolInfos := model.GetCommonOptions(nil, opts...).Tools
	if len(toolInfos) == 0 && len(boundToolNames) > 0 {
		toolInfos = make([]*schema.ToolInfo, 0, len(boundToolNames))
		for _, name := range boundToolNames {
			toolInfos = append(toolInfos, &schema.ToolInfo{Name: name})
		}
	}
	hasPlan := false
	hasRespond := false
	for _, toolInfo := range toolInfos {
		switch toolInfo.Name {
		case "plan":
			hasPlan = true
		case "respond":
			hasRespond = true
		}
	}
	if hasPlan && hasRespond {
		return "replanner"
	}
	if hasPlan {
		return "planner"
	}
	return "executor"
}

func collectToolInfoNames(tools []*schema.ToolInfo) []string {
	names := make([]string, 0, len(tools))
	for _, toolInfo := range tools {
		names = append(names, toolInfo.Name)
	}
	return names
}

type replanningToolCallingModel struct {
	boundToolNames []string
	state          *replanningToolCallingState
}

type replanningToolCallingState struct {
	replannerCalls      int
	respondAfterReplans int
}

func (m *replanningToolCallingModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	callKind := classifyPlanExecuteCall(m.boundToolNames, opts...)
	switch callKind {
	case "planner":
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:   "plan-1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "plan",
				Arguments: `{"steps":["inspect architecture"]}`,
			},
		}}), nil
	case "replanner":
		m.state.replannerCalls++
		if m.state.replannerCalls >= m.state.respondAfterReplans {
			return schema.AssistantMessage("", []schema.ToolCall{{
				ID:   fmt.Sprintf("respond-%d", m.state.replannerCalls),
				Type: "function",
				Function: schema.FunctionCall{
					Name:      "respond",
					Arguments: `{"response":"final answer"}`,
				},
			}}), nil
		}
		return schema.AssistantMessage("", []schema.ToolCall{{
			ID:   fmt.Sprintf("plan-%d", m.state.replannerCalls+1),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "plan",
				Arguments: `{"steps":["inspect architecture"]}`,
			},
		}}), nil
	default:
		return schema.AssistantMessage("unexpected call kind", nil), nil
	}
}

func (m *replanningToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *replanningToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &replanningToolCallingModel{
		boundToolNames: collectToolInfoNames(tools),
		state:          m.state,
	}, nil
}

type loopingExecutorAgent struct{}

func (a *loopingExecutorAgent) Name(context.Context) string        { return "looping-executor" }
func (a *loopingExecutorAgent) Description(context.Context) string { return "looping executor" }

func (a *loopingExecutorAgent) Run(ctx context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer gen.Close()
		adk.AddSessionValue(ctx, planexecute.ExecutedStepSessionKey, "executor result")
		gen.Send(adk.EventFromMessage(schema.AssistantMessage("executor result", nil), nil, schema.Assistant, ""))
	}()
	return iter
}
