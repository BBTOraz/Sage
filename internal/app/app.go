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

	service, err := agent.NewAdkService(ctx, &env, cfg.Mode)
	if err != nil {
		return err
	}
	manager := runtime.NewManager(cfg.Mode, service.Runner())
	return tui.Run(ctx, manager)
}
