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
	handlers = append(handlers, &executorPlanExecuteContextMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
	})

	return deep.New(ctx, &deep.Config{
		Name:           "sage",
		Description:    executorDeepDescription,
		Instruction:    executorDeepInstruction,
		ChatModel:      cfg.Model,
		SubAgents:      cfg.SubAgents,
		Handlers:       handlers,
		Backend:        runtimeCapabilities.Backend,
		Shell:          runtimeCapabilities.Shell,
		StreamingShell: runtimeCapabilities.StreamingShell,
		OutputKey:      planexecute.ExecutedStepSessionKey,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
			},
		},
	})
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

type executorPlanExecuteContextMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
}

func (m *executorPlanExecuteContextMiddleware) BeforeAgent(ctx context.Context, runCtx *adk.ChatModelAgentContext) (context.Context, *adk.ChatModelAgentContext, error) {
	planValue, ok := adk.GetSessionValue(ctx, planexecute.PlanSessionKey)
	if !ok {
		return ctx, runCtx, nil
	}

	plan, ok := planValue.(planexecute.Plan)
	if !ok {
		return ctx, runCtx, fmt.Errorf("planexecute plan session value has unexpected type %T", planValue)
	}

	userInput := getExecutorUserInput(ctx)
	executedSteps := getExecutedSteps(ctx)
	executionContext, err := formatExecutorPlanExecuteContext(userInput, plan, executedSteps)
	if err != nil {
		return ctx, runCtx, err
	}

	nextRunCtx := *runCtx
	if nextRunCtx.Instruction == "" {
		nextRunCtx.Instruction = executionContext
	} else {
		nextRunCtx.Instruction = nextRunCtx.Instruction + "\n\n" + executionContext
	}

	return ctx, &nextRunCtx, nil
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
