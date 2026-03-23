package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestAtFileSuggestStillActivates(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "auth_handler.go")
	if err := os.WriteFile(filePath, []byte("package auth"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	ui := newInteractionModel(t)
	ui.fileSuggest = newFileSuggest(tmp)

	updated, _ := ui.Update(tea.KeyPressMsg{Text: "@", Code: '@'})
	ui = expectModel(t, updated)
	updated, _ = ui.Update(tea.KeyPressMsg{Text: "a", Code: 'a'})
	ui = expectModel(t, updated)

	if !ui.fileSuggest.active {
		t.Fatal("fileSuggest.active = false, want true")
	}
	if len(ui.fileSuggest.items) == 0 {
		t.Fatal("fileSuggest.items empty, want matched files")
	}

	view := stripANSITest(ui.viewport.GetContent())
	if !strings.Contains(view, "Files") {
		t.Fatalf("viewport overlay missing Files picker:\n%s", view)
	}
}

func TestIngestDirectorySuggestActivatesAndCompletesPath(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "docs")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("os.Mkdir() error = %v", err)
	}

	ui := newInteractionModel(t)
	ui.dirSuggest = newDirSuggest(tmp)
	ui.area.SetValue("/ingest ")
	ui.updateSlashState()
	ui.refreshViewport()

	if !ui.dirSuggest.active {
		t.Fatal("dirSuggest.active = false, want true for /ingest prompt")
	}

	updated, _ := ui.Update(tea.KeyPressMsg{Text: "d", Code: 'd'})
	ui = expectModel(t, updated)
	updated, _ = ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	ui = expectModel(t, updated)

	if got := ui.area.Value(); !strings.Contains(got, filepath.ToSlash(targetDir)) {
		t.Fatalf("textarea value = %q, want selected directory path", got)
	}
}

func TestCtrlETogglesFocusedToolOnly(t *testing.T) {
	ui := newInteractionModel(t)
	ui.messages = append(ui.messages, Message{Kind: MessageRun, RunID: runtime.RunID("run-tools")})

	ui.transcript.ApplyEvent(runtime.Event{
		RunID:  runtime.RunID("run-tools"),
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"planner", "sage"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{
				{ID: "call-1", Name: "read_file", Arguments: `{"path":"a.go"}`},
				{ID: "call-2", Name: "bash", Arguments: `{"cmd":"go test ./..."}`},
			},
		},
	})

	updated, _ := ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	ui = expectModel(t, updated)
	if ui.focusedToolID != "tool:run-tools:call-1" {
		t.Fatalf("focusedToolID = %q, want first tool", ui.focusedToolID)
	}

	updated, _ = ui.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	ui = expectModel(t, updated)
	if !ui.transcript.nodes["tool:run-tools:call-1"].Expanded {
		t.Fatal("first tool not expanded")
	}
	if ui.transcript.nodes["tool:run-tools:call-2"].Expanded {
		t.Fatal("second tool unexpectedly expanded")
	}

	updated, _ = ui.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	ui = expectModel(t, updated)
	if ui.focusedToolID != "tool:run-tools:call-2" {
		t.Fatalf("focusedToolID = %q, want second tool", ui.focusedToolID)
	}

	updated, _ = ui.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	ui = expectModel(t, updated)
	if !ui.transcript.nodes["tool:run-tools:call-2"].Expanded {
		t.Fatal("second tool not expanded")
	}

	updated, _ = ui.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	ui = expectModel(t, updated)
	if ui.transcript.nodes["tool:run-tools:call-2"].Expanded {
		t.Fatal("second tool still expanded after toggle")
	}
}

func newInteractionModel(t *testing.T) *model {
	t.Helper()

	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return expectModel(t, updated)
}
