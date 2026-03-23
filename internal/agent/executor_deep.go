package agent

import (
	"bilge-lib/internal/approval"
	middleware2 "bilge-lib/internal/middleware"
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type ExecutorDeepConfig struct {
	Model        model.BaseChatModel
	ApprovalMode approval.Mode
	SubAgents    []adk.Agent
	Capabilities ExecutorDeepCapabilities
}

func NewExecutorDeep(ctx context.Context, cfg ExecutorDeepConfig) (adk.ResumableAgent, error) {
	runtimeCapabilities, err := buildExecutorDeepRuntimeCapabilities(ctx, cfg.Capabilities)
	if err != nil {
		return nil, err
	}
	handlers, err := BuildExecutorDeepHandlers(ctx, AgentHandlerConfig{
		ApprovalMode: cfg.ApprovalMode,
		Model:        cfg.Model,
		Capabilities: cfg.Capabilities,
	})
	if err != nil {
		return nil, err
	}
	wrappedModel := wrapExecutorContextModel(cfg.Model)

	inner, err := deep.New(ctx, &deep.Config{
		Name:           "sage",
		Description:    executorDeepDescription,
		Instruction:    executorDeepInstruction,
		ChatModel:      wrappedModel,
		MaxIteration:   defaultAgentIterationCap,
		SubAgents:      cfg.SubAgents,
		Handlers:       handlers,
		Backend:        runtimeCapabilities.Backend,
		Shell:          runtimeCapabilities.Shell,
		StreamingShell: runtimeCapabilities.StreamingShell,
		OutputKey:      planexecute.ExecutedStepSessionKey,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
				ToolArgumentsHandler: toolArgumentsRepairHandler(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return wrapUnknownToolRecoveryAgent(inner).(adk.ResumableAgent), nil
}

func (a *Application) ExecutorDeep(ctx context.Context) (adk.ResumableAgent, error) {
	docAgent, err := a.DocAgent(ctx)
	if err != nil {
		return nil, err
	}

	return NewExecutorDeep(ctx, ExecutorDeepConfig{
		Model:        a.model,
		ApprovalMode: a.config.ApprovalMode,
		SubAgents:    []adk.Agent{docAgent},
		Capabilities: a.executorDeepCapabilities,
	})
}

func wrapExecutorContextModel(base model.BaseChatModel) model.BaseChatModel {
	toolCalling, ok := base.(model.ToolCallingChatModel)
	if ok {
		return &executorContextToolCallingModel{
			base: executorContextBaseModel{base: toolCalling},
			tool: toolCalling,
		}
	}

	return &executorContextBaseModel{base: base}
}

func getExecutorUserInput(ctx context.Context) []adk.Message {
	userInputValue, ok := adk.GetSessionValue(ctx, planexecute.UserInputSessionKey)
	if !ok {
		return nil
	}

	userInput, ok := userInputValue.([]adk.Message)
	if !ok {
		return nil
	}

	return userInput
}

func getExecutedSteps(ctx context.Context) []planexecute.ExecutedStep {
	executedStepsValue, ok := adk.GetSessionValue(ctx, planexecute.ExecutedStepsSessionKey)
	if !ok {
		return nil
	}

	executedSteps, ok := executedStepsValue.([]planexecute.ExecutedStep)
	if !ok {
		return nil
	}

	return executedSteps
}

func executorPlanExecuteContextFromSession(ctx context.Context) (string, error) {
	planValue, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
	if !ok {
		return "", nil
	}

	plan, ok := planValue.(planexecute.Plan)
	if !ok {
		return "", fmt.Errorf("planexecute plan session value has unexpected type %T", planValue)
	}

	return formatExecutorPlanExecuteContext(
		getExecutorUserInput(ctx),
		plan,
		getExecutedSteps(ctx),
	)
}

func mergeExecutorContextIntoMessages(input []*schema.Message, executionContext string) []*schema.Message {
	if strings.TrimSpace(executionContext) == "" {
		return input
	}

	if len(input) == 0 {
		return []*schema.Message{schema.SystemMessage(executionContext)}
	}

	cloned := make([]*schema.Message, len(input))
	copy(cloned, input)

	if cloned[0] != nil && cloned[0].Role == schema.System {
		msgCopy := *cloned[0]
		if strings.TrimSpace(msgCopy.Content) == "" {
			msgCopy.Content = executionContext
		} else {
			msgCopy.Content = msgCopy.Content + "\n\n" + executionContext
		}
		cloned[0] = &msgCopy
		return cloned
	}

	return append([]*schema.Message{schema.SystemMessage(executionContext)}, cloned...)
}

type executorContextBaseModel struct {
	base model.BaseChatModel
}

func (m *executorContextBaseModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	merged, err := m.withExecutorContext(ctx, input)
	if err != nil {
		return nil, err
	}
	return m.base.Generate(ctx, merged, opts...)
}

func (m *executorContextBaseModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	merged, err := m.withExecutorContext(ctx, input)
	if err != nil {
		return nil, err
	}
	return m.base.Stream(ctx, merged, opts...)
}

func (m *executorContextBaseModel) withExecutorContext(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
	executionContext, err := executorPlanExecuteContextFromSession(ctx)
	if err != nil {
		return nil, err
	}
	return mergeExecutorContextIntoMessages(input, executionContext), nil
}

type executorContextToolCallingModel struct {
	base executorContextBaseModel
	tool model.ToolCallingChatModel
}

func (m *executorContextToolCallingModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.base.Generate(ctx, input, opts...)
}

func (m *executorContextToolCallingModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.base.Stream(ctx, input, opts...)
}

func (m *executorContextToolCallingModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	bound, err := m.tool.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &executorContextToolCallingModel{
		base: executorContextBaseModel{base: bound},
		tool: bound,
	}, nil
}

func formatExecutorPlanExecuteContext(userInput []adk.Message, plan planexecute.Plan, executedSteps []planexecute.ExecutedStep) (string, error) {
	planJSON, err := plan.MarshalJSON()
	if err != nil {
		return "", err
	}

	var prompt strings.Builder
	prompt.WriteString("## PlanExecute Executor Context\n")
	prompt.WriteString("Objective:\n")
	prompt.WriteString(strings.TrimSpace(formatExecutorUserInput(userInput)))
	prompt.WriteString("\n\nCurrent step:\n")
	prompt.WriteString(strings.TrimSpace(plan.FirstStep()))
	prompt.WriteString("\n\nRemaining plan:\n")
	prompt.Write(planJSON)
	prompt.WriteString("\n\nCompleted steps and results:\n")
	prompt.WriteString(strings.TrimSpace(formatExecutorExecutedSteps(executedSteps)))
	prompt.WriteString("\n\nReturn the result of the current step so the replanner can decide what to do next.")

	return prompt.String(), nil
}

func formatExecutorUserInput(input []adk.Message) string {
	if len(input) == 0 {
		return "(missing objective)"
	}

	var prompt strings.Builder
	for _, msg := range input {
		if msg.Content == "" {
			continue
		}
		if prompt.Len() > 0 {
			prompt.WriteString("\n")
		}
		prompt.WriteString(msg.Content)
	}
	if prompt.Len() == 0 {
		return "(missing objective)"
	}
	return prompt.String()
}

func formatExecutorExecutedSteps(executedSteps []planexecute.ExecutedStep) string {
	if len(executedSteps) == 0 {
		return "(none)"
	}

	var prompt strings.Builder
	for _, executedStep := range executedSteps {
		if prompt.Len() > 0 {
			prompt.WriteString("\n\n")
		}
		prompt.WriteString("Step: ")
		prompt.WriteString(executedStep.Step)
		prompt.WriteString("\nResult: ")
		prompt.WriteString(executedStep.Result)
	}

	return prompt.String()
}
