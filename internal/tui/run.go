package tui

import (
	"bilge-lib/internal/runtime"
	"context"

	tea "charm.land/bubbletea/v2"
)

func Run(ctx context.Context, manager *runtime.Manager) error {
	program := tea.NewProgram(newModel(manager, ctx), tea.WithContext(ctx))
	_, err := program.Run()
	return err
}
