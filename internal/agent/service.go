package agent

import (
	"bilge-lib/core"
	"bilge-lib/internal/approval"
	middleware2 "bilge-lib/internal/middleware"
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
)

type AdkService struct {
	runner *adk.Runner
}

func NewAdkService(ctx context.Context, env *core.EnvConfig, mode approval.Mode) (*AdkService, error) {
	model, err := core.NewChatModel(ctx, *env)
	if err != nil {
		return nil, err
	}
	middleware, err := FileSystemMiddleware(ctx)
	if err != nil {
		return nil, err
	}

	approvalMiddleware := &ApprovalMiddleware{
		mode: mode,
	}

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "eino-practice-core",
		Description: "file system agent have access to os files",
		Instruction: "ты агент который работает с файлами,",
		Model:       model,
		Handlers: []adk.ChatModelAgentMiddleware{
			middleware,
			approvalMiddleware,
		},
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				UnknownToolsHandler: middleware2.UnknownToolHandler(),
			},
		},
	})

	if err != nil {
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           agent,
		EnableStreaming: true,
		CheckPointStore: NewInMemoryCheckPointStore(),
	})

	return &AdkService{
		runner: runner,
	}, nil
}

func (s *AdkService) Runner() *adk.Runner {
	return s.runner
}
