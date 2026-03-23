package tui

import (
	"bilge-lib/internal/runtime"
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPlanStoreTracksPlannerAndReplannerSteps(t *testing.T) {
	store := newPlanStore()
	runID := runtime.RunID("run-plan")

	store.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "planner",
			RunPath:   []string{"plan_execute_replan", "planner"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "plan-1",
				Name:      "plan",
				Arguments: `{"steps":["Собрать контекст","Подготовить ответ"]}`,
			}},
		},
	})

	plan := store.planFor(runID)
	if plan == nil {
		t.Fatal("plan = nil, want planner steps to initialize pinned plan")
	}
	if len(plan.Items) != 2 {
		t.Fatalf("len(plan.Items) = %d, want 2", len(plan.Items))
	}
	if plan.Items[0].Text != "Собрать контекст" || plan.Items[0].Status != planItemActive {
		t.Fatalf("first item = %#v, want active first step", plan.Items[0])
	}
	if plan.Items[1].Status != planItemQueued {
		t.Fatalf("second item = %#v, want queued second step", plan.Items[1])
	}

	store.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "replanner",
			RunPath:   []string{"plan_execute_replan", "replanner"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "plan-2",
				Name:      "plan",
				Arguments: `{"steps":["Подготовить ответ"]}`,
			}},
		},
	})

	plan = store.planFor(runID)
	if len(plan.Items) != 2 {
		t.Fatalf("len(plan.Items) after replan = %d, want 2", len(plan.Items))
	}
	if plan.Items[0].Status != planItemDone {
		t.Fatalf("first item after replan = %#v, want done", plan.Items[0])
	}
	if plan.Items[1].Status != planItemActive {
		t.Fatalf("second item after replan = %#v, want active", plan.Items[1])
	}
}

func TestPlanStoreUsesWriteTodosPayloadWhenAvailable(t *testing.T) {
	store := newPlanStore()
	runID := runtime.RunID("run-todos")

	store.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "todos-1",
				Name:      "write_todos",
				Arguments: `{"todos":[{"content":"Собрать контекст","status":"completed"},{"content":"Подготовить ответ","status":"in_progress"},{"content":"Отправить результат","status":"pending"}]}`,
			}},
		},
	})

	plan := store.planFor(runID)
	if plan == nil {
		t.Fatal("plan = nil, want write_todos to initialize pinned plan")
	}
	if len(plan.Items) != 3 {
		t.Fatalf("len(plan.Items) = %d, want 3", len(plan.Items))
	}
	if plan.Items[0].Status != planItemDone {
		t.Fatalf("first todo = %#v, want done", plan.Items[0])
	}
	if plan.Items[1].Status != planItemActive {
		t.Fatalf("second todo = %#v, want active", plan.Items[1])
	}
	if plan.Items[2].Status != planItemQueued {
		t.Fatalf("third todo = %#v, want queued", plan.Items[2])
	}
}

func TestPlanStoreUsesWriteTodosToolResultContent(t *testing.T) {
	store := newPlanStore()
	runID := runtime.RunID("run-todos-result")

	store.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "tool",
			ToolResult: &runtime.ToolResultPayload{
				ToolCallID: "todos-1",
				ToolName:   "write_todos",
				Content:    `Updated todo list to [{"content":"Собрать контекст","status":"completed"},{"content":"Подготовить ответ","status":"in_progress"},{"content":"Отправить результат","status":"pending"}]`,
			},
		},
	})

	plan := store.planFor(runID)
	if plan == nil {
		t.Fatal("plan = nil, want write_todos tool result to initialize pinned plan")
	}
	if len(plan.Items) != 3 {
		t.Fatalf("len(plan.Items) = %d, want 3", len(plan.Items))
	}
	if plan.Items[0].Status != planItemDone {
		t.Fatalf("first todo = %#v, want done", plan.Items[0])
	}
	if plan.Items[1].Status != planItemActive {
		t.Fatalf("second todo = %#v, want active", plan.Items[1])
	}
	if plan.Items[2].Status != planItemQueued {
		t.Fatalf("third todo = %#v, want queued", plan.Items[2])
	}
}

func TestViewPinsPlanPanelAboveTranscript(t *testing.T) {
	manager := runtime.NewManager("", nil, nil)
	m := newModel(manager, context.Background())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	ui := expectModel(t, updated)

	runID := runtime.RunID("run-view-plan")
	ui.messages = append(ui.messages,
		Message{Kind: MessageUser, Text: "Привет"},
		Message{Kind: MessageRun, RunID: runID},
	)
	ui.planStore.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Payload: &runtime.EventPayload{
			AgentName: "planner",
			RunPath:   []string{"plan_execute_replan", "planner"},
			Role:      "assistant",
			ToolCalls: []runtime.ToolCallPayload{{
				ID:        "plan-1",
				Name:      "plan",
				Arguments: `{"steps":["Понять запрос","Подготовить ответ"]}`,
			}},
		},
	})
	ui.transcript.ApplyEvent(runtime.Event{
		RunID:  runID,
		Status: runtime.RunStatusRunning,
		Type:   runtime.EventAssistantChunk,
		Text:   "Привет! Чем могу помочь?",
		Payload: &runtime.EventPayload{
			AgentName: "sage",
			RunPath:   []string{"plan_execute_replan", "execute_replan", "sage"},
			Role:      "assistant",
		},
	})
	ui.refreshViewport()

	view := stripANSITest(ui.View().Content)
	if !strings.Contains(view, "Plan") {
		t.Fatalf("view missing pinned plan panel:\n%s", view)
	}
	if !strings.Contains(view, "[●] Понять запрос") {
		t.Fatalf("view missing active plan item:\n%s", view)
	}
	if strings.Index(view, "Plan") > strings.Index(view, "> Привет") {
		t.Fatalf("plan panel rendered below transcript instead of above it:\n%s", view)
	}
}
