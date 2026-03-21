package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestIngestActivityReflowsLayoutWithoutResize(t *testing.T) {
	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	ui, ok := updated.(*model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}

	initialHeight := ui.viewport.Height()

	updated, _ = ui.Update(ingestEventMsg{
		Event: runtime.IngestEvent{
			JobID:  "job-1",
			Root:   `C:\Docs`,
			Status: runtime.IngestStatusRunning,
			Type:   runtime.EventIngestStarted,
		},
	})
	ui, ok = updated.(*model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}

	if got := ui.viewport.Height(); got >= initialHeight {
		t.Fatalf("viewport height = %d, want less than %d after activity dock appears", got, initialHeight)
	}

	view := ui.View().Content
	if !strings.Contains(view, "Activity") {
		t.Fatalf("view does not contain activity dock:\n%s", view)
	}
	if !strings.Contains(view, "enter send · / commands") {
		t.Fatalf("view does not contain help footer after activity dock appears:\n%s", view)
	}
}
