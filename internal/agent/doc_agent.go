package agent

import (
	"bilge-lib/internal/approval"
	middleware2 "bilge-lib/internal/middleware"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

func NewDocAgent(ctx context.Context, model model.BaseChatModel, mode approval.Mode, tools ...tool.BaseTool) (adk.Agent, error) {
	return NewDocAgentWithConfig(ctx, AgentHandlerConfig{
		ApprovalMode: mode,
	}, model, tools...)
}

func NewDocAgentWithConfig(ctx context.Context, handlerConfig AgentHandlerConfig, model model.BaseChatModel, tools ...tool.BaseTool) (adk.Agent, error) {
	handlers, err := BuildDocAgentHandlers(ctx, handlerConfig)
	if err != nil {
		return nil, err
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "sage-doc",
		Description: docAgentDescription,
		Instruction: docAgentInstruction,
		Model:       model,
		Handlers:    handlers,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
				ToolArgumentsHandler: toolArgumentsRepairHandler(),
				Tools:               tools,
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return wrapUnknownToolRecoveryAgent(agent), nil
}

func (a *Application) DocAgent(ctx context.Context) (adk.Agent, error) {
	return NewDocAgentWithConfig(ctx, AgentHandlerConfig{
		ApprovalMode: a.config.ApprovalMode,
		Capabilities: a.executorDeepCapabilities,
	}, a.model, a.DocTools()...)
}
