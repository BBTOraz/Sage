package app

import (
	"bilge-lib/internal/agent"
	"bilge-lib/internal/runtime"
	"bilge-lib/internal/tui"
	"context"
)

func Run(ctx context.Context, cfg *Config) error {
	env, err := LoadEnvConfig()
	if err != nil {
		return err
	}

	application, err := agent.NewApplication(ctx, agent.ApplicationConfig{
		Env:          env,
		ApprovalMode: cfg.Mode,
	})
	if err != nil {
		return err
	}
	runner, err := application.Runner(ctx)
	if err != nil {
		return err
	}

	manager := runtime.NewManager(cfg.Mode, runner, application)
	return tui.Run(ctx, manager)
}
