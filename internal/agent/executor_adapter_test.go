package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"bilge-lib/internal/approval"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestExecutorDeepAdapterInjectsPlanExecuteContext(t *testing.T) {
	model := &inputCapturingToolCallingModel{
		response: "executor result",
	}

	agent, err := NewExecutorDeep(context.Background(), ExecutorDeepConfig{
		Model:        model,
		ApprovalMode: approval.Guard,
		Capabilities: ExecutorDeepCapabilities{},
	})
	if err != nil {
		t.Fatalf("NewExecutorDeep() error = %v", err)
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: false,
	})

	iter := runner.Run(context.Background(), []adk.Message{schema.UserMessage("ignored")}, adk.WithSessionValues(map[string]any{
		planexecute.UserInputSessionKey: []adk.Message{schema.UserMessage("build migration architecture")},
		planexecute.PlanSessionKey: &testPlan{
			Steps: []string{"analyze agent assembly"},
		},
		planexecute.ExecutedStepsSessionKey: []planexecute.ExecutedStep{
			{Step: "inspect runtime", Result: "done"},
		},
	}))

	if err := drainAgentEvents(iter); err != nil {
		t.Fatalf("runner.Run() error = %v", err)
	}

	got := strings.Join(model.lastInput, "\n")
	for _, want := range []string{
		"build migration architecture",
		"analyze agent assembly",
		"inspect runtime",
		"done",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("model input = %q, want fragment %q", got, want)
		}
	}
}

func TestExecutorDeepAdapterWritesExecutedStepSessionKeyForReplanner(t *testing.T) {
	executorModel := &inputCapturingToolCallingModel{
		response: "executor result",
	}
	executor, err := NewExecutorDeep(context.Background(), ExecutorDeepConfig{
		Model:        executorModel,
		ApprovalMode: approval.Guard,
		Capabilities: ExecutorDeepCapabilities{},
	})
	if err != nil {
		t.Fatalf("NewExecutorDeep() error = %v", err)
	}

	replanner := &testReplannerAgent{}
	root, err := planexecute.New(context.Background(), &planexecute.Config{
		Planner:   &testPlannerAgent{},
		Executor:  executor,
		Replanner: replanner,
	})
	if err != nil {
		t.Fatalf("planexecute.New() error = %v", err)
	}

	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{
		Agent:           root,
		EnableStreaming: false,
	})

	if err := drainAgentEvents(runner.Query(context.Background(), "start")); err != nil {
		t.Fatalf("runner.Query() error = %v", err)
	}

	if replanner.executedStep != "executor result" {
		t.Fatalf("replanner executed step = %q, want %q", replanner.executedStep, "executor result")
	}
}

type inputCapturingToolCallingModel struct {
	response  string
	lastInput []string
}

func (m *inputCapturingToolCallingModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.lastInput = flattenMessages(input)
	return schema.AssistantMessage(m.response, nil), nil
}

func (m *inputCapturingToolCallingModel) Stream(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.lastInput = flattenMessages(input)
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage(m.response, nil)}), nil
}

func (m *inputCapturingToolCallingModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func flattenMessages(messages []*schema.Message) []string {
	out := make([]string, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.Content)
	}
	return out
}

type testPlan struct {
	Steps []string `json:"steps"`
}

func (p *testPlan) FirstStep() string {
	if len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[0]
}

func (p *testPlan) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Steps []string `json:"steps"`
	}{
		Steps: p.Steps,
	})
}

func (p *testPlan) UnmarshalJSON(data []byte) error {
	var payload struct {
		Steps []string `json:"steps"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	p.Steps = payload.Steps
	return nil
}

type testPlannerAgent struct{}

func (a *testPlannerAgent) Name(context.Context) string        { return "test-planner" }
func (a *testPlannerAgent) Description(context.Context) string { return "test planner" }

func (a *testPlannerAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, writer := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer writer.Close()
		adk.AddSessionValue(ctx, planexecute.UserInputSessionKey, input.Messages)
		adk.AddSessionValue(ctx, planexecute.PlanSessionKey, &testPlan{
			Steps: []string{"execute step"},
		})
		writer.Send(adk.EventFromMessage(schema.AssistantMessage("planned", nil), nil, schema.Assistant, ""))
	}()
	return iter
}

type testReplannerAgent struct {
	executedStep string
}

func (a *testReplannerAgent) Name(context.Context) string        { return "test-replanner" }
func (a *testReplannerAgent) Description(context.Context) string { return "test replanner" }

func (a *testReplannerAgent) Run(ctx context.Context, _ *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, writer := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer writer.Close()
		executedStep, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepSessionKey)
		if !ok {
			writer.Send(&adk.AgentEvent{Err: context.Canceled})
			return
		}
		a.executedStep = executedStep.(string)
		writer.Send(&adk.AgentEvent{Action: adk.NewBreakLoopAction("test-replanner")})
	}()
	return iter
}
