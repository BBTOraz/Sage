package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/planexecute"
	"github.com/cloudwego/eino/components/model"
)

func NewRootPlanExecute(ctx context.Context, plannerModel model.ToolCallingChatModel, executor adk.Agent) (adk.ResumableAgent, error) {
	planner, err := planexecute.NewPlanner(ctx, &planexecute.PlannerConfig{
		ToolCallingChatModel: plannerModel,
	})
	if err != nil {
		return nil, err
	}

	replanner, err := planexecute.NewReplanner(ctx, &planexecute.ReplannerConfig{
		ChatModel: plannerModel,
	})
	if err != nil {
		return nil, err
	}

	return planexecute.New(ctx, &planexecute.Config{
		Planner:   planner,
		Executor:  executor,
		Replanner: replanner,
	})
}

func (a *Application) RootAgent(ctx context.Context) (adk.ResumableAgent, error) {
	plannerModel, err := toolCallingChatModel(a.model)
	if err != nil {
		return nil, err
	}

	executor, err := a.ExecutorDeep(ctx)
	if err != nil {
		return nil, err
	}

	return NewRootPlanExecute(ctx, plannerModel, executor)
}

func toolCallingChatModel(chatModel model.BaseChatModel) (model.ToolCallingChatModel, error) {
	toolCalling, ok := chatModel.(model.ToolCallingChatModel)
	if !ok {
		return nil, fmt.Errorf("agent root requires tool-calling compatible model, got %T", chatModel)
	}

	return toolCalling, nil
}
