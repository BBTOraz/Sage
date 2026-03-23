package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStreamingKeepsUserScrollPositionWhenAutoFollowDisabled(t *testing.T) {
	ui := newScrollableModel(t)
	ui.activeRunID = runtime.RunID("run-scroll")

	updated, _ := ui.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	ui = expectModel(t, updated)

	if ui.followTranscript {
		t.Fatal("followTranscript = true, want false after manual scroll up")
	}
	if ui.viewport.AtBottom() {
		t.Fatal("viewport unexpectedly at bottom after page up")
	}

	offsetBefore := ui.viewport.YOffset()

	updated, _ = ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-scroll"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "stream update",
		},
	})
	ui = expectModel(t, updated)

	if got := ui.viewport.YOffset(); got != offsetBefore {
		t.Fatalf("viewport YOffset = %d, want %d while auto-follow disabled", got, offsetBefore)
	}
	if ui.viewport.AtBottom() {
		t.Fatal("viewport jumped back to bottom while user had scrolled away")
	}
}

func TestStreamingStaysAtBottomWhileAutoFollowEnabled(t *testing.T) {
	ui := newScrollableModel(t)
	ui.activeRunID = runtime.RunID("run-follow")

	if !ui.followTranscript {
		t.Fatal("followTranscript = false, want true by default")
	}
	if !ui.viewport.AtBottom() {
		t.Fatal("viewport should start at bottom")
	}

	updated, _ := ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-follow"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "stream update",
		},
	})
	ui = expectModel(t, updated)

	if !ui.followTranscript {
		t.Fatal("followTranscript = false, want true while user stays at bottom")
	}
	if !ui.viewport.AtBottom() {
		t.Fatal("viewport left bottom while auto-follow was enabled")
	}
}

func TestPageDownAtBottomReEnablesAutoFollow(t *testing.T) {
	ui := newScrollableModel(t)
	ui.activeRunID = runtime.RunID("run-bottom")

	updated, _ := ui.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	ui = expectModel(t, updated)

	if ui.followTranscript {
		t.Fatal("followTranscript = true, want false after page up")
	}

	for i := 0; i < 8 && !ui.viewport.AtBottom(); i++ {
		updated, _ = ui.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
		ui = expectModel(t, updated)
	}

	if !ui.viewport.AtBottom() {
		t.Fatal("viewport did not return to bottom after page down")
	}
	if !ui.followTranscript {
		t.Fatal("followTranscript = false, want true after returning to bottom")
	}

	updated, _ = ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-bottom"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "stream update",
		},
	})
	ui = expectModel(t, updated)

	if !ui.viewport.AtBottom() {
		t.Fatal("viewport left bottom after auto-follow was re-enabled")
	}
}

func newScrollableModel(t *testing.T) *model {
	t.Helper()

	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 12})
	ui := expectModel(t, updated)

	for i := 0; i < 24; i++ {
		ui.messages = append(ui.messages, Message{
			Kind: MessageAgent,
			Text: fmt.Sprintf("message %02d with enough text to keep the transcript scrollable", i),
		})
	}
	ui.refreshViewport()

	if ui.viewport.TotalLineCount() <= ui.viewport.Height() {
		t.Fatalf("viewport line count = %d, want greater than height %d", ui.viewport.TotalLineCount(), ui.viewport.Height())
	}

	return ui
}

func expectModel(t *testing.T, updated tea.Model) *model {
	t.Helper()

	ui, ok := updated.(*model)
	if !ok {
		t.Fatalf("updated model type = %T, want *model", updated)
	}
	return ui
}
