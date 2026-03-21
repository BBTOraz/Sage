package agent

import (
	"bilge-lib/internal/agent/middlewares"
	"bilge-lib/internal/approval"
	middleware2 "bilge-lib/internal/middleware"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/prebuilt/deep"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

func NewDeepAgent(ctx context.Context, model model.BaseChatModel, mode approval.Mode, subs ...adk.Agent) (adk.Agent, error) {
	return deep.New(ctx, &deep.Config{
		Name:        "sage",
		Description: deepAgentDescription,
		Instruction: deepAgentInstruction,
		ChatModel:   model,
		SubAgents:   subs,
		Handlers:    middlewares.DefaultMiddlewares(mode),
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
			},
		},
	})

}

func NewDocAgent(ctx context.Context, model model.BaseChatModel, mode approval.Mode, tools ...tool.BaseTool) (adk.Agent, error) {
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "sage-doc",
		Description: docAgentDescription,
		Instruction: docAgentInstruction,
		Model:       model,
		Handlers:    middlewares.DefaultMiddlewares(mode),
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
				Tools:               tools,
			},
		},
	})
}

func (a *Application) DocAgent(ctx context.Context) (adk.Agent, error) {
	return NewDocAgent(ctx, a.model, a.config.ApprovalMode, a.DocTools()...)
}

func (a *Application) DeepAgent(ctx context.Context) (adk.Agent, error) {
	docAgent, err := a.DocAgent(ctx)
	if err != nil {
		return nil, err
	}

	return NewDeepAgent(ctx, a.model, a.config.ApprovalMode, docAgent)
}
