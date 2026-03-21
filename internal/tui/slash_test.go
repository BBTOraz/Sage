package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestResolveSlashCommand(t *testing.T) {
	cmd, arg, ok := resolveSlashCommand("/ingest C:\\Docs")
	if !ok {
		t.Fatal("resolveSlashCommand() ok = false, want true")
	}
	if cmd.Name != "ingest" {
		t.Fatalf("command = %q, want ingest", cmd.Name)
	}
	if arg != "C:\\Docs" {
		t.Fatalf("arg = %q, want %q", arg, "C:\\Docs")
	}
}

func TestAutocompleteSlashCommand(t *testing.T) {
	got, ok := autocompleteSlashCommand("/in")
	if !ok {
		t.Fatal("autocompleteSlashCommand() ok = false, want true")
	}
	if got != "/ingest " {
		t.Fatalf("autocompleteSlashCommand() = %q, want %q", got, "/ingest ")
	}
}

func TestSlashOverlayVisibleAfterTypingSlash(t *testing.T) {
	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())

	if _, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30}); cmd != nil {
		_ = cmd
	}
	updated, _ := m.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	model, ok := updated.(*model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}
	if !model.slash.Active {
		t.Fatal("slash.Active = false, want true after typing '/'")
	}

	view := model.View().Content
	if !strings.Contains(view, "Commands") {
		t.Fatalf("view does not contain command overlay:\n%s", view)
	}
	if !strings.Contains(view, "enter send · / commands") {
		t.Fatalf("view does not contain help footer while slash overlay is active:\n%s", view)
	}
}
