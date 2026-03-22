package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

func (a *Application) Runner(ctx context.Context) (*adk.Runner, error) {
	rootAgent, err := a.RootAgent(ctx)
	if err != nil {
		return nil, err
	}

	return adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           rootAgent,
		EnableStreaming: true,
		CheckPointStore: a.checkpoints,
	}), nil
}
