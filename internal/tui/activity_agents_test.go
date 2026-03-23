package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestActiveAgentsDockShowsOnlyCurrentAgentName(t *testing.T) {
	ui := newActivityTestModel(t)
	ui.activeRunID = runtime.RunID("run-agent")
	ui.messages = append(ui.messages, Message{Kind: MessageRun, RunID: runtime.RunID("run-agent")})

	updated, _ := ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-agent"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   `{"steps":["Собрать контекст","Подготовить ответ"]}`,
			Payload: &runtime.EventPayload{
				AgentName: "sage-doc",
				RunPath:   []string{"planner", "sage", "sage-doc"},
				Role:      "assistant",
			},
		},
	})
	ui = expectModel(t, updated)

	activity := stripANSITest(ui.activityDockView(90, ui.spinner.View()))
	for _, want := range []string{"Activity", "Agents", "sage-doc"} {
		if !strings.Contains(activity, want) {
			t.Fatalf("activity dock missing %q:\n%s", want, activity)
		}
	}
	for _, unwanted := range []string{"planner", "delegating work", `"steps"`} {
		if strings.Contains(activity, unwanted) {
			t.Fatalf("activity dock still contains %q noise:\n%s", unwanted, activity)
		}
	}
}

func TestFinishedAgentsDisappearFromDockButStayInTranscript(t *testing.T) {
	ui := newActivityTestModel(t)
	ui.activeRunID = runtime.RunID("run-finished")
	ui.messages = append(ui.messages, Message{Kind: MessageRun, RunID: runtime.RunID("run-finished")})

	updated, _ := ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-finished"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "Нахожу причину бага.",
			Payload: &runtime.EventPayload{
				AgentName: "sage",
				RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
				Role:      "assistant",
			},
		},
	})
	ui = expectModel(t, updated)

	updated, _ = ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-finished"),
			Status: runtime.RunStatusCompleted,
			Type:   runtime.EventRunCompleted,
		},
	})
	ui = expectModel(t, updated)

	activity := stripANSITest(ui.activityDockView(90, ui.spinner.View()))
	if strings.Contains(activity, "sage-doc") {
		t.Fatalf("finished agent still shown in activity dock:\n%s", activity)
	}

	view := stripANSITest(ui.View().Content)
	if !strings.Contains(view, "A Sage") || !strings.Contains(view, "Нахожу причину бага.") {
		t.Fatalf("finished sage turn disappeared from transcript view:\n%s", view)
	}
}

func TestIngestAndAgentActivityCoexist(t *testing.T) {
	ui := newActivityTestModel(t)
	ui.activeRunID = runtime.RunID("run-mixed")
	ui.messages = append(ui.messages, Message{Kind: MessageRun, RunID: runtime.RunID("run-mixed")})

	updated, _ := ui.Update(runnerEventMsg{
		Event: runtime.Event{
			RunID:  runtime.RunID("run-mixed"),
			Status: runtime.RunStatusRunning,
			Type:   runtime.EventAssistantChunk,
			Text:   "Проверяю документацию.",
			Payload: &runtime.EventPayload{
				AgentName: "sage",
				RunPath:   []string{"planner", "sage"},
				Role:      "assistant",
			},
		},
	})
	ui = expectModel(t, updated)

	updated, _ = ui.Update(ingestEventMsg{
		Event: runtime.IngestEvent{
			JobID:  "job-1",
			Root:   `C:\Docs`,
			Status: runtime.IngestStatusRunning,
			Type:   runtime.EventIngestStarted,
		},
	})
	ui = expectModel(t, updated)

	activity := stripANSITest(ui.activityDockView(90, ui.spinner.View()))
	for _, want := range []string{"Agents", "Jobs", "Sage", "Docs"} {
		if !strings.Contains(activity, want) {
			t.Fatalf("activity dock missing %q:\n%s", want, activity)
		}
	}
}

func newActivityTestModel(t *testing.T) *model {
	t.Helper()

	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return expectModel(t, updated)
}
